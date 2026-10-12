/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/pb-go/v2/consensus/maxbft"
	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/pb-go/v2/consensus"

	"chainmaker.org/chainmaker/common/v2/msgbus"

	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/common/v2/json"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
)

type certACProvider struct {
	acService *accessControlService

	// local cache for certificates (reduce the size of block)
	certCache *ShardCache

	// local cache for certificate revocation list and frozen list
	crl        sync.Map
	frozenList sync.Map

	// verification options for organization members
	opts bcx509.VerifyOptions

	localOrg *organization

	//third-party trusted members
	trustMembers *sync.Map

	// used to cache the deduction account address to avoid reading the database every time
	payerList *ShardCache

	store protocol.BlockchainStore

	//consensus type
	consensusType consensus.ConsensusType

	taskCron *cron.Cron
	log      protocol.Logger
}

type trustMemberCached struct {
	trustMember *config.TrustMemberConfig
	cert        *bcx509.Certificate
}

var _ protocol.AccessControlProvider = (*certACProvider)(nil)

var nilCertACProvider ACProvider = (*certACProvider)(nil)

// NewACProvider 构造一个AccessControlProvider
// @param chainConf
// @param localOrgId
// @param store
// @param log
// @param msgBus
// @return protocol.AccessControlProvider
// @return error
func (cp *certACProvider) NewACProvider(chainConf protocol.ChainConf, localOrgId string,
	store protocol.BlockchainStore, log protocol.Logger, msgBus msgbus.MessageBus) (
	protocol.AccessControlProvider, error) {
	certACProvider, err := newCertACProvider(chainConf.ChainConfig(), localOrgId, store, log)
	if err != nil {
		return nil, err
	}

	msgBus.Register(msgbus.ChainConfig, certACProvider)
	msgBus.Register(msgbus.CertManageCertsDelete, certACProvider)
	msgBus.Register(msgbus.CertManageCertsUnfreeze, certACProvider)
	msgBus.Register(msgbus.CertManageCertsFreeze, certACProvider)
	msgBus.Register(msgbus.CertManageCertsRevoke, certACProvider)
	msgBus.Register(msgbus.CertManageCertsAliasDelete, certACProvider)
	msgBus.Register(msgbus.CertManageCertsAliasUpdate, certACProvider)
	msgBus.Register(msgbus.MaxbftEpochConf, certACProvider)
	msgBus.Register(msgbus.BlockInfo, certACProvider)

	//v220_compat Deprecated
	chainConf.AddWatch(certACProvider)   //nolint: staticcheck
	chainConf.AddVmWatch(certACProvider) //nolint: staticcheck
	certACProvider.log = log
	return certACProvider, nil
}

// Start the ac service.
func (cp *certACProvider) Start() error {
	cp.taskCron = cron.New(
		cron.WithSeconds(),          // 启用秒级调度
		cron.WithLocation(time.UTC)) // 设置时区为 UTC
	// 添加任务，每小时执行一次
	_, err := cp.taskCron.AddFunc("0 0 * * * *", cp.runTask)
	if err != nil {
		cp.log.Errorf("add task cron failed:", err)
		return err
	}
	cp.log.Infof("scheduled task added successfully")

	cp.taskCron.Start()
	return nil
}

// Stop the ac service.
func (cp *certACProvider) Stop() error {
	cp.log.Infof("certACProvider stop")
	if cp.taskCron != nil {
		cp.taskCron.Stop()
		return nil
	}
	return fmt.Errorf("task cron is nil")
}

func (cp *certACProvider) runTask() {
	cp.log.Debugf("run task in ac, cache size:%d", cp.certCache.Long())
	// 遍历并打印所有键值对
	cp.certCache.Iterate(func(key string, value interface{}) {
		cp.log.Debugf("Key: %s", key)
		var (
			cert *bcx509.Certificate
			err  error
		)
		certBytes, ok := value.([]byte)
		if !ok {
			certBytes = []byte(value.(string))
		}
		certBlock, rest := pem.Decode(certBytes)
		if certBlock == nil {
			cert, err = bcx509.ParseCertificate(rest)
			if err != nil {
				cp.log.Warnf("new cert member failed, invalid certificate")
				return
			}
		} else {
			cert, err = bcx509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				cp.log.Warnf("new cert member failed, invalid certificate")
				return
			}
		}
		expired := isCertExpired(*cert)
		if expired {
			cp.log.Infof("the certificate has expired[%v]", key)
			cp.certCache.Remove(key)
			cp.log.Infof("remove [%v] in cache", key)
		} else {
			cp.log.Infof("the certificate has not expired[%v]", key)
		}
	})
}

func isCertExpired(cert bcx509.Certificate) bool {
	now := time.Now()
	return now.After(cert.NotAfter)
}

func newCertACProvider(chainConfig *config.ChainConfig, localOrgId string,
	store protocol.BlockchainStore, log protocol.Logger) (*certACProvider, error) {
	certACProvider := &certACProvider{
		certCache:  NewShardCache(GetCertCacheSize()),
		crl:        sync.Map{},
		frozenList: sync.Map{},
		payerList:  NewShardCache(GetCertCacheSize()),
		opts: bcx509.VerifyOptions{
			Intermediates: bcx509.NewCertPool(),
			Roots:         bcx509.NewCertPool(),
		},
		localOrg:     nil,
		trustMembers: &sync.Map{},
		store:        store,
	}

	var maxbftCfg *maxbft.GovernanceContract
	var err error
	certACProvider.consensusType = chainConfig.Consensus.Type
	if certACProvider.consensusType == consensus.ConsensusType_MAXBFT {
		maxbftCfg, err = certACProvider.loadChainConfigFromGovernance()
		if err != nil {
			return nil, err
		}
		//omit 1'st epoch, GovernanceContract don't save chainConfig in 1'st epoch
		if maxbftCfg != nil && maxbftCfg.ChainConfig != nil {
			chainConfig = maxbftCfg.ChainConfig
		}
	}
	log.DebugDynamic(func() string {
		return fmt.Sprintf("init ac from chainconfig: %+v", chainConfig)
	})

	err = certACProvider.initTrustMembers(chainConfig.TrustMembers)
	if err != nil {
		return nil, err
	}

	certACProvider.acService = initAccessControlService(chainConfig.GetCrypto().Hash,
		chainConfig.AuthType, chainConfig.Vm.AddrType, store, log)
	certACProvider.acService.setVerifyOptionsFunc(certACProvider.getVerifyOptions)

	err = certACProvider.initTrustRoots(chainConfig.TrustRoots, localOrgId)
	if err != nil {
		return nil, err
	}

	certACProvider.acService.initResourcePolicy(chainConfig.ResourcePolicies, localOrgId)

	certACProvider.opts.KeyUsages = make([]x509.ExtKeyUsage, 1)
	certACProvider.opts.KeyUsages[0] = x509.ExtKeyUsageAny

	if certACProvider.consensusType == consensus.ConsensusType_MAXBFT && maxbftCfg != nil {
		err = certACProvider.updateFrozenAndCRL(maxbftCfg)
		if err != nil {
			return nil, err
		}
	} else {
		if err := certACProvider.loadCRL(); err != nil {
			return nil, err
		}
		if err := certACProvider.loadCertFrozenList(); err != nil {
			return nil, err
		}
	}
	return certACProvider, nil
}

func (cp *certACProvider) getVerifyOptions() *bcx509.VerifyOptions {
	return &cp.opts
}

func (cp *certACProvider) initTrustRoots(roots []*config.TrustRootConfig, localOrgId string) error {

	for _, orgRoot := range roots {
		org := &organization{
			id:                       orgRoot.OrgId,
			trustedRootCerts:         map[string]*bcx509.Certificate{},
			trustedIntermediateCerts: map[string]*bcx509.Certificate{},
		}
		for _, root := range orgRoot.Root {
			certificateChain, err := cp.buildCertificateChain(root, orgRoot.OrgId, org)
			if err != nil {
				return err
			}
			if certificateChain == nil || !certificateChain[len(certificateChain)-1].IsCA {
				return fmt.Errorf("the certificate configured as root for organization %s is not a CA certificate", orgRoot.OrgId)
			}
			org.trustedRootCerts[string(certificateChain[len(certificateChain)-1].Raw)] =
				certificateChain[len(certificateChain)-1]
			cp.opts.Roots.AddCert(certificateChain[len(certificateChain)-1])
			for i := 0; i < len(certificateChain); i++ {
				org.trustedIntermediateCerts[string(certificateChain[i].Raw)] = certificateChain[i]
				cp.opts.Intermediates.AddCert(certificateChain[i])
			}

			/*for _, certificate := range certificateChain {
				if certificate.IsCA {
					org.trustedRootCerts[string(certificate.Raw)] = certificate
					ac.opts.Roots.AddCert(certificate)
				} else {
					org.trustedIntermediateCerts[string(certificate.Raw)] = certificate
					ac.opts.Intermediates.AddCert(certificate)
				}
			}*/

			if len(org.trustedRootCerts) <= 0 {
				return fmt.Errorf(
					"setup organization failed, no trusted root (for %s): "+
						"please configure trusted root certificate or trusted public key whitelist",
					orgRoot.OrgId,
				)
			}
		}
		cp.acService.addOrg(orgRoot.OrgId, org)
	}

	localOrg := cp.acService.getOrgInfoByOrgId(localOrgId)
	if localOrg == nil {
		localOrg = &organization{
			id:                       localOrgId,
			trustedRootCerts:         map[string]*bcx509.Certificate{},
			trustedIntermediateCerts: map[string]*bcx509.Certificate{},
		}
	}
	cp.localOrg, _ = localOrg.(*organization)
	return nil
}

func (cp *certACProvider) buildCertificateChain(root, orgId string, org *organization) ([]*bcx509.Certificate, error) {

	var certificates, certificateChain []*bcx509.Certificate
	pemBlock, rest := pem.Decode([]byte(root))
	for pemBlock != nil {
		cert, errCert := bcx509.ParseCertificate(pemBlock.Bytes)
		if errCert != nil || cert == nil {
			return nil, fmt.Errorf("invalid entry int trusted root cert list")
		}
		if len(cert.Signature) == 0 {
			return nil, fmt.Errorf("invalid certificate [SN: %s]", cert.SerialNumber)
		}
		certificates = append(certificates, cert)
		pemBlock, rest = pem.Decode(rest)
	}
	certificateChain = bcx509.BuildCertificateChain(certificates)
	return certificateChain, nil
}

func (cp *certACProvider) initTrustMembers(trustMembers []*config.TrustMemberConfig) error {
	var syncMap sync.Map
	for _, member := range trustMembers {
		certBlock, _ := pem.Decode([]byte(member.MemberInfo))
		if certBlock == nil {
			return fmt.Errorf("init trust members failed, none certificate given, memberInfo:[%s]",
				member.MemberInfo)
		}
		trustMemberCert, err := bcx509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return fmt.Errorf("init trust members failed, parse certificate failed, memberInfo:[%s]",
				member.MemberInfo)
		}
		cached := &trustMemberCached{
			trustMember: member,
			cert:        trustMemberCert,
		}
		syncMap.Store(member.MemberInfo, cached)
	}
	cp.trustMembers = &syncMap

	return nil
}

func (cp *certACProvider) loadTrustMembers(memberInfo string) (*trustMemberCached, bool) {
	cached, ok := cp.trustMembers.Load(string(memberInfo))
	if ok {
		return cached.(*trustMemberCached), ok
	}
	return nil, ok
}

func (cp *certACProvider) loadCRL() error {
	if cp.acService.dataStore == nil {
		return nil
	}

	crlAKIList, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(),
		[]byte(protocol.CertRevokeKey))
	if err != nil {
		return fmt.Errorf("fail to update CRL list: %v", err)
	}
	if crlAKIList == nil {
		cp.acService.log.Debugf("empty CRL")
		return nil
	}

	var crlAKIs []string
	err = json.Unmarshal(crlAKIList, &crlAKIs)
	if err != nil {
		return fmt.Errorf("fail to update CRL list: %v", err)
	}

	err = cp.storeEffectiveCrls(crlAKIs)
	return err
}

// storeEffectiveCrls 重启时，加载crl，跳过校验失败的子项（比如根证书已经从trustroot中删除）
func (cp *certACProvider) storeEffectiveCrls(crlAKIs []string) error {
	for _, crlAKI := range crlAKIs {
		crlbytes, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), []byte(crlAKI))
		if err != nil {
			return fmt.Errorf("fail to load CRL [%s]: %v", hex.EncodeToString([]byte(crlAKI)), err)
		}
		if crlbytes == nil {
			return fmt.Errorf("fail to load CRL [%s]: CRL is nil", hex.EncodeToString([]byte(crlAKI)))
		}
		crls, err := cp.ValidateCRL(crlbytes)
		if err != nil {
			continue
		}
		if crls == nil {
			continue
		}

		for _, crl := range crls {
			aki, _, err := bcx509.GetAKIFromExtensions(crl.TBSCertList.Extensions)
			if err != nil {
				continue
			}
			cp.crl.Store(string(aki), crl)
		}
	}
	return nil
}

func (cp *certACProvider) storeCrls(crlAKIs []string) error {
	for _, crlAKI := range crlAKIs {
		crlbytes, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), []byte(crlAKI))
		if err != nil {
			return fmt.Errorf("fail to load CRL [%s]: %v", hex.EncodeToString([]byte(crlAKI)), err)
		}
		if crlbytes == nil {
			return fmt.Errorf("fail to load CRL [%s]: CRL is nil", hex.EncodeToString([]byte(crlAKI)))
		}
		crls, err := cp.ValidateCRL(crlbytes)
		if err != nil {
			return err
		}
		if crls == nil {
			return fmt.Errorf("empty CRL")
		}

		for _, crl := range crls {
			aki, _, err := bcx509.GetAKIFromExtensions(crl.TBSCertList.Extensions)
			if err != nil {
				return fmt.Errorf("fail to load CRL, fail to get AKI from CRL: %v", err)
			}
			cp.crl.Store(string(aki), crl)
		}
	}
	return nil
}

// ValidateCRL validates whether the CRL is issued by a trusted CA
func (cp *certACProvider) ValidateCRL(crlBytes []byte) ([]*pkix.CertificateList, error) {
	crlPEM, rest := pem.Decode(crlBytes)
	if crlPEM == nil {
		return nil, fmt.Errorf("empty CRL")
	}
	var crls []*pkix.CertificateList
	orgInfos := cp.acService.getAllOrgInfos()
	for crlPEM != nil {
		crl, err := x509.ParseCRL(crlPEM.Bytes)
		if err != nil {
			return nil, fmt.Errorf("invalid CRL: %v\n[%s]", err, hex.EncodeToString(crlPEM.Bytes))
		}

		err = cp.validateCrlVersion(crlPEM.Bytes, crl)
		if err != nil {
			return nil, err
		}
		orgs := make([]*organization, 0)
		for _, org := range orgInfos {
			orgs = append(orgs, org.(*organization))
		}
		err1 := cp.checkCRLAgainstTrustedCerts(crl, orgs, false)
		err2 := cp.checkCRLAgainstTrustedCerts(crl, orgs, true)
		if err1 != nil && err2 != nil {
			return nil, fmt.Errorf("invalid CRL: \n\t[verification against trusted root certs: %v], \n\t["+
				"verification against trusted intermediate certs: %v]", err1, err2)
		}

		crls = append(crls, crl)
		crlPEM, rest = pem.Decode(rest)
	}
	return crls, nil
}

func (cp *certACProvider) validateCrlVersion(crlPemBytes []byte, crl *pkix.CertificateList) error {
	if cp.acService.dataStore != nil {
		aki, isASN1Encoded, err := bcx509.GetAKIFromExtensions(crl.TBSCertList.Extensions)
		if err != nil {
			return fmt.Errorf("invalid CRL: %v\n[%s]", err, hex.EncodeToString(crlPemBytes))
		}
		cp.acService.log.Debugf("AKI is ASN1 encoded: %v", isASN1Encoded)
		crlOldBytes, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), aki)
		if err != nil {
			return fmt.Errorf("lookup CRL [%s] failed: %v", hex.EncodeToString(aki), err)
		}
		if crlOldBytes != nil {
			crlOldBlock, _ := pem.Decode(crlOldBytes)
			crlOld, err := x509.ParseCRL(crlOldBlock.Bytes)
			if err != nil {
				return fmt.Errorf("parse old CRL failed: %v", err)
			}
			if crlOld.TBSCertList.Version > crl.TBSCertList.Version {
				return fmt.Errorf("validate CRL failed: version of new CRL should be greater than the old one")
			}
		}
	}
	return nil
}

// check CRL against trusted certs
func (cp *certACProvider) checkCRLAgainstTrustedCerts(crl *pkix.CertificateList,
	orgList []*organization, isIntermediate bool) error {
	aki, isASN1Encoded, err := bcx509.GetAKIFromExtensions(crl.TBSCertList.Extensions)
	if err != nil {
		return fmt.Errorf("fail to get AKI of CRL [%s]: %v", crl.TBSCertList.Issuer.String(), err)
	}
	cp.acService.log.Debugf("AKI is ASN1 encoded: %v", isASN1Encoded)
	for _, org := range orgList {
		var targetCerts map[string]*bcx509.Certificate
		if !isIntermediate {
			targetCerts = org.trustedRootCerts
		} else {
			targetCerts = org.trustedIntermediateCerts
		}
		for _, cert := range targetCerts {
			if bytes.Equal(aki, cert.SubjectKeyId) {
				if err := cert.CheckCRLSignature(crl); err != nil {
					return fmt.Errorf("CRL [AKI: %s] is not signed by CA it claims: %v", string(aki), err)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("CRL [AKI: %s] is not signed by ac trusted CA", hex.EncodeToString(aki))
}

func (cp *certACProvider) checkCRL(certChain []*bcx509.Certificate) error {
	if len(certChain) < 1 {
		return fmt.Errorf("given certificate chain is empty")
	}

	for _, cert := range certChain {
		akiCert := cert.AuthorityKeyId

		crl, ok := cp.crl.Load(string(akiCert))
		if ok {
			// we have ac CRL, check whether the serial number is revoked
			for _, rc := range crl.(*pkix.CertificateList).TBSCertList.RevokedCertificates {
				if rc.SerialNumber.Cmp(cert.SerialNumber) == 0 {
					return errors.New("certificate is revoked")
				}
			}
		}
	}

	return nil
}

func (cp *certACProvider) loadCertFrozenList() error {
	if cp.acService.dataStore == nil {
		return nil
	}

	certList, err := cp.acService.dataStore.
		ReadObject(syscontract.SystemContract_CERT_MANAGE.String(),
			[]byte(protocol.CertFreezeKey))
	if err != nil {
		return fmt.Errorf("update frozen certificate list failed: %v", err)
	}
	if certList == nil {
		return nil
	}

	var certIDs []string
	err = json.Unmarshal(certList, &certIDs)
	if err != nil {
		return fmt.Errorf("update frozen certificate list failed: %v", err)
	}

	for _, certID := range certIDs {
		certBytes, err := cp.acService.dataStore.
			ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), []byte(certID))
		if err != nil {
			return fmt.Errorf("load frozen certificate failed: %s", certID)
		}
		if certBytes == nil {
			return fmt.Errorf("load frozen certificate failed: empty certificate [%s]", certID)
		}

		certBlock, _ := pem.Decode(certBytes)
		cp.frozenList.Store(string(certBlock.Bytes), true)
	}
	return nil
}

func (cp *certACProvider) checkCertFrozenList(certChain []*bcx509.Certificate) error {
	if len(certChain) < 1 {
		return fmt.Errorf("given certificate chain is empty")
	}
	_, ok := cp.frozenList.Load(string(certChain[0].Raw))
	if ok {
		return fmt.Errorf("certificate is frozen")
	}
	return nil
}

// GetHashAlg return hash algorithm the access control provider uses
func (cp *certACProvider) GetHashAlg() string {
	return cp.acService.hashType
}

// NewMember 基于参数Member构建Member接口的实例
// @param pbMember
// @return protocol.Member
// @return error
func (cp *certACProvider) NewMember(pbMember *pbac.Member) (protocol.Member, error) {

	var memberTmp *pbac.Member
	if pbMember.MemberType != pbac.MemberType_CERT &&
		pbMember.MemberType != pbac.MemberType_ALIAS &&
		pbMember.MemberType != pbac.MemberType_CERT_HASH {
		return nil, fmt.Errorf("new member failed: the member type does not match")
	}

	if pbMember.MemberType == pbac.MemberType_CERT_HASH || pbMember.MemberType == pbac.MemberType_ALIAS {
		memInfoBytes, ok := cp.lookUpCertCache(pbMember.MemberInfo)
		if !ok {
			return nil, fmt.Errorf("new member failed, the provided certificate ID is not registered")
		}
		memberTmp = &pbac.Member{
			OrgId:      pbMember.OrgId,
			MemberType: pbMember.MemberType,
			MemberInfo: memInfoBytes,
		}
	} else {
		memberTmp = pbMember
	}

	memberCache, ok := cp.acService.lookUpMemberInCache(string(memberTmp.MemberInfo))
	if !ok {
		remoteMember, isTrustMember, err := cp.newNoCacheMember(memberTmp)
		if err != nil {
			return nil, fmt.Errorf("new member failed: %s", err.Error())
		}

		var certChain []*bcx509.Certificate
		if !isTrustMember {
			certChain, err = cp.verifyMember(remoteMember)
			if err != nil {
				return nil, fmt.Errorf("new member failed: %s", err.Error())
			}
		}

		cp.acService.memberCache.Add(string(memberTmp.MemberInfo), &memberCached{
			member:    remoteMember,
			certChain: certChain,
		})
		return remoteMember, nil
	}
	return memberCache.member, nil
}

func (cp *certACProvider) newNoCacheMember(pbMember *pbac.Member) (member protocol.Member,
	isTrustMember bool, err error) {
	cached, ok := cp.loadTrustMembers(string(pbMember.MemberInfo))
	if ok {
		var isCompressed bool
		if pbMember.MemberType == pbac.MemberType_CERT {
			isCompressed = false
		}
		var certMember *certificateMember
		certMember, err = newCertMemberFromParam(cached.trustMember.OrgId, cached.trustMember.Role,
			cp.acService.hashType, isCompressed, []byte(cached.trustMember.MemberInfo))
		if err != nil {
			return nil, isTrustMember, err
		}
		isTrustMember = true
		return certMember, isTrustMember, nil
	}

	member, err = cp.acService.newCertMember(pbMember)
	if err != nil {
		return nil, isTrustMember, fmt.Errorf("new member failed: %s", err.Error())
	}
	return member, isTrustMember, nil
}

// ValidateResourcePolicy checks whether the given resource principal is valid
func (cp *certACProvider) ValidateResourcePolicy(resourcePolicy *config.ResourcePolicy) bool {
	return cp.acService.validateResourcePolicy(resourcePolicy)
}

func (cp *certACProvider) GetMemberStatus(pbMember *pbac.Member) (pbac.MemberStatus, error) {

	member, err := cp.NewMember(pbMember)
	if err != nil {
		cp.acService.log.Infof("get member status: %s", err.Error())
		return pbac.MemberStatus_INVALID, err
	}

	var certChain []*bcx509.Certificate
	cert := member.(*certificateMember).cert

	certChain = append(certChain, cert)
	err = cp.checkCRL(certChain)
	if err != nil && err.Error() == "certificate is revoked" {
		return pbac.MemberStatus_REVOKED, err
	}
	err = cp.checkCertFrozenList(certChain)
	if err != nil && err.Error() == "certificate is frozen" {
		return pbac.MemberStatus_FROZEN, err
	}
	return pbac.MemberStatus_NORMAL, nil
}

func (cp *certACProvider) VerifyRelatedMaterial(verifyType pbac.VerifyType, data []byte) (bool, error) {
	if verifyType != pbac.VerifyType_CRL {
		return false, fmt.Errorf("verify related material failed: only CRL allowed in permissionedWithCert mode")
	}

	crlPEM, rest := pem.Decode(data)
	if crlPEM == nil {
		cp.acService.log.Debug("verify member's related material failed: empty CRL")
		return false, fmt.Errorf("empty CRL")
	}
	orgInfos := cp.acService.getAllOrgInfos()

	var err1, err2 error

	for crlPEM != nil {
		crl, err := x509.ParseCRL(crlPEM.Bytes)
		if err != nil {
			return false, fmt.Errorf("invalid CRL: %v\n[%s]", err, hex.EncodeToString(crlPEM.Bytes))
		}

		err = cp.validateCrlVersion(crlPEM.Bytes, crl)
		if err != nil {
			return false, err
		}
		orgs := make([]*organization, 0)
		for _, org := range orgInfos {
			orgs = append(orgs, org.(*organization))
		}
		err1 = cp.checkCRLAgainstTrustedCerts(crl, orgs, false)
		err2 = cp.checkCRLAgainstTrustedCerts(crl, orgs, true)
		if err1 != nil && err2 != nil {
			return false, fmt.Errorf(
				"invalid CRL: \n\t[verification against trusted root certs: %v], "+
					"\n\t[verification against trusted intermediate certs: %v]",
				err1,
				err2,
			)
		}
		crlPEM, rest = pem.Decode(rest)
	}
	return true, nil
}

// all-in-one validation for signing members: certificate chain/whitelist, signature, policies
func (cp *certACProvider) refinePrincipal(principal protocol.Principal) (protocol.Principal, error) {
	endorsements := principal.GetEndorsement()
	msg := principal.GetMessage()
	refinedEndorsement := cp.RefineEndorsements(endorsements, msg)
	if len(refinedEndorsement) <= 0 {
		return nil, fmt.Errorf("refine endorsements failed, all endorsers have failed verification")
	}

	refinedPrincipal, err := cp.CreatePrincipal(principal.GetResourceName(), refinedEndorsement, msg)
	if err != nil {
		return nil, fmt.Errorf("create principal failed: [%s]", err.Error())
	}

	return refinedPrincipal, nil
}

func (cp *certACProvider) refinePrincipalForCertOptimization(principal protocol.Principal) (protocol.Principal, error) {
	endorsements := principal.GetEndorsement()
	msg := principal.GetMessage()
	refinedEndorsement := cp.RefineEndorsementsForCertOptimization(endorsements, msg)
	if len(refinedEndorsement) <= 0 {
		return nil, fmt.Errorf("refine endorsements failed, all endorsers have failed verification")
	}

	refinedPrincipal, err := cp.CreatePrincipal(principal.GetResourceName(), refinedEndorsement, msg)
	if err != nil {
		return nil, fmt.Errorf("create principal failed: [%s]", err.Error())
	}

	return refinedPrincipal, nil
}

func (cp *certACProvider) RefineEndorsements(endorsements []*common.EndorsementEntry,
	msg []byte) []*common.EndorsementEntry {

	refinedSigners := map[string]bool{}
	var refinedEndorsement []*common.EndorsementEntry
	var memInfo string

	for _, endorsementEntry := range endorsements {
		endorsement := &common.EndorsementEntry{
			Signer: &pbac.Member{
				OrgId:      endorsementEntry.Signer.OrgId,
				MemberInfo: endorsementEntry.Signer.MemberInfo,
				MemberType: endorsementEntry.Signer.MemberType,
			},
			Signature: endorsementEntry.Signature,
		}
		if endorsement.Signer.MemberType == pbac.MemberType_CERT {
			cp.acService.log.Debugf("target endorser uses full certificate")
			memInfo = string(endorsement.Signer.MemberInfo)
		}
		if endorsement.Signer.MemberType == pbac.MemberType_CERT_HASH ||
			endorsement.Signer.MemberType == pbac.MemberType_ALIAS {
			cp.acService.log.Debugf("target endorser uses compressed certificate")
			memInfoBytes, ok := cp.lookUpCertCache(endorsement.Signer.MemberInfo)
			if !ok {
				cp.acService.log.Warnf("authentication failed, unknown signer, the provided certificate ID is not registered")
				continue
			}
			memInfo = string(memInfoBytes)
			endorsement.Signer.MemberInfo = memInfoBytes
		}

		signerInfo, ok := cp.acService.lookUpMemberInCache(memInfo)
		if !ok {
			cp.acService.log.Debugf("certificate not in local cache, should verify it against the trusted root certificates: "+
				"\n%s", memInfo)
			remoteMember, certChain, ok, err := cp.verifyPrincipalSignerNotInCache(endorsement, msg, memInfo)
			if !ok {
				cp.acService.log.Warnf("verify principal signer not in cache failed, [endorsement: %v],[err: %s]",
					endorsement, err.Error())
				continue
			}

			signerInfo = &memberCached{
				member:    remoteMember,
				certChain: certChain,
			}
			cp.acService.addMemberToCache(endorsement.Signer, signerInfo)
		} else {
			flat, err := cp.verifyPrincipalSignerInCache(signerInfo, endorsement, msg, memInfo)
			if !flat {
				cp.acService.log.Warnf("verify principal signer in cache failed, [endorsement: %v],[err: %s]",
					endorsement, err.Error())
				continue
			}
		}

		if _, ok := refinedSigners[memInfo]; !ok {
			refinedSigners[memInfo] = true
			refinedEndorsement = append(refinedEndorsement, endorsement)
		}
	}
	return refinedEndorsement
}

func (cp *certACProvider) RefineEndorsementsForCertOptimization(endorsements []*common.EndorsementEntry,
	msg []byte) []*common.EndorsementEntry {

	refinedSigners := map[string]bool{}
	var refinedEndorsement []*common.EndorsementEntry
	var memInfo string

	for _, endorsementEntry := range endorsements {
		endorsement := &common.EndorsementEntry{
			Signer: &pbac.Member{
				OrgId:      endorsementEntry.Signer.OrgId,
				MemberInfo: endorsementEntry.Signer.MemberInfo,
				MemberType: endorsementEntry.Signer.MemberType,
			},
			Signature: endorsementEntry.Signature,
		}
		if endorsement.Signer.MemberType == pbac.MemberType_CERT {
			cp.acService.log.Debugf("target endorser uses full certificate")
			memInfo = string(endorsement.Signer.MemberInfo)
		}
		if endorsement.Signer.MemberType == pbac.MemberType_CERT_HASH ||
			endorsement.Signer.MemberType == pbac.MemberType_ALIAS {
			cp.acService.log.Debugf("target endorser uses compressed certificate")
			memInfoBytes, ok := cp.lookUpCertCache(endorsement.Signer.MemberInfo)
			if !ok {
				cp.acService.log.Warnf("authentication failed, unknown signer, the provided certificate ID is not registered")
				continue
			}
			memInfo = string(memInfoBytes)
			endorsement.Signer.MemberInfo = memInfoBytes
		}

		if _, ok := refinedSigners[memInfo]; !ok {
			refinedSigners[memInfo] = true
			refinedEndorsement = append(refinedEndorsement, endorsement)
		}
	}
	return refinedEndorsement
}

// lookUpCertCache Cache for compressed certificate
func (cp *certACProvider) lookUpCertCache(certId []byte) ([]byte, bool) {
	ret, ok := cp.certCache.Get(string(certId))
	if !ok {
		cp.acService.log.Debugf("looking up the full certificate for the compressed one [%v]", certId)
		if cp.acService.dataStore == nil {
			cp.acService.log.Errorf("local data storage is not set up")
			return nil, false
		}
		certIdHex := hex.EncodeToString(certId)
		cert, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), []byte(certIdHex))
		if err != nil {
			cp.acService.log.Errorf("fail to load compressed certificate from local storage [%s]", certIdHex)
			return nil, false
		}
		if cert == nil {
			cp.acService.log.Warnf("cert id [%s] does not exist in local storage", certIdHex)
			return nil, false
		}
		cp.addCertCache(string(certId), cert)
		cp.acService.log.Debugf("compressed certificate [%s] found and stored in cache", certIdHex)
		return cert, true
	} else if ret != nil {
		cp.acService.log.Debugf("compressed certificate [%v] found in cache", certId)
		return ret.([]byte), true
	}
	cp.acService.log.Debugf("fail to look up compressed certificate [%v] due to an internal error of local cache",
		certId)
	return nil, false
}

func (cp *certACProvider) addCertCache(certId string, cert []byte) {
	cp.log.Debugf("add [%v] in cert cache", certId)
	cp.certCache.Add(certId, cert)
}

func (cp *certACProvider) verifyPrincipalSignerNotInCache(endorsement *common.EndorsementEntry, msg []byte,
	memInfo string) (remoteMember protocol.Member, certChain []*bcx509.Certificate, ok bool, err error) {
	var isTrustMember bool
	remoteMember, isTrustMember, err = cp.newNoCacheMember(endorsement.Signer)
	if err != nil {
		err = fmt.Errorf("new member failed: [%s]", err.Error())
		ok = false
		return
	}

	if !isTrustMember {
		certChain, err = cp.verifyMember(remoteMember)
		if err != nil {
			err = fmt.Errorf("verify member failed: [%s]", err.Error())
			ok = false
			return
		}
	}

	if err = remoteMember.Verify(cp.acService.hashType, msg, endorsement.Signature); err != nil {
		err = fmt.Errorf("member verify signature failed: [%s]", err.Error())
		cp.acService.log.Warnf("information for invalid signature:\norganization: %s\ncertificate: %s\nmessage: %s\n"+
			"signature: %s", endorsement.Signer.OrgId, memInfo, hex.Dump(msg), hex.Dump(endorsement.Signature))
		ok = false
		return
	}
	ok = true
	return
}

func (cp *certACProvider) verifyPrincipalSignerInCache(signerInfo *memberCached, endorsement *common.EndorsementEntry,
	msg []byte, memInfo string) (bool, error) {
	// check CRL and certificate frozen list

	_, isTrustMember := cp.loadTrustMembers(memInfo)

	if !isTrustMember {
		err := cp.checkCRL(signerInfo.certChain)
		if err != nil {
			return false, fmt.Errorf("check CRL, error: [%s]", err.Error())
		}
		err = cp.checkCertFrozenList(signerInfo.certChain)
		if err != nil {
			return false, fmt.Errorf("check cert forzen list, error: [%s]", err.Error())
		}
		cp.acService.log.Debugf("certificate is already seen, no need to verify against the trusted root certificates")

		if endorsement.Signer.OrgId != signerInfo.member.GetOrgId() {
			err := fmt.Errorf("authentication failed, signer does not belong to the organization it claims "+
				"[claim: %s, root cert: %s]", endorsement.Signer.OrgId, signerInfo.member.GetOrgId())
			return false, err
		}
	}
	if err := signerInfo.member.Verify(cp.acService.hashType, msg, endorsement.Signature); err != nil {
		err = fmt.Errorf("signer member verify signature failed: [%s]", err.Error())
		cp.acService.log.Warnf("information for invalid signature:\norganization: %s\ncertificate: %s\nmessage: %s\n"+
			"signature: %s", endorsement.Signer.OrgId, memInfo, hex.Dump(msg), hex.Dump(endorsement.Signature))
		return false, err
	}
	return true, nil
}

// Check whether the provided member is a valid member of this group
func (cp *certACProvider) verifyMember(mem protocol.Member) ([]*bcx509.Certificate, error) {
	if mem == nil {
		return nil, fmt.Errorf("invalid member: member should not be nil")
	}
	certMember, ok := mem.(*certificateMember)
	if !ok {
		return nil, fmt.Errorf("invalid member: member type err")
	}

	orgIdFromCert := certMember.cert.Subject.Organization[0]
	org := cp.acService.getOrgInfoByOrgId(orgIdFromCert)

	// the Third-party CA
	if certMember.cert.IsCA && org == nil {
		cp.acService.log.Info("the Third-party CA verify the member")
		certChain := []*bcx509.Certificate{certMember.cert}
		err := cp.checkCRL(certChain)
		if err != nil {
			return nil, err
		}

		err = cp.checkCertFrozenList(certChain)
		if err != nil {
			return nil, err
		}

		return certChain, nil
	}

	if mem.GetOrgId() != orgIdFromCert {
		return nil, fmt.Errorf(
			"signer does not belong to the organization it claims [claim: %s, certificate: %s]",
			mem.GetOrgId(),
			orgIdFromCert,
		)
	}

	if org == nil {
		return nil, fmt.Errorf("no orgnization found")
	}

	certChains, err := certMember.cert.Verify(cp.opts)
	if err != nil {
		return nil, fmt.Errorf("not ac valid certificate from trusted CAs: %v", err)
	}

	if len(org.(*organization).trustedRootCerts) <= 0 {
		return nil, fmt.Errorf("no trusted root: please configure trusted root certificate")
	}

	certChain := cp.findCertChain(org.(*organization), certChains)
	if certChain != nil {
		return certChain, nil
	}
	return nil, fmt.Errorf("authentication failed, signer does not belong to the organization it claims"+
		" [claim: %s]", mem.GetOrgId())
}

func (cp *certACProvider) findCertChain(org *organization, certChains [][]*bcx509.Certificate) []*bcx509.Certificate {
	for _, chain := range certChains {
		rootCert := chain[len(chain)-1]
		_, ok := org.trustedRootCerts[string(rootCert.Raw)]
		if ok {
			var err error
			// check CRL and frozen list
			err = cp.checkCRL(chain)
			if err != nil {
				cp.acService.log.Warnf("authentication failed, CRL: %v", err)
				continue
			}
			err = cp.checkCertFrozenList(chain)
			if err != nil {
				cp.acService.log.Warnf("authentication failed, certificate frozen list: %v", err)
				continue
			}
			return chain
		}
	}
	return nil
}

func (cp *certACProvider) initTrustRootsForUpdatingChainConfig(chainConfig *config.ChainConfig,
	localOrgId string) error {

	var orgNum int32
	orgList := sync.Map{}
	opts := bcx509.VerifyOptions{
		Intermediates: bcx509.NewCertPool(),
		Roots:         bcx509.NewCertPool(),
	}
	for _, orgRoot := range chainConfig.TrustRoots {
		org := &organization{
			id:                       orgRoot.OrgId,
			trustedRootCerts:         map[string]*bcx509.Certificate{},
			trustedIntermediateCerts: map[string]*bcx509.Certificate{},
		}

		for _, root := range orgRoot.Root {
			certificateChain, err := cp.buildCertificateChain(root, orgRoot.OrgId, org)
			if err != nil {
				return err
			}
			for _, certificate := range certificateChain {
				if certificate.IsCA {
					org.trustedRootCerts[string(certificate.Raw)] = certificate
					opts.Roots.AddCert(certificate)
				} else {
					org.trustedIntermediateCerts[string(certificate.Raw)] = certificate
					opts.Intermediates.AddCert(certificate)
				}
			}

			if len(org.trustedRootCerts) <= 0 {
				return fmt.Errorf(
					"update configuration failed, no trusted root (for %s): "+
						"please configure trusted root certificate or trusted public key whitelist",
					orgRoot.OrgId,
				)
			}
		}
		orgList.Store(org.id, org)
		orgNum++
	}
	atomic.StoreInt32(&cp.acService.orgNum, orgNum)
	cp.acService.orgList = &orgList
	cp.opts = opts
	localOrg := cp.acService.getOrgInfoByOrgId(localOrgId)
	if localOrg == nil {
		localOrg = &organization{
			id:                       localOrgId,
			trustedRootCerts:         map[string]*bcx509.Certificate{},
			trustedIntermediateCerts: map[string]*bcx509.Certificate{},
		}
	}
	cp.localOrg, _ = localOrg.(*organization)
	return nil
}

// GetAllPolicy returns all default policies
func (p *certACProvider) GetAllPolicy() (map[string]*pbac.Policy, error) {
	var policyMap = make(map[string]*pbac.Policy)
	p.acService.resourceNamePolicyMap.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		v, _ := value.(*policy)
		policyMap[k] = newPbPolicyFromPolicy(v)
		return true
	})
	p.acService.senderPolicyMap.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		v, _ := value.(*policy)
		policyMap[k] = newPbPolicyFromPolicy(v)
		return true
	})
	return policyMap, nil
}

// VerifyPrincipalLT2330 verifies if the principal for the resource is met
func (cp *certACProvider) VerifyPrincipalLT2330(principal protocol.Principal, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(cp, principal)

	} else if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(cp, principal)
	}

	return false, fmt.Errorf("`VerifyPrincipalLT2330` should not used by blockVersion(%d)", blockVersion)
}

// GetValidEndorsements filters all endorsement entries and returns all valid ones
func (cp *certACProvider) GetValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*common.EndorsementEntry, error) {

	if blockVersion <= blockVersion220 {
		return cp.getValidEndorsements220(principal)
	}

	if blockVersion < blockVersion2330 {
		return cp.getValidEndorsements2320(principal)
	}

	return cp.getValidEndorsements(principal, blockVersion)
}

// VerifyMsgPrincipal verifies if the principal for the resource is met
func (cp *certACProvider) VerifyMsgPrincipal(principal protocol.Principal, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(cp, principal)
	}

	if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(cp, principal)
	}

	return verifyMsgTypePrincipal(cp, principal, blockVersion)
}

// VerifyTxPrincipal verifies if the principal for the resource is met
func (cp *certACProvider) VerifyTxPrincipal(tx *common.Transaction,
	resourceName string, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		if err := verifyTxPrincipal220(tx, cp); err != nil {
			return false, err
		}
		return true, nil
	}

	if blockVersion < blockVersion2330 {
		if err := verifyTxPrincipal2320(tx, resourceName, cp); err != nil {
			return false, err
		}
		return true, nil
	}

	return verifyTxPrincipal(tx, resourceName, cp, blockVersion)
}

// VerifyMultiSignTxPrincipal verify if the multi-sign tx should be finished
func (cp *certACProvider) VerifyMultiSignTxPrincipal(
	mInfo *syscontract.MultiSignInfo,
	blockVersion uint32) (syscontract.MultiSignStatus, error) {

	if blockVersion < blockVersion2330 {
		return mInfo.Status, fmt.Errorf(
			"func `verifyMultiSignTxPrincipal` cannot be used in blockVersion(%v)", blockVersion)
	}
	return verifyMultiSignTxPrincipal(cp, mInfo, blockVersion, cp.acService.log)
}

// IsRuleSupportedByMultiSign verify the policy of resourceName is supported by multi-sign
// it's implements must be the same with vm-native/supportRule
func (cp *certACProvider) IsRuleSupportedByMultiSign(resourceName string, blockVersion uint32) error {
	if blockVersion < blockVersion220 {
		return isRuleSupportedByMultiSign220(cp, resourceName, cp.acService.log)
	}

	if blockVersion < blockVersion2330 {
		return isRuleSupportedByMultiSign2320(resourceName, cp, cp.acService.log)
	}

	return isRuleSupportedByMultiSign(cp, resourceName, blockVersion, cp.acService.log)
}

// GetCertFromCache get cert from cache
func (cp *certACProvider) GetAddressFromCache(pkBytes []byte) (string, crypto.PublicKey, error) {
	pkPem := string(pkBytes)
	acs := cp.acService
	// pk 一定恢复不成证书模式下的member
	// 所以重新创建缓存的key
	indexKey := "pk_" + pkPem
	cached, ok := acs.lookUpMemberInCache(indexKey)
	if ok {
		acs.log.Debugf("member address found in local cache")
		return cached.address, cached.pk, nil
	}

	pk, err := asym.PublicKeyFromPEM(pkBytes)
	if err != nil {
		return "", nil, fmt.Errorf("new public key member failed: parse the public key from PEM failed")
	}

	publicKeyString, err := utils.PkToAddrStr(pk, acs.addressType, crypto.HashAlgoMap[acs.hashType])
	if err != nil {
		return "", nil, err
	}

	if acs.addressType == config.AddrType_ZXL {
		publicKeyString = "ZX" + publicKeyString
	}

	acs.memberCache.Add(indexKey, &memberCached{address: publicKeyString, pk: pk})

	return publicKeyString, pk, nil
}

// GetCertFromCache get cert from cache
func (cp *certACProvider) GetCertFromCache(certId []byte) ([]byte, error) {
	ret, ok := cp.certCache.Get(string(certId))
	if !ok {
		cp.acService.log.Debugf("looking up the full certificate for the compressed one [%v]", certId)
		if cp.acService.dataStore == nil {
			cp.acService.log.Errorf("local data storage is not set up")
			return nil, fmt.Errorf("local data storage is not set up")
		}
		certIdHex := hex.EncodeToString(certId)
		cert, err := cp.acService.dataStore.ReadObject(syscontract.SystemContract_CERT_MANAGE.String(), []byte(certIdHex))
		if err != nil {
			cp.acService.log.Errorf("fail to load compressed certificate from local storage [%s]", certIdHex)
			return nil, fmt.Errorf("fail to load compressed certificate from local storage [%s]", certIdHex)
		}
		if cert == nil {
			cp.acService.log.Warnf("cert id [%s] does not exist in local storage", certIdHex)
			return nil, fmt.Errorf("cert id [%s] does not exist in local storage", certIdHex)
		}
		cp.addCertCache(string(certId), cert)
		cp.acService.log.Debugf("compressed certificate [%s] found and stored in cache", certIdHex)
		return cert, nil
	} else if ret != nil {
		cp.acService.log.Debugf("compressed certificate [%v] found in cache", certId)
		return ret.([]byte), nil
	}
	cp.acService.log.Debugf("fail to look up compressed certificate [%v] due to an internal error of local cache",
		certId)
	return nil, fmt.Errorf("fail to look up compressed certificate due to an internal error of local cache")
}

// GetPayerFromCache get payer from cache
func (cp *certACProvider) GetPayerFromCache(key []byte) ([]byte, error) {
	cp.acService.log.Debugf("get from cache, key=", string(key))
	value, ok := cp.payerList.Get(string(key))
	if !ok {
		return nil, fmt.Errorf("not found %s", string(key))
	}
	byteValue, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value is not a []byte]: %v", value)
	}
	return []byte(byteValue), nil
}

// SetPayerToCache set payer to cache
func (cp *certACProvider) SetPayerToCache(key []byte, value []byte) error {
	cp.acService.log.Debugf("set cache, key=", string(key), "#value=", string(value))
	cp.payerList.Add(string(key), string(value))
	return nil
}

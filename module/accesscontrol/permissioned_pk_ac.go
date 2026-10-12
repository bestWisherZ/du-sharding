/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/common/v2/msgbus"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"

	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
)

var _ protocol.AccessControlProvider = (*permissionedPkACProvider)(nil)

var nilPermissionedPkACProvider ACProvider = (*permissionedPkACProvider)(nil)

type permissionedPkACProvider struct {
	acService *accessControlService

	// local org id
	localOrg string

	// admin list in permissioned public key mode
	adminMember *sync.Map

	// consensus list in permissioned public key mode
	consensusMember *sync.Map

	// used to cache the deduction account address to avoid reading the database every time
	payerList *ShardCache
}

type adminMemberModel struct {
	publicKey crypto.PublicKey
	pkBytes   []byte
	orgId     string
}

type consensusMemberModel struct {
	nodeId string
	orgId  string
}

func (pp *permissionedPkACProvider) NewACProvider(chainConf protocol.ChainConf, localOrgId string,
	store protocol.BlockchainStore, log protocol.Logger, msgBus msgbus.MessageBus) (
	protocol.AccessControlProvider, error) {
	pPkACProvider, err := newPermissionedPkACProvider(chainConf.ChainConfig(), localOrgId, store, log)
	if err != nil {
		return nil, err
	}
	msgBus.Register(msgbus.ChainConfig, pPkACProvider)
	msgBus.Register(msgbus.PubkeyManageDelete, pPkACProvider)
	msgBus.Register(msgbus.BlockInfo, pPkACProvider)
	// v220_compat Deprecated
	{
		chainConf.AddWatch(pPkACProvider)   //nolint: staticcheck
		chainConf.AddVmWatch(pPkACProvider) //nolint: staticcheck
	}
	return pPkACProvider, nil
}

// Start the ac service.
func (cp *permissionedPkACProvider) Start() error {
	return nil
}

// Stop the ac service.
func (cp *permissionedPkACProvider) Stop() error {
	return nil
}

func newPermissionedPkACProvider(chainConfig *config.ChainConfig, localOrgId string,
	store protocol.BlockchainStore, log protocol.Logger) (*permissionedPkACProvider, error) {
	ppacProvider := &permissionedPkACProvider{
		adminMember:     &sync.Map{},
		consensusMember: &sync.Map{},
		payerList:       NewShardCache(GetCertCacheSize()),
		localOrg:        localOrgId,
	}
	chainConfig.AuthType = strings.ToLower(chainConfig.AuthType)
	ppacProvider.acService = initAccessControlService(chainConfig.GetCrypto().Hash,
		chainConfig.AuthType, chainConfig.Vm.AddrType, store, log)
	ppacProvider.acService.pwkNewMember = ppacProvider.NewMemberFromAcs

	err := ppacProvider.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		return nil, err
	}

	err = ppacProvider.initConsensusMember(chainConfig.Consensus.Nodes)
	if err != nil {
		return nil, err
	}

	ppacProvider.acService.initResourcePolicy(chainConfig.ResourcePolicies, localOrgId)

	return ppacProvider, nil
}

func (pp *permissionedPkACProvider) initAdminMembers(trustRootList []*config.TrustRootConfig) error {
	var (
		tempSyncMap, orgList sync.Map
		orgNum               int32
	)
	for _, trustRoot := range trustRootList {
		for _, root := range trustRoot.Root {
			pk, err := asym.PublicKeyFromPEM([]byte(root))
			if err != nil {
				return fmt.Errorf("init admin member failed: parse the public key from PEM failed")
			}

			pkBytes, err := pk.Bytes()
			if err != nil {
				return fmt.Errorf("init admin member failed: %s", err.Error())
			}

			adminMember := &adminMemberModel{
				publicKey: pk,
				pkBytes:   pkBytes,
				orgId:     trustRoot.OrgId,
			}
			adminKey := hex.EncodeToString(pkBytes)
			tempSyncMap.Store(adminKey, adminMember)
		}

		_, ok := orgList.Load(trustRoot.OrgId)
		if !ok {
			orgList.Store(trustRoot.OrgId, struct{}{})
			orgNum++
		}
	}
	atomic.StoreInt32(&pp.acService.orgNum, orgNum)
	pp.acService.orgList = &orgList
	pp.adminMember = &tempSyncMap
	return nil
}

func (pp *permissionedPkACProvider) initConsensusMember(consensusConf []*config.OrgConfig) error {
	var tempSyncMap sync.Map
	for _, conf := range consensusConf {
		for _, node := range conf.NodeId {

			consensusMember := &consensusMemberModel{
				nodeId: node,
				orgId:  conf.OrgId,
			}
			tempSyncMap.Store(node, consensusMember)
		}
	}
	pp.consensusMember = &tempSyncMap
	return nil
}

// all-in-one validation for signing members: certificate chain/whitelist, signature, policies
func (pp *permissionedPkACProvider) refinePrincipal(principal protocol.Principal) (protocol.Principal, error) {
	endorsements := principal.GetEndorsement()
	msg := principal.GetMessage()
	refinedEndorsement := pp.RefineEndorsements(endorsements, msg)
	if len(refinedEndorsement) <= 0 {
		return nil, fmt.Errorf("refine endorsements failed, all endorsers have failed verification")
	}

	refinedPrincipal, err := pp.CreatePrincipal(principal.GetResourceName(), refinedEndorsement, msg)
	if err != nil {
		return nil, fmt.Errorf("create principal failed: [%s]", err.Error())
	}

	return refinedPrincipal, nil
}

func (pp *permissionedPkACProvider) refinePrincipalForCertOptimization(
	principal protocol.Principal) (
	protocol.Principal, error) {
	return nil, fmt.Errorf("this method should not be called")
}

func (pp *permissionedPkACProvider) RefineEndorsements(endorsements []*common.EndorsementEntry,
	msg []byte) []*common.EndorsementEntry {

	refinedSigners := map[string]bool{}
	var refinedEndorsement []*common.EndorsementEntry

	for _, endorsementEntry := range endorsements {
		if endorsementEntry == nil || endorsementEntry.Signer == nil {
			continue
		}
		endorsement := &common.EndorsementEntry{
			Signer: &pbac.Member{
				OrgId:      endorsementEntry.Signer.OrgId,
				MemberInfo: endorsementEntry.Signer.MemberInfo,
				MemberType: endorsementEntry.Signer.MemberType,
			},
			Signature: endorsementEntry.Signature,
		}

		memInfo := string(endorsement.Signer.MemberInfo)

		remoteMember, err := pp.NewMember(endorsement.Signer)
		if err != nil {
			pp.acService.log.Infof("new member failed: [%s]", err.Error())
			continue
		}

		if err := remoteMember.Verify(pp.GetHashAlg(), msg, endorsement.Signature); err != nil {
			pp.acService.log.Infof("signer member verify signature failed: [%s]", err.Error())
			pp.acService.log.Debugf("information for invalid signature:\norganization: %s\npubkey: %s\nmessage: %s\n"+
				"signature: %s", endorsement.Signer.OrgId, memInfo, hex.Dump(msg), hex.Dump(endorsement.Signature))
			continue
		}

		if _, ok := refinedSigners[memInfo]; !ok {
			refinedSigners[memInfo] = true
			refinedEndorsement = append(refinedEndorsement, endorsement)
		}
	}
	return refinedEndorsement
}

// NewMember creates a member from pb Member
func (pp *permissionedPkACProvider) NewMember(member *pbac.Member) (protocol.Member, error) {
	return pp.acService.newPkMember(member, pp.adminMember, pp.consensusMember)
}

// NewMemberFromAcs creates a member from pb Member
func (pp *permissionedPkACProvider) NewMemberFromAcs(member *pbac.Member) (protocol.Member, error) {
	pkMember, err := newPkMemberFromAcs(member, pp.adminMember, pp.consensusMember, pp.acService)
	if err != nil {
		return nil, fmt.Errorf("new public key member failed: %s", err.Error())
	}
	if pkMember.GetOrgId() != member.OrgId && member.OrgId != "" {
		return nil, fmt.Errorf("new public key member failed: member orgId does not match on chain")
	}
	return pkMember, err
}

// GetHashAlg return hash algorithm the access control provider uses
func (pp *permissionedPkACProvider) GetHashAlg() string {
	return pp.acService.hashType
}

// ValidateResourcePolicy checks whether the given resource principal is valid
func (pp *permissionedPkACProvider) ValidateResourcePolicy(resourcePolicy *config.ResourcePolicy) bool {
	return pp.acService.validateResourcePolicy(resourcePolicy)
}

// GetMemberStatus get the status information of the member
func (pp *permissionedPkACProvider) GetMemberStatus(member *pbac.Member) (pbac.MemberStatus, error) {
	if _, err := pp.newNodeMember(member); err != nil {
		pp.acService.log.Infof("get member status: %s", err.Error())
		return pbac.MemberStatus_INVALID, err
	}
	return pbac.MemberStatus_NORMAL, nil
}

// VerifyRelatedMaterial verify the member's relevant identity material
func (pp *permissionedPkACProvider) VerifyRelatedMaterial(verifyType pbac.VerifyType, data []byte) (bool, error) {
	return true, nil
}

func (pp *permissionedPkACProvider) newNodeMember(member *pbac.Member) (protocol.Member, error) {
	return pp.acService.newNodePkMember(member, pp.consensusMember)
}

// GetAllPolicy returns all default policies
func (p *permissionedPkACProvider) GetAllPolicy() (map[string]*pbac.Policy, error) {
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
func (pp *permissionedPkACProvider) VerifyPrincipalLT2330(
	principal protocol.Principal, blockVersion uint32) (bool, error) {

	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(pp, principal)

	} else if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(pp, principal)
	}

	return false, fmt.Errorf("`VerifyPrincipalLT2330` should not used by blockVersion(%d)", blockVersion)
}

// GetValidEndorsements filters all endorsement entries and returns all valid ones
func (pp *permissionedPkACProvider) GetValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*common.EndorsementEntry, error) {

	if blockVersion <= blockVersion220 {
		return pp.getValidEndorsements220(principal)
	}

	if blockVersion < blockVersion2330 {
		return pp.getValidEndorsements2320(principal)
	}
	return pp.getValidEndorsements(principal, blockVersion)
}

// VerifyMsgPrincipal verifies if the principal for the resource is met
func (pp *permissionedPkACProvider) VerifyMsgPrincipal(
	principal protocol.Principal, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(pp, principal)
	}

	if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(pp, principal)
	}

	return verifyMsgTypePrincipal(pp, principal, blockVersion)
}

// VerifyTxPrincipal verifies if the principal for the resource is met
func (pp *permissionedPkACProvider) VerifyTxPrincipal(
	tx *common.Transaction, resourceName string, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		if err := verifyTxPrincipal220(tx, pp); err != nil {
			return false, err
		}
		return true, nil
	}

	if blockVersion < blockVersion2330 {
		if err := verifyTxPrincipal2320(tx, resourceName, pp); err != nil {
			return false, err
		}
		return true, nil
	}

	return verifyTxPrincipal(tx, resourceName, pp, blockVersion)
}

// VerifyMultiSignTxPrincipal verify if the multi-sign tx should be finished
func (pp *permissionedPkACProvider) VerifyMultiSignTxPrincipal(
	mInfo *syscontract.MultiSignInfo,
	blockVersion uint32) (syscontract.MultiSignStatus, error) {

	if blockVersion < blockVersion2330 {
		return mInfo.Status, fmt.Errorf(
			"func `verifyMultiSignTxPrincipal` cannot be used in blockVersion(%v)", blockVersion)
	}
	return verifyMultiSignTxPrincipal(pp, mInfo, blockVersion, pp.acService.log)
}

// IsRuleSupportedByMultiSign verify the policy of resourceName is supported by multi-sign
// it's implements must be the same with vm-native/supportRule
func (pp *permissionedPkACProvider) IsRuleSupportedByMultiSign(resourceName string, blockVersion uint32) error {
	if blockVersion < blockVersion220 {
		return isRuleSupportedByMultiSign220(pp, resourceName, pp.acService.log)
	}

	if blockVersion < blockVersion2330 {
		return isRuleSupportedByMultiSign2320(resourceName, pp, pp.acService.log)
	}

	return isRuleSupportedByMultiSign(pp, resourceName, blockVersion, pp.acService.log)
}

// GetCertFromCache get cert from cache
func (pp *permissionedPkACProvider) GetAddressFromCache(pkBytes []byte) (string, crypto.PublicKey, error) {
	pkPem := string(pkBytes)
	acs := pp.acService
	cached, ok := acs.lookUpMemberInCache(pkPem)
	if ok {
		acs.log.Debugf("member address found in local cache")
		return cached.address, cached.pk, nil
	}

	// in case 缓存被清空，找不到原来保存的member信息
	// 又因为 pk 没办法直接恢复成member信息，所以创建新的index key
	indexKey := "pk_" + pkPem
	cached, ok = acs.lookUpMemberInCache(indexKey)
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

func (pp *permissionedPkACProvider) GetCertFromCache(keyBytes []byte) ([]byte, error) {
	return nil, fmt.Errorf("not support in permissionedPkACProvider")
}

// GetPayerFromCache get payer from cache
func (pp *permissionedPkACProvider) GetPayerFromCache(key []byte) ([]byte, error) {
	value, ok := pp.payerList.Get(string(key))
	if !ok {
		return nil, fmt.Errorf("not found %s", key)
	}
	byteValue, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("value is not a []byte]: %v", value)
	}
	return []byte(byteValue), nil
}

// SetPayerToCache set payer to cache
func (pp *permissionedPkACProvider) SetPayerToCache(key []byte, value []byte) error {
	pp.payerList.Add(string(key), string(value))
	return nil
}

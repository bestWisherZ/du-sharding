/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0

This file is for version compatibility
*/

package accesscontrol

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/localconf/v2"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
)

// ModuleNameAccessControl  "Access Control"
const ModuleNameAccessControl = "Access Control"

var _ protocol.Watcher = (*pkACProvider)(nil)

var _ protocol.Watcher = (*permissionedPkACProvider)(nil)
var _ protocol.VmWatcher = (*permissionedPkACProvider)(nil)

var _ protocol.Watcher = (*certACProvider)(nil)
var _ protocol.VmWatcher = (*certACProvider)(nil)

func (cp *certACProvider) Module() string {
	return ModuleNameAccessControl
}

func (cp *certACProvider) Watch(chainConfig *config.ChainConfig) error {
	cp.acService.hashType = chainConfig.GetCrypto().GetHash()
	err := cp.initTrustRootsForUpdatingChainConfig(chainConfig, cp.localOrg.id)
	if err != nil {
		return err
	}

	cp.acService.initResourcePolicy(chainConfig.ResourcePolicies, cp.localOrg.id)
	cp.acService.initResourcePolicy_220(chainConfig.ResourcePolicies, cp.localOrg.id)

	cp.opts.KeyUsages = make([]x509.ExtKeyUsage, 1)
	cp.opts.KeyUsages[0] = x509.ExtKeyUsageAny

	cp.acService.memberCache.Clear()
	cp.certCache.Clear()
	err = cp.initTrustMembers(chainConfig.TrustMembers)
	if err != nil {
		return err
	}
	return nil
}

func (cp *certACProvider) ContractNames() []string {
	return []string{syscontract.SystemContract_CERT_MANAGE.String()}
}

func (cp *certACProvider) Callback(contractName string, payloadBytes []byte) error {
	switch contractName {
	case syscontract.SystemContract_CERT_MANAGE.String():
		cp.acService.log.Infof("[AC] callback msg, contract name: %s", contractName)
		return cp.systemContractCallbackCertManagementCase(payloadBytes)
	default:
		cp.acService.log.Debugf("unwatched smart contract [%s]", contractName)
		return nil
	}
}

func (cp *certACProvider) systemContractCallbackCertManagementCase(payloadBytes []byte) error {
	var payload commonPb.Payload
	err := proto.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return fmt.Errorf("resolve payload failed: %v", err)
	}
	switch payload.Method {
	case syscontract.CertManageFunction_CERTS_FREEZE.String():
		return cp.systemContractCallbackCertManagementCertFreezeCase(&payload)
	case syscontract.CertManageFunction_CERTS_UNFREEZE.String():
		return cp.systemContractCallbackCertManagementCertUnfreezeCase(&payload)
	case syscontract.CertManageFunction_CERTS_REVOKE.String():
		return cp.systemContractCallbackCertManagementCertRevokeCase(&payload)
	case syscontract.CertManageFunction_CERTS_DELETE.String():
		return cp.systemContractCallbackCertManagementCertsDeleteCase(&payload)
	case syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String():
		return cp.systemContractCallbackCertManagementAliasDeleteCase(&payload)
	case syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String():
		return cp.systemContractCallbackCertManagementAliasUpdateCase(&payload)

	default:
		cp.acService.log.Debugf("unwatched method [%s]", payload.Method)
		return nil
	}
}
func (cp *certACProvider) systemContractCallbackCertManagementCertFreezeCase(payload *commonPb.Payload) error {
	for _, param := range payload.Parameters {
		if param.Key == PARAM_CERTS {
			certList := strings.Replace(string(param.Value), ",", "\n", -1)
			certBlock, rest := pem.Decode([]byte(certList))
			for certBlock != nil {
				cp.frozenList.Store(string(certBlock.Bytes), true)

				certBlock, rest = pem.Decode(rest)
			}
			return nil
		}
	}
	return nil
}

func (cp *certACProvider) systemContractCallbackCertManagementCertUnfreezeCase(payload *commonPb.Payload) error {
	for _, param := range payload.Parameters {
		if param.Key == PARAM_CERTS {
			certList := strings.Replace(string(param.Value), ",", "\n", -1)
			certBlock, rest := pem.Decode([]byte(certList))
			for certBlock != nil {
				_, ok := cp.frozenList.Load(string(certBlock.Bytes))
				if ok {
					cp.frozenList.Delete(string(certBlock.Bytes))
				}
				certBlock, rest = pem.Decode(rest)
			}
			return nil
		}
	}
	return nil
}

func (cp *certACProvider) systemContractCallbackCertManagementCertRevokeCase(payload *commonPb.Payload) error {
	for _, param := range payload.Parameters {
		if param.Key == "cert_crl" {
			crl := strings.Replace(string(param.Value), ",", "\n", -1)
			crls, err := cp.ValidateCRL([]byte(crl))
			if err != nil {
				return fmt.Errorf("update CRL failed, invalid CRLS: %v", err)
			}
			for _, crl := range crls {
				aki, _, err := bcx509.GetAKIFromExtensions(crl.TBSCertList.Extensions)
				if err != nil {
					return fmt.Errorf("update CRL failed: %v", err)
				}
				cp.crl.Store(string(aki), crl)
			}
			return nil
		}
	}
	return nil
}

func (cp *certACProvider) systemContractCallbackCertManagementCertsDeleteCase(payload *commonPb.Payload) error {
	cp.acService.log.Debugf("callback for certsdelete")
	for _, param := range payload.Parameters {
		if param.Key == PARAM_CERTHASHES {
			certHashStr := strings.TrimSpace(string(param.Value))
			certHashes := strings.Split(certHashStr, ",")
			for _, hash := range certHashes {
				cp.acService.log.Debugf("certHashes in certsdelete = [%s]", hash)
				bin, err := hex.DecodeString(string(hash))
				if err != nil {
					cp.acService.log.Warnf("decode error for certhash: %s", string(hash))
					return nil
				}
				_, ok := cp.certCache.Get(string(bin))
				if ok {
					cp.acService.log.Infof("remove certhash from certcache: %s", string(hash))
					cp.certCache.Remove(string(bin))
				}
			}

			return nil
		}
	}
	return nil
}

func (cp *certACProvider) systemContractCallbackCertManagementAliasDeleteCase(payload *commonPb.Payload) error {
	cp.acService.log.Debugf("callback for aliasdelete")
	for _, param := range payload.Parameters {
		if param.Key == PARAM_ALIASES {
			names := strings.TrimSpace(string(param.Value))
			nameList := strings.Split(names, ",")
			cp.acService.log.Debugf("names in aliasdelete = [%s]", nameList)
			for _, name := range nameList {
				_, ok := cp.certCache.Get(string(name))
				if ok {
					cp.acService.log.Infof("remove alias from certcache: %s", string(name))
					cp.certCache.Remove(string(name))
				}
			}
			return nil
		}
	}
	return nil
}

func (cp *certACProvider) systemContractCallbackCertManagementAliasUpdateCase(payload *commonPb.Payload) error {
	cp.acService.log.Debugf("callback for aliasupdate")
	for _, param := range payload.Parameters {
		if param.Key == PARAM_ALIAS {
			name := strings.TrimSpace(string(param.Value))
			cp.acService.log.Infof("name in aliasupdate = [%s]", name)
			_, ok := cp.certCache.Get(string(name))
			if ok {
				cp.acService.log.Infof("remove alias from certcache: %s", string(name))
				cp.certCache.Remove(string(name))
			}
			return nil
		}
	}
	return nil
}

func (pp *permissionedPkACProvider) Module() string {
	return ModuleNameAccessControl
}

func (pp *permissionedPkACProvider) Watch(chainConfig *config.ChainConfig) error {
	pp.acService.hashType = chainConfig.GetCrypto().GetHash()

	err := pp.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		return fmt.Errorf("update chainconfig error: %s", err.Error())
	}

	err = pp.initConsensusMember(chainConfig.Consensus.Nodes)
	if err != nil {
		return fmt.Errorf("update chainconfig error: %s", err.Error())
	}

	pp.acService.initResourcePolicy(chainConfig.ResourcePolicies, pp.localOrg)
	pp.acService.initResourcePolicy_220(chainConfig.ResourcePolicies, pp.localOrg)

	pp.acService.memberCache.Clear()

	return nil
}

func (pp *permissionedPkACProvider) ContractNames() []string {
	return []string{syscontract.SystemContract_PUBKEY_MANAGE.String()}
}

func (pp *permissionedPkACProvider) Callback(contractName string, payloadBytes []byte) error {
	switch contractName {
	case syscontract.SystemContract_PUBKEY_MANAGE.String():
		return pp.systemContractCallbackPublicKeyManagementCase(payloadBytes)
	default:
		pp.acService.log.Debugf("unwatched smart contract [%s]", contractName)
		return nil
	}
}

func (pp *permissionedPkACProvider) systemContractCallbackPublicKeyManagementCase(payloadBytes []byte) error {
	var payload commonPb.Payload
	err := proto.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return fmt.Errorf("resolve payload failed: %v", err)
	}
	switch payload.Method {
	case syscontract.PubkeyManageFunction_PUBKEY_DELETE.String():
		return pp.systemContractCallbackPublicKeyManagementDeleteCase(&payload)
	default:
		pp.acService.log.Debugf("unwatched method [%s]", payload.Method)
		return nil
	}
}

func (pp *permissionedPkACProvider) systemContractCallbackPublicKeyManagementDeleteCase(
	payload *commonPb.Payload) error {
	for _, param := range payload.Parameters {
		if param.Key == PUBLIC_KEYS {
			pk, err := asym.PublicKeyFromPEM(param.Value)
			if err != nil {
				return fmt.Errorf("delete member cache failed, [%v]", err.Error())
			}
			pkStr, err := pk.String()
			if err != nil {
				return fmt.Errorf("delete member cache failed, [%v]", err.Error())
			}
			pp.acService.memberCache.Remove(pkStr)
			pp.acService.log.Debugf("The public key was removed from the cache,[%v]", pkStr)
		}
	}
	return nil
}

func (pk *pkACProvider) Module() string {
	return ModuleNameAccessControl
}

func (pk *pkACProvider) Watch(chainConfig *config.ChainConfig) error {

	pk.hashType = chainConfig.GetCrypto().GetHash()
	err := pk.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		return fmt.Errorf("new public AC provider failed: %s", err.Error())
	}

	err = pk.initConsensusMember(chainConfig)
	if err != nil {
		return fmt.Errorf("new public AC provider failed: %s", err.Error())
	}
	pk.memberCache.Clear()
	return nil
}

// ****************************************************
//  cert mode
// ****************************************************

func (cp *certACProvider) lookUpPolicy220(resourceName string) (*acPb.Policy, error) {
	return cp.acService.lookUpPolicy220(resourceName)
}

func (cp *certACProvider) lookUpExceptionalPolicy220(resourceName string) (*acPb.Policy, error) {
	return cp.acService.lookUpExceptionalPolicy220(resourceName)
}

func (cp *certACProvider) lookUpPolicyByResourceName220(resourceName string) (*policy, error) {
	return cp.acService.lookUpPolicyByResourceName220(resourceName)
}

func (cp *certACProvider) getValidEndorsements220(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&cp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := cp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := cp.lookUpPolicyByResourceName220(principal.GetResourceName())
	if err != nil {
		return nil, fmt.Errorf("authentication fail: [%v]", err)
	}
	orgListRaw := p.GetOrgList()
	roleListRaw := p.GetRoleList()
	orgList := map[string]bool{}
	roleList := map[protocol.Role]bool{}
	for _, orgRaw := range orgListRaw {
		orgList[orgRaw] = true
	}
	for _, roleRaw := range roleListRaw {
		roleList[roleRaw] = true
	}
	return cp.acService.getValidEndorsements(orgList, roleList, endorsements), nil
}

// ****************************************************
//  permitted public-key mode
// ****************************************************

func (pp *permissionedPkACProvider) lookUpPolicy220(resourceName string) (*acPb.Policy, error) {
	return pp.acService.lookUpPolicy220(resourceName)
}

func (pp *permissionedPkACProvider) lookUpExceptionalPolicy220(resourceName string) (*acPb.Policy, error) {
	return pp.acService.lookUpExceptionalPolicy220(resourceName)
}

func (pp *permissionedPkACProvider) lookUpPolicyByResourceName220(resourceName string) (*policy, error) {
	return pp.acService.lookUpPolicyByResourceName220(resourceName)
}

func (pp *permissionedPkACProvider) getValidEndorsements220(
	principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&pp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := pp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := pp.lookUpPolicyByResourceName220(principal.GetResourceName())
	if err != nil {
		return nil, fmt.Errorf("authentication fail: [%v]", err)
	}
	orgListRaw := p.GetOrgList()
	roleListRaw := p.GetRoleList()
	orgList := map[string]bool{}
	roleList := map[protocol.Role]bool{}
	for _, orgRaw := range orgListRaw {
		orgList[orgRaw] = true
	}
	for _, roleRaw := range roleListRaw {
		roleList[roleRaw] = true
	}
	return pp.acService.getValidEndorsements(orgList, roleList, endorsements), nil
}

// ****************************************************
//  public-key mode
// ****************************************************

func (pk *pkACProvider) lookUpPolicy220(resourceName string) (*acPb.Policy, error) {
	if p, ok := pk.resourceNamePolicyMap220.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}
	return nil, fmt.Errorf("policy not found for resource %s", resourceName)
}

func (pk *pkACProvider) lookUpExceptionalPolicy220(resourceName string) (*acPb.Policy, error) {
	if p, ok := pk.exceptionalPolicyMap220.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil

	}
	return nil, fmt.Errorf("exceptional policy not found for resource %s", resourceName)
}

func (pk *pkACProvider) lookUpPolicyByResourceName220(resourceName string) (*policy, error) {
	p, ok := pk.resourceNamePolicyMap220.Load(resourceName)
	if !ok {
		if p, ok = pk.exceptionalPolicyMap220.Load(resourceName); !ok {
			return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
				"for resource %s", resourceName)
		}
	}
	return p.(*policy), nil
}

func (pk *pkACProvider) getValidEndorsements220(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	refinedPolicy, err := pk.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("refinePrincipal fail in GetValidEndorsements: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	pol, err := pk.lookUpPolicyByResourceName220(principal.GetResourceName())
	if err != nil {
		return nil, fmt.Errorf("lookUpPolicyByResourceName fail in GetValidEndorsements: [%v]", err)
	}
	roleListRaw := pol.GetRoleList()
	orgList := map[string]bool{}
	roleList := map[protocol.Role]bool{}
	for _, roleRaw := range roleListRaw {
		roleList[roleRaw] = true
	}
	return pk.getValidEndorsementsInner(orgList, roleList, endorsements), nil
}

// ****************************************************
//  access control service
// ****************************************************

func (acs *accessControlService) initResourcePolicy_220(resourcePolicies []*config.ResourcePolicy,
	localOrgId string) {
	authType := strings.ToLower(acs.authType)
	switch authType {
	case protocol.PermissionedWithCert, protocol.Identity:
		acs.createDefaultResourcePolicyForCert_220()
	case protocol.PermissionedWithKey:
		acs.createDefaultResourcePolicyForPK_220()
	}
	for _, resourcePolicy := range resourcePolicies {
		if acs.validateResourcePolicy(resourcePolicy) {
			policy := newPolicyFromPb(resourcePolicy.Policy)
			acs.resourceNamePolicyMap220.Store(resourcePolicy.ResourceName, policy)
		}
	}
}

func (acs *accessControlService) lookUpPolicy220(resourceName string) (*acPb.Policy, error) {
	if p, ok := acs.resourceNamePolicyMap220.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}
	return nil, fmt.Errorf("policy not found for resource %s", resourceName)
}

func (acs *accessControlService) lookUpExceptionalPolicy220(resourceName string) (*acPb.Policy, error) {
	if p, ok := acs.exceptionalPolicyMap220.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil

	}
	return nil, fmt.Errorf("exceptional policy not found for resource %s", resourceName)
}

func (acs *accessControlService) lookUpPolicyByResourceName220(resourceName string) (*policy, error) {
	p, ok := acs.resourceNamePolicyMap220.Load(resourceName)
	if !ok {
		if p, ok = acs.exceptionalPolicyMap220.Load(resourceName); !ok {
			return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
				"for resource %s", resourceName)
		}
	}
	return p.(*policy), nil
}

// ****************************************************
//  function utils
// ****************************************************

func verifyPrincipal220(p acProvider220, principal protocol.Principal) (bool, error) {

	refinedPrincipal, err := p.refinePrincipal(principal)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}

	pol, err := p.lookUpPolicyByResourceName220(principal.GetResourceName())
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

// verifyTxPrincipal220
// verify transaction sender's authentication (include signature verification,
// cert-chain verification, access verification)
// move from ChainMaker/utils/transaction.go
// nolint:gocyclo
func verifyTxPrincipal220(t *commonPb.Transaction, ac acProvider220) error {
	var principal protocol.Principal
	var err error
	txBytes, err := utils.CalcUnsignedTxBytes(t)
	if err != nil {
		return fmt.Errorf("get tx bytes failed, err = %v", err)
	}

	endorsements := []*commonPb.EndorsementEntry{t.Sender}
	txType := t.Payload.TxType
	resourceId := t.Payload.ContractName + "-" + t.Payload.Method

	// sender authentication
	_, err = ac.lookUpExceptionalPolicy220(resourceId)
	if err == nil {
		principal, err = ac.CreatePrincipal(resourceId, endorsements, txBytes)
		if err != nil {
			return fmt.Errorf("fail to construct authentication principal for %s : %s", resourceId, err)
		}
	} else {
		principal, err = ac.CreatePrincipal(txType.String(), endorsements, txBytes)
		if err != nil {
			return fmt.Errorf("fail to construct authentication principal for %s : %s", txType.String(), err)
		}
	}
	ok, err := verifyPrincipal220(ac, principal)
	if err != nil {
		return fmt.Errorf("authentication error: %s", err)
	}
	if !ok {
		return fmt.Errorf("authentication failed")
	}
	// endorsers authentication for invoke_contract
	if t.Payload.TxType == commonPb.TxType_INVOKE_CONTRACT {
		// if there is payer endorsement in transaction, we just need to verify its sign
		if payer := t.Payer; payer != nil {
			payerValidEndorsement := ac.RefineEndorsements([]*commonPb.EndorsementEntry{payer}, txBytes)
			if len(payerValidEndorsement) == 0 {
				return fmt.Errorf("invalid payer endorsement")
			}
		}
		p, err := ac.lookUpPolicy220(resourceId)
		if err != nil {
			return nil
		}
		endorsements := t.Endorsers
		if endorsements == nil {
			endorsements = []*commonPb.EndorsementEntry{t.Sender}
		}

		if p.Rule == string(protocol.RuleSelf) {
			var targetOrg string
			parameterPairs := t.Payload.Parameters
			if parameterPairs != nil {
				for i := 0; i < len(parameterPairs); i++ {
					key := parameterPairs[i].Key
					if key == protocol.ConfigNameOrgId {
						targetOrg = string(parameterPairs[i].Value)
						break
					}
				}
				if targetOrg == "" {
					return fmt.Errorf("verification rule is [SELF], but org_id is not set in the parameter")
				}
				principal, err = ac.CreatePrincipalForTargetOrg(resourceId, endorsements, txBytes, targetOrg)
				if err != nil {
					return fmt.Errorf("fail to construct authentication principal with orgId %s for %s-%s: %s",
						targetOrg, t.Payload.ContractName, t.Payload.Method, err)
				}
			}
		} else {
			principal, err = ac.CreatePrincipal(resourceId, endorsements, txBytes)
			if err != nil {
				return fmt.Errorf("fail to construct authentication principal for %s-%s: %s",
					t.Payload.ContractName, t.Payload.Method, err)
			}
		}

		ok, err := verifyPrincipal220(ac, principal)
		if err != nil {
			return fmt.Errorf("authentication error for %s-%s: %s", t.Payload.ContractName, t.Payload.Method, err)
		}
		if !ok {
			return fmt.Errorf("authentication failed for %s-%s", t.Payload.ContractName, t.Payload.Method)
		}
	}
	return nil
}

func isRuleSupportedByMultiSign220(p acProvider220, resourceName string, log protocol.Logger) error {
	policy, err2 := p.lookUpPolicy220(resourceName)
	if err2 != nil {
		// not found then there is no authority which means no need to sign multi sign
		log.Warn(err2)
		return errors.New("this resource[" + resourceName + "] doesn't support to online multi sign")
	}
	if policy.Rule == string(protocol.RuleSelf) {
		return errors.New("this resource[" + resourceName + "] is the self rule and doesn't support to online multi sign")
	}
	return nil
}

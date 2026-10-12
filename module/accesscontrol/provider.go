package accesscontrol

import (
	"errors"
	"fmt"
	"sync/atomic"

	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"

	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
)

// *************************************
// 		lookUpPolicyByTxType
// *************************************

func (cp *certACProvider) lookUpPolicyByTxType(txType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByTxType(
		txType, blockVersion,
		cp.acService.latestPolicyMap, cp.acService.txTypePolicyMap)
}

func (pp *permissionedPkACProvider) lookUpPolicyByTxType(txType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByTxType(
		txType, blockVersion,
		pp.acService.latestPolicyMap, pp.acService.txTypePolicyMap)
}

func (pk *pkACProvider) lookUpPolicyByTxType(txType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByTxType(
		txType, blockVersion,
		pk.latestPolicyMap, pk.txTypePolicyMap)
}

// *************************************
// 		lookUpPolicyByTxType
// *************************************

func (cp *certACProvider) lookUpPolicyByMsgType(msgType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByMsgType(
		msgType, blockVersion,
		cp.acService.latestPolicyMap, cp.acService.msgTypePolicyMap)
}

func (pp *permissionedPkACProvider) lookUpPolicyByMsgType(msgType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByMsgType(
		msgType, blockVersion,
		pp.acService.latestPolicyMap, pp.acService.msgTypePolicyMap)
}

func (pk *pkACProvider) lookUpPolicyByMsgType(msgType string, blockVersion uint32) (*policy, error) {
	return lookUpPolicyByMsgType(
		msgType, blockVersion,
		pk.latestPolicyMap, pk.msgTypePolicyMap)
}

// *************************************
// 		findFromSenderPolicies
// *************************************

func (cp *certACProvider) findFromSenderPolicies(resourceName string, blockVersion uint32) (*policy, error) {
	return findFromSenderPolicies(
		resourceName, blockVersion,
		cp.acService.latestPolicyMap, cp.acService.senderPolicyMap)
}

func (pp *permissionedPkACProvider) findFromSenderPolicies(resourceName string, blockVersion uint32) (*policy, error) {
	return findFromSenderPolicies(
		resourceName, blockVersion,
		pp.acService.latestPolicyMap, pp.acService.senderPolicyMap)
}

func (pk *pkACProvider) findFromSenderPolicies(resourceName string, blockVersion uint32) (*policy, error) {
	return findFromSenderPolicies(
		resourceName, blockVersion,
		pk.latestPolicyMap, pk.senderPolicyMap)
}

// *************************************
// 		findFromEndorsementsPolicies
// *************************************

func (cp *certACProvider) findFromEndorsementsPolicies(resourceName string, blockVersion uint32) (*policy, error) {
	return findFromEndorsementsPolicies(
		resourceName, blockVersion,
		cp.acService.latestPolicyMap, cp.acService.resourceNamePolicyMap)
}

func (pp *permissionedPkACProvider) findFromEndorsementsPolicies(
	resourceName string, blockVersion uint32) (*policy, error) {
	return findFromEndorsementsPolicies(
		resourceName, blockVersion,
		pp.acService.latestPolicyMap, pp.acService.resourceNamePolicyMap)
}

func (pk *pkACProvider) findFromEndorsementsPolicies(resourceName string, blockVersion uint32) (*policy, error) {
	return findFromEndorsementsPolicies(
		resourceName, blockVersion,
		pk.latestPolicyMap, pk.resourceNamePolicyMap)
}

// getValidEndorsements get Valid Endorsements
func (cp *certACProvider) getValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&cp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := cp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := cp.findFromEndorsementsPolicies(principal.GetResourceName(), blockVersion)
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

func (pp *permissionedPkACProvider) getValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&pp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := pp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := pp.findFromEndorsementsPolicies(principal.GetResourceName(), blockVersion)
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

func (p *pkACProvider) getValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*commonPb.EndorsementEntry, error) {

	refinedPolicy, err := p.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("refinePrincipal fail in GetValidEndorsements: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	pol, err := p.findFromEndorsementsPolicies(principal.GetResourceName(), blockVersion)
	if err != nil {
		return nil, fmt.Errorf("lookUpPolicyByResourceName fail in GetValidEndorsements: [%v]", err)
	}
	roleListRaw := pol.GetRoleList()
	orgList := map[string]bool{}
	roleList := map[protocol.Role]bool{}
	for _, roleRaw := range roleListRaw {
		roleList[roleRaw] = true
	}
	return p.getValidEndorsementsInner(orgList, roleList, endorsements), nil
}

// ****************************************************
//  function utils
// ****************************************************

func verifyMsgTypePrincipal(p acProvider,
	principal protocol.Principal, blockVersion uint32) (allow bool, err error) {

	if p.getTotalVoterNum() <= 0 {
		return false, fmt.Errorf("authentication failed: empty organization list or trusted node list on this chain")
	}

	refinedPrincipal, err := p.refinePrincipal(principal)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}

	pol, err := p.lookUpPolicyByMsgType(principal.GetResourceName(), blockVersion)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

func verifyTxTypePrincipal(p acProvider, tx *commonPb.Transaction,
	txBytes []byte, blockVersion uint32, bypassSignVerify bool) (allow bool, err error) {

	if p.getTotalVoterNum() <= 0 {
		return false, fmt.Errorf("authentication failed: empty organization list or trusted node list on this chain")
	}

	txType := tx.Payload.TxType
	principal, err := p.CreatePrincipal(
		txType.String(),
		[]*commonPb.EndorsementEntry{tx.Sender},
		txBytes,
	)
	if err != nil {
		return false, fmt.Errorf("fail to construct authentication principal for %s : %s", txType.String(), err)
	}

	refinedPrincipal := principal
	if !bypassSignVerify {
		refinedPrincipal, err = p.refinePrincipal(principal)
		if err != nil {
			return false, fmt.Errorf("authentication failed, [%s]", err.Error())
		}
	} else {
		//cert-hash 、alias 模式时，重置memInfo
		//多签场景中tx.Sender是nil
		if tx.Sender != nil && (tx.Sender.Signer.MemberType == pbac.MemberType_CERT_HASH ||
			tx.Sender.Signer.MemberType == pbac.MemberType_ALIAS) {

			refinedPrincipal, err = p.refinePrincipalForCertOptimization(principal)
			if err != nil {
				return false, fmt.Errorf("authentication failed, [%s]", err.Error())
			}
		}
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}

	pol, err := p.lookUpPolicyByTxType(principal.GetResourceName(), blockVersion)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

func verifySenderPrincipal(p acProvider, tx *commonPb.Transaction, txBytes []byte,
	blockVersion uint32, bypassVerifySign bool, currentResourceName string) (allow bool, err error) {

	if p.getTotalVoterNum() <= 0 {
		return false, fmt.Errorf("authentication failed: empty organization list or trusted node list on this chain")
	}
	resourceName := ""
	if bypassVerifySign {
		resourceName = currentResourceName
	} else {
		resourceName = utils.GetTxResourceName(tx)
	}

	principal, err := p.CreatePrincipal(
		resourceName,
		[]*commonPb.EndorsementEntry{tx.Sender},
		txBytes,
	)
	if err != nil {
		return false, fmt.Errorf("fail to construct authentication principal for %s : %s", resourceName, err)
	}

	refinedPrincipal := principal
	if !bypassVerifySign {
		refinedPrincipal, err = p.refinePrincipal(principal)
		if err != nil {
			return false, fmt.Errorf("authentication failed, [%s]", err.Error())
		}
	} else {
		//cert-hash 、alias 模式时，重置memInfo
		//多签场景中tx.Sender是nil
		if tx.Sender != nil && (tx.Sender.Signer.MemberType == pbac.MemberType_CERT_HASH ||
			tx.Sender.Signer.MemberType == pbac.MemberType_ALIAS) {

			refinedPrincipal, err = p.refinePrincipalForCertOptimization(principal)
			if err != nil {
				return false, fmt.Errorf("authentication failed, [%s]", err.Error())
			}
		}
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}

	pol, err := p.findFromSenderPolicies(principal.GetResourceName(), blockVersion)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}
	if pol == nil {
		return true, nil
	}

	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

func verifyEndorsementsPrincipal(p acProvider, tx *commonPb.Transaction, txBytes []byte,
	blockVersion uint32, bypassVerifySign bool, currentResourceName string) (allow bool, err error) {
	resourceName := ""
	if bypassVerifySign {
		resourceName = currentResourceName
	} else {
		resourceName = utils.GetTxResourceName(tx)
	}

	return verifyEndorsementsPrincipalCommon(p, tx, txBytes, resourceName, blockVersion, bypassVerifySign)
}

func verifyMultiSignEndorsementsPrincipal(p acProvider, tx *commonPb.Transaction, resourceName string,
	blockVersion uint32, bypassVerifySign bool) (allow bool, err error) {
	txBytes, err := utils.CalcUnsignedTxBytes(tx)
	if err != nil {
		return false, err
	}
	return verifyEndorsementsPrincipalCommon(p, tx, txBytes, resourceName, blockVersion, bypassVerifySign)
}

func verifyEndorsementsPrincipalCommon(p acProvider, tx *commonPb.Transaction, txBytes []byte, resourceName string,
	blockVersion uint32, bypassVerifySign bool) (allow bool, err error) {

	if p.getTotalVoterNum() <= 0 {
		return false, fmt.Errorf("authentication failed: empty organization list or trusted node list on this chain")
	}
	// 查找 resourceName 的策略
	pol, err := p.findFromEndorsementsPolicies(resourceName, blockVersion)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}
	if pol == nil {
		return true, nil
	}
	// 构建 endorsements
	endorsements := tx.Endorsers
	if endorsements == nil {
		endorsements = []*commonPb.EndorsementEntry{tx.Sender}
	} else {
		if tx.Sender != nil {
			endorsements = append(endorsements, tx.Sender)
		}
	}
	var principal protocol.Principal
	if pol.rule == protocol.RuleSelf {
		var targetOrg string
		parameterPairs := tx.Payload.Parameters
		if parameterPairs != nil {
			for i := 0; i < len(parameterPairs); i++ {
				key := parameterPairs[i].Key
				if key == protocol.ConfigNameOrgId {
					targetOrg = string(parameterPairs[i].Value)
					break
				}
			}
		}
		if targetOrg == "" {
			return false, fmt.Errorf("verification rule is [SELF], but org_id is not set in the parameter")
		}
		principal, err = p.CreatePrincipalForTargetOrg(resourceName, endorsements, txBytes, targetOrg)
	} else {
		principal, err = p.CreatePrincipal(resourceName, endorsements, txBytes)
	}
	if err != nil {
		return false, fmt.Errorf("fail to construct authentication principal for %s: %s",
			resourceName, err)
	}
	refinedPrincipal := principal
	if !bypassVerifySign {
		refinedPrincipal, err = p.refinePrincipal(principal)
		if err != nil {
			return false, fmt.Errorf("authentication failed, [%s]", err.Error())
		}
	} else {
		//cert-hash 、alias 模式时，重置memInfo
		//多签场景中tx.Sender是nil
		if tx.Sender != nil && (tx.Sender.Signer.MemberType == pbac.MemberType_CERT_HASH ||
			tx.Sender.Signer.MemberType == pbac.MemberType_ALIAS) {

			refinedPrincipal, err = p.refinePrincipalForCertOptimization(principal)
			if err != nil {
				return false, fmt.Errorf("authentication failed, [%s]", err.Error())
			}
		}
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}
	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)

}

func verifyTxPrincipal(tx *commonPb.Transaction, resourceId string,
	p acProvider, blockVersion uint32) (bool, error) {
	var err error
	var allow bool
	var crossCall bool

	txBytes := []byte("")
	txType := tx.Payload.TxType
	txResourceId := utils.GetTxResourceName(tx)
	crossCall = false
	//resourceId 被调用合约资源序号
	//txResourceId 用户发送原始交易中资源序号
	if txResourceId != resourceId {
		crossCall = true
	} else {
		txBytes, err = utils.CalcUnsignedTxBytes(tx)
		if err != nil {
			return false, fmt.Errorf("get tx bytes failed, err = %v", err)
		}
	}

	// check tx_type
	allow, err = verifyTxTypePrincipal(p, tx, txBytes, blockVersion, crossCall)
	if err != nil {
		return false, fmt.Errorf("[verifyTxTypePrincipal]authentication error: %s", err)
	}
	if !allow {
		return false, fmt.Errorf("[verifyEndorsementsPrincipal]authentication failed")
	}

	if txType != commonPb.TxType_INVOKE_CONTRACT {
		return true, nil
	}

	// check sender: because sender has been verified by tx_type checking
	allow, err = verifySenderPrincipal(p, tx, txBytes, blockVersion, true, resourceId)
	if err != nil {
		return false, fmt.Errorf("[verifySenderPrincipal]authentication error: %s", err)
	}
	if !allow {
		return false, fmt.Errorf("[verifyEndorsementsPrincipal]authentication failed")
	}

	// check endorsements
	allow, err = verifyEndorsementsPrincipal(p, tx, txBytes, blockVersion, crossCall, resourceId)
	if err != nil {
		return false, fmt.Errorf("[verifyEndorsementsPrincipal]authentication error for %s: %s", resourceId, err)
	}
	if !allow {
		return false, fmt.Errorf("[verifyEndorsementsPrincipal]authentication failed for %s", resourceId)
	}

	return true, nil
}

func isRuleSupportedByMultiSign(
	p acProvider, resourceName string, blockVersion uint32, log protocol.Logger) error {
	policy, err := p.findFromEndorsementsPolicies(resourceName, blockVersion)
	if err != nil {
		err = fmt.Errorf("find endorsement policy for resource[%s] failed, err = %v", resourceName, err)
		log.Warn(err)
		return err
	}
	if policy == nil {
		// not found then there is no authority which means no need to sign multi sign
		err = fmt.Errorf("this resource[%s] doesn't support to online multi sign", resourceName)
		log.Warn(err)
		return err
	}
	if policy.GetRule() == protocol.RuleSelf {
		return errors.New("this resource[" + resourceName + "] is the self rule and doesn't support to online multi sign")
	}
	return nil
}

func isMultiSignPassed(p acProvider, tx *commonPb.Transaction, resourceName string,
	policy *policy, blockVersion uint32) (bool, error) {

	return verifyMultiSignEndorsementsPrincipal(p, tx, resourceName, blockVersion, true)
}

func isMultiSignRefused(p acProvider, tx *commonPb.Transaction, resourceName string,
	pol *policy, blockVersion uint32) (bool, error) {

	refusedPolicy := &policy{
		orgList:  pol.orgList,
		roleList: pol.roleList,
	}
	switch pol.GetRule() {
	case protocol.RuleForbidden:
		return true, fmt.Errorf("policy of multi-sign tx should not be `%v`", protocol.RuleForbidden)
	case protocol.RuleSelf:
		return true, fmt.Errorf("policy of multi-sign tx should not be `%v`", protocol.RuleSelf)
	case protocol.RuleAny:
		refusedPolicy.rule = protocol.RuleAll
	case protocol.RuleAll:
		refusedPolicy.rule = protocol.RuleAny
	case protocol.RuleMajority:
		refusedPolicy.rule = protocol.RuleMajority
	}

	refused, err := verifyMultiSignEndorsementsPrincipal(p, tx, resourceName, blockVersion, true)
	if refused {
		return true, err
	}

	if pol.GetRule() == protocol.RuleMajority {
		if 2*len(tx.Endorsers) == p.getTotalVoterNum() {
			return true, nil
		}
	}

	return false, nil
}

func verifyMultiSignTxPrincipal(p acProvider, mInfo *syscontract.MultiSignInfo,
	blockVersion uint32, log protocol.Logger) (syscontract.MultiSignStatus, error) {

	if mInfo.Status != syscontract.MultiSignStatus_PROCESSING {
		return mInfo.Status, fmt.Errorf("multi-sign status `%v` is not permitted to verify", mInfo.Status)
	}

	resourceName := mInfo.ContractName + "-" + mInfo.Method
	var agreeEndorsements []*commonPb.EndorsementEntry
	var rejectEndorsements []*commonPb.EndorsementEntry
	for _, voteInfo := range mInfo.VoteInfos {
		if voteInfo.Vote == syscontract.VoteStatus_AGREE {
			agreeEndorsements = append(agreeEndorsements, voteInfo.Endorsement)
		} else if voteInfo.Vote == syscontract.VoteStatus_REJECT {
			rejectEndorsements = append(rejectEndorsements, voteInfo.Endorsement)
		} else {
			log.Warnf("unknown vote action, voteInfo.Vote = %v", voteInfo.Vote)
		}
	}
	log.Debugf("multiSignInfo => %v", mInfo)
	log.Debugf("endorsers agreed num => %v", len(agreeEndorsements))
	log.Debugf("endorsers rejected num => %v", len(rejectEndorsements))

	policy, err := p.findFromEndorsementsPolicies(resourceName, blockVersion)
	if err != nil {
		return mInfo.Status, err
	}
	if policy == nil {
		return mInfo.Status, nil
	}
	// 根据 agree 的数量判断多签状态
	if len(agreeEndorsements) > 0 {
		tx := commonPb.Transaction{
			Payload:   mInfo.Payload,
			Endorsers: agreeEndorsements,
		}
		agree, err := isMultiSignPassed(p, &tx, resourceName, policy, blockVersion)
		if err != nil {
			log.Infof("isMultiSignPassed(...) return error, err = %v", err)
		}
		if agree {
			mInfo.Status = syscontract.MultiSignStatus_PASSED
			return mInfo.Status, nil
		}

	}
	// 根据 agree 的数量判断多签状态
	if len(rejectEndorsements) > 0 {
		tx := commonPb.Transaction{
			Payload:   mInfo.Payload,
			Endorsers: rejectEndorsements,
		}
		// 根据 reject 的数量判断多签状态
		refuse, err := isMultiSignRefused(p, &tx, resourceName, policy, blockVersion)
		if refuse {
			mInfo.Status = syscontract.MultiSignStatus_REFUSED
			return mInfo.Status, nil
		}
		if err != nil {
			log.Infof("isMultiSignRefused(...) return error, err = %v", err)
		}
	}

	return mInfo.Status, nil
}

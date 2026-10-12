package accesscontrol

import (
	"errors"
	"fmt"
	"sync/atomic"

	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/localconf/v2"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// ****************************************************
//  Cert Mode
// ****************************************************

func (cp *certACProvider) lookUpPolicy2320(resourceName string) (*acPb.Policy, error) {
	return cp.acService.lookUpPolicy2320(resourceName)
}

func (cp *certACProvider) lookUpExceptionalPolicy2320(resourceName string) (*acPb.Policy, error) {
	return cp.acService.lookUpExceptionalPolicy2320(resourceName)
}

func (cp *certACProvider) lookUpPolicyByResourceName2320(resourceName string) (*policy, error) {
	return cp.acService.lookUpPolicyByResourceName2320(resourceName)
}

func (cp *certACProvider) getValidEndorsements2320(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&cp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := cp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := cp.lookUpPolicyByResourceName2320(principal.GetResourceName())
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
//  Permitted public-key Mode
// ****************************************************

func (pp *permissionedPkACProvider) lookUpPolicy2320(resourceName string) (*acPb.Policy, error) {
	return pp.acService.lookUpPolicy2320(resourceName)
}

func (pp *permissionedPkACProvider) lookUpExceptionalPolicy2320(resourceName string) (*acPb.Policy, error) {
	return pp.acService.lookUpExceptionalPolicy2320(resourceName)
}

func (pp *permissionedPkACProvider) lookUpPolicyByResourceName2320(resourceName string) (*policy, error) {
	return pp.acService.lookUpPolicyByResourceName2320(resourceName)
}

func (pp *permissionedPkACProvider) getValidEndorsements2320(
	principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	if atomic.LoadInt32(&pp.acService.orgNum) <= 0 {
		return nil, fmt.Errorf("authentication fail: empty organization list or trusted node list on this chain")
	}
	refinedPolicy, err := pp.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("authentication fail, not a member on this chain: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	p, err := pp.lookUpPolicyByResourceName2320(principal.GetResourceName())
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
//  public-key Mode
// ****************************************************

// lookUpPolicy2320 returns corresponding policy configured for the given resource name
func (p *pkACProvider) lookUpPolicy2320(resourceName string) (*acPb.Policy, error) {

	if p, ok := p.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}

	pol, ok := p.resourceNamePolicyMap2320.Load(resourceName)
	if !ok {
		return nil, fmt.Errorf("policy not found for resource %s", resourceName)
	}
	pbPolicy := pol.(*policy).GetPbPolicy()
	return pbPolicy, nil
}

// lookUpExceptionalPolicy2320 returns corresponding exceptional policy configured for the given resource name
func (p *pkACProvider) lookUpExceptionalPolicy2320(resourceName string) (*acPb.Policy, error) {

	if p, ok := p.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}

	pol, ok := p.exceptionalPolicyMap2320.Load(resourceName)
	if !ok {
		return nil, fmt.Errorf("exceptional policy not found for resource %s", resourceName)
	}
	pbPolicy := pol.(*policy).GetPbPolicy()
	return pbPolicy, nil
}

func (p *pkACProvider) lookUpPolicyByResourceName2320(resourceName string) (*policy, error) {

	if p, ok := p.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy), nil
	}
	pol, ok := p.resourceNamePolicyMap2320.Load(resourceName)
	if !ok {
		if pol, ok = p.exceptionalPolicyMap2320.Load(resourceName); !ok {
			return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
				"for resource %s", resourceName)
		}
	}
	return pol.(*policy), nil
}

func (p *pkACProvider) getValidEndorsements2320(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error) {

	refinedPolicy, err := p.refinePrincipal(principal)
	if err != nil {
		return nil, fmt.Errorf("refinePrincipal fail in GetValidEndorsements: [%v]", err)
	}
	endorsements := refinedPolicy.GetEndorsement()

	pol, err := p.lookUpPolicyByResourceName2320(principal.GetResourceName())
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

// lookUpPolicy2320 loop up policy with version v2.3.2
func (acs *accessControlService) lookUpPolicy2320(resourceName string) (*acPb.Policy, error) {

	if p, ok := acs.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}
	if p, ok := acs.resourceNamePolicyMap2320.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}
	return nil, fmt.Errorf("policy not found for resource %s", resourceName)
}

func (acs *accessControlService) lookUpExceptionalPolicy2320(resourceName string) (*acPb.Policy, error) {

	if p, ok := acs.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}
	if p, ok := acs.exceptionalPolicyMap2320.Load(resourceName); ok {
		return p.(*policy).GetPbPolicy(), nil

	}
	return nil, fmt.Errorf("exceptional policy not found for resource %s", resourceName)
}

func (acs *accessControlService) lookUpPolicyByResourceName2320(resourceName string) (*policy, error) {

	if p, ok := acs.latestPolicyMap.Load(resourceName); ok {
		return p.(*policy), nil
	}
	pol, ok := acs.resourceNamePolicyMap2320.Load(resourceName)
	if !ok {
		if pol, ok = acs.exceptionalPolicyMap2320.Load(resourceName); !ok {
			return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
				"for resource %s", resourceName)
		}
	}
	return pol.(*policy), nil
}

// **********************************************
// 	function utils
// **********************************************

func verifyPrincipal2320(p acProvider2320, principal protocol.Principal) (bool, error) {

	refinedPrincipal, err := p.refinePrincipal(principal)
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	if localconf.ChainMakerConfig.DebugConfig.IsSkipAccessControl {
		return true, nil
	}

	pol, err := p.lookUpPolicyByResourceName2320(principal.GetResourceName())
	if err != nil {
		return false, fmt.Errorf("authentication failed, [%s]", err.Error())
	}

	return p.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

// verifyTxPrincipal2320
// verify transaction sender's authentication (include signature verification,
// cert-chain verification, access verification)
// move from ChainMaker/utils/transaction.go
// nolint:gocyclo
func verifyTxPrincipal2320(t *commonPb.Transaction, resourceId string, ac acProvider2320) error {
	var principal protocol.Principal
	var err error
	txBytes, err := utils.CalcUnsignedTxBytes(t)
	if err != nil {
		return fmt.Errorf("get tx bytes failed, err = %v", err)
	}

	endorsements := t.Endorsers
	if endorsements == nil {
		endorsements = []*commonPb.EndorsementEntry{t.Sender}
	} else {
		if t.Sender != nil {
			endorsements = append(endorsements, t.Sender)
		}
	}
	txType := t.Payload.TxType

	// sender authentication
	_, err = ac.lookUpExceptionalPolicy2320(resourceId)
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
	ok, err := verifyPrincipal2320(ac, principal)
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
		p, err := ac.lookUpPolicy2320(resourceId)
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

		ok, err := verifyPrincipal2320(ac, principal)
		if err != nil {
			return fmt.Errorf("authentication error for %s-%s: %s", t.Payload.ContractName, t.Payload.Method, err)
		}
		if !ok {
			return fmt.Errorf("authentication failed for %s-%s", t.Payload.ContractName, t.Payload.Method)
		}
	}
	return nil
}

func isRuleSupportedByMultiSign2320(resourceName string, p acProvider2320, log protocol.Logger) error {

	policy, err2 := p.lookUpPolicy2320(resourceName)
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

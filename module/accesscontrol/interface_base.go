package accesscontrol

import (
	"fmt"
	"sync/atomic"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

type acProviderBase interface {
	getTotalVoterNum() int
	verifyPrincipalPolicy(principal, refinedPrincipal protocol.Principal, pol *policy) (bool, error)
	refinePrincipal(principal protocol.Principal) (protocol.Principal, error)
	refinePrincipalForCertOptimization(principal protocol.Principal) (protocol.Principal, error)

	CreatePrincipal(resourceName string,
		endorsements []*commonPb.EndorsementEntry, message []byte) (protocol.Principal, error)
	CreatePrincipalForTargetOrg(resourceName string,
		endorsements []*commonPb.EndorsementEntry, message []byte, targetOrgId string) (protocol.Principal, error)
	RefineEndorsements(endorsements []*commonPb.EndorsementEntry, msg []byte) []*commonPb.EndorsementEntry
}

// **********************************************
// 			cert mode
// **********************************************

func (cp *certACProvider) getTotalVoterNum() int {
	return int(atomic.LoadInt32(&cp.acService.orgNum))
}

func (cp *certACProvider) verifyPrincipalPolicy(
	principal, refinedPrincipal protocol.Principal, pol *policy) (bool, error) {

	return cp.acService.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

// CreatePrincipalForTargetOrg creates a principal for "SELF" type principal,
// which needs to convert SELF to a specific organization id in one authentication
func (cp *certACProvider) CreatePrincipalForTargetOrg(resourceName string,
	endorsements []*commonPb.EndorsementEntry, message []byte,
	targetOrgId string) (protocol.Principal, error) {
	return cp.acService.createPrincipalForTargetOrg(resourceName, endorsements, message, targetOrgId)
}

// CreatePrincipal creates a principal for one time authentication
func (cp *certACProvider) CreatePrincipal(resourceName string, endorsements []*commonPb.EndorsementEntry,
	message []byte) (
	protocol.Principal, error) {
	return cp.acService.createPrincipal(resourceName, endorsements, message)
}

// **********************************************
// 		Permitted public-key mode
// **********************************************

func (pp *permissionedPkACProvider) verifyPrincipalPolicy(
	principal, refinedPrincipal protocol.Principal, pol *policy) (bool, error) {

	return pp.acService.verifyPrincipalPolicy(principal, refinedPrincipal, pol)
}

func (pp *permissionedPkACProvider) getTotalVoterNum() int {
	return int(atomic.LoadInt32(&pp.acService.orgNum))
}

// CreatePrincipalForTargetOrg creates a principal for "SELF" type principal,
// which needs to convert SELF to a sepecific organization id in one authentication
func (pp *permissionedPkACProvider) CreatePrincipalForTargetOrg(resourceName string,
	endorsements []*commonPb.EndorsementEntry, message []byte,
	targetOrgId string) (protocol.Principal, error) {
	return pp.acService.createPrincipalForTargetOrg(resourceName, endorsements, message, targetOrgId)
}

// CreatePrincipal creates a principal for one time authentication
func (pp *permissionedPkACProvider) CreatePrincipal(resourceName string, endorsements []*commonPb.EndorsementEntry,
	message []byte) (
	protocol.Principal, error) {
	return pp.acService.createPrincipal(resourceName, endorsements, message)
}

// **********************************************
// 		public-key mode
// **********************************************

func (pk *pkACProvider) verifyPrincipalPolicy(principal,
	refinedPrincipal protocol.Principal, pol *policy) (bool, error) {
	endorsements := refinedPrincipal.GetEndorsement()
	rule := pol.GetRule()
	switch rule {
	case protocol.RuleForbidden:
		return false, fmt.Errorf("public authentication fail: [%s] is forbidden to access",
			refinedPrincipal.GetResourceName())
	case protocol.RuleAny:
		return pk.verifyRuleAnyCase(pol, endorsements)
	case protocol.RuleAll:
		return pk.verifyRuleAllCase(pol, endorsements)
	case protocol.RuleMajority:
		return pk.verifyRuleMajorityCase(pol, endorsements)
	default:
		return pk.verifyRuleDefaultCase(pol, endorsements)
	}
}

func (pk *pkACProvider) getTotalVoterNum() int {
	return int(atomic.LoadInt32(&pk.adminNum))
}

// CreatePrincipal creates a principal for one time authentication
func (p *pkACProvider) CreatePrincipal(resourceName string, endorsements []*commonPb.EndorsementEntry,
	message []byte) (protocol.Principal, error) {

	if len(endorsements) == 0 || message == nil {
		return nil, fmt.Errorf("setup access control principal failed, a principal should contain valid (non-empty)" +
			" signer information, signature, and message")
	}
	if endorsements[0] == nil {
		return nil, fmt.Errorf("setup access control principal failed, signer-signature pair should not be nil")
	}
	return &principal{
		resourceName: resourceName,
		endorsement:  endorsements,
		message:      message,
		targetOrg:    "",
	}, nil
}

func (p *pkACProvider) CreatePrincipalForTargetOrg(resourceName string,
	endorsements []*commonPb.EndorsementEntry, message []byte, targetOrgId string) (protocol.Principal, error) {

	return nil, fmt.Errorf("setup access control principal failed, CreatePrincipalForTargetOrg is not supported")
}

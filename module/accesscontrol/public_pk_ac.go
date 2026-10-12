/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/common/v2/msgbus"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/consensus"

	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
)

var _ protocol.AccessControlProvider = (*pkACProvider)(nil)

var nilPkACProvider ACProvider = (*pkACProvider)(nil)

const (
	//AdminPublicKey admin trust orgId
	AdminPublicKey = "public"
	//DposOrgId chainconfig the DPoS of orgId
	DposOrgId = "dpos_org_id"

	//PermissionConsensusOrgId chainconfig orgId for permission consensus, such as tbft
	PermissionConsensusOrgId = "public"
)

// List of access principals which should not be customized
var restrainedResourceListPK = map[string]bool{
	protocol.ResourceNameAllTest:       true,
	protocol.ResourceNameP2p:           true,
	protocol.ResourceNameConsensusNode: true,

	common.TxType_QUERY_CONTRACT.String():  true,
	common.TxType_INVOKE_CONTRACT.String(): true,
	common.TxType_SUBSCRIBE.String():       true,
	common.TxType_ARCHIVE.String():         true,

	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERT_ALIAS_ADD.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERT_ALIAS_UPDATE.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_ALIAS_QUERY.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_QUERY.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERT_ADD.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_FREEZE.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_UNFREEZE.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_DELETE.String(): true,
	syscontract.SystemContract_CERT_MANAGE.String() + "-" +
		syscontract.CertManageFunction_CERTS_REVOKE.String(): true,
	syscontract.SystemContract_CHAIN_CONFIG.String() + "-" +
		syscontract.ChainConfigFunction_ALTER_ADDR_TYPE.String(): true,
}

var (
	pubPolicyConsensus = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleConsensusNode,
		},
	)
	// 表示orgList中的任意一个管理员即可，orgList为空表示所有组织
	pubPolicyManage = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)
	pubPolicyMajorityAdmin = newPolicy(
		protocol.RuleMajority,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)
	pubPolicyTransaction = newPolicy(
		protocol.RuleAny,
		nil,
		nil,
	)
	pubPolicyForbidden = newPolicy(
		protocol.RuleForbidden,
		nil,
		nil,
	)
)

type pkACProvider struct {

	//chainconfig authType
	authType string

	hashType string

	addressType config.AddrType

	adminNum int32

	log protocol.Logger

	adminMember *sync.Map

	consensusMember *sync.Map

	// used to cache the deduction account address to avoid reading the database every time
	payerList *ShardCache

	memberCache *ShardCache

	dataStore protocol.BlockchainStore

	txTypePolicyMap           *sync.Map
	msgTypePolicyMap          *sync.Map
	senderPolicyMap           *sync.Map
	resourceNamePolicyMap     *sync.Map
	resourceNamePolicyMap220  *sync.Map
	exceptionalPolicyMap220   *sync.Map
	resourceNamePolicyMap2320 *sync.Map
	exceptionalPolicyMap2320  *sync.Map
	latestPolicyMap           *sync.Map // map[string]*policy , resourceName -> *policy
}

type publicAdminMemberModel struct {
	publicKey crypto.PublicKey
	pkBytes   []byte
}

func (p *pkACProvider) NewACProvider(chainConf protocol.ChainConf, localOrgId string,
	store protocol.BlockchainStore, log protocol.Logger, msgBus msgbus.MessageBus) (
	protocol.AccessControlProvider, error) {
	pkAcProvider, err := newPkACProvider(chainConf.ChainConfig(), store, log)
	if err != nil {
		return nil, err
	}

	msgBus.Register(msgbus.ChainConfig, pkAcProvider)
	msgBus.Register(msgbus.BlockInfo, pkAcProvider)
	//v220_compat Deprecated
	chainConf.AddWatch(pkAcProvider) //nolint: staticcheck
	return pkAcProvider, nil
}

// Start the ac service.
func (cp *pkACProvider) Start() error {
	return nil
}

// Stop the ac service.
func (cp *pkACProvider) Stop() error {
	return nil
}

func newPkACProvider(chainConfig *config.ChainConfig,
	store protocol.BlockchainStore, log protocol.Logger) (*pkACProvider, error) {
	pkAcProvider := &pkACProvider{
		adminNum:                  0,
		hashType:                  chainConfig.Crypto.Hash,
		authType:                  chainConfig.AuthType,
		addressType:               chainConfig.Vm.AddrType,
		adminMember:               &sync.Map{},
		consensusMember:           &sync.Map{},
		memberCache:               NewShardCache(GetCertCacheSize()),
		log:                       log,
		dataStore:                 store,
		txTypePolicyMap:           &sync.Map{},
		msgTypePolicyMap:          &sync.Map{},
		senderPolicyMap:           &sync.Map{},
		resourceNamePolicyMap:     &sync.Map{},
		resourceNamePolicyMap220:  &sync.Map{},
		exceptionalPolicyMap220:   &sync.Map{},
		resourceNamePolicyMap2320: &sync.Map{},
		exceptionalPolicyMap2320:  &sync.Map{},
		latestPolicyMap:           &sync.Map{},
		payerList:                 NewShardCache(GetCertCacheSize()),
	}

	if chainConfig.Consensus.Type == consensus.ConsensusType_DPOS {

		pkAcProvider.createDefaultResourcePolicyForDPoS_220()
		pkAcProvider.createDefaultResourcePolicyForDPoS_2320()
		pkAcProvider.createDefaultResourcePolicyForPKDPoS()
	} else {
		pkAcProvider.createDefaultResourcePolicyForCommon_220()
		pkAcProvider.createDefaultResourcePolicyForCommon_2320()
		pkAcProvider.createDefaultResourcePolicyForPK()
	}

	lastestPolicyMap := &sync.Map{}
	for _, resourcePolicy := range chainConfig.ResourcePolicies {
		if pkAcProvider.ValidateResourcePolicy(resourcePolicy) {
			policy := newPolicyFromPb(resourcePolicy.Policy)
			lastestPolicyMap.Store(resourcePolicy.ResourceName, policy)
		}
	}
	pkAcProvider.latestPolicyMap = lastestPolicyMap

	err := pkAcProvider.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		return nil, fmt.Errorf("new public AC provider failed: %s", err.Error())
	}
	err = pkAcProvider.initConsensusMember(chainConfig)
	if err != nil {
		return nil, fmt.Errorf("new public AC provider failed: %s", err.Error())
	}
	return pkAcProvider, nil
}

func (p *pkACProvider) initResourcePolicy(resourcePolicies []*config.ResourcePolicy) {
	lastestPolicyMap := &sync.Map{}
	for _, resourcePolicy := range resourcePolicies {
		if p.ValidateResourcePolicy(resourcePolicy) {
			policy := newPolicyFromPb(resourcePolicy.Policy)
			lastestPolicyMap.Store(resourcePolicy.ResourceName, policy)
		}
	}
	p.latestPolicyMap = lastestPolicyMap
}

func (p *pkACProvider) initAdminMembers(trustRootList []*config.TrustRootConfig) error {
	var (
		tempSyncMap sync.Map
	)

	if len(trustRootList) == 0 {
		p.log.Debugf("no super administrator is configured")
		return nil
	}

	var adminNum int32

	for _, trustRoot := range trustRootList {
		if strings.ToLower(trustRoot.OrgId) == AdminPublicKey {
			for _, root := range trustRoot.Root {
				pk, err := asym.PublicKeyFromPEM([]byte(root))
				if err != nil {
					return fmt.Errorf("init admin member failed: parse the public key from PEM failed")
				}
				pkBytes, err := pk.Bytes()
				if err != nil {
					return fmt.Errorf("init admin member failed: %s", err.Error())
				}
				adminMember := &publicAdminMemberModel{
					publicKey: pk,
					pkBytes:   pkBytes,
				}
				adminKey := hex.EncodeToString(pkBytes)
				tempSyncMap.Store(adminKey, adminMember)
				adminNum++
			}
		}
	}
	p.adminMember = &tempSyncMap
	atomic.StoreInt32(&p.adminNum, adminNum)
	return nil
}

func (p *pkACProvider) initConsensusMember(chainConfig *config.ChainConfig) error {
	if chainConfig.Consensus.Type == consensus.ConsensusType_DPOS {
		return p.initDPoSMember(chainConfig.Consensus.Nodes)
	} else if chainConfig.Consensus.Type == consensus.ConsensusType_TBFT {
		return p.initPermissionMember(chainConfig.Consensus.Nodes)
	}
	return fmt.Errorf("public chain mode does not support other consensus")
}

func (p *pkACProvider) initDPoSMember(consensusConf []*config.OrgConfig) error {
	if len(consensusConf) == 0 {
		return fmt.Errorf("update dpos consensus member failed: DPoS config can't be empty in chain config")
	}

	var consensusMember sync.Map
	if consensusConf[0].OrgId != DposOrgId {
		return fmt.Errorf("update dpos consensus member failed: DPoS node config orgId do not match")
	}
	for _, nodeId := range consensusConf[0].NodeId {
		consensusMember.Store(nodeId, struct{}{})
	}
	p.consensusMember = &consensusMember
	p.log.Infof("update consensus list: [%v]", p.consensusMember)
	return nil
}

func (p *pkACProvider) initPermissionMember(consensusConf []*config.OrgConfig) error {
	if len(consensusConf) == 0 {
		return fmt.Errorf("update permission consensus member failed: consensus node config can't be empty in chain config")
	}

	var consensusMember sync.Map
	if consensusConf[0].OrgId != PermissionConsensusOrgId {
		return fmt.Errorf("update permission consensus member failed: node config orgId do not match")
	}
	for _, nodeId := range consensusConf[0].NodeId {
		consensusMember.Store(nodeId, struct{}{})
	}
	p.consensusMember = &consensusMember
	p.log.Infof("update permission consensus list: [%v]", p.consensusMember)
	return nil
}

func (p *pkACProvider) lookUpMemberInCache(memberInfo string) (*memberCached, bool) {
	ret, ok := p.memberCache.Get(memberInfo)
	if ok {
		return ret.(*memberCached), true
	}
	return nil, false
}

func (p *pkACProvider) getMemberFromCache(member *pbac.Member) protocol.Member {
	cached, ok := p.lookUpMemberInCache(string(member.MemberInfo))
	if ok {
		p.log.Debugf("member found in local cache")
		return cached.member
	}
	// handle false positive when member cache is cleared
	if p.authType == protocol.Public {
		tmpMember, err := p.NewMemberFromAcs(member)
		if err != nil {
			p.log.Debugf("new member failed, authType = %s, err = %s", p.authType, err.Error())
			return nil
		}
		p.addMemberToCache(member, &memberCached{
			member:    tmpMember,
			certChain: nil,
		})
		return tmpMember
	}
	return nil
}

//func (p *pkACProvider) Module() string {
//	return ModuleNameAccessControl
//}
//
//
//func (p *pkACProvider) Watch(chainConfig *config.ChainConfig) error {
//
//	p.hashType = chainConfig.GetCrypto().GetHash()
//	err := p.initAdminMembers(chainConfig.TrustRoots)
//	if err != nil {
//		return fmt.Errorf("new public AC provider failed: %s", err.Error())
//	}
//
//	err = p.initConsensusMember(chainConfig)
//	if err != nil {
//		return fmt.Errorf("new public AC provider failed: %s", err.Error())
//	}
//	p.memberCache.Clear()
//	return nil
//}

// NewMember creates a member from pb Member
func (p *pkACProvider) NewMember(pbMember *pbac.Member) (protocol.Member, error) {
	cache := p.getMemberFromCache(pbMember)
	if cache != nil {
		return cache, nil
	}
	member, err := publicNewPkMemberFromAcs(pbMember, p.adminMember, p.consensusMember, p.hashType)
	if err != nil {
		return nil, fmt.Errorf("new member failed: %s", err.Error())
	}
	p.addMemberToCache(pbMember, &memberCached{
		member:    member,
		certChain: nil,
	})
	return member, nil
}

func (p *pkACProvider) memberToAddress(member *pbac.Member) (string, error) {
	//计算地址
	var err error
	var pk []byte
	var publicKey crypto.PublicKey
	switch member.MemberType {
	case pbac.MemberType_PUBLIC_KEY:
		pk = member.MemberInfo
		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("error member.MemberType=%s", member.MemberType)
	}
	publicKeyString, err := utils.PkToAddrStr(publicKey, p.addressType, crypto.HashAlgoMap[p.hashType])
	if err != nil {
		return "", err
	}

	if p.addressType == configPb.AddrType_ZXL {
		publicKeyString = "ZX" + publicKeyString
	}
	return publicKeyString, nil
}

func (p *pkACProvider) addMemberToCache(member *pbac.Member, memberCached *memberCached) {

	address, err := p.memberToAddress(member)
	if err != nil {
		p.log.Errorf("add member to cache failed, err = %s", err.Error())
		return
	}

	memberCached.address = address
	memberCached.pk = memberCached.member.GetPk()

	p.memberCache.Add(string(member.MemberInfo), memberCached)
}

// NewMember creates a member from pb Member
func (p *pkACProvider) NewMemberFromAcs(pbMember *pbac.Member) (protocol.Member, error) {
	member, err := publicNewPkMemberFromAcs(pbMember, p.adminMember, p.consensusMember, p.hashType)
	if err != nil {
		return nil, fmt.Errorf("new member failed: %s", err.Error())
	}
	return member, nil
}

func (p *pkACProvider) verifyRuleAnyCase(pol *policy, endorsements []*common.EndorsementEntry) (bool, error) {
	roleList := p.buildRoleListForVerifyPrincipal(pol)
	for _, endorsement := range endorsements {
		if len(roleList) == 0 {
			return true, nil
		}
		member := p.getMemberFromCache(endorsement.Signer)
		if member == nil {
			p.log.Infof(
				"authentication warning: the member is not in member cache, memberInfo[%s]",
				string(endorsement.Signer.MemberInfo))
			continue
		}
		//In PK mode, the client obtains an empty string through getrole()
		role := member.GetRole()
		if role == "" {
			role = protocol.RoleClient
		}
		if _, ok := roleList[role]; ok {
			return true, nil
		}
		p.log.Infof("authentication warning, the member role is not in roleList, role: [%s]",
			member.GetRole())
	}

	err := fmt.Errorf("authentication fail for any rule, policy: rule: [%v],roleList: [%v]",
		pol.rule, pol.roleList)
	return false, err
}

func (p *pkACProvider) verifyRuleAllCase(pol *policy, endorsements []*common.EndorsementEntry) (bool, error) {
	role := protocol.RoleAdmin
	refinedEndorsements := p.getValidEndorsementsInner(
		map[string]bool{}, map[protocol.Role]bool{role: true}, endorsements)
	numOfValid := len(refinedEndorsements)
	p.log.Debugf("verifyRuleMajorityAdminCase: numOfValid=[%d], p.adminNum=[%d]", numOfValid, p.adminNum)
	if numOfValid >= int(p.adminNum) {
		return true, nil
	}
	return false, fmt.Errorf("%s: %d valid endorsements required, %d valid endorsements received",
		notEnoughParticipantsSupportError, p.adminNum, numOfValid)

}

func (p *pkACProvider) verifyRuleMajorityCase(pol *policy, endorsements []*common.EndorsementEntry) (bool, error) {
	role := protocol.RoleAdmin
	refinedEndorsements := p.getValidEndorsementsInner(
		map[string]bool{}, map[protocol.Role]bool{role: true}, endorsements)
	numOfValid := len(refinedEndorsements)
	p.log.Debugf("verifyRuleMajorityAdminCase: numOfValid=[%d], p.adminNum=[%d]", numOfValid, p.adminNum)
	if float64(numOfValid) > float64(p.adminNum)/2.0 {
		return true, nil
	}
	return false, fmt.Errorf("%s: %d valid endorsements required, %d valid endorsements received",
		notEnoughParticipantsSupportError, int(float64(p.adminNum)/2.0+1), numOfValid)
}

func (p *pkACProvider) verifyRuleDefaultCase(pol *policy, endorsements []*common.EndorsementEntry) (bool, error) {
	rule := pol.GetRule()
	nums := strings.Split(string(rule), LIMIT_DELIMITER)

	refinedEndorsements := p.getValidEndorsementsInner(
		map[string]bool{}, map[protocol.Role]bool{protocol.RoleAdmin: true}, endorsements)
	numOfValid := len(refinedEndorsements)

	switch len(nums) {
	case 1:
		threshold, err := strconv.Atoi(nums[0])
		if err != nil {
			return false, fmt.Errorf("authentication fail: unrecognized rule, should be ANY, MAJORITY, ALL, " +
				"SELF, ac threshold (integer), or ac portion (fraction)")
		}

		if numOfValid >= threshold {
			return true, nil
		}
		return false, fmt.Errorf("%s: %d valid endorsements required, %d valid endorsements received",
			notEnoughParticipantsSupportError, threshold, numOfValid)

	case 2:
		numerator, err := strconv.Atoi(nums[0])
		denominator, err2 := strconv.Atoi(nums[1])
		if err != nil || err2 != nil {
			return false, fmt.Errorf("authentication fail: unrecognized rule, should be ANY, MAJORITY, ALL, " +
				"SELF, an integer, or ac fraction")
		}

		if denominator <= 0 {
			denominator = int(p.adminNum)
		}

		var numRequired float64
		numRequired = float64(p.adminNum) * float64(numerator) / float64(denominator)

		if float64(numOfValid) >= numRequired {
			return true, nil
		}
		return false, fmt.Errorf("%s: %f valid endorsements required, %d valid endorsements received",
			notEnoughParticipantsSupportError, numRequired, numOfValid)
	default:
		return false, fmt.Errorf("authentication fail: unrecognized principle type, should be ANY, MAJORITY, " +
			"ALL, an integer (Threshold), or ac fraction (Portion)")
	}
}

func (p *pkACProvider) buildRoleListForVerifyPrincipal(pol *policy) map[protocol.Role]bool {
	roleListRaw := pol.GetRoleList()
	roleList := map[protocol.Role]bool{}
	for _, roleRaw := range roleListRaw {
		roleList[roleRaw] = true
	}
	return roleList
}

// all-in-one validation for signing members: signature, policies
func (p *pkACProvider) refinePrincipal(principal protocol.Principal) (protocol.Principal, error) {
	endorsements := principal.GetEndorsement()
	msg := principal.GetMessage()
	refinedEndorsement := p.RefineEndorsements(endorsements, msg)
	if len(refinedEndorsement) <= 0 {
		return nil, fmt.Errorf("refine endorsements failed, all endorsers have failed verification")
	}

	refinedPrincipal, err := p.CreatePrincipal(principal.GetResourceName(), refinedEndorsement, msg)
	if err != nil {
		return nil, fmt.Errorf("create principal failed: [%s]", err.Error())
	}

	return refinedPrincipal, nil
}

func (p *pkACProvider) refinePrincipalForCertOptimization(principal protocol.Principal) (protocol.Principal, error) {
	return nil, fmt.Errorf("this method should not be called")
}

func (p *pkACProvider) RefineEndorsements(endorsements []*common.EndorsementEntry,
	msg []byte) []*common.EndorsementEntry {

	refinedSigners := map[string]bool{}
	var refinedEndorsement []*common.EndorsementEntry

	for _, endorsementEntry := range endorsements {
		endorsement := &common.EndorsementEntry{
			Signer: &pbac.Member{
				OrgId:      endorsementEntry.Signer.OrgId,
				MemberInfo: endorsementEntry.Signer.MemberInfo,
				MemberType: endorsementEntry.Signer.MemberType,
			},
			Signature: endorsementEntry.Signature,
		}
		memInfo := string(endorsement.Signer.MemberInfo)

		remoteMember, err := p.NewMember(endorsement.Signer)
		if err != nil {
			p.log.Infof("new member failed: [%s]", err.Error())
			continue
		}

		if err := remoteMember.Verify(p.hashType, msg, endorsement.Signature); err != nil {
			p.log.Infof("signer member verify signature failed: [%s]", err.Error())
			p.log.Debugf("information for invalid signature:\norganization: %s\npubkey: %s\nmessage: %s\n"+
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

func (p *pkACProvider) getValidEndorsementsInner(orgList map[string]bool, roleList map[protocol.Role]bool,
	endorsements []*common.EndorsementEntry) []*common.EndorsementEntry {
	var refinedEndorsements []*common.EndorsementEntry
	for _, endorsement := range endorsements {
		if len(roleList) == 0 {
			refinedEndorsements = append(refinedEndorsements, endorsement)
			continue
		}

		member := p.getMemberFromCache(endorsement.Signer)
		if member == nil {
			p.log.Debugf(
				"authentication warning: the member is not in member cache, memberInfo[%s]",
				string(endorsement.Signer.MemberInfo))
			continue
		}

		p.log.Debugf("getValidEndorsements: signer's role [%v]", member.GetRole())

		if _, ok := roleList[member.GetRole()]; ok {
			refinedEndorsements = append(refinedEndorsements, endorsement)
		} else {
			p.log.Debugf("authentication warning: signer's role [%v] is not permitted, requires [%v]",
				member.GetRole(), roleList)
		}
	}

	return refinedEndorsements
}

// GetHashAlg return hash algorithm the access control provider uses
func (p *pkACProvider) GetHashAlg() string {
	return p.hashType
}

// ValidateResourcePolicy checks whether the given resource principal is valid
func (p *pkACProvider) ValidateResourcePolicy(resourcePolicy *config.ResourcePolicy) bool {
	if _, ok := restrainedResourceListPK[resourcePolicy.ResourceName]; ok {
		p.log.Errorf("bad configuration: should not modify the access policy of the resource: %s",
			resourcePolicy.ResourceName)
		return false
	}

	if resourcePolicy.Policy == nil {
		p.log.Errorf("bad configuration: access principle should not be nil when modifying access control configurations")
		return false
	}

	if !p.checkResourcePolicyOrgList(resourcePolicy.Policy) {
		return false
	}

	return p.checkResourcePolicyRule(resourcePolicy)
}

func (p *pkACProvider) checkResourcePolicyOrgList(policy *pbac.Policy) bool {
	if policy.OrgList == nil {
		return true
	}

	if len(policy.OrgList) == 0 {
		return true
	}

	if len(policy.OrgList) == 1 && policy.OrgList[0] != "public" {
		return true
	}

	p.log.Errorf("bad configuration: error [%s] in organization list", policy.OrgList)

	return false
}

func (p *pkACProvider) checkResourcePolicyRule(resourcePolicy *config.ResourcePolicy) bool {
	switch resourcePolicy.Policy.Rule {
	case string(protocol.RuleAny), string(protocol.RuleAll), string(protocol.RuleForbidden):
		return true
	case string(protocol.RuleMajority):
		return p.checkResourcePolicyRuleMajorityCase(resourcePolicy.Policy)
	case string(protocol.RuleDelete):
		p.log.Debugf("delete policy configuration of %s", resourcePolicy.ResourceName)
		return true
	case string(protocol.RuleSelf):
		//In order to be compatible with historical versions, the warn log is printed here but returns true.
		p.log.Warnf("pk mode should not set self, please modify %s", resourcePolicy.ResourceName)
		return true
	default:
		return p.checkResourcePolicyRuleDefaultCase(resourcePolicy.Policy)
	}
}

func (p *pkACProvider) checkResourcePolicyRuleMajorityCase(policy *pbac.Policy) bool {

	switch len(policy.RoleList) {
	case 0:
		p.log.Warnf("role allowed in [%s] is [%s]", protocol.RuleMajority, protocol.RoleAdmin)
		return true
	case 1:
		if policy.RoleList[0] != string(protocol.RoleAdmin) {
			p.log.Errorf("role allowed in [%s] is only [%s]", protocol.RuleMajority,
				protocol.RoleAdmin)
			return false
		}
		return true
	default:

		p.log.Errorf("role allowed in [%s] is only [%s]",
			protocol.RuleMajority, protocol.RoleAdmin)
		return false
	}
}

func (p *pkACProvider) checkResourcePolicyRuleDefaultCase(policy *pbac.Policy) bool {
	nums := strings.Split(policy.Rule, LIMIT_DELIMITER)
	switch len(nums) {
	case 1:
		_, err := strconv.Atoi(nums[0])
		if err != nil {
			p.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		return true
	case 2:
		numerator, err := strconv.Atoi(nums[0])
		if err != nil {
			p.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		denominator, err := strconv.Atoi(nums[1])
		if err != nil {
			p.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		if numerator <= 0 || denominator <= 0 {
			p.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		return true
	default:
		p.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
		return false
	}
}

// LookUpPolicy returns corresponding policy configured for the given resource name
func (p *pkACProvider) LookUpPolicy(resourceName string) (*pbac.Policy, error) {
	blockVersion, policyResourceName := getBlockVersionAndResourceName(resourceName)

	if blockVersion > 0 && blockVersion <= blockVersion220 {
		return p.lookUpPolicy220(policyResourceName)
	}

	if p, ok := p.latestPolicyMap.Load(policyResourceName); ok {
		return p.(*policy).GetPbPolicy(), nil
	}

	pol, ok := p.resourceNamePolicyMap.Load(policyResourceName)
	if !ok {
		return nil, fmt.Errorf("policy not found for resource %s", resourceName)
	}
	pbPolicy := pol.(*policy).GetPbPolicy()
	return pbPolicy, nil
}

// GetMemberStatus get the status information of the member
func (p *pkACProvider) GetMemberStatus(member *pbac.Member) (pbac.MemberStatus, error) {
	return pbac.MemberStatus_NORMAL, nil
}

// VerifyRelatedMaterial verify the member's relevant identity material
func (p *pkACProvider) VerifyRelatedMaterial(verifyType pbac.VerifyType, data []byte) (bool, error) {
	return true, nil
}

// GetAllPolicy returns all default policies
func (p *pkACProvider) GetAllPolicy() (map[string]*pbac.Policy, error) {
	var policyMap = make(map[string]*pbac.Policy)
	p.resourceNamePolicyMap.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		v, _ := value.(*policy)
		policyMap[k] = newPbPolicyFromPolicy(v)
		return true
	})
	p.senderPolicyMap.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		v, _ := value.(*policy)
		policyMap[k] = newPbPolicyFromPolicy(v)
		return true
	})
	return policyMap, nil
}

// VerifyPrincipalLT2330 verifies if the principal for the resource is met
func (p *pkACProvider) VerifyPrincipalLT2330(principal protocol.Principal, blockVersion uint32) (bool, error) {

	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(p, principal)

	} else if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(p, principal)
	}

	return false, fmt.Errorf("`VerifyPrincipalLT2330` should not used by blockVersion(%d)", blockVersion)
}

// GetValidEndorsements filters all endorsement entries and returns all valid ones
func (p *pkACProvider) GetValidEndorsements(
	principal protocol.Principal, blockVersion uint32) ([]*common.EndorsementEntry, error) {

	if blockVersion <= blockVersion220 {
		return p.getValidEndorsements220(principal)
	}

	if blockVersion < blockVersion2330 {
		return p.getValidEndorsements2320(principal)
	}
	return p.getValidEndorsements(principal, blockVersion)
}

// VerifyMsgPrincipal verifies if the principal for the resource is met
func (p *pkACProvider) VerifyMsgPrincipal(principal protocol.Principal, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		return verifyPrincipal220(p, principal)
	}

	if blockVersion < blockVersion2330 {
		return verifyPrincipal2320(p, principal)
	}

	return verifyMsgTypePrincipal(p, principal, blockVersion)
}

// VerifyTxPrincipal verifies if the principal for the resource is met
func (p *pkACProvider) VerifyTxPrincipal(tx *common.Transaction,
	resourceName string, blockVersion uint32) (bool, error) {
	if blockVersion <= blockVersion220 {
		if err := verifyTxPrincipal220(tx, p); err != nil {
			return false, err
		}
		return true, nil
	}

	if blockVersion < blockVersion2330 {
		if err := verifyTxPrincipal2320(tx, resourceName, p); err != nil {
			return false, err
		}
		return true, nil
	}

	return verifyTxPrincipal(tx, resourceName, p, blockVersion)
}

// VerifyMultiSignTxPrincipal verify if the multi-sign tx should be finished
func (p *pkACProvider) VerifyMultiSignTxPrincipal(
	mInfo *syscontract.MultiSignInfo,
	blockVersion uint32) (syscontract.MultiSignStatus, error) {

	if blockVersion < blockVersion2330 {
		return mInfo.Status, fmt.Errorf(
			"func `verifyMultiSignTxPrincipal` cannot be used in blockVersion(%v)", blockVersion)
	}
	return verifyMultiSignTxPrincipal(p, mInfo, blockVersion, p.log)
}

// IsRuleSupportedByMultiSign verify the policy of resourceName is supported by multi-sign
// it's implements must be the same with vm-native/supportRule
func (p *pkACProvider) IsRuleSupportedByMultiSign(resourceName string, blockVersion uint32) error {
	if blockVersion < blockVersion220 {
		return isRuleSupportedByMultiSign220(p, resourceName, p.log)
	}

	if blockVersion < blockVersion2330 {
		return isRuleSupportedByMultiSign2320(resourceName, p, p.log)
	}

	return isRuleSupportedByMultiSign(p, resourceName, blockVersion, p.log)
}

// GetCertFromCache get cert from cache
func (p *pkACProvider) GetAddressFromCache(pkBytes []byte) (string, crypto.PublicKey, error) {

	pkPem := string(pkBytes)
	cached, ok := p.lookUpMemberInCache(pkPem)
	if ok {
		p.log.Debugf("member address found in local cache")
		return cached.address, cached.pk, nil
	}

	// in case 缓存被清空，找不到原来保存的member信息
	// 又因为 pk 没办法直接恢复成member信息，所以创建新的index key
	indexKey := "pk_" + pkPem
	cached, ok = p.lookUpMemberInCache(indexKey)
	if ok {
		p.log.Debugf("member address found in local cache")
		return cached.address, cached.pk, nil
	}

	pk, err := asym.PublicKeyFromPEM(pkBytes)
	if err != nil {
		return "", nil, fmt.Errorf("new public key member failed: parse the public key from PEM failed")
	}

	publicKeyString, err := utils.PkToAddrStr(pk, p.addressType, crypto.HashAlgoMap[p.hashType])
	if err != nil {
		return "", cached.pk, err
	}

	if p.addressType == config.AddrType_ZXL {
		publicKeyString = "ZX" + publicKeyString
	}

	p.memberCache.Add(indexKey, &memberCached{address: publicKeyString, pk: pk})

	return publicKeyString, pk, nil
}

// GetCertFromCache get cert from cache
func (p *pkACProvider) GetCertFromCache(keyBytes []byte) ([]byte, error) {
	return nil, fmt.Errorf("not support in pkACProvider")
}

// GetPayerFromCache get payer from cache
func (p *pkACProvider) GetPayerFromCache(key []byte) ([]byte, error) {
	p.log.Debugf("get from cache, key=", string(key))
	value, ok := p.payerList.Get(string(key))
	if !ok {
		p.log.Debugf("not found %s", key)
		return nil, fmt.Errorf("not found %s", key)
	}
	byteValue, ok := value.(string)
	if !ok {
		p.log.Debugf("value is not a []byte]: %v", value)
		return nil, fmt.Errorf("value is not a []byte]: %v", value)
	}
	return []byte(byteValue), nil
}

// SetPayerToCache set payer to cache
func (p *pkACProvider) SetPayerToCache(key []byte, value []byte) error {
	p.log.Debugf("set from cache, key=", string(key), "#value=", string(value))
	p.payerList.Add(string(key), string(value))
	return nil
}

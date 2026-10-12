/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/localconf/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
)

// Special characters allowed to define customized access rules
const (
	LIMIT_DELIMITER              = "/"
	PARAM_CERTS                  = "certs"
	PARAM_CERTHASHES             = "cert_hashes"
	PARAM_ALIASES                = "aliases"
	PARAM_ALIAS                  = "alias"
	PUBLIC_KEYS                  = "pubkey"
	unsupportedRuleErrorTemplate = "bad configuration: unsupported rule [%s]"

	defaultCertCacheSize = 1024
)

var notEnoughParticipantsSupportError = "authentication fail: not enough participants support this action"

// hsmHandleMap is a global handle map for pkcs11 or sdf hsm
var hsmHandleMap = map[string]interface{}{}

// List of access principals which should not be customized
var restrainedResourceList = map[string]bool{
	protocol.ResourceNameAllTest:       true,
	protocol.ResourceNameP2p:           true,
	protocol.ResourceNameConsensusNode: true,

	common.TxType_QUERY_CONTRACT.String():  true,
	common.TxType_INVOKE_CONTRACT.String(): true,
	common.TxType_SUBSCRIBE.String():       true,
	common.TxType_ARCHIVE.String():         true,
	common.TxType_NODE_CONFIG.String():     true,
}

// predifined policies
var (
	policyRead = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleConsensusNode,
			protocol.RoleCommonNode,
			protocol.RoleClient,
			protocol.RoleAdmin,
		},
	)

	// policySpecialRead 任意一个角色{RoleClient/RoleAdmin/RoleLight/RoleConsensusNode/RoleCommonNode}
	policySpecialRead = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleConsensusNode,
			protocol.RoleCommonNode,
			protocol.RoleClient,
			protocol.RoleAdmin,
			protocol.RoleLight,
		},
	)

	// policySpecialWrite 任意一个客户端{RoleClient/RoleAdmin/RoleLight}
	policySpecialWrite = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleClient,
			protocol.RoleAdmin,
			protocol.RoleLight,
		},
	)

	policyWrite = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleClient,
			protocol.RoleAdmin,
			protocol.RoleConsensusNode,
		},
	)
	// policyConsensus 任意一个共识节点
	policyConsensus = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleConsensusNode,
		},
	)
	// policyP2P 任意一个节点
	policyP2P = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleConsensusNode,
			protocol.RoleCommonNode,
		},
	)
	policyAdmin = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policySubscribe = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleLight,
			protocol.RoleClient,
			protocol.RoleAdmin,
		},
	)

	policyArchive = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policyConfig = newPolicy(
		protocol.RuleMajority,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policySelfConfig = newPolicy(
		protocol.RuleSelf,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policyForbidden = newPolicy(
		protocol.RuleForbidden,
		nil,
		nil)

	policyAllTest = newPolicy(
		protocol.RuleAll,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policyLimitTestAny = newPolicy(
		"2",
		nil,
		nil,
	)

	policyLimitTestAdmin = newPolicy(
		"2",
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policyPortionTestAny = newPolicy(
		"3/4",
		nil,
		nil,
	)

	policyPortionTestAnyAdmin = newPolicy(
		"3/4",
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)

	policyRelayCross = newPolicy(
		protocol.RuleAny,
		nil,
		[]protocol.Role{
			protocol.RoleAdmin,
		},
	)
)

// accessControlService ac模块控制服务
// 对于任何一笔交易请求、网络消息等都会涉及到对该部分的检查
// 其检查顺序为：txType、sender、resource
// txType：即对交易类型进行检查，包括QUERY、INVOKE、SUBSCRIBE等，其优先检查latestPolicyMap，不存在的情况再检查txTypePolicyMap
// sender：即对交易的Sender进行检查（即签发这笔交易的用户，非endorsement(背书)），
// 其优先检查latestPolicyMap，不存在的情况下再检查senderPolicyMap
// resource：即对交易的Sender和Endorsement进行检查，会将Sender也算作Endorsement的一部分，
// 同样的，优先检查latestPolicyMap，不存在的情况下再检查resourceNamePolicyMap
// 上述检查中Invoke交易会全部检查，其他交易仅检查txType
// 网络消息会检查latestPolicyMap和msgTypePolicyMap
type accessControlService struct {
	orgNum  int32
	orgList *sync.Map // map[string]interface{} , orgId -> interface{}
	// 交易类型对应的策略，query、invoke等
	txTypePolicyMap *sync.Map
	// 网络消息对应检查策略，如p2p交易广播、共识消息等
	msgTypePolicyMap *sync.Map // map[string]*policy , messageType  -> *policy
	// 合约（含方法）对应的默认检查策略，key为合约名+方法，value目前设置为policyForbidden，用于拒绝
	// 后续可将名称修改为forbiddenPolicyMap
	senderPolicyMap *sync.Map // map[string]*policy , resourceName -> *policy
	// 合约（含方法）对应的默认检查策略，key为合约名+方法，value目前设置为对应的策略
	resourceNamePolicyMap *sync.Map // map[string]*policy , resourceName -> *policy
	// 用户配置的策略，优先级最高，会覆盖txTypePolicyMap、msgTypePolicyMap、senderPolicyMap、resourceNamePolicyMap
	// 该配置策略可由配置文件(bc.yml)中resource_policies部分进行设置
	latestPolicyMap *sync.Map // map[string]*policy , resourceName -> *policy

	resourceNamePolicyMap220 *sync.Map
	exceptionalPolicyMap220  *sync.Map

	resourceNamePolicyMap2320 *sync.Map
	exceptionalPolicyMap2320  *sync.Map

	//local cache for member
	memberCache *ShardCache

	dataStore protocol.BlockchainStore

	log protocol.Logger

	// hash algorithm for chains
	hashType string

	authType string

	addressType config.AddrType

	pwkNewMember func(member *pbac.Member) (protocol.Member, error)

	getCertVerifyOptions func() *bcx509.VerifyOptions
}

type memberCached struct {
	member    protocol.Member
	certChain []*bcx509.Certificate
	address   string
	pk        crypto.PublicKey
}

func GetCertCacheSize() int {
	if localconf.ChainMakerConfig.NodeConfig.CertCacheSize > 0 {
		return localconf.ChainMakerConfig.NodeConfig.CertCacheSize
	}
	return defaultCertCacheSize
}
func initAccessControlService(hashType, authType string, addressType config.AddrType,
	store protocol.BlockchainStore, log protocol.Logger) *accessControlService {
	acService := &accessControlService{
		orgNum:                    0,
		orgList:                   &sync.Map{},
		txTypePolicyMap:           &sync.Map{},
		msgTypePolicyMap:          &sync.Map{},
		senderPolicyMap:           &sync.Map{},
		resourceNamePolicyMap:     &sync.Map{},
		latestPolicyMap:           &sync.Map{},
		resourceNamePolicyMap220:  &sync.Map{},
		exceptionalPolicyMap220:   &sync.Map{},
		resourceNamePolicyMap2320: &sync.Map{},
		exceptionalPolicyMap2320:  &sync.Map{},
		memberCache:               NewShardCache(GetCertCacheSize()),
		dataStore:                 store,
		log:                       log,
		hashType:                  hashType,
		authType:                  authType,
		addressType:               addressType,
	}
	return acService
}

func (acs *accessControlService) initResourcePolicy(resourcePolicies []*config.ResourcePolicy,
	localOrgId string) {
	acs.createDefaultResourcePolicy(localOrgId)
	acs.loadResourcePolicy(resourcePolicies)
}

func (acs *accessControlService) createDefaultResourcePolicy(localOrgId string) {
	authType := strings.ToLower(acs.authType)
	switch authType {
	case protocol.PermissionedWithCert, protocol.Identity:
		acs.createDefaultResourcePolicyForCert_220()
		acs.createDefaultResourcePolicyForCert_2320()
		acs.createDefaultResourcePolicyForCert(localOrgId)
	case protocol.PermissionedWithKey:
		acs.createDefaultResourcePolicyForPK_220()
		acs.createDefaultResourcePolicyForPK_2320()
		acs.createDefaultResourcePolicyForPWK(localOrgId)
	}
}

func (acs *accessControlService) loadResourcePolicy(resourcePolicies []*config.ResourcePolicy) {

	lastestPolicyMap := &sync.Map{}
	for _, resourcePolicy := range resourcePolicies {
		if acs.validateResourcePolicy(resourcePolicy) {
			policy := newPolicyFromPb(resourcePolicy.Policy)
			lastestPolicyMap.Store(resourcePolicy.ResourceName, policy)
		}
	}
	acs.latestPolicyMap = lastestPolicyMap
}

func (acs *accessControlService) checkResourcePolicyOrgList(policy *pbac.Policy) bool {
	orgCheckList := map[string]bool{}
	for _, org := range policy.OrgList {
		if _, ok := acs.orgList.Load(org); !ok {
			acs.log.Errorf("bad configuration: configured organization list contains unknown organization [%s]", org)
			return false
		} else if _, alreadyIn := orgCheckList[org]; alreadyIn {
			acs.log.Errorf("bad configuration: duplicated entries [%s] in organization list", org)
			return false
		}
		orgCheckList[org] = true
	}
	return true
}

func (acs *accessControlService) checkResourcePolicyRule(resourcePolicy *config.ResourcePolicy) bool {
	switch resourcePolicy.Policy.Rule {
	case string(protocol.RuleAny), string(protocol.RuleAll), string(protocol.RuleForbidden):
		return true
	case string(protocol.RuleSelf):
		return acs.checkResourcePolicyRuleSelfCase(resourcePolicy)
	case string(protocol.RuleMajority):
		return acs.checkResourcePolicyRuleMajorityCase(resourcePolicy.Policy)
	case string(protocol.RuleDelete):
		acs.log.Debugf("delete policy configuration of %s", resourcePolicy.ResourceName)
		return true
	default:
		return acs.checkResourcePolicyRuleDefaultCase(resourcePolicy.Policy)
	}
}

func (acs *accessControlService) checkResourcePolicyRuleSelfCase(resourcePolicy *config.ResourcePolicy) bool {
	switch resourcePolicy.ResourceName {
	case syscontract.SystemContract_CHAIN_CONFIG.String() + "-" +
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		syscontract.SystemContract_CHAIN_CONFIG.String() + "-" +
			syscontract.ChainConfigFunction_NODE_ID_UPDATE.String():
		//验证rolelist
		if len(resourcePolicy.GetPolicy().RoleList) == 1 &&
			resourcePolicy.GetPolicy().RoleList[0] == string(protocol.RoleAdmin) {
			return true
		}
		acs.log.Errorf("bad configuration: the access rule.RoleList of [%s] should be [ADMIN]", resourcePolicy.ResourceName)
		return false
	default:
		acs.log.Errorf("bad configuration: the access rule of [%s] should not be [%s]", resourcePolicy.ResourceName,
			resourcePolicy.Policy.Rule)
		return false
	}
}

func (acs *accessControlService) checkResourcePolicyRuleMajorityCase(policy *pbac.Policy) bool {
	if len(policy.OrgList) != int(atomic.LoadInt32(&acs.orgNum)) {
		acs.log.Warnf("[%s] rule considers all the organizations on the chain, any customized configuration for "+
			"organization list will be overridden, should use [Portion] rule for customized organization list",
			protocol.RuleMajority)
	}
	switch len(policy.RoleList) {
	case 0:
		acs.log.Warnf("role allowed in [%s] is [%s]", protocol.RuleMajority, protocol.RoleAdmin)
		return true
	case 1:
		if policy.RoleList[0] != string(protocol.RoleAdmin) {
			acs.log.Warnf("role allowed in [%s] is only [%s], [%s] will be overridden", protocol.RuleMajority,
				protocol.RoleAdmin, policy.RoleList[0])
		}
		return true
	default:
		acs.log.Warnf("role allowed in [%s] is only [%s], the other roles in the list will be ignored",
			protocol.RuleMajority, protocol.RoleAdmin)
		return true
	}
}

func (acs *accessControlService) checkResourcePolicyRuleDefaultCase(policy *pbac.Policy) bool {
	nums := strings.Split(policy.Rule, LIMIT_DELIMITER)
	switch len(nums) {
	case 1:
		_, err := strconv.Atoi(nums[0])
		if err != nil {
			acs.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		return true
	case 2:
		numerator, err := strconv.Atoi(nums[0])
		if err != nil {
			acs.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		denominator, err := strconv.Atoi(nums[1])
		if err != nil {
			acs.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		if numerator <= 0 || denominator <= 0 {
			acs.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
			return false
		}
		return true
	default:
		acs.log.Errorf(unsupportedRuleErrorTemplate, policy.Rule)
		return false
	}
}

func (acs *accessControlService) lookUpMemberInCache(memberInfo string) (*memberCached, bool) {
	ret, ok := acs.memberCache.Get(memberInfo)
	if ok {
		return ret.(*memberCached), true
	}
	return nil, false
}

// parseUserAddress
func publicKeyFromCert(member []byte) ([]byte, error) {
	certificate, err := utils.ParseCert(member)
	if err != nil {
		return nil, err
	}
	pubKeyStr, err := certificate.PublicKey.String()
	if err != nil {
		return nil, err
	}
	return []byte(pubKeyStr), nil
}
func (acs *accessControlService) memberToAddress(member *pbac.Member) (string, error) {
	//计算地址
	var err error
	var pk []byte
	var publicKey crypto.PublicKey
	switch member.MemberType {
	case pbac.MemberType_CERT, pbac.MemberType_CERT_HASH, pbac.MemberType_ALIAS:
		pk, err = publicKeyFromCert(member.MemberInfo)
		if err != nil {
			return "", err
		}
		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return "", err
		}

	case pbac.MemberType_PUBLIC_KEY:
		pk = member.MemberInfo
		publicKey, err = asym.PublicKeyFromPEM(pk)
		if err != nil {
			return "", err
		}
	}
	publicKeyString, err := utils.PkToAddrStr(publicKey, acs.addressType, crypto.HashAlgoMap[acs.hashType])
	if err != nil {
		return "", err
	}

	if acs.addressType == configPb.AddrType_ZXL {
		publicKeyString = "ZX" + publicKeyString
	}
	return publicKeyString, nil
}

func (acs *accessControlService) addMemberToCache(member *pbac.Member, memberCached *memberCached) {

	address, err := acs.memberToAddress(member)
	if err != nil {
		acs.log.Errorf("add member to cache failed, err = %s", err.Error())
		acs.memberCache.Add(string(member.MemberInfo), memberCached)
		return
	}

	memberCached.address = address
	memberCached.pk = memberCached.member.GetPk()

	acs.memberCache.Add(string(member.MemberInfo), memberCached)
}

func (acs *accessControlService) addOrg(orgId string, orgInfo interface{}) {
	_, loaded := acs.orgList.LoadOrStore(orgId, orgInfo)
	if loaded {
		acs.orgList.Store(orgId, orgInfo)
	} else {
		acs.orgNum++
	}
}

func (acs *accessControlService) getOrgInfoByOrgId(orgId string) interface{} {
	orgInfo, ok := acs.orgList.Load(orgId)
	if !ok {
		return nil
	}
	return orgInfo
}

func (acs *accessControlService) getAllOrgInfos() []interface{} {
	orgInfos := make([]interface{}, 0)
	acs.orgList.Range(func(_, value interface{}) bool {
		orgInfos = append(orgInfos, value)
		return true
	})
	return orgInfos
}

func (acs *accessControlService) validateResourcePolicy(resourcePolicy *config.ResourcePolicy) bool {
	if _, ok := restrainedResourceList[resourcePolicy.ResourceName]; ok {
		acs.log.Errorf("bad configuration: should not modify the access policy of the resource: %s",
			resourcePolicy.ResourceName)
		return false
	}

	if resourcePolicy.Policy == nil {
		acs.log.Errorf("bad configuration: access principle should not be nil when modifying access control configurations")
		return false
	}

	if !acs.checkResourcePolicyOrgList(resourcePolicy.Policy) {
		return false
	}

	return acs.checkResourcePolicyRule(resourcePolicy)
}

func (acs *accessControlService) createPrincipal(resourceName string, endorsements []*common.EndorsementEntry,
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

func (acs *accessControlService) createPrincipalForTargetOrg(resourceName string,
	endorsements []*common.EndorsementEntry, message []byte, targetOrgId string) (protocol.Principal, error) {
	p, err := acs.createPrincipal(resourceName, endorsements, message)
	if err != nil {
		return nil, err
	}
	p.(*principal).targetOrg = targetOrgId
	return p, nil
}

func (acs *accessControlService) newCertMember(pbMember *pbac.Member) (protocol.Member, error) {
	return newCertMemberFromPb(pbMember, acs)
}

func (acs *accessControlService) newPkMember(member *pbac.Member, adminList,
	consensusList *sync.Map) (protocol.Member, error) {

	memberCache := acs.getMemberFromCache(member)
	if memberCache != nil {
		return memberCache, nil
	}
	pkMember, err := newPkMemberFromAcs(member, adminList, consensusList, acs)
	if err != nil {
		return nil, fmt.Errorf("new public key member failed: %s", err.Error())
	}
	if pkMember.GetOrgId() != member.OrgId && member.OrgId != "" {
		return nil, fmt.Errorf("new public key member failed: member orgId does not match on chain")
	}
	cached := &memberCached{
		member:    pkMember,
		certChain: nil,
	}
	acs.addMemberToCache(member, cached)
	return pkMember, nil
}

func (acs *accessControlService) newNodePkMember(member *pbac.Member,
	consensusList *sync.Map) (protocol.Member, error) {

	memberCache := acs.getMemberFromCache(member)
	if memberCache != nil {
		if memberCache.GetRole() != protocol.RoleConsensusNode &&
			memberCache.GetRole() != protocol.RoleCommonNode {
			return nil, fmt.Errorf("get member from cache, the public key is not a node member")
		}
		return memberCache, nil
	}
	pkMember, err := newPkNodeMemberFromAcs(member, consensusList, acs)
	if err != nil {
		return nil, err
	}
	if pkMember.GetOrgId() != member.OrgId && member.OrgId != "" {
		return nil, fmt.Errorf("new public key node member failed: member orgId does not match on chain")
	}
	cached := &memberCached{
		member:    pkMember,
		certChain: nil,
	}
	acs.addMemberToCache(member, cached)
	return pkMember, nil
}

func (acs *accessControlService) getMemberFromCache(member *pbac.Member) protocol.Member {
	cached, ok := acs.lookUpMemberInCache(string(member.MemberInfo))
	if ok {
		acs.log.Debugf("member found in local cache")
		if cached.member.GetOrgId() != member.OrgId {
			acs.log.Debugf("get member from cache failed: member orgId does not match on chain")
			return nil
		}
		return cached.member
	}

	//handle false positive when member cache is cleared
	var tmpMember protocol.Member
	var err error
	var certChains [][]*bcx509.Certificate
	if acs.authType == protocol.PermissionedWithCert || acs.authType == protocol.Identity {
		tmpMember, err = acs.newCertMember(member)
		certMember, ok := tmpMember.(*certificateMember)
		if !ok {
			return nil
		}
		certChains, err = certMember.cert.Verify(*acs.getCertVerifyOptions())
		if err != nil {
			acs.log.Debugf("certMember verify cert chain failed, err = %s", err.Error())
			return nil
		}
		if len(certChains) == 0 {
			acs.log.Debugf("certMember verify cert chain failed, len(certChains) = %d", len(certChains))
			return nil
		}

	} else if acs.authType == protocol.PermissionedWithKey {
		tmpMember, err = acs.pwkNewMember(member)
	}
	if err != nil {
		acs.log.Debugf("new member failed, authType = %s, err = %s", acs.authType, err.Error())
		return nil
	}
	//add to cache
	if certChains != nil {
		cached = &memberCached{
			member:    tmpMember,
			certChain: certChains[0],
		}
	} else {
		cached = &memberCached{
			member:    tmpMember,
			certChain: nil,
		}
	}
	acs.addMemberToCache(member, cached)

	return tmpMember
}

func (acs *accessControlService) verifyPrincipalPolicy(principal, refinedPrincipal protocol.Principal, p *policy) (
	bool, error) {
	endorsements := refinedPrincipal.GetEndorsement()
	rule := p.GetRule()

	switch rule {
	case protocol.RuleForbidden:
		return false, fmt.Errorf("authentication fail: [%s] is forbidden to access", refinedPrincipal.GetResourceName())
	case protocol.RuleMajority:
		return acs.verifyPrincipalPolicyRuleMajorityCase(p, endorsements)
	case protocol.RuleSelf:
		return acs.verifyPrincipalPolicyRuleSelfCase(principal.GetTargetOrgId(), endorsements)
	case protocol.RuleAny:
		return acs.verifyPrincipalPolicyRuleAnyCase(p, endorsements, principal.GetResourceName())
	case protocol.RuleAll:
		return acs.verifyPrincipalPolicyRuleAllCase(p, endorsements)
	default:
		return acs.verifyPrincipalPolicyRuleDefaultCase(p, endorsements)
	}
}

func (acs *accessControlService) verifyPrincipalPolicyRuleMajorityCase(p *policy,
	endorsements []*common.EndorsementEntry) (bool, error) {
	// notice: accept admin role only, and require majority of all the organizations on the chain
	role := protocol.RoleAdmin
	// orgList, _ := buildOrgListRoleListOfPolicyForVerifyPrincipal(p)

	// warning: majority keywork with non admin constraints
	/*
		if roleList[0] != protocol.protocol.RoleAdmin {
			return false, fmt.Errorf("authentication fail: MAJORITY keyword only allows admin role")
		}
	*/

	numOfValid := acs.countValidEndorsements(map[string]bool{}, map[protocol.Role]bool{role: true}, endorsements)

	if float64(numOfValid) > float64(acs.orgNum)/2.0 {
		return true, nil
	}
	return false, fmt.Errorf("%s: %d valid endorsements required, %d valid endorsements received",
		notEnoughParticipantsSupportError, int(float64(acs.orgNum)/2.0+1), numOfValid)
}

func (acs *accessControlService) verifyPrincipalPolicyRuleSelfCase(targetOrg string,
	endorsements []*common.EndorsementEntry) (bool, error) {
	role := protocol.RoleAdmin
	if targetOrg == "" {
		return false, fmt.Errorf("authentication fail: SELF keyword requires the owner of the affected target")
	}
	for _, entry := range endorsements {
		if entry.Signer.OrgId != targetOrg {
			acs.log.Warnf("endorsement.Signer.OrgId=[%s]", entry.Signer.OrgId)
			continue
		}

		member := acs.getMemberFromCache(entry.Signer)
		if member == nil {
			acs.log.Debugf(
				"authentication warning: the member is not in member cache, memberInfo[%s]",
				string(entry.Signer.MemberInfo))
			continue
		}

		if member.GetRole() == role {
			return true, nil
		}
		acs.log.Warnf("member.role=[%s], role=[%s]", member.GetRole(), role)

	}
	return false, fmt.Errorf("authentication fail: target [%s] does not belong to the signer", targetOrg)
}

func (acs *accessControlService) verifyPrincipalPolicyRuleAnyCase(p *policy, endorsements []*common.EndorsementEntry,
	resourceName string) (bool, error) {
	orgList, roleList := buildOrgListRoleListOfPolicyForVerifyPrincipal(p)
	for _, endorsement := range endorsements {
		if len(orgList) > 0 {
			if _, ok := orgList[endorsement.Signer.OrgId]; !ok {
				acs.log.Debugf("authentication warning: signer's organization [%s] is not permitted, requires [%v]",
					endorsement.Signer.OrgId, p.GetOrgList())
				continue
			}
		}

		if len(roleList) == 0 {
			return true, nil
		}

		member := acs.getMemberFromCache(endorsement.Signer)
		if member == nil {
			acs.log.Debugf(
				"authentication warning: the member is not in member cache, memberInfo[%s]",
				string(endorsement.Signer.MemberInfo))
			continue
		}

		if _, ok := roleList[member.GetRole()]; ok {
			return true, nil
		}
		acs.log.Debugf("authentication warning: signer's role [%v] is not permitted, requires [%v]",
			member.GetRole(), p.GetRoleList())
	}

	return false, fmt.Errorf("authentication fail: signers do not meet the requirement (%s)",
		resourceName)
}

func (acs *accessControlService) verifyPrincipalPolicyRuleAllCase(p *policy, endorsements []*common.EndorsementEntry) (
	bool, error) {
	orgList, roleList := buildOrgListRoleListOfPolicyForVerifyPrincipal(p)
	numOfValid := acs.countValidEndorsements(orgList, roleList, endorsements)
	if len(orgList) <= 0 && numOfValid == int(atomic.LoadInt32(&acs.orgNum)) {
		return true, nil
	}
	if len(orgList) > 0 && numOfValid == len(orgList) {
		return true, nil
	}
	return false, fmt.Errorf("authentication fail: not all of the listed organtizations consend to this action")
}

func (acs *accessControlService) verifyPrincipalPolicyRuleDefaultCase(p *policy,
	endorsements []*common.EndorsementEntry) (bool, error) {
	rule := p.GetRule()
	orgList, roleList := buildOrgListRoleListOfPolicyForVerifyPrincipal(p)
	nums := strings.Split(string(rule), LIMIT_DELIMITER)
	switch len(nums) {
	case 1:
		threshold, err := strconv.Atoi(nums[0])
		if err != nil {
			return false, fmt.Errorf("authentication fail: unrecognized rule, should be ANY, MAJORITY, ALL, " +
				"SELF, ac threshold (integer), or ac portion (fraction)")
		}

		numOfValid := acs.countValidEndorsements(orgList, roleList, endorsements)
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
			denominator = int(atomic.LoadInt32(&acs.orgNum))
		}

		numOfValid := acs.countValidEndorsements(orgList, roleList, endorsements)

		var numRequired float64
		if len(orgList) <= 0 {
			numRequired = float64(atomic.LoadInt32(&acs.orgNum)) * float64(numerator) / float64(denominator)
		} else {
			numRequired = float64(len(orgList)) * float64(numerator) / float64(denominator)
		}
		if float64(numOfValid) >= numRequired {
			return true, nil
		}
		return false, fmt.Errorf("%s: %f valid endorsements required, %d valid endorsements received",
			notEnoughParticipantsSupportError, numRequired, numOfValid)
	default:
		return false, fmt.Errorf("authentication fail: unrecognized principle type, should be ANY, MAJORITY, " +
			"ALL, SELF, an integer (Threshold), or ac fraction (Portion)")
	}
}

func (acs *accessControlService) countValidEndorsements(orgList map[string]bool, roleList map[protocol.Role]bool,
	endorsements []*common.EndorsementEntry) int {
	refinedEndorsements := acs.getValidEndorsements(orgList, roleList, endorsements)
	return countOrgsFromEndorsements(refinedEndorsements)
}

func (acs *accessControlService) getValidEndorsements(orgList map[string]bool, roleList map[protocol.Role]bool,
	endorsements []*common.EndorsementEntry) []*common.EndorsementEntry {
	var refinedEndorsements []*common.EndorsementEntry
	for _, endorsement := range endorsements {
		if len(orgList) > 0 {
			if _, ok := orgList[endorsement.Signer.OrgId]; !ok {
				acs.log.Debugf("authentication warning: signer's organization [%s] is not permitted, requires",
					endorsement.Signer.OrgId, orgList)
				continue
			}
		}

		if len(roleList) == 0 {
			refinedEndorsements = append(refinedEndorsements, endorsement)
			continue
		}

		if endorsement.Signer == nil {
			continue
		}
		member := acs.getMemberFromCache(endorsement.Signer)
		if member == nil {
			acs.log.Debugf(
				"authentication warning: the member is not in member cache, memberInfo[%s]",
				string(endorsement.Signer.MemberInfo))
			continue
		}

		isRoleMatching := isRoleMatching(member.GetRole(), roleList, &refinedEndorsements, endorsement)
		if !isRoleMatching {
			acs.log.Debugf(
				"authentication warning: signer's role [%v] is not permitted, requires [%v]",
				member.GetRole(),
				roleList,
			)
		}
	}

	return refinedEndorsements
}

func isRoleMatching(signerRole protocol.Role, roleList map[protocol.Role]bool,
	refinedEndorsements *[]*common.EndorsementEntry, endorsement *common.EndorsementEntry) bool {
	isRoleMatching := false
	if _, ok := roleList[signerRole]; ok {
		*refinedEndorsements = append(*refinedEndorsements, endorsement)
		isRoleMatching = true
	}
	return isRoleMatching
}

func countOrgsFromEndorsements(endorsements []*common.EndorsementEntry) int {
	mapOrg := map[string]int{}
	for _, endorsement := range endorsements {
		mapOrg[endorsement.Signer.OrgId]++
	}
	return len(mapOrg)
}

func buildOrgListRoleListOfPolicyForVerifyPrincipal(p *policy) (map[string]bool, map[protocol.Role]bool) {
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
	return orgList, roleList
}

// setVerifyOptionsFunc used to set verifyOptionsFunc which will check if  certificate chain valid
func (acs *accessControlService) setVerifyOptionsFunc(verifyOptionsFunc func() *bcx509.VerifyOptions) {
	acs.getCertVerifyOptions = verifyOptionsFunc
}

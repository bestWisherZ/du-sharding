package accesscontrol

import (
	"fmt"
	"testing"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/utils/v2"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"github.com/stretchr/testify/require"
)

func TestGetMemberStatus(t *testing.T) {
	logger := &test.GoLogger{}
	certProvider, err := newCertACProvider(testChainConfig, testOrg1, nil, logger)
	require.Nil(t, err)
	require.NotNil(t, certProvider)

	pbMember := &acPb.Member{
		OrgId:      testOrg1,
		MemberType: acPb.MemberType_CERT,
		MemberInfo: []byte(testConsensusSignOrg1.cert),
	}

	memberStatus, err := certProvider.GetMemberStatus(pbMember)
	require.Nil(t, err)
	require.Equal(t, acPb.MemberStatus_NORMAL, memberStatus)
}

func testInitCertFunc(t *testing.T) map[string]*orgMember {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	var orgMemberMap = make(map[string]*orgMember, len(orgMemberInfoMap))
	for orgId, info := range orgMemberInfoMap {
		orgMemberMap[orgId] = initOrgMember(t, info)
	}

	return orgMemberMap
}

func TestCert_VerifyPolicyDefault(t *testing.T) {
	testCert_VerifyPolicyDefault(blockVersion220, t)

	testCert_VerifyPolicyDefault(blockVersion2320, t)

	testCert_VerifyPolicyDefault(blockVersion2330, t)
}

func testCert_VerifyPolicyDefault(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]

	var (
		err error
		ok  bool
	)

	//【valid】test case
	defaultPolicyTx := testCreateTx(
		"CHAIN_CONFIG", "NODE_ID_UPDATE", "test-txid-12345")

	err = testAppendSender2Tx(defaultPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(defaultPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(defaultPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)
}

func TestCert_VerifyPolicyMajority(t *testing.T) {
	testCert_VerifyPolicyMajority(blockVersion220, t)

	testCert_VerifyPolicyMajority(blockVersion2320, t)

	testCert_VerifyPolicyMajority2330(blockVersion2330, t)
}

func testCert_VerifyPolicyMajority(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]
	orgMemberInfo2 := testCertOrgMember[testOrg2]
	orgMemberInfo3 := testCertOrgMember[testOrg3]
	orgMemberInfo4 := testCertOrgMember[testOrg4]

	var (
		err error
		ok  bool
	)

	//【valid】test case
	majorityPolicyTx := testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo4.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(majorityPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(majorityPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】no enough endorsers
	majorityPolicyTx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo4.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(majorityPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(majorityPolicyTx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
}

func testCert_VerifyPolicyMajority2330(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]
	orgMemberInfo2 := testCertOrgMember[testOrg2]
	orgMemberInfo3 := testCertOrgMember[testOrg3]
	orgMemberInfo4 := testCertOrgMember[testOrg4]

	var (
		err error
		ok  bool
	)

	//【valid】test case
	majorityPolicyTx := testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo4.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(majorityPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(majorityPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】no enough endorsers
	majorityPolicyTx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(majorityPolicyTx, testPKHashType, orgMemberInfo4.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(majorityPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(majorityPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)
}

func TestCert_VerifyPolicyAny(t *testing.T) {
	testCert_VerifyPolicyAny(blockVersion220, t)

	testCert_VerifyPolicyAny(blockVersion2320, t)

	testCert_VerifyPolicyAny(blockVersion2330, t)
}

func testCert_VerifyPolicyAny(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]

	var (
		err error
		ok  bool
	)

	// 【valid】 test case
	anyPolicyTx := testCreateTx(
		syscontract.SystemContract_CERT_MANAGE.String(),
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(anyPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(anyPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(anyPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	// 【invalid】role is not match
	anyPolicyTx = testCreateTx(
		syscontract.SystemContract_CERT_MANAGE.String(),
		syscontract.CertManageFunction_CERTS_ALIAS_DELETE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(anyPolicyTx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(anyPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(anyPolicyTx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("Got expected error, err = %v \n\n", err)
}

func TestCert_VerifyPolicyAll(t *testing.T) {
	//testCert_VerifyPolicyAll(blockVersion220, t)
	//testCert_VerifyPolicyAll(blockVersion2320, t)

	testCert_VerifyPolicyAll(blockVersion2330, t)
}

func testCert_VerifyPolicyAll(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]
	orgMemberInfo2 := testCertOrgMember[testOrg2]
	orgMemberInfo3 := testCertOrgMember[testOrg3]
	orgMemberInfo4 := testCertOrgMember[testOrg4]

	var (
		err error
		ok  bool
	)

	// 【valid】 test case
	allPolicyTx := testCreateTx(
		"TEST_CONTRACT", "TEST_METHOD_ALL", "test-txid-12345")

	err = testAppendSender2Tx(allPolicyTx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo4.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(allPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(allPolicyTx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	// 【invalid】no enough endorsement
	allPolicyTx = testCreateTx(
		"TEST_CONTRACT", "TEST_METHOD_ALL", "test-txid-12345")

	err = testAppendSender2Tx(allPolicyTx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)

	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(allPolicyTx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(allPolicyTx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(allPolicyTx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("Got expected error, err = %v \n\n", err)
}

func TestCert_VerifyPolicySelf(t *testing.T) {
	testCert_VerifyPolicySelf(blockVersion220, t)

	testCert_VerifyPolicySelf(blockVersion2320, t)

	testCert_VerifyPolicySelf(blockVersion2330, t)
}

func testCert_VerifyPolicySelf(blockVersion uint32, t *testing.T) {
	// initialize
	testCertOrgMember := testInitCertFunc(t)
	orgMemberInfo1 := testCertOrgMember[testOrg1]
	orgMemberInfo2 := testCertOrgMember[testOrg2]

	var (
		err error
		ok  bool
	)

	// 【valid】 test case
	txDeleteCertAlias := testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(txDeleteCertAlias, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(txDeleteCertAlias)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(txDeleteCertAlias, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	// 【invalid】role is not match
	txDeleteCertAlias = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(txDeleteCertAlias, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(txDeleteCertAlias)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(txDeleteCertAlias, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("Got expected error, err = %v \n\n", err)

	// 【invalid】orgId is not match
	txDeleteCertAlias = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(txDeleteCertAlias, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(txDeleteCertAlias)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(txDeleteCertAlias, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("Got expected error, err = %v \n\n", err)

}

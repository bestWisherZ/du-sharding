package accesscontrol

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/common/v2/helper"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// ****************************************************
//		Init Organization Member
// *****************************************************

type testPkOrgMember struct {
	orgId      string
	acProvider protocol.AccessControlProvider
	consensus  protocol.SigningMember
	admin      protocol.SigningMember
	client     protocol.SigningMember
}

type testPkOrgMemberInfo struct {
	testPkMemberInfo
	orgId string
}

var testPKOrgMemberInfoMap = map[string]*testPkOrgMemberInfo{
	testOrg1: {
		testPkMemberInfo: testPkMemberInfo{
			consensus: testConsensus1PKInfo,
			admin:     testAdmin1PKInfo,
			client:    testClient1PKInfo,
		},
		orgId: testOrg1,
	},
	testOrg2: {
		testPkMemberInfo: testPkMemberInfo{
			consensus: testConsensus2PKInfo,
			admin:     testAdmin2PKInfo,
			client:    testClient2PKInfo,
		},
		orgId: testOrg2,
	},
	testOrg3: {
		testPkMemberInfo: testPkMemberInfo{
			consensus: testConsensus3PKInfo,
			admin:     testAdmin3PKInfo,
			client:    testClient3PKInfo,
		},
		orgId: testOrg3,
	},
	testOrg4: {
		testPkMemberInfo: testPkMemberInfo{
			consensus: testConsensus4PKInfo,
			admin:     testAdmin4PKInfo,
			client:    testClient4PKInfo,
		},
		orgId: testOrg4,
	},
}

func testInitPermissionedPKFunc(t *testing.T) map[string]*testPkOrgMember {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	var testPkOrgMember = make(map[string]*testPkOrgMember, len(testPKOrgMemberInfoMap))
	store := testInitBlockchainStore(t)
	for orgId, info := range testPKOrgMemberInfoMap {
		testPkOrgMember[orgId] = initPKOrgMember(t, info, store)
	}
	test1PermissionedPKACProvider = testPkOrgMember[testOrg1].acProvider
	test2PermissionedPKACProvider = testPkOrgMember[testOrg2].acProvider
	return testPkOrgMember
}

func testInitBlockchainStore(t *testing.T) protocol.BlockchainStore {
	ctl := gomock.NewController(t)
	store := mock.NewMockBlockchainStore(ctl)

	for orgId, info := range testPKOrgMemberInfoMap {
		pk, err := asym.PublicKeyFromPEM([]byte(info.client.pk))
		require.Nil(t, err)
		pkBytes, err := pk.Bytes()
		require.Nil(t, err)
		publicKeyIndex := pubkeyHash(pkBytes)
		pkInfo := acPb.PKInfo{
			PkBytes: pkBytes,
			Role:    string(protocol.RoleClient),
			OrgId:   orgId,
		}
		pkInfoBytes, err := proto.Marshal(&pkInfo)
		require.Nil(t, err)
		store.EXPECT().ReadObject(
			syscontract.SystemContract_PUBKEY_MANAGE.String(),
			[]byte(publicKeyIndex)).Return(pkInfoBytes, nil).AnyTimes()
	}
	return store
}

func initPKOrgMember(t *testing.T, info *testPkOrgMemberInfo, store protocol.BlockchainStore) *testPkOrgMember {
	td, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()
	logger := &test.GoLogger{}

	ppkProvider, err := newPermissionedPkACProvider(testPermissionedPKChainConfig,
		info.orgId, store, logger)
	require.Nil(t, err)
	require.NotNil(t, ppkProvider)

	localPrivKeyFile := filepath.Join(td, info.orgId+".key")

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.consensus.sk), os.ModePerm)
	require.Nil(t, err)

	consensus, err := InitPKSigningMember(ppkProvider.GetHashAlg(), info.orgId, localPrivKeyFile, "")
	require.Nil(t, err)

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.admin.sk), os.ModePerm)
	require.Nil(t, err)

	admin, err := InitPKSigningMember(ppkProvider.GetHashAlg(), info.orgId, localPrivKeyFile, "")
	require.Nil(t, err)

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.client.sk), os.ModePerm)
	client, err := InitPKSigningMember(ppkProvider.GetHashAlg(), info.orgId, localPrivKeyFile, "")

	return &testPkOrgMember{
		orgId:      info.orgId,
		acProvider: ppkProvider,
		consensus:  consensus,
		admin:      admin,
		client:     client,
	}
}

// ************************************************
// 		Test Case
// ************************************************

func TestParsePublicKey(t *testing.T) {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()
	sk, err := asym.PrivateKeyFromPEM([]byte(ConsensusSK1), nil)
	if err != nil {
		fmt.Println(err)
	}
	commonNodeId, err := helper.CreateLibp2pPeerIdWithPublicKey(sk.PublicKey())
	if err != nil {
		fmt.Println(err)
	}

	pk, err := asym.PublicKeyFromPEM([]byte(ConsensusPK1))
	if err != nil {
		fmt.Println(err)
	}
	openNodeId, err := helper.CreateLibp2pPeerIdWithPublicKey(pk)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("common:", commonNodeId)
	fmt.Println("open:", openNodeId)
}

func TestPermissionedPKGetMemberStatus(t *testing.T) {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	// 构建 store
	ctl := gomock.NewController(t)
	store := mock.NewMockBlockchainStore(ctl)
	store.EXPECT().ReadObject(syscontract.SystemContract_PUBKEY_MANAGE.String(),
		gomock.Any()).AnyTimes().Return(nil, nil)
	logger := &test.GoLogger{}

	// 构建 pwk 的 provider
	ppkProvider, err := newPermissionedPkACProvider(testPermissionedPKChainConfig, testOrg1, store, logger)
	require.Nil(t, err)
	require.NotNil(t, ppkProvider)

	// 查询在配置文件中的 consensus 公钥
	pbMember := &acPb.Member{
		OrgId:      testOrg1,
		MemberType: acPb.MemberType_PUBLIC_KEY,
		MemberInfo: []byte(ConsensusPK1),
	}
	memberStatus, err := ppkProvider.GetMemberStatus(pbMember)
	require.Nil(t, err)
	require.Equal(t, acPb.MemberStatus_NORMAL, memberStatus)

	// 查询不在配置文件中的 consensus 公钥
	pbMember = &acPb.Member{
		OrgId:      testOrg1,
		MemberType: acPb.MemberType_PUBLIC_KEY,
		MemberInfo: []byte(TestPK9),
	}
	memberStatus, err = ppkProvider.GetMemberStatus(pbMember)
	require.NotNil(t, err)
	require.Equal(t, acPb.MemberStatus_INVALID, memberStatus)
}

// ***************************************************
// 		Verify Message
// ***************************************************

func TestPermissionedPKVerifyP2PPrincipal(t *testing.T) {
	testPkOrgMember := testInitPermissionedPKFunc(t)
	orgMemberInfo1 := testPkOrgMember[testOrg1]
	orgMemberInfo2 := testPkOrgMember[testOrg2]

	var (
		endorsement *common.EndorsementEntry
		principal   protocol.Principal
		ok          bool
		err         error
	)

	//【valid】
	endorsement, err = testPermissionedPKCreateEndorsementEntry(
		orgMemberInfo1, protocol.RoleConsensusNode, testPKHashType, []byte(testMsg))
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	principal, err = orgMemberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameP2p, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = orgMemberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】
	endorsement, err = testPermissionedPKCreateEndorsementEntry(
		orgMemberInfo2, protocol.RoleAdmin, testPKHashType, []byte(testMsg))
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	principal, err = orgMemberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameP2p, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = orgMemberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
}

func TestPermissionedPKVerifyConsensusPrincipal(t *testing.T) {
	testPkOrgMember := testInitPermissionedPKFunc(t)
	orgMemberInfo1 := testPkOrgMember[testOrg1]

	var (
		endorsement *common.EndorsementEntry
		principal   protocol.Principal
		ok          bool
		err         error
	)

	//【valid】
	endorsement, err = testPermissionedPKCreateEndorsementEntry(
		orgMemberInfo1, protocol.RoleConsensusNode, testPKHashType, []byte(testMsg))
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	principal, err = orgMemberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameConsensusNode, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = orgMemberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】 the role of signer is not matched with principal
	endorsement, err = testPermissionedPKCreateEndorsementEntry(
		orgMemberInfo1, protocol.RoleAdmin, testPKHashType, []byte(testMsg))
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	principal, err = orgMemberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameConsensusNode, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = orgMemberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)
}

func testPermissionedPKCreateEndorsementEntry(
	testOrgPKMember *testPkOrgMember,
	roleType protocol.Role,
	hashType string,
	msg []byte) (*common.EndorsementEntry, error) {
	var (
		sigResource    []byte
		err            error
		signerResource *acPb.Member
	)
	switch roleType {
	case protocol.RoleConsensusNode:
		sigResource, err = testOrgPKMember.consensus.Sign(hashType, msg)
		if err != nil {
			return nil, err
		}

		signerResource, err = testOrgPKMember.consensus.GetMember()
		if err != nil {
			return nil, err
		}
	case protocol.RoleAdmin:
		sigResource, err = testOrgPKMember.admin.Sign(hashType, msg)
		if err != nil {
			return nil, err
		}

		signerResource, err = testOrgPKMember.admin.GetMember()
		if err != nil {
			return nil, err
		}
	case protocol.RoleClient:
		sigResource, err = testOrgPKMember.client.Sign(hashType, msg)
		if err != nil {
			return nil, err
		}

		signerResource, err = testOrgPKMember.client.GetMember()
		if err != nil {
			return nil, err
		}
	}

	return &common.EndorsementEntry{
		Signer:    signerResource,
		Signature: sigResource,
	}, nil
}

// ***************************************************
// 		Verify Transaction
// ***************************************************

func TestPWK_VerifyPolicySelf(t *testing.T) {
	testPWK_VerifyPoliicySelf(blockVersion220, t)

	testPWK_VerifyPoliicySelf(blockVersion2320, t)

	testPWK_VerifyPoliicySelf(blockVersion2330, t)
}

func testPWK_VerifyPoliicySelf(blockVersion uint32, t *testing.T) {
	// initialize
	testPkOrgMember := testInitPermissionedPKFunc(t)
	orgMemberInfo1 := testPkOrgMember[testOrg1]
	orgMemberInfo2 := testPkOrgMember[testOrg2]

	var (
		tx  *common.Transaction
		err error
		ok  bool
	)

	//【valid】test case
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】target org not matched
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n\n", err)

	//【invalid】 sender role not matched
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n\n", err)
}

func TestPWK_VerifyPolicyMajority(t *testing.T) {
	testPWK_VerifyPolicyMajority(blockVersion220, t)

	testPWK_VerifyPolicyMajority(blockVersion2320, t)

	testPWK_VerifyPolicyMajority2330(blockVersion2330, t)
}

func testPWK_VerifyPolicyMajority(blockVersion uint32, t *testing.T) {
	var (
		err error
		ok  bool
		tx  *common.Transaction
	)
	testPkOrgMember := testInitPermissionedPKFunc(t)
	orgMemberInfo1 := testPkOrgMember[testOrg1]
	orgMemberInfo2 := testPkOrgMember[testOrg2]
	orgMemberInfo3 := testPkOrgMember[testOrg3]
	orgMemberInfo4 := testPkOrgMember[testOrg4]

	//【valid】test case
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo4.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo2.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】 test number of endorsers not enough
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)

	// 【invalid】 test role of endorsers not match
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n\n", err)
}

func testPWK_VerifyPolicyMajority2330(blockVersion uint32, t *testing.T) {
	var (
		err error
		ok  bool
		tx  *common.Transaction
	)
	testPkOrgMember := testInitPermissionedPKFunc(t)
	orgMemberInfo1 := testPkOrgMember[testOrg1]
	orgMemberInfo2 := testPkOrgMember[testOrg2]
	orgMemberInfo3 := testPkOrgMember[testOrg3]
	orgMemberInfo4 := testPkOrgMember[testOrg4]

	//【valid】test case
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo4.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo2.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】 test number of endorsers not enough
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	// 【invalid】 test role of endorsers not match
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ORG_ADD.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, orgMemberInfo1.client)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, orgMemberInfo3.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = orgMemberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
}

package accesscontrol

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chainmaker.org/chainmaker/utils/v2"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"github.com/stretchr/testify/require"
)

// ******************************************************
// 		Init PK Member
// ******************************************************

type testPkMember struct {
	acProvider protocol.AccessControlProvider
	consensus  protocol.SigningMember
	admin      protocol.SigningMember
	client     protocol.SigningMember
}

type testPkMemberInfo struct {
	consensus *testPKInfo
	admin     *testPKInfo
	client    *testPKInfo
}

var testPKMemberInfoMap = map[string]*testPkMemberInfo{
	testOrg1: {
		consensus: testConsensus1PKInfo,
		admin:     testAdmin1PKInfo,
		client:    testClient1PKInfo,
	},
	testOrg2: {
		consensus: testConsensus2PKInfo,
		admin:     testAdmin2PKInfo,
		client:    testClient2PKInfo,
	},
	testOrg3: {
		consensus: testConsensus3PKInfo,
		admin:     testAdmin3PKInfo,
		client:    testClient3PKInfo,
	},
	testOrg4: {
		consensus: testConsensus4PKInfo,
		admin:     testAdmin4PKInfo,
		client:    testClient4PKInfo,
	},
}

func testInitPublicPKFunc(t *testing.T) map[string]*testPkMember {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	var testPkMember = make(map[string]*testPkMember, len(testPKMemberInfoMap))
	for orgId, info := range testPKMemberInfoMap {
		testPkMember[orgId] = initPKMember(t, info)
	}
	test1PublicPKACProvider = testPkMember[testOrg1].acProvider
	test2PublicPKACProvider = testPkMember[testOrg2].acProvider
	return testPkMember
}

func initPKMember(t *testing.T, info *testPkMemberInfo) *testPkMember {
	td, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()
	logger := &test.GoLogger{}

	pkProvider, err := newPkACProvider(testPublicPKChainConfig, nil, logger)
	require.Nil(t, err)
	require.NotNil(t, pkProvider)

	localPrivKeyFile := filepath.Join(td, "public.key")

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.consensus.sk), os.ModePerm)
	require.Nil(t, err)
	consensus, err := InitPKSigningMember(pkProvider.GetHashAlg(), "", localPrivKeyFile, "")
	require.Nil(t, err)

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.admin.sk), os.ModePerm)
	require.Nil(t, err)
	admin, err := InitPKSigningMember(pkProvider.GetHashAlg(), "", localPrivKeyFile, "")
	require.Nil(t, err)

	err = ioutil.WriteFile(localPrivKeyFile, []byte(info.client.sk), os.ModePerm)
	require.Nil(t, err)
	client, err := InitPKSigningMember(pkProvider.GetHashAlg(), "", localPrivKeyFile, "")
	require.Nil(t, err)

	return &testPkMember{
		acProvider: pkProvider,
		consensus:  consensus,
		admin:      admin,
		client:     client,
	}
}

// **********************************************************
// 	Verify Message
// **********************************************************

func TestPublicPKVerifyConsensusPrincipal(t *testing.T) {
	testPkMember := testInitPublicPKFunc(t)
	memberInfo1 := testPkMember[testOrg1]

	var (
		endorsement *common.EndorsementEntry
		principal   protocol.Principal
		ok          bool
		err         error
	)

	//【valid】
	// 1) 让节点(ConsensusNode)为消息(testMsg)做个背书(endorsement)
	endorsement, err = testPublicPKCreateEndorsementEntry(
		memberInfo1, protocol.RoleConsensusNode, testPKHashType, testMsg)
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	// 2) 校验 ConsensusNode 做的背书是否符合访问策略
	principal, err = memberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameConsensusNode, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = memberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】
	// 1）让用户(Admin1)为消息(testMsg)做个背书(endorsement)
	endorsement, err = testPublicPKCreateEndorsementEntry(memberInfo1, protocol.RoleAdmin, testPKHashType, testMsg)
	require.Nil(t, err)
	require.NotNil(t, endorsement)

	// 2) 校验用户(Admin1)做的背书是否符合访问策略
	principal, err = memberInfo1.acProvider.CreatePrincipal(
		protocol.ResourceNameConsensusNode, []*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err = memberInfo1.acProvider.VerifyMsgPrincipal(principal, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)
}

func testPublicPKCreateEndorsementEntry(testPKMember *testPkMember, roleType protocol.Role, hashType, msg string) (*common.EndorsementEntry, error) {
	var (
		sigResource    []byte
		err            error
		signerResource *acPb.Member
	)
	switch roleType {
	case protocol.RoleConsensusNode:
		sigResource, err = testPKMember.consensus.Sign(hashType, []byte(msg))
		if err != nil {
			return nil, err
		}

		signerResource, err = testPKMember.consensus.GetMember()
		if err != nil {
			return nil, err
		}
	case protocol.RoleAdmin:
		sigResource, err = testPKMember.admin.Sign(hashType, []byte(msg))
		if err != nil {
			return nil, err
		}

		signerResource, err = testPKMember.admin.GetMember()
		if err != nil {
			return nil, err
		}
	case protocol.RoleClient:
		sigResource, err = testPKMember.client.Sign(hashType, []byte(msg))
		if err != nil {
			return nil, err
		}

		signerResource, err = testPKMember.client.GetMember()
		if err != nil {
			return nil, err
		}
	}

	return &common.EndorsementEntry{
		Signer:    signerResource,
		Signature: sigResource,
	}, nil
}

// TestPublicPKVerifyAnyPolicy
func TestPublicPKVerifyAnyPolicy(t *testing.T) {
	testPkMember := testInitPublicPKFunc(t)
	memberInfo1 := testPkMember[testOrg1]

	var (
		tx  *common.Transaction
		err error
		ok  bool
	)
	//【valid】
	tx = testCreateTx(syscontract.SystemContract_CONTRACT_MANAGE.String(),
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err = memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】
	tx = testCreateTx(syscontract.SystemContract_CONTRACT_MANAGE.String(),
		syscontract.ContractManageFunction_UPGRADE_CONTRACT.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)
}

func TestPublicPKVerifyMajorityPolicy(t *testing.T) {
	testPkMember := testInitPublicPKFunc(t)
	memberInfo1 := testPkMember[testOrg1]
	memberInfo2 := testPkMember[testOrg2]
	memberInfo3 := testPkMember[testOrg3]
	memberInfo4 := testPkMember[testOrg4]
	var (
		tx  *common.Transaction
		err error
		ok  bool
	)

	//【valid】
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.client)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo1.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo3.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err = memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.Nil(t, err)
	require.Equal(t, true, ok)

	//【invalid】
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.client)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo3.admin)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)

	//【invalid】
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_TRUST_ROOT_UPDATE.String(),
		"test-txid-12345")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.client)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo2.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo3.admin)
	require.Nil(t, err)
	err = testAppendEndorsement2Tx(tx, testPKHashType, memberInfo4.client)
	require.Nil(t, err)

	resourceName = utils.GetTxResourceName(tx)
	ok, err = memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)
}

func TestPublicPKVerifyForbiddenPrincipal(t *testing.T) {
	testPkMember := testInitPublicPKFunc(t)
	memberInfo1 := testPkMember[testOrg1]

	var (
		tx  *common.Transaction
		err error
	)

	//Forbidden
	tx = testCreateTx(
		syscontract.SystemContract_CHAIN_CONFIG.String(),
		syscontract.ChainConfigFunction_NODE_ID_ADD.String(),
		"test-txid-123456")

	err = testAppendSender2Tx(tx, testPKHashType, memberInfo1.admin)
	require.Nil(t, err)

	resourceName := utils.GetTxResourceName(tx)
	ok, err := memberInfo1.acProvider.VerifyTxPrincipal(tx, resourceName, blockVersion2330)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
	fmt.Printf("【invalid case】: err = %v \n", err)
}

/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"github.com/stretchr/testify/require"
)

func TestInitAccessControlService(t *testing.T) {
	logger := test.NewTestLogger(t)
	acServices := initAccessControlService(testHashType, protocol.PermissionedWithCert, config.AddrType_CHAINMAKER, nil, logger)
	acServices.initResourcePolicy(testChainConfig.ResourcePolicies, testOrg1)
	require.NotNil(t, acServices)

	// check resource name policy number
	resourceNamePolicyNum := 0
	acServices.resourceNamePolicyMap.Range(func(key, val interface{}) bool {
		resourceNamePolicyNum++
		return true
	})
	require.Equal(t, resourceNamePolicyNum, 60)

	// check sender policy number
	senderPolicyNum := 0
	acServices.senderPolicyMap.Range(func(key, val interface{}) bool {
		senderPolicyNum++
		return true
	})
	require.Equal(t, senderPolicyNum, 2)

	// tx_type policy number
	txTypePolicyNum := 0
	acServices.txTypePolicyMap.Range(func(key, val interface{}) bool {
		txTypePolicyNum++
		return true
	})
	require.Equal(t, txTypePolicyNum, 5)

	// msg_type policy number
	msgTypePolicyNum := 0
	acServices.msgTypePolicyMap.Range(func(key, val interface{}) bool {
		msgTypePolicyNum++
		return true
	})
	require.Equal(t, msgTypePolicyNum, 2)

	// latest policy number
	latestPolicyNum := 0
	acServices.latestPolicyMap.Range(func(key, val interface{}) bool {
		latestPolicyNum++
		return true
	})
	require.Equal(t, latestPolicyNum, 1)
}

func TestValidateResourcePolicy(t *testing.T) {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	logger := test.NewTestLogger(t)
	acServices := initAccessControlService(testHashType, protocol.Identity, config.AddrType_CHAINMAKER, nil, logger)
	acServices.initResourcePolicy(testChainConfig.ResourcePolicies, testOrg1)
	require.NotNil(t, acServices)

	resourcePolicy := &config.ResourcePolicy{
		ResourceName: "INIT_CONTRACT",
		Policy:       &pbac.Policy{Rule: "ANY"},
	}
	ok := acServices.validateResourcePolicy(resourcePolicy)
	require.Equal(t, true, ok)

	resourcePolicy = &config.ResourcePolicy{
		ResourceName: "P2P",
		Policy:       &pbac.Policy{Rule: "ANY"},
	}
	ok = acServices.validateResourcePolicy(resourcePolicy)
	require.Equal(t, false, ok)
}

func TestCertMemberInfo(t *testing.T) {
	td, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()

	logger := test.NewTestLogger(t)
	acServices := initAccessControlService(testHashType, protocol.Identity, config.AddrType_CHAINMAKER, nil, logger)
	acServices.initResourcePolicy(testChainConfig.ResourcePolicies, testOrg1)
	require.NotNil(t, acServices)

	pbMember := &pbac.Member{
		OrgId:      testOrg1,
		MemberType: pbac.MemberType_CERT,
		MemberInfo: []byte(testConsensusSignOrg1.cert),
	}
	member, err := acServices.newCertMember(pbMember)
	require.Nil(t, err)
	require.Equal(t, testOrg1, member.GetOrgId())
	require.Equal(t, testConsensusRole, member.GetRole())
	require.Equal(t, testConsensusCN, member.GetMemberId())

	localPrivKeyFile := filepath.Join(td, tempOrg1KeyFileName)
	localCertFile := filepath.Join(td, tempOrg1CertFileName)
	err = ioutil.WriteFile(localPrivKeyFile, []byte(testConsensusSignOrg2.sk), os.ModePerm)
	require.Nil(t, err)
	err = ioutil.WriteFile(localCertFile, []byte(testConsensusSignOrg2.cert), os.ModePerm)
	require.Nil(t, err)
	signingMember, err := InitCertSigningMember(testChainConfig, testOrg2, localPrivKeyFile, "", localCertFile)
	require.Nil(t, err)
	require.NotNil(t, signingMember)
	signRead, err := signingMember.Sign(testChainConfig.Crypto.Hash, []byte(testMsg))
	require.Nil(t, err)
	err = signingMember.Verify(testChainConfig.Crypto.Hash, []byte(testMsg), signRead)
	require.Nil(t, err)

	cachedMember := &memberCached{
		member:    member,
		certChain: nil,
	}
	mem, err := member.GetMember()
	require.Nil(t, err)
	require.NotNil(t, mem)
	acServices.addMemberToCache(mem, cachedMember)
	memCache, ok := acServices.lookUpMemberInCache(string(mem.MemberInfo))
	require.Equal(t, true, ok)
	require.Equal(t, cachedMember, memCache)
}

func TestVerifyPrincipalPolicy(t *testing.T) {
	_, cleanFunc, err := createTempDirWithCleanFunc()
	require.Nil(t, err)
	defer cleanFunc()
	hashType := testHashType
	logger := test.NewTestLogger(t)
	acServices := initAccessControlService(testHashType, protocol.Identity, config.AddrType_CHAINMAKER, nil, logger)
	acServices.initResourcePolicy(testChainConfig.ResourcePolicies, testOrg1)
	acServices.setVerifyOptionsFunc(func() *bcx509.VerifyOptions {
		return &bcx509.VerifyOptions{}
	})
	require.NotNil(t, acServices)

	var orgMemberMap = make(map[string]*orgMember, len(orgMemberInfoMap))
	for orgId, info := range orgMemberInfoMap {
		orgMemberMap[orgId] = initOrgMember(t, info)
	}

	org1Member := orgMemberMap[testOrg1]

	org1AdminSig, err := org1Member.admin.Sign(hashType, []byte(testMsg))
	require.Nil(t, err)
	org1AdminPb, err := org1Member.admin.GetMember()
	require.Nil(t, err)
	endorsement := &common.EndorsementEntry{
		Signer:    org1AdminPb,
		Signature: org1AdminSig,
	}
	policy, err := acServices.lookUpPolicy2320(common.TxType_QUERY_CONTRACT.String())
	require.Nil(t, err)
	require.Equal(t, policyRead.GetPbPolicy(), policy)

	principal, err := acServices.createPrincipal(common.TxType_QUERY_CONTRACT.String(),
		[]*common.EndorsementEntry{endorsement}, []byte(testMsg))
	require.Nil(t, err)

	ok, err := acServices.verifyPrincipalPolicy(principal, principal, policyRead)
	require.NotNil(t, err)
	require.Equal(t, false, ok)
}

func walkResourceName(key, value interface{}) bool {
	fmt.Printf("%v \t [resource_name] \t %v \n", key, value)
	return true
}

func walkExceptional(key, value interface{}) bool {
	fmt.Printf("%v \t [exceptional] \t %v \n", key, value)
	return true
}

func TestCertInitData_2320(t *testing.T) {
	ac := accessControlService{
		resourceNamePolicyMap2320: &sync.Map{},
		exceptionalPolicyMap2320:  &sync.Map{},
	}

	ac.createDefaultResourcePolicyForCert_2320()

	ac.resourceNamePolicyMap2320.Range(walkResourceName)

	ac.exceptionalPolicyMap2320.Range(walkExceptional)
}

func TestCertInitData(t *testing.T) {
	ac := accessControlService{
		txTypePolicyMap:       &sync.Map{},
		msgTypePolicyMap:      &sync.Map{}, // map[string]*policy , messageType  -> *policy
		senderPolicyMap:       &sync.Map{}, // map[string]*policy , resourceName -> *policy
		resourceNamePolicyMap: &sync.Map{}, // map[string]*policy , resourceName -> *policy
	}
	ac.createDefaultResourcePolicyForCert("wx-org.chainmaker.org")

	ac.resourceNamePolicyMap.Range(walkResourceName)

	fmt.Println()

	ac.senderPolicyMap.Range(walkExceptional)
}

func TestPwkInitData_2320(t *testing.T) {
	ac := accessControlService{
		resourceNamePolicyMap2320: &sync.Map{},
		exceptionalPolicyMap2320:  &sync.Map{},
	}
	ac.createDefaultResourcePolicyForPK_2320()

	ac.resourceNamePolicyMap2320.Range(walkResourceName)

	ac.exceptionalPolicyMap2320.Range(walkExceptional)
}

func TestPwkInitData(t *testing.T) {
	ac := accessControlService{
		msgTypePolicyMap:      &sync.Map{},
		txTypePolicyMap:       &sync.Map{},
		resourceNamePolicyMap: &sync.Map{},
		senderPolicyMap:       &sync.Map{},
	}
	ac.createDefaultResourcePolicyForPWK("wx-org.chainmaker.org")

	ac.resourceNamePolicyMap.Range(walkResourceName)

	ac.senderPolicyMap.Range(walkExceptional)
}

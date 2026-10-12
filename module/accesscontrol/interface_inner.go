package accesscontrol

import (
	"fmt"
	"sync"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"

	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
)

type acProvider220 interface {
	acProviderBase

	lookUpPolicy220(resourceName string) (*acPb.Policy, error)
	lookUpExceptionalPolicy220(resourceName string) (*acPb.Policy, error)
	lookUpPolicyByResourceName220(resourceName string) (*policy, error)
	getValidEndorsements220(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error)
}

type acProvider2320 interface {
	acProviderBase

	lookUpPolicy2320(resourceName string) (*acPb.Policy, error)
	lookUpExceptionalPolicy2320(resourceName string) (*acPb.Policy, error)
	lookUpPolicyByResourceName2320(resourceName string) (*policy, error)
	getValidEndorsements2320(principal protocol.Principal) ([]*commonPb.EndorsementEntry, error)
}

type acProvider interface {
	acProviderBase

	lookUpPolicyByTxType(txType string, blockVersion uint32) (*policy, error)
	lookUpPolicyByMsgType(msgType string, blockVersion uint32) (*policy, error)
	findFromSenderPolicies(resourceName string, blockVersion uint32) (*policy, error)
	findFromEndorsementsPolicies(resourceName string, blockVersion uint32) (*policy, error)

	getValidEndorsements(principal protocol.Principal, blockVersion uint32) ([]*commonPb.EndorsementEntry, error)
}

func lookUpPolicyByTxType(txType string, blockVersion uint32,
	latestPolicyMap *sync.Map, policyMap *sync.Map) (*policy, error) {

	if blockVersion < blockVersion2330 {
		panic(fmt.Errorf("bad blockVersion(%d) for calling blockVersion specified func(>=2030300)", blockVersion))
	}

	if p, ok := latestPolicyMap.Load(txType); ok {
		return p.(*policy), nil
	}
	pol, ok := policyMap.Load(txType)
	if !ok {
		// 没找到，报错
		return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
			"for resource %s", txType)
	}
	return pol.(*policy), nil
}

func lookUpPolicyByMsgType(msgType string, blockVersion uint32,
	latestPolicyMap *sync.Map, policyMap *sync.Map) (*policy, error) {

	if blockVersion < blockVersion2330 {
		panic(fmt.Errorf("bad blockVersion(%d) for calling blockVersion specified func(>=2030300)", blockVersion))
	}

	if p, ok := latestPolicyMap.Load(msgType); ok {
		return p.(*policy), nil
	}
	pol, ok := policyMap.Load(msgType)
	if !ok {
		// 没找到，报错
		return nil, fmt.Errorf("look up access policy failed, did not configure access policy "+
			"for resource %s", msgType)
	}
	return pol.(*policy), nil
}

func findFromSenderPolicies(resourceName string, blockVersion uint32,
	latestPolicyMap *sync.Map, policyMap *sync.Map) (*policy, error) {
	if blockVersion < blockVersion2330 {
		panic(fmt.Errorf("bad blockVersion(%d) for calling blockVersion specified func(>=2030300)", blockVersion))
	}

	//这里不能从latestPolicyMap中取自定义权限
	//比如一个资源设置为权限 Majorty-Admin，那么在sender时校验会失败。
	if pol, ok := policyMap.Load(resourceName); ok {
		return pol.(*policy), nil
	}

	// 没找到，不报错
	return nil, nil
}

func findFromEndorsementsPolicies(resourceName string, blockVersion uint32,
	latestPolicyMap *sync.Map, policyMap *sync.Map) (*policy, error) {
	if blockVersion < blockVersion2330 {
		panic(fmt.Errorf("bad blockVersion(%d) for calling blockVersion specified func(>=2030300)", blockVersion))
	}

	if pol, ok := latestPolicyMap.Load(resourceName); ok {
		return pol.(*policy), nil
	}
	if pol, ok := policyMap.Load(resourceName); ok {
		return pol.(*policy), nil
	}

	// 没找到，不报错
	return nil, nil
}

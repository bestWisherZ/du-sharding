/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package accesscontrol

import (
	"encoding/hex"
	"fmt"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"github.com/gogo/protobuf/proto"
)

var _ msgbus.Subscriber = (*pkACProvider)(nil)

// OnMessage contract event data is a []string, hexToString(proto.Marshal(data))
func (p *pkACProvider) OnMessage(msg *msgbus.Message) {
	switch msg.Topic {
	case msgbus.ChainConfig:
		p.log.Infof("[AC_PK] receive msg, topic: %s", msg.Topic.String())
		p.onMessageChainConfig(msg)
	case msgbus.BlockInfo:
		p.onMessageBlockInfo(msg)
	}

}

func (p *pkACProvider) OnQuit() {

}

// onMessageChainConfig used to handle chain conf message
func (p *pkACProvider) onMessageChainConfig(msg *msgbus.Message) {
	dataStr, _ := msg.Payload.([]string)
	dataBytes, err := hex.DecodeString(dataStr[0])
	if err != nil {
		p.log.Error(err)
		return
	}
	chainConfig := &config.ChainConfig{}
	_ = proto.Unmarshal(dataBytes, chainConfig)

	p.initResourcePolicy(chainConfig.ResourcePolicies)

	p.hashType = chainConfig.GetCrypto().GetHash()
	p.addressType = chainConfig.Vm.AddrType
	err = p.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		err = fmt.Errorf("new public AC provider failed: %s", err.Error())
		p.log.Error(err)
	}

	err = p.initConsensusMember(chainConfig)
	if err != nil {
		err = fmt.Errorf("new public AC provider failed: %s", err.Error())
		p.log.Error(err)
	}
	p.memberCache.Clear()

}

func (p *pkACProvider) onMessageBlockInfo(msg *msgbus.Message) {

	switch blockInfo := msg.Payload.(type) {
	case *commonPb.BlockInfo:
		if blockInfo == nil || blockInfo.Block == nil {
			p.log.Errorf("error message BlockInfo = nil")
			return
		}
		//（set-payer）配置交易 + gas交易
		if len(blockInfo.Block.Txs) > 2 || len(blockInfo.Block.Txs) <= 0 {
			return
		}
		// 是set-payer交易,并且交易执行成功
		if blockInfo.Block.Txs[0].Payload.ContractName == syscontract.SystemContract_ACCOUNT_MANAGER.String() &&
			blockInfo.Block.Txs[0].Payload.Method == syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String() &&
			blockInfo.Block.Txs[0].Result.Code == commonPb.TxStatusCode_SUCCESS {
			p.handleSetPayer(blockInfo)
		} else if (blockInfo.Block.Txs[0].Payload.ContractName == syscontract.SystemContract_ACCOUNT_MANAGER.String() &&
			blockInfo.Block.Txs[0].Payload.Method == syscontract.GasAccountFunction_UNSET_CONTRACT_METHOD_PAYER.String()) &&
			blockInfo.Block.Txs[0].Result.Code == commonPb.TxStatusCode_SUCCESS {
			p.handleUnsetPayer(blockInfo)
		}

	default:
		p.log.Errorf("error type(%s)", blockInfo)
	}
}

func (p *pkACProvider) handleSetPayer(blockInfo *commonPb.BlockInfo) {
	//解析交易入参，根据入参更新缓存
	params := &syscontract.SetContractMethodPayerParams{}
	var valueParams, valueEndorsementEntry []byte
	for i, pair := range blockInfo.Block.Txs[0].Payload.Parameters {
		if pair.Key == syscontract.SetContractMethodPayer_PARAMS.String() {
			valueParams = blockInfo.Block.Txs[0].Payload.Parameters[i].Value
		}
		if pair.Key == syscontract.SetContractMethodPayer_ENDORSEMENT.String() {
			valueEndorsementEntry = blockInfo.Block.Txs[0].Payload.Parameters[i].Value
		}
	}
	//获取pk
	pkStr := p.getPK(valueEndorsementEntry)

	_ = proto.Unmarshal(valueParams, params)
	//获取缓存key
	dbKey := utils.PrefixContractMethodPayer
	if params.Method != "" && params.ContractName != "" {
		dbKey += params.ContractName + utils.Separator + params.Method
	} else if params.ContractName != "" {
		dbKey += params.ContractName
	} else {
		p.log.Errorf("err Parameters (%v)", blockInfo.Block.Txs[0].Payload.Parameters)
		return
	}

	p.payerList.Add(dbKey, pkStr)
	p.log.Debugf("set payer in cache, key=%s, value=%s", dbKey, pkStr)
}

func (p *pkACProvider) getPK(endorsementBytes []byte) string {
	// 获取 payer 签名
	endorsementEntry := commonPb.EndorsementEntry{}
	if err := proto.Unmarshal(endorsementBytes, &endorsementEntry); err != nil {
		p.log.Errorf(err.Error())
		return ""
	}
	signerMember, err := p.NewMember(endorsementEntry.GetSigner())
	if err != nil {
		p.log.Errorf(err.Error())
		return ""
	}
	pk := signerMember.GetPk()
	pkStr, err := pk.String()
	if err != nil {
		p.log.Errorf(err.Error())
		return ""
	}
	return pkStr
}

func (p *pkACProvider) handleUnsetPayer(blockInfo *commonPb.BlockInfo) {
	//解析交易入参，根据入参删除缓存
	var contractName, method string
	for i, pair := range blockInfo.Block.Txs[0].Payload.Parameters {
		if pair.Key == syscontract.UnsetContractMethodPayer_CONTRACT_NAME.String() {
			contractName = string(blockInfo.Block.Txs[0].Payload.Parameters[i].Value)
		} else if pair.Key == syscontract.UnsetContractMethodPayer_METHOD.String() {
			method = string(blockInfo.Block.Txs[0].Payload.Parameters[i].Value)
		}
	}
	//获取缓存key
	dbKey := utils.PrefixContractMethodPayer
	if method != "" && contractName != "" {
		dbKey += contractName + utils.Separator + method
	} else if contractName != "" {
		dbKey += contractName
	} else {
		p.log.Errorf("err Parameters (%v)", blockInfo.Block.Txs[0].Payload.Parameters)
	}

	p.payerList.Remove(dbKey)
	p.log.Debugf("unset payer in cache, key=%s", dbKey)
}

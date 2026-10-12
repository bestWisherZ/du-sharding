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

	"chainmaker.org/chainmaker/common/v2/crypto/asym"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	"github.com/gogo/protobuf/proto"
)

var _ msgbus.Subscriber = (*permissionedPkACProvider)(nil)

// OnMessage contract event data is a []string, hexToString(proto.Marshal(data))
func (pp *permissionedPkACProvider) OnMessage(msg *msgbus.Message) {
	switch msg.Topic {
	case msgbus.ChainConfig:
		pp.acService.log.Infof("[AC_PWK] receive msg, topic: %s", msg.Topic.String())
		pp.onMessageChainConfig(msg)
	case msgbus.PubkeyManageDelete:
		pp.acService.log.Infof("[AC_PWK] receive msg, topic: %s", msg.Topic.String())
		pp.onMessagePublishKeyManageDelete(msg)
	case msgbus.BlockInfo:
		pp.onMessageBlockInfo(msg)
	}

}

func (pp *permissionedPkACProvider) OnQuit() {

}

func (pp *permissionedPkACProvider) onMessageChainConfig(msg *msgbus.Message) {
	dataStr, _ := msg.Payload.([]string)
	dataBytes, err := hex.DecodeString(dataStr[0])
	if err != nil {
		pp.acService.log.Error(err)
		return
	}
	chainConfig := &config.ChainConfig{}
	_ = proto.Unmarshal(dataBytes, chainConfig)

	pp.acService.hashType = chainConfig.GetCrypto().GetHash()

	err = pp.initAdminMembers(chainConfig.TrustRoots)
	if err != nil {
		err = fmt.Errorf("update chainconfig error: %s", err.Error())
		pp.acService.log.Error(err)
	}

	err = pp.initConsensusMember(chainConfig.Consensus.Nodes)
	if err != nil {
		err = fmt.Errorf("update chainconfig error: %s", err.Error())
		pp.acService.log.Error(err)
	}

	pp.acService.initResourcePolicy(chainConfig.ResourcePolicies, pp.localOrg)

	pp.acService.memberCache.Clear()
}

func (pp *permissionedPkACProvider) onMessageBlockInfo(msg *msgbus.Message) {

	switch blockInfo := msg.Payload.(type) {
	case *commonPb.BlockInfo:
		if blockInfo == nil || blockInfo.Block == nil {
			pp.acService.log.Errorf("error message BlockInfo = nil")
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
			pp.handleSetPayer(blockInfo)
		} else if (blockInfo.Block.Txs[0].Payload.ContractName == syscontract.SystemContract_ACCOUNT_MANAGER.String() &&
			blockInfo.Block.Txs[0].Payload.Method == syscontract.GasAccountFunction_UNSET_CONTRACT_METHOD_PAYER.String()) &&
			blockInfo.Block.Txs[0].Result.Code == commonPb.TxStatusCode_SUCCESS {
			pp.handleUnsetPayer(blockInfo)
		}

	default:
		pp.acService.log.Errorf("error type(%s)", blockInfo)
	}
}

func (pp *permissionedPkACProvider) handleSetPayer(blockInfo *commonPb.BlockInfo) {
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
	pkStr := pp.getPK(valueEndorsementEntry)

	_ = proto.Unmarshal(valueParams, params)
	//获取缓存key
	dbKey := utils.PrefixContractMethodPayer
	if params.Method != "" && params.ContractName != "" {
		dbKey += params.ContractName + utils.Separator + params.Method
	} else if params.ContractName != "" {
		dbKey += params.ContractName
	} else {
		pp.acService.log.Errorf("err Parameters (%v)", blockInfo.Block.Txs[0].Payload.Parameters)
	}

	pp.payerList.Add(dbKey, pkStr)
	pp.acService.log.Debugf("set payer in cache, key=%s, value=%s", dbKey, params.PayerAddress)
}

func (pp *permissionedPkACProvider) getPK(endorsementBytes []byte) string {
	// 获取 payer 签名
	endorsementEntry := commonPb.EndorsementEntry{}
	if err := proto.Unmarshal(endorsementBytes, &endorsementEntry); err != nil {
		pp.acService.log.Errorf(err.Error())
		return ""
	}
	signerMember, err := pp.NewMember(endorsementEntry.GetSigner())
	if err != nil {
		pp.acService.log.Errorf(err.Error())
		return ""
	}
	pk := signerMember.GetPk()
	pkStr, err := pk.String()
	if err != nil {
		pp.acService.log.Errorf(err.Error())
		return ""
	}
	return pkStr
}

func (pp *permissionedPkACProvider) handleUnsetPayer(blockInfo *commonPb.BlockInfo) {
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
		pp.acService.log.Errorf("err Parameters (%v)", blockInfo.Block.Txs[0].Payload.Parameters)
	}

	pp.payerList.Remove(dbKey)
	pp.acService.log.Debugf("unset payer in cache, key=%s", dbKey)
}

func (pp *permissionedPkACProvider) onMessagePublishKeyManageDelete(msg *msgbus.Message) {
	data, _ := msg.Payload.([]string)
	publishKey := data[1]

	pk, err := asym.PublicKeyFromPEM([]byte(publishKey))
	if err != nil {
		err = fmt.Errorf("delete member cache failed, [%v]", err.Error())
		pp.acService.log.Error(err)
	}
	pkStr, err := pk.String()
	if err != nil {
		err = fmt.Errorf("delete member cache failed, [%v]", err.Error())
		pp.acService.log.Error(err)
	}
	pp.acService.memberCache.Remove(pkStr)
	pp.acService.log.Debugf("The public key was removed from the cache,[%v]", pkStr)
}

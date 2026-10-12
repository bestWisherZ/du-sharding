/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"fmt"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/common/v2/crypto"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	sdkutils "chainmaker.org/chainmaker/sdk-go/v2/utils"
	"chainmaker.org/chainmaker/utils/v2"
)

// StressBuilder 构建不同场景下压测所需请求参数的接口，并返回请求对象
// 非压测请求实现Builder接口以满足参数构建需要
// 目前支持4种实现方式：Invoke、Query、Create和Upgrade每个类型都定义了自己的Build方法，用于生成不同类型的交易请求：
// - Invoke：构建调用智能合约的交易请求。
// - Query：构建查询智能合约的交易请求。
// - Create：构建部署（安装）智能合约的交易请求。
// - Upgrade：构建升级智能合约的交易请求。
type StressBuilder interface {
	Build(requestId int64, index int) (*commonPb.TxRequest, error)
}

type Invoke struct{}

// Build 实现StressBuilder接口，用于构建调用智能合约的交易请求
func (i Invoke) Build(requestId int64, index int) (*commonPb.TxRequest, error) {
	index = index % nodeNum
	pairs, err := makeKvs(requestId)
	if err != nil {
		return nil, err
	}
	// 如果有ABI路径，则读取ABI数据，并根据ABI调整方法名和参数对
	var abiData *[]byte
	if abiPath != "" {
		abiData = abiCache.Read(abiPath)
		runTime = 5 //evm
	}
	method1, pairs1, err := makePairs(method, abiPath, pairs, commonPb.RuntimeType(runTime), abiData)
	if err != nil {
		return nil, err
	}

	var limit *commonPb.Limit
	if gasLimit > 0 {
		limit = &commonPb.Limit{GasLimit: gasLimit}
	}
	// 构造调用智能合约的Payload
	payload := defaultSdkClients[index].CreatePayload("", commonPb.TxType_INVOKE_CONTRACT, contractName, method1, pairs1, 0, limit)
	if err != nil {
		return nil, err
	}
	var endorsers []*commonPb.EndorsementEntry = nil
	// 这里如果输入了adminKey则默认需要背书， 否则不需要
	if len(adminKeyPaths) > 0 {
		endorsers, err = util.MakeEndorsement(adminKeyPaths, adminCrtPaths, orgIDs, defaultSdkClients[index], payload)
		if err != nil {
			fmt.Printf("MakeEndorsement failed, %s", err)
			return nil, err
		}
	}
	// 构建完整的交易请求
	return defaultSdkClients[index].GenerateTxRequest(payload, endorsers)
}

// makeKvs 生成一组键值对（KeyValuePairs）基于全局配置的规则，用于构造交易请求的Payload。
// 每个键值对的生成会考虑是否需要唯一性、随机性、递增或递减等特性。
// 请求ID（requestId）会被嵌入到满足条件的键值对的值中，以确保唯一性或生成随机数据。
// 全局变量globalPairs定义了一系列键值对的基础配置，包括是否具有唯一性（Unique）、
// 随机生成的比率（RandomRate）、以及整数值的递增或递减（Increase/Decrease）。
// 函数内部通过原子操作来安全地更新全局统计变量，如总发送交易数（totalSentTxs）和
// 随机发送的交易数（totalRandomSentTxs）。
func makeKvs(requestId int64) ([]*commonPb.KeyValuePair, error) {
	var outKvs []*commonPb.KeyValuePair
	// 原子增加总发送交易计数器
	atomic.AddInt64(&totalSentTxs, 1)
	// 遍历全局键值对配置列表
	for _, p := range globalPairs {
		var val []byte
		// 根据配置特性生成键值对的值
		switch {
		case p.Unique:
			// 如果要求唯一性，格式化字符串并加入requestId和当前时间戳
			val = []byte(fmt.Sprintf(templateStr, p.Value, requestId, time.Now().UnixNano()))
		case 0 < p.RandomRate && p.RandomRate < 100:
			// 按照随机率判断是否生成随机数据，如果满足则同样加入requestId和时间戳
			if isRandom(p.RandomRate) {
				val = []byte(fmt.Sprintf(templateStr, p.Value, requestId, time.Now().UnixNano()))
				// 原子增加随机发送交易计数器
				atomic.AddInt64(&totalRandomSentTxs, 1)
			} else {
				val = []byte(p.Value)
			}
		case p.Decrease:
			// 如果配置了递减，锁定并递减整数值，然后使用新值
			p.mu.Lock()
			val = []byte(fmt.Sprintf("%d", p.IntValue))
			p.IntValue--
			p.mu.Unlock()
			atomic.AddInt64(&totalRandomSentTxs, 1)
		case p.Increase:
			// 如果配置了递增，同上，但递增整数值
			p.mu.Lock()
			val = []byte(fmt.Sprintf("%d", p.IntValue))
			p.IntValue++
			p.mu.Unlock()
			atomic.AddInt64(&totalRandomSentTxs, 1)
		case p.ValueFormat != "":
			var err error
			val, err = addFormatValue(p)
			if err != nil {
				return nil, err
			}
		default:
			val = []byte(p.Value)
		}
		// 将键值对添加到输出列表
		outKvs = append(outKvs, &commonPb.KeyValuePair{
			Key:   p.Key,
			Value: val,
		})
	}
	return outKvs, nil
}

// buildRequestParam 是一个辅助函数，用于构建TxRequest的通用部分，如签名、发送者信息等。
// 它接收私钥、组织ID、证书路径、Payload以及背书者列表作为参数，并返回一个完整的TxRequest实例。
func buildRequestParam(sk3 crypto.PrivateKey, orgId, userCrtPath string,
	payload *commonPb.Payload, endorsers []*commonPb.EndorsementEntry) (*commonPb.TxRequest, error) {
	req := &commonPb.TxRequest{}
	sender := &acPb.Member{}
	switch {
	case authType == sdk.Public || authType == sdk.PermissionedWithKey:
		pubKey := sk3.PublicKey()
		memberInfo, err := pubKey.String()
		if err != nil {
			return nil, err
		}
		if authType == sdk.PermissionedWithKey {
			sender.OrgId = orgId
		}
		sender.MemberInfo = []byte(memberInfo)
		sender.MemberType = acPb.MemberType_PUBLIC_KEY
	default:
		sender.OrgId = orgId
		file := fileCache.Read(userCrtPath)
		if useShortCrt {
			certId, err := certCache.Read(userCrtPath, *file, hashAlgo)
			if err != nil {
				return nil, fmt.Errorf("fail to compute the identity for certificate [%v]", err)
			}

			sender.MemberInfo = *certId
			sender.MemberType = acPb.MemberType_CERT_HASH
		} else {
			sender.MemberInfo = *file
		}
	}
	hashType, err := getHashType(hashAlgo)
	if err != nil {
		return nil, err
	}
	req.Payload = payload
	// 拼接后，计算Hash，对hash计算签名
	rawTxBytes, err := utils.CalcUnsignedTxRequestBytes(req)
	if err != nil {
		return nil, err
	}
	signBytes, err := sdkutils.SignPayloadBytesWithHashType(sk3, hashType, rawTxBytes)
	if err != nil {
		return nil, err
	}
	if len(endorsers) > 0 {
		req.Endorsers = endorsers
	}
	req.Sender = &commonPb.EndorsementEntry{Signer: sender, Signature: signBytes}
	return req, nil
}

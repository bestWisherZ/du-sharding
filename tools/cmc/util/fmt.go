/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
)

// FormatBlockInfo 格式化区块信息到map
func FormatBlockInfo(bi *common.BlockInfo) map[string]interface{} {
	m := make(map[string]interface{})
	m["block"] = formatBlock(bi.Block)
	if len(bi.RwsetList) > 0 {
		m["rwset_list"] = formatTxRWSet(bi.RwsetList)
	}
	return m
}

// formatTxRWSet 读写集格式化到map
func formatTxRWSet(tr []*common.TxRWSet) interface{} {
	arr := make([]interface{}, 0)
	for _, tx := range tr {
		m := make(map[string]interface{})
		m["tx_id"] = tx.TxId
		m["tx_reads"] = formatTxReads(tx.TxReads)
		m["tx_writes"] = formatTxWrites(tx.TxWrites)
		arr = append(arr, m)
	}
	return arr
}

// formatTxReads 格式化读集
func formatTxReads(reads []*common.TxRead) []interface{} {
	arr := make([]interface{}, 0)
	for _, read := range reads {
		m := make(map[string]interface{})
		m["key"] = string(read.Key)
		var dest []byte
		if len(read.Value) > 0 {
			_, err := base64.StdEncoding.Decode(dest, read.Value)
			if err != nil {
				fmt.Println(err)
				continue
			}
			m["value"] = string(dest)
		} else {
			m["value"] = ""
		}
		m["contract_name"] = read.ContractName
		m["version"] = formatKeyVersion(read.Version)
		arr = append(arr, m)
	}
	return arr
}

// formatKeyVersion 格式化key version
func formatKeyVersion(kv *common.KeyVersion) interface{} {
	m := make(map[string]interface{})
	m["ref_tx_id"] = kv.RefTxId
	m["ref_offset"] = kv.RefOffset
	return m
}

// 格式化写集
func formatTxWrites(writes []*common.TxWrite) []interface{} {
	arr := make([]interface{}, 0)
	for _, write := range writes {
		m := make(map[string]interface{})
		m["key"] = string(write.Key)
		var dest []byte
		if len(write.Value) > 0 {
			_, err := base64.StdEncoding.Decode(dest, write.Value)
			if err != nil {
				fmt.Println(err)
				continue
			}
			m["value"] = string(dest)
		} else {
			m["value"] = ""
		}
		m["contract_name"] = write.ContractName
		arr = append(arr, m)
	}
	return arr
}

// formatBlock 格式化区块
func formatBlock(b *common.Block) map[string]interface{} {
	m := make(map[string]interface{})
	m["header"] = FormatHeader(b.Header)
	m["dag"] = b.Dag
	m["txs"] = FormatTxs(b.Txs)
	m["additional_data"] = b.AdditionalData
	return m
}

// FormatHeader 格式化区块头
func FormatHeader(h *common.BlockHeader) map[string]interface{} {
	m := make(map[string]interface{})
	m["block_version"] = h.BlockVersion
	m["block_type"] = h.BlockType
	m["chain_id"] = h.ChainId
	m["block_height"] = h.BlockHeight
	m["block_hash"] = fmt.Sprintf("%x", h.BlockHash)
	m["pre_block_hash"] = fmt.Sprintf("%x", h.PreBlockHash)
	m["pre_conf_height"] = h.PreConfHeight
	m["tx_count"] = h.TxCount
	m["tx_root"] = fmt.Sprintf("%x", h.TxRoot)
	m["dag_hash"] = fmt.Sprintf("%x", h.DagHash)
	m["rw_set_root"] = fmt.Sprintf("%x", h.RwSetRoot)
	m["block_timestamp"] = h.BlockTimestamp
	m["consensus_args"] = string(h.ConsensusArgs)
	m["proposer"] = formatMember(h.Proposer)
	m["signature"] = base64.StdEncoding.EncodeToString(h.Signature)
	return m
}

// FormatTxs 格式化交易
func FormatTxs(h []*common.Transaction) interface{} {
	arr := make([]interface{}, len(h))
	for _, v := range h {
		m := make(map[string]interface{})
		m["payload"] = formatPayload(v.Payload)
		m["sender"] = formatSender(v.Sender)
		m["endorsers"] = formatEndorsers(v.Endorsers)
		m["result"] = formatResult(v.Result)
		m["payer"] = formatSender(v.Payer)
		arr = append(arr, m)
	}
	return arr
}

// formatPayload 格式化交易的payload
func formatPayload(h *common.Payload) map[string]interface{} {
	m := make(map[string]interface{})
	m["chain_id"] = h.ChainId
	m["tx_type"] = h.TxType
	m["tx_id"] = h.TxId
	m["timestamp"] = h.Timestamp
	m["expiration_time"] = h.ExpirationTime
	m["contract_name"] = h.ContractName
	m["method"] = h.Method
	m["parameters"] = FormatParameters(h.Parameters)
	m["sequence"] = h.Sequence
	m["parameters"] = FormatParameters(h.Parameters)
	return m
}

// formatSender 格式化发送对象
func formatSender(e *common.EndorsementEntry) map[string]interface{} {
	if e == nil {
		return nil
	}
	m := make(map[string]interface{})
	m["signer"] = formatMember(e.Signer)
	m["signature"] = base64.StdEncoding.EncodeToString(e.Signature)
	return m
}

// formatEndorsers 格式化签名
func formatEndorsers(e []*common.EndorsementEntry) interface{} {
	arr := make([]interface{}, len(e))
	for _, v := range e {
		m := make(map[string]interface{})
		m["signer"] = formatMember(v.Signer)
		m["signature"] = fmt.Sprintf("%x", v.Signature)
		arr = append(arr, m)
	}
	return arr
}

// formatMember 格式化member对象
func formatMember(mem *accesscontrol.Member) map[string]interface{} {
	if mem == nil {
		return nil
	}
	m := make(map[string]interface{})
	m["org_id"] = mem.OrgId
	m["member_type"] = mem.MemberType
	m["member_info"] = base64.StdEncoding.EncodeToString(mem.MemberInfo)
	return m
}

// formatResult 格式化交易的执行结果
func formatResult(r *common.Result) map[string]interface{} {
	m := make(map[string]interface{})
	m["code"] = r.Code
	m["contract_result"] = formatContractResult(r.ContractResult)
	m["rw_set_hash"] = fmt.Sprintf("%x", r.RwSetHash)
	m["message"] = r.Message
	return m
}

// formatContractResult 格式化合约的result
func formatContractResult(cr *common.ContractResult) map[string]interface{} {
	m := make(map[string]interface{})
	m["code"] = cr.Code
	m["result"] = base64.StdEncoding.EncodeToString(cr.Result)
	m["message"] = cr.Message
	m["gas_used"] = cr.GasUsed
	m["contract_event"] = formatContractEvent(cr.ContractEvent)
	return m
}

// formatContractEvent 格式化合约事件
func formatContractEvent(ce []*common.ContractEvent) interface{} {
	arr := make([]interface{}, 0)
	for _, v := range ce {
		m := make(map[string]interface{})
		m["topic"] = v.Topic
		m["tx_id"] = v.TxId
		m["contract_name"] = v.ContractName
		m["contract_version"] = v.ContractVersion
		m["event_data"] = v.EventData
		arr = append(arr, m)
	}
	return arr
}

// formatLimit 格式化Limit
func formatLimit(l *common.Limit) map[string]interface{} {
	m := make(map[string]interface{})
	if l == nil {
		return nil
	}
	m["gas_limit"] = l.GasLimit
	return m
}

type kvParam struct {
	key   string      `json:"key"`
	value interface{} `json:"value"`
}

// FormatParameters 格式化用户自定义参数
func FormatParameters(p []*common.KeyValuePair) interface{} {
	arr := make([]kvParam, 0)
	for _, kv := range p {
		if kv.Key == "CHAIN_CONFIG" {
			arr = append(arr, kvParam{
				key:   kv.Key,
				value: string(kv.Value),
			})
		}
	}
	return arr
}

// PrintJson 转换成json格式并输出到控制台
func PrintJson(info interface{}) {
	j, err := json.Marshal(info)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(j))
}

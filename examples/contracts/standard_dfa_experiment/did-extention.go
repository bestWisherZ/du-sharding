/*
 Copyright (C) BABEC. All rights reserved.
 Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

 SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"errors"
	"strconv"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/pb/protogo"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/standard"
)

// DidContract DID合约go接口
type DidContract struct {
	contractName string
}

// DidMethod 查询DID方法
func (d *DidContract) DidMethod() string {
	result := sdk.Instance.CallContract(d.contractName, "DidMethod", nil)
	return (string)(result.Payload)
}

// IsValidDid 判断DID是否合法
func (d *DidContract) IsValidDid(did string) (bool, error) {
	resp := sdk.Instance.CallContract(d.contractName, "IsValidDid", map[string][]byte{"did": []byte(did)})
	return Response2Bool(resp)
}

// AddDidDocument 添加DID文档
func (d *DidContract) AddDidDocument(didDocument string) error {
	//TODO implement me
	panic("implement me")
}

// GetDidDocument 根据DID获取DID文档
func (d *DidContract) GetDidDocument(did string) (string, error) {
	resp := sdk.Instance.CallContract(d.contractName, "GetDidDocument", map[string][]byte{"did": []byte(did)})
	return Response2String(resp)
}

// GetDidByPubkey 根据公钥获取DID
func (d *DidContract) GetDidByPubkey(pk string) (string, error) {
	resp := sdk.Instance.CallContract(d.contractName, "GetDidByPubkey", map[string][]byte{"pk": []byte(pk)})
	return Response2String(resp)
}

// GetDidByAddress 根据地址获取DID
func (d *DidContract) GetDidByAddress(address string) (string, error) {
	resp := sdk.Instance.CallContract(d.contractName, "GetDidByAddress", map[string][]byte{"address": []byte(address)})
	return Response2String(resp)
}

// VerifyVc 验证vc
func (d *DidContract) VerifyVc(vcJson string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// VerifyVp 验证vp
func (d *DidContract) VerifyVp(vpJson string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// EmitSetDidDocumentEvent 发送添加DID文档事件
func (d *DidContract) EmitSetDidDocumentEvent(did string, didDocument string) {
	//TODO implement me
	panic("implement me")
}

// RevokeVc 撤销vc，因为vc是隐私，所以vcID可以使用vc hash从而不暴露隐私
func (d *DidContract) RevokeVc(vcID string) error {
	//TODO implement me
	panic("implement me")
}

// GetRevokedVcList 获取撤销vc列表
func (d *DidContract) GetRevokedVcList(vcIDSearch string, start int, count int) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

// EmitRevokeVcEvent 发送撤销vc事件
func (d *DidContract) EmitRevokeVcEvent(vcID string) {
	//TODO implement me
	panic("implement me")
}

var _ standard.CMDID = (*DidContract)(nil)

// Response2String 将Response转换为string
func Response2String(resp protogo.Response) (string, error) {
	if resp.Status != sdk.OK {
		return "", errors.New(resp.Message)
	}
	return string(resp.Payload), nil
}

// Response2Bool 将Response转换为bool
func Response2Bool(resp protogo.Response) (bool, error) {
	if resp.Status != sdk.OK {
		return false, errors.New(resp.Message)
	}
	return strconv.ParseBool(string(resp.Payload))
}

// GetSenderDid 获取发送者DID
func (d *DidContract) GetSenderDid() (string, error) {
	pk, err := sdk.Instance.GetSenderPk()
	if err != nil {
		return "", err
	}
	return d.GetDidByPubkey(pk)
}

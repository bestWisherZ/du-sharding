/*
 Copyright (C) BABEC. All rights reserved.
 Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

 SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/pb/protogo"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sandbox"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/safemath"
)

const (
	pName        = "name"
	pSymbol      = "symbol"
	pDecimals    = "decimals"
	pAccount     = "account"
	pAmount      = "amount"
	pTotalSupply = "totalSupply"
	pFrom        = "from"
	pTo          = "to"
	pSpender     = "spender"
	pOwner       = "owner"
)

func main() {
	contract := NewCmdfaContract()
	err := sandbox.Start(contract)
	if err != nil {
		sdk.Instance.Errorf(err.Error())
	}
}

////////////////////////////////Helper//////////////////////////////////

// ReturnUint256 封装返回SafeUint256类型为Response，如果有error则忽略num，封装error
// @param num
// @param err
// @return Response
func ReturnUint256(num *safemath.SafeUint256, err error) protogo.Response {
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success([]byte(num.ToString()))
}

// ReturnString 封装返回string类型为Response，如果有error则忽略str，封装error
// @param str
// @param err
// @return Response
func ReturnString(str string, err error) protogo.Response {
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success([]byte(str))
}

// ReturnBytes 封装返回[]byte类型为Response，如果有error则忽略str，封装error
func ReturnBytes(str []byte, err error) protogo.Response {
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success(str)
}

// ReturnJson 封装返回interface类型为json string Response
// @param data
// @return Response
func ReturnJson(data interface{}) protogo.Response {
	standardsBytes, err := json.Marshal(data)
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success(standardsBytes)
}

// Return 封装返回Bool类型为Response，如果有error则忽略bool，封装error
// @param err
// @return Response
func Return(err error) protogo.Response {
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.SuccessResponse
}

// ReturnUint8 封装返回uint8类型为Response，如果有error则忽略num，封装error
// @param num
// @param err
// @return Response
func ReturnUint8(num uint8, err error) protogo.Response {
	if err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success([]byte(strconv.Itoa(int(num))))
}

// RequireString 必须要有参数 string类型
// @param key
// @return string
// @return error
func RequireString(key string) (string, error) {
	args := sdk.Instance.GetArgs()
	b, ok := args[key]
	if !ok || len(b) == 0 {
		return "", fmt.Errorf("CMDFA: require parameter:'%s'", key)
	}
	return string(b), nil
}

// RequireStrings 必须要有参数 []string类型
func RequireStrings(key string) ([]string, error) {
	args := sdk.Instance.GetArgs()
	b, ok := args[key]
	if !ok || len(b) == 0 {
		return nil, fmt.Errorf("CMDFA: require parameter:'%s'", key)
	}
	var strs []string
	err := json.Unmarshal(b, &strs)
	if err != nil {
		return nil, err
	}
	return strs, nil
}

// RequireUint256 必须要有参数 Uint256类型
// @param key
// @return *safemath.SafeUint256
// @return error
func RequireUint256(key string) (*safemath.SafeUint256, error) {
	args := sdk.Instance.GetArgs()
	b, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("CMDFA: require parameter:'%s'", key)
	}
	num, ok := safemath.ParseSafeUint256(string(b))
	if !ok {
		return nil, fmt.Errorf("CMDFA: parameter:'%s' not a valid uint256", key)
	}
	return num, nil
}

// RequireUint256s 必须要有参数 []*safemath.SafeUint256类型
func RequireUint256s(key string) ([]*safemath.SafeUint256, error) {
	args := sdk.Instance.GetArgs()
	b, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("CMDFA: require parameter:'%s'", key)
	}
	var nums []*safemath.SafeUint256
	err := json.Unmarshal(b, &nums)
	if err != nil {
		return nil, err
	}
	return nums, nil
}

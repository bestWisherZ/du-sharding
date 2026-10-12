/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker/common/v2/evmutils/abi"
)

// loadFromJSON string into ABI data
func loadFromJSON(jString string) ([]Param, error) {
	if len(jString) == 0 {
		return nil, nil
	}
	data := []Param{}
	err := json.Unmarshal([]byte(jString), &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Pack data into bytes
func Pack(a *abi.ABI, method string, paramsJson string) ([]byte, error) {
	param, err := loadFromJSON(paramsJson)
	if err != nil {
		return nil, err
	}
	var args []interface{}
	for _, p := range param {
		for k, v := range p {
			arg, err := parse(k, v)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	}
	return a.Pack(method, args...)

}

// parse 把interface{}类型，解析成为solidity类型中对应的go的类型
func parse(sType string, value interface{}) (arg interface{}, err error) {
	// 正则表达式将解析solidity中的类型
	// 例：string[3] 分解为长度为3的string数组：matches[0] = string[3] ,matches[1] = string, matches[2] = [3]
	typeRegex := regexp.MustCompile(`^([a-zA-Z]+[0-9]*)((?:\[[0-9]*\])*)$`)
	matches := typeRegex.FindStringSubmatch(sType)
	if matches == nil {
		return nil, fmt.Errorf("invalid type format: %s", sType)
	}
	if len(matches) < 3 {
		return nil, fmt.Errorf("irregular solidity type input: %s", sType)
	}
	arrayPart := matches[2]
	// 处理非数组类型
	if arrayPart == "" {
		return baseTypeParse(sType, value)
	}
	// 数组类型
	arrayType := matches[1]
	// 处理数组类型 拿出数组大小N，如果不存在就是0
	arrayRegex := regexp.MustCompile(`\[([0-9]*)\]`)
	arrayMatches := arrayRegex.FindStringSubmatch(arrayPart)
	n := 0
	if arrayMatches[1] != "" {
		n, err = strconv.Atoi(arrayMatches[1])
		if err != nil {
			return nil, err
		}
	}
	return parseArr(arrayType, value, n)
}

// baseTypeParse 基本数据类型解析
func baseTypeParse(sType string, value interface{}) (interface{}, error) {
	switch {
	case isStringType(sType):
		return parseStr(value), nil
	case isIntType(sType):
		return parseInt(sType, value)
	case isUintType(sType):
		return parseUint(sType, value)
	case isBoolType(sType):
		return parseBool(value)
	case isAddressType(sType):
		return parseAddress(value)
	case isBytesType(sType):
		return parseBytes(sType, value)
	default:
		return value, nil
	}
}

// isStringType 检查给定的字符串是否为Solidity中的string类型。
func isStringType(sType string) bool {
	return sType == "string"
}

// isIntType 检查给定的字符串是否为Solidity中的int类型。
func isIntType(sType string) bool {
	// solidity中的全部int类型列表
	intTypes := []string{
		"int", "int8", "int16", "int24", "int32", "int40", "int48", "int56", "int64",
		"int72", "int80", "int88", "int96", "int104", "int112", "int120", "int128",
		"int136", "int144", "int152", "int160", "int168", "int176", "int184", "int192",
		"int200", "int208", "int216", "int224", "int232", "int240", "int248", "int256",
	}
	for _, iType := range intTypes {
		if iType == sType {
			return true
		}
	}
	return false
}

// isIntType 检查给定的字符串是否为Solidity中的uint类型。
func isUintType(sType string) bool {
	uintTypes := []string{
		"uint", "uint8", "uint16", "uint24", "uint32", "uint40", "uint48", "uint56", "uint64",
		"uint72", "uint80", "uint88", "uint96", "uint104", "uint112", "uint120", "uint128",
		"uint136", "uint144", "uint152", "uint160", "uint168", "uint176", "uint184", "uint192",
		"uint200", "uint208", "uint216", "uint224", "uint232", "uint240", "uint248", "uint256",
	}
	for _, iType := range uintTypes {
		if iType == sType {
			return true
		}
	}
	return false
}

// isBoolType 检查给定的字符串是否为Solidity中的bool类型。
func isBoolType(sType string) bool {
	return sType == "bool"
}

// isBoolType 检查给定的字符串是否为Solidity中的address类型。
func isAddressType(sType string) bool {
	return sType == "address"
}

// isBoolType 检查给定的字符串是否为Solidity中的bytes类型。
func isBytesType(sType string) bool {
	// solidity中全部的bytes类型列表
	bytesTypes := []string{
		"bytes", "bytes1", "bytes2", "bytes3", "bytes4", "bytes5", "bytes6", "bytes7", "bytes8",
		"bytes9", "bytes10", "bytes11", "bytes12", "bytes13", "bytes14", "bytes15", "bytes16",
		"bytes17", "bytes18", "bytes19", "bytes20", "bytes21", "bytes22", "bytes23", "bytes24",
		"bytes25", "bytes26", "bytes27", "bytes28", "bytes29", "bytes30", "bytes31", "bytes32",
	}
	for _, iType := range bytesTypes {
		if iType == sType {
			return true
		}
	}
	return false
}

// Param list
type Param map[string]interface{}

// parseInt 处理int类型数据
func parseInt(key string, value interface{}) (interface{}, error) {
	// 如果是intN类型的数据则使用正则表达式取出N的值
	isM := regexp.MustCompile("(int)([0-9]+)")
	bitStr := isM.FindStringSubmatch(key)
	if len(bitStr) < 3 {
		return nil, fmt.Errorf("irregular solidity int type input: %s", key)
	}
	// 先把value转换成string类型
	valueStr := fmt.Sprint(value)
	if bitStr != nil {
		bitNum, err := strconv.ParseInt(bitStr[2], 10, 32)
		if err != nil {
			panic(fmt.Sprintf("parse int err: %s", err.Error()))
		}
		var result interface{}
		// 根据solidity中int类型的大小赋予合适的int类型
		switch {
		case bitNum <= 8:
			num, err := strconv.ParseInt(valueStr, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int8: %s", err.Error())
			}
			result = int8(num)
		case bitNum <= 16:
			num, err := strconv.ParseInt(valueStr, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int16: %s", err.Error())
			}
			result = int16(num)
		case bitNum <= 32:
			num, err := strconv.ParseInt(valueStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int32: %s", err.Error())
			}
			result = int32(num)
		case bitNum <= 64:
			num, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int64: %s", err.Error())
			}
			result = int64(num)
		default:
			// 对于大于 64 位的整数，使用 math/big 包
			bigInt := big.NewInt(0)
			num, ok := bigInt.SetString(valueStr, 10)
			if !ok {
				return nil, fmt.Errorf("failed to set big.Int from value: %s", valueStr)
			}
			result = num
		}
		return result, nil
	}
	// 如果 key 不是有效的 int 类型，尝试将其转换为 big.Int
	bigInt := big.NewInt(0)
	num, ok := bigInt.SetString(valueStr, 10)
	if !ok {
		return nil, fmt.Errorf("failed to set big.Int from value: %s", valueStr)
	}
	return num, nil
}

// parseUInt 处理uint类型数据
func parseUint(key string, value interface{}) (interface{}, error) {
	// solidity 不支持浮点数所以直接转换成uint类型
	isM := regexp.MustCompile("(uint)([0-9]+)")
	bitStr := isM.FindStringSubmatch(key)
	// 先把value转换成string类型
	valueStr := fmt.Sprint(value)
	if bitStr != nil {
		bitNum, err := strconv.ParseUint(bitStr[2], 10, 32)
		if err != nil {
			panic(fmt.Sprintf("parse int err: %s", err.Error()))
		}
		var result interface{}
		switch {
		case bitNum <= 8:
			num, err := strconv.ParseUint(valueStr, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int8: %s", err.Error())
			}
			result = uint8(num)
		case bitNum <= 16:
			num, err := strconv.ParseUint(valueStr, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int16: %s", err.Error())
			}
			result = uint16(num)
		case bitNum <= 32:
			num, err := strconv.ParseUint(valueStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int32: %s", err.Error())
			}
			result = uint32(num)
		case bitNum <= 64:
			num, err := strconv.ParseUint(valueStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int64: %s", err.Error())
			}
			result = uint64(num)
		default:
			// 对于大于 64 位的整数，使用 math/big 包
			bigInt := big.NewInt(0)
			num, ok := bigInt.SetString(valueStr, 10)
			if !ok {
				return nil, fmt.Errorf("failed to set big.Int from value: %s", valueStr)
			}
			result = num
		}
		return result, nil
	}
	// 如果 key 不是有效的 int 类型，尝试将其转换为 big.Int
	bigInt := big.NewInt(0)
	num, ok := bigInt.SetString(valueStr, 10)
	if !ok {
		return nil, fmt.Errorf("failed to set big.Int from value: %s", valueStr)
	}
	return num, nil
}

// 处理string类型数据
func parseStr(value interface{}) string {
	return fmt.Sprint(value)
}

// 处理布尔类型数据
func parseBool(value interface{}) (bool, error) {
	v, ok := value.(bool)
	if !ok {
		fmt.Printf("value %v is not bool\n", value)
	}
	return v, nil
}

func parseAddress(value interface{}) ([]byte, error) {
	sAddress := fmt.Sprint(value)
	hexStr := fmt.Sprint(value)
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	// 将 uint64 编码为字节（小端字节序）
	b, err := hex.DecodeString(sAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %s", value)
	}
	return b, nil
}

func parseBytes(key string, value interface{}) ([]byte, error) {
	rest := key[len("bytes"):]
	// 将剩余部分转换为整数
	if rest != "" {
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid number: %s", rest)
		}
		if n < 1 || n > 32 {
			return nil, fmt.Errorf("bytes number must be between 1 and 32")
		}
	}
	hexStr := fmt.Sprint(value)
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	// 将 uint64 编码为字节（小端字节序）
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %s", value)
	}
	return b, nil
}

func parseArr(key string, value interface{}, n int) (interface{}, error) {
	slice, ok := value.([]interface{})
	if !ok {
		return nil, errors.New("value is not a []interface{}")
	}
	switch {
	case isStringType(key):
		return parseStrArr(slice, n), nil
	case strings.Contains(key, "int") && !strings.Contains(key, "uint"):
		return parseIntArray(key, slice, n)
	case strings.Contains(key, "uint"):
		return parseUintArray(key, slice, n)
	case isBoolType(key):
		return parseBoolArr(slice, n)
	case isAddressType(key):
		return parseAddressArr(slice, n)
	case strings.Contains(key, "bytes"):
		return parseBytesArr(key, slice, n)
	default:
		return value, nil
	}
}

func parseStrArr(value []interface{}, n int) []string {
	if n != 0 {
		s := make([]string, n)
		for i := 0; i < len(value); i++ {
			s[i] = parseStr(value[i])
		}
		return s
	}
	s := make([]string, 0)
	for i := 0; i < len(value); i++ {
		s = append(s, parseStr(value[i]))
	}
	return s
}

func parseIntArray(key string, value []interface{}, n int) (interface{}, error) {
	isM := regexp.MustCompile("(int)([0-9]+)")
	bitStr := isM.FindStringSubmatch(key)
	if bitStr != nil {
		bitNum, err := strconv.ParseInt(bitStr[2], 10, 32)
		if err != nil {
			panic(fmt.Sprintf("parse int err: %s", err.Error()))
		}
		switch {
		case bitNum <= 8:
			return int8Arr(value, n)
		case bitNum <= 16:
			return int16Arr(value, n)
		case bitNum <= 32:
			return int32Arr(value, n)
		case bitNum <= 64:
			return int64Arr(value, n)
		default:
			return bigIntArr(value, n)
		}
	} else {
		return bigIntArr(value, n)
	}
}

func int8Arr(value []interface{}, n int) ([]int8, error) {
	if n != 0 {
		arr := make([]int8, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseInt(valueStr, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int8 N: %s", err.Error())
			}
			arr[i] = int8(num)
		}
		return arr, nil
	}
	arr := make([]int8, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseInt(valueStr, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int8: %s", err.Error())
		}
		arr = append(arr, int8(num))
	}
	return arr, nil
}

func int16Arr(value []interface{}, n int) ([]int16, error) {
	if n != 0 {
		arr := make([]int16, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseInt(valueStr, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int16 N: %s", err.Error())
			}
			arr[i] = int16(num)
		}
		return arr, nil
	}
	arr := make([]int16, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseInt(valueStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int16: %s", err.Error())
		}
		arr = append(arr, int16(num))
	}
	return arr, nil
}

func int32Arr(value []interface{}, n int) ([]int32, error) {

	if n != 0 {
		arr := make([]int32, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseInt(valueStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int32 N: %s", err.Error())
			}
			arr[i] = int32(num)
		}
		return arr, nil
	}
	arr := make([]int32, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseInt(valueStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int32: %s", err.Error())
		}
		arr = append(arr, int32(num))
	}
	return arr, nil
}

func int64Arr(value []interface{}, n int) ([]int64, error) {
	if n != 0 {
		arr := make([]int64, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int64 N: %s", err.Error())
			}
			arr[i] = int64(num)
		}
		return arr, nil
	}
	arr := make([]int64, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int64 N: %s", err.Error())
		}
		arr = append(arr, int64(num))
	}
	return arr, nil
}

func bigIntArr(value []interface{}, n int) ([]*big.Int, error) {
	if n != 0 {
		arr := make([]*big.Int, n)
		for i := 0; i < len(value); i++ {
			bigInt := big.NewInt(0)
			valueStr := fmt.Sprint(value[i])
			num, ok := bigInt.SetString(valueStr, 10)
			if !ok {
				return nil, fmt.Errorf("failed to set big.Int from value N: %s", valueStr)
			}
			arr[i] = num
		}
		return arr, nil
	}
	arr := make([]*big.Int, n)
	for i := 0; i < len(value); i++ {
		bigInt := big.NewInt(0)
		valueStr := fmt.Sprint(value[i])
		num, ok := bigInt.SetString(valueStr, 10)
		if !ok {
			return nil, fmt.Errorf("failed to set big.Int from value: %s", valueStr)
		}
		arr = append(arr, num)
	}
	return arr, nil
}

func parseUintArray(key string, value []interface{}, n int) (interface{}, error) {
	isM := regexp.MustCompile("(uint)([0-9]+)")
	bitStr := isM.FindStringSubmatch(key)
	if bitStr != nil {
		bitNum, err := strconv.ParseInt(bitStr[2], 10, 32)
		if err != nil {
			panic(fmt.Sprintf("parse int err: %s", err.Error()))
		}
		switch {
		case bitNum <= 8:
			return uint8Arr(value, n)
		case bitNum <= 16:
			return uint16Arr(value, n)
		case bitNum <= 32:
			return uint32Arr(value, n)
		case bitNum <= 64:
			return uint64Arr(value, n)
		default:
			return bigIntArr(value, n)
		}
	} else {
		return bigIntArr(value, n)
	}
}

func uint8Arr(value []interface{}, n int) ([]uint8, error) {
	if n != 0 {
		arr := make([]uint8, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseUint(valueStr, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int8 N: %s", err.Error())
			}
			arr[i] = uint8(num)
		}
		return arr, nil
	}
	arr := make([]uint8, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseUint(valueStr, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int8: %s", err.Error())
		}
		arr = append(arr, uint8(num))
	}
	return arr, nil
}

func uint16Arr(value []interface{}, n int) ([]uint16, error) {
	if n != 0 {
		arr := make([]uint16, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseUint(valueStr, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int16 N: %s", err.Error())
			}
			arr[i] = uint16(num)
		}
		return arr, nil
	}
	arr := make([]uint16, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseUint(valueStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int16: %s", err.Error())
		}
		arr = append(arr, uint16(num))
	}
	return arr, nil
}

func uint32Arr(value []interface{}, n int) ([]uint32, error) {
	if n != 0 {
		arr := make([]uint32, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseUint(valueStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int32 N: %s", err.Error())
			}
			arr[i] = uint32(num)
		}
		return arr, nil
	}
	arr := make([]uint32, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseUint(valueStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int32: %s", err.Error())
		}
		arr = append(arr, uint32(num))
	}
	return arr, nil
}

func uint64Arr(value []interface{}, n int) ([]uint64, error) {
	if n != 0 {
		arr := make([]uint64, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			num, err := strconv.ParseUint(valueStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse int64 N: %s", err.Error())
			}
			arr[i] = uint64(num)
		}
		return arr, nil
	}
	arr := make([]uint64, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		num, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse int64: %s", err.Error())
		}
		arr = append(arr, uint64(num))
	}
	return arr, nil
}

func parseBoolArr(value []interface{}, n int) ([]bool, error) {
	if n != 0 {
		arr := make([]bool, n)
		for i := 0; i < len(value); i++ {
			valueStr := fmt.Sprint(value[i])
			b, err := strconv.ParseBool(valueStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse bool N: %s", err.Error())
			}
			arr[i] = b
		}
		return arr, nil
	}
	arr := make([]bool, 0)
	for i := 0; i < len(value); i++ {
		valueStr := fmt.Sprint(value[i])
		b, err := strconv.ParseBool(valueStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bool: %s", err.Error())
		}
		arr = append(arr, b)
	}
	return arr, nil
}

func parseAddressArr(value []interface{}, n int) (interface{}, error) {
	if n != 0 {
		arr := make([][]byte, n)
		for i := 0; i < len(value); i++ {
			addr, err := parseAddress(value[i])
			if err != nil {
				return nil, err
			}
			arr[i] = addr
		}
		return arr, nil
	}
	arr := make([][]byte, 0)
	for i := 0; i < len(value); i++ {
		addr, err := parseAddress(value[i])
		if err != nil {
			return nil, err
		}
		arr = append(arr, addr)
	}
	return arr, nil
}

func parseBytesArr(key string, value []interface{}, n int) (interface{}, error) {
	if n != 0 {
		arr := make([][]byte, n)
		for i := 0; i < len(value); i++ {
			b, err := parseBytes(key, value[i])
			if err != nil {
				return nil, fmt.Errorf("failed to parse bytes N: %s", err.Error())
			}
			arr[i] = b
		}
		return arr, nil
	}
	arr := make([][]byte, 0)
	for i := 0; i < len(value); i++ {
		b, err := parseBytes(key, value[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse bytes : %s", err.Error())
		}
		arr = append(arr, b)
	}
	return arr, nil
}

func TestDemo(method string, paramsJson string) {
	param, err := loadFromJSON(paramsJson)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var args []interface{}
	for _, p := range param {
		for k, v := range p {
			arg, err := parse(k, v)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			args = append(args, arg)
		}
	}
}

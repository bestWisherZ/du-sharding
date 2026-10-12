/*
 Copyright (C) BABEC. All rights reserved.
 Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

 SPDX-License-Identifier: Apache-2.0
*/

/*
ContractStandardNameCMDFA ChainMaker - Contract Standard - Digital Fungible Assets
https://git.chainmaker.org.cn/contracts/standard/-/blob/master/draft/CM-CS-221221-DFA.md
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/pb/protogo"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/address"
	"chainmaker.org/chainmaker/contract-utils/standard"

	"chainmaker.org/chainmaker/contract-utils/safemath"
)

const (
	//db key
	balanceKey     = "b"
	allowanceKey   = "a"
	totalSupplyKey = "totalSupplyKey"
	nameKey        = "name"
	symbolKey      = "symbol"
	decimalKey     = "decimal"
	adminKey       = "admin"
)

const (
	defaultName        = "TestToken"
	defaultSymbol      = "TT"
	defaultDecimals    = 18
	defaultTotalSupply = 0

	//TODO 出于性能考虑，我们直接将DID合约名写死在这个合约中，就不用每次调用都先去读取DID合约名了，如果不启用DID，则将这个值置为空字符串
	defaultDidContractName = ""
)

var _ standard.CMDFA = (*CmdfaContract)(nil)
var _ standard.CMBC = (*CmdfaContract)(nil)
var _ standard.CMDFAOption = (*CmdfaContract)(nil)

// CmdfaContract erc20 contract
type CmdfaContract struct {
	didContract   standard.CMDID
	accessControl standard.AccessControl
}

// NewCmdfaContract create CMDFA contract instance
func NewCmdfaContract() *CmdfaContract {
	c := &CmdfaContract{}
	c.accessControl = standard.NewDefaultAccessControl(dfaAdminStore{})
	if len(defaultDidContractName) > 0 {
		c.didContract = &DidContract{contractName: defaultDidContractName}
	}
	return c
}

// InitContract init contract func
func (c *CmdfaContract) InitContract() protogo.Response {
	err := c.updateErc20Info()
	if err != nil {
		return sdk.Error(err.Error())
	}
	if err = c.updateExperimentConfig(); err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success([]byte("Init contract success"))
}

// UpgradeContract upgrade contract func
func (c *CmdfaContract) UpgradeContract() protogo.Response {
	err := c.updateErc20Info()
	if err != nil {
		return sdk.Error(err.Error())
	}
	if err = c.updateExperimentConfig(); err != nil {
		return sdk.Error(err.Error())
	}
	return sdk.Success([]byte("Upgrade contract success"))
}

// UpgradeContract upgrade contract func
func (c *CmdfaContract) updateErc20Info() error {
	args := sdk.Instance.GetArgs()
	// name, symbol and decimal are optional
	name := args[pName]
	symbol := args[pSymbol]
	decimalsStr := args[pDecimals]
	totalSupply := args[pTotalSupply]
	//check name and save name
	tokenName := defaultName
	if len(name) > 0 {
		tokenName = string(name)
	}
	err := sdk.Instance.PutState(nameKey, "", tokenName)
	if err != nil {
		return err
	}

	//check symbol and save symbol
	tokenSymbol := defaultSymbol
	if len(symbol) > 0 {
		tokenSymbol = string(symbol)
	}
	err = sdk.Instance.PutState(symbolKey, "", tokenSymbol)
	if err != nil {
		return err
	}
	//check decimals and save decimals
	tokenDecimals := defaultDecimals
	//decimals default to 18
	if len(decimalsStr) > 0 {
		num, err1 := strconv.Atoi(string(decimalsStr))
		if err1 != nil {
			return fmt.Errorf("param decimals err")
		}
		tokenDecimals = num
	}
	err = sdk.Instance.PutState(decimalKey, "", strconv.Itoa(tokenDecimals))
	if err != nil {
		return err
	}

	//set admin
	admin, err := sdk.Instance.Origin()
	if err != nil {
		return fmt.Errorf("get sender failed, err:%s", err)
	}
	//如果设置了DID合约，那么就将admin地址转换为did
	if len(defaultDidContractName) > 0 {
		adminDid, err1 := c.didContract.GetDidByAddress(admin)
		if err1 != nil {
			return fmt.Errorf("get origin did by address[%s] failed, err:%s", admin, err1)
		}
		admin = adminDid
	}
	err = sdk.Instance.PutState(adminKey, "", admin)
	if err != nil {
		return err
	}
	//set totalSupply
	tokenTotalSupply := safemath.NewSafeUint256(defaultTotalSupply)
	if len(totalSupply) > 0 {
		totalSupplyNum, ok := safemath.ParseSafeUint256(string(totalSupply))
		if !ok {
			return errors.New("invalid totalSupply number")
		}
		tokenTotalSupply = totalSupplyNum
	}
	//Mint
	if tokenTotalSupply.ToString() != "0" {
		return c.baseMint(admin, tokenTotalSupply)
	}
	return nil
}

// InvokeContract the entry func of invoke contract func
func (c *CmdfaContract) InvokeContract(method string) protogo.Response { // nolint:gocyclo
	if len(method) == 0 {
		return sdk.Error("method of param should not be empty")
	}
	switch method {
	case "Standards":
		return ReturnJson(c.Standards())
	case "Version":
		return ReturnString(c.Version(), nil)
	case "Name":
		return ReturnString(c.Name())
	case "Symbol":
		return ReturnString(c.Symbol())
	case "Decimals":
		return ReturnUint8(c.Decimals())
	case "TotalSupply":
		return ReturnUint256(c.TotalSupply())
	case "BalanceOf":
		account, err := RequireString(pAccount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return ReturnUint256(c.BalanceOf(account))
	case "Transfer":
		to, err := RequireString(pTo)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.Transfer(to, amount))
	case "TransferExperiment":
		from, err := RequireString(pFrom)
		if err != nil {
			return sdk.Error(err.Error())
		}
		to, err := RequireString(pTo)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.TransferExperiment(from, to, amount))
	case "TransferFrom":
		from, err := RequireString(pFrom)
		if err != nil {
			return sdk.Error(err.Error())
		}
		to, err := RequireString(pTo)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.TransferFrom(from, to, amount))
	case "Approve":
		spender, err := RequireString(pSpender)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.Approve(spender, amount))
	case "Allowance":
		spender, err := RequireString(pSpender)
		if err != nil {
			return sdk.Error(err.Error())
		}
		owner, err := RequireString(pOwner)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return ReturnUint256(c.Allowance(owner, spender))
	case "Mint":
		account, err := RequireString(pAccount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.Mint(account, amount))
	case "Burn":
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.Burn(amount))
	case "BurnFrom":
		account, err := RequireString(pAccount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.BurnFrom(account, amount))
	case "Metadata":
		return ReturnBytes(c.Metadata())
	case "BatchTransfer":
		to, err := RequireStrings(pTo)
		if err != nil {
			return sdk.Error(err.Error())
		}
		amount, err := RequireUint256s(pAmount)
		if err != nil {
			return sdk.Error(err.Error())
		}
		return Return(c.BatchTransfer(to, amount))
	default:
		return sdk.Error("Invalid method")
	}
}

// BatchTransfer 批量转账
func (c *CmdfaContract) BatchTransfer(to []string, amount []*safemath.SafeUint256) error {
	if len(to) != len(amount) {
		return fmt.Errorf("to and amount length not match")
	}
	for i := 0; i < len(to); i++ {
		err := c.Transfer(to[i], amount[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// Metadata 合约元数据结构体
type Metadata struct {
	Image       string                `json:"image"`
	Description string                `json:"description"`
	Name        string                `json:"name"`
	Symbol      string                `json:"symbol"`
	Decimals    uint8                 `json:"decimals"`
	TotalSupply *safemath.SafeUint256 `json:"totalSupply"`
}

// Metadata 查询合约元数据
func (c *CmdfaContract) Metadata() (metadata []byte, err error) {
	m := &Metadata{}
	m.Image = "https://www.chainmaker.org.cn/logo.png"
	m.Description = "这是一个测试Token"
	m.Name, err = c.Name()
	if err != nil {
		return nil, err
	}
	m.Symbol, err = c.Symbol()
	if err != nil {
		return nil, err
	}
	m.Decimals, err = c.Decimals()
	if err != nil {
		return nil, err
	}
	m.TotalSupply, err = c.TotalSupply()
	if err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Standards 合约支持的标准
// @return []string
func (c *CmdfaContract) Standards() []string {
	return []string{standard.ContractStandardNameCMBC, standard.ContractStandardNameCMDFA}
}

// SupportStandard 查询本合约是否支持某合约标准
// @param standardName
// @return bool
func (c *CmdfaContract) SupportStandard(standardName string) bool {
	return standardName == standard.ContractStandardNameCMDFA || standardName == standard.ContractStandardNameCMBC
}

// Version returns current contract version.
func (c *CmdfaContract) Version() string {
	return "2.0"
}

// TotalSupply 发行总量
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) TotalSupply() (*safemath.SafeUint256, error) {
	return c.GetTotalSupply()
}

// BalanceOf 账户余额查询
// @param account
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) BalanceOf(account string) (*safemath.SafeUint256, error) {
	return c.GetBalance(account)
}

func (c *CmdfaContract) getSender() (string, error) {
	senderAddr, err := sdk.Instance.Sender()
	if err != nil {
		return "", err
	}
	if c.didContract != nil {
		return c.didContract.GetDidByAddress(senderAddr)
	}
	return senderAddr, nil
}

// onlyAdmin checks whether the current caller is the contract admin.
// This is the Go equivalent of an EVM onlyAdmin modifier.
func (c *CmdfaContract) onlyAdmin() error {
	sender, err := c.getSender()
	if err != nil {
		return fmt.Errorf("get sender address failed, err:%v", err)
	}
	isAdmin, err := c.isAdmin(sender)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errors.New("only admin can invoke this method")
	}
	return nil
}

// isAdmin delegates admin checks to the pluggable access control implementation.
func (c *CmdfaContract) isAdmin(account string) (bool, error) {
	return c.getAccessControl().IsAdmin(account)
}

// Transfer 转账操作
// @param to
// @param amount
// @return error
func (c *CmdfaContract) Transfer(to string, amount *safemath.SafeUint256) error {
	from, err := c.getSender()

	if err != nil {
		return fmt.Errorf("Get sender address failed, err:%s", err)
	}
	err = c.baseTransfer(from, to, amount)
	if err != nil {
		return err
	}
	return nil
}

// TransferFrom 代为转账操作
// @param from
// @param to
// @param amount
// @return error
func (c *CmdfaContract) TransferFrom(from, to string, amount *safemath.SafeUint256) error {
	sender, err := c.getSender()
	if err != nil {
		return fmt.Errorf("Get sender address failed, err:%s", err)
	}
	err = c.baseSpendAllowance(from, sender, amount)
	if err != nil {
		return fmt.Errorf("spend allowance failed, err:%s", err)
	}
	err = c.baseTransfer(from, to, amount)
	if err != nil {
		return err
	}
	return nil
}

// Approve 授权额度
// @param spender
// @param amount
// @return error
func (c *CmdfaContract) Approve(spender string, amount *safemath.SafeUint256) error {
	sender, err := c.getSender()
	if err != nil {
		return fmt.Errorf("Get sender address failed, err:%s", err)
	}
	err = c.baseApprove(sender, spender, amount)
	if err != nil {
		return err
	}
	return nil
}

// Allowance 查询授权额度
// @param owner
// @param spender
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) Allowance(owner, spender string) (*safemath.SafeUint256, error) {
	return c.GetAllowance(owner, spender)
}

// Name Token的名字
// @return string
// @return error
func (c *CmdfaContract) Name() (string, error) {
	return c.GetName()
}

// Symbol Token的符号
// @return string
// @return error
func (c *CmdfaContract) Symbol() (string, error) {
	return c.GetSymbol()
}

// Decimals Token的小数位数
// @return uint8
// @return error
func (c *CmdfaContract) Decimals() (uint8, error) {
	return c.GetDecimals()
}

// Mint 铸造Token
// @param account
// @param amount
// @return error
func (c *CmdfaContract) Mint(account string, amount *safemath.SafeUint256) error {
	if err := c.onlyAdmin(); err != nil {
		return err
	}
	//call base mint
	if err := c.baseMint(account, amount); err != nil {
		return err
	}
	return nil
}

// Burn 销毁Token
// @param amount
// @return error
func (c *CmdfaContract) Burn(amount *safemath.SafeUint256) error {
	spender, err := c.getSender()
	if err != nil {
		return fmt.Errorf("Get sender address failed, err:%s", err)
	}
	//call base burn
	err = c.baseBurn(spender, amount)
	if err != nil {
		return err
	}
	return nil
}

// BurnFrom 代为销毁Token
// @param account
// @param amount
// @return error
func (c *CmdfaContract) BurnFrom(account string, amount *safemath.SafeUint256) error {
	spender, err := c.getSender()
	if err != nil {
		return fmt.Errorf("Get sender address failed, err:%s", err)
	}
	err = c.baseSpendAllowance(account, spender, amount)
	if err != nil {
		return err
	}
	//call base burn
	err = c.baseBurn(account, amount)
	if err != nil {
		return err
	}
	return nil
}

// EmitTransferEvent 触发转账事件
// @param spender
// @param to
// @param amount
func (c *CmdfaContract) EmitTransferEvent(spender, to string, amount *safemath.SafeUint256) {
	sdk.Instance.EmitEvent(standard.TopicTransfer, []string{spender, to, amount.ToString()})
}

// EmitApproveEvent 触发授权事件
// @param owner
// @param spender
// @param amount
func (c *CmdfaContract) EmitApproveEvent(owner, spender string, amount *safemath.SafeUint256) {
	sdk.Instance.EmitEvent(standard.TopicApprove, []string{owner, spender, amount.ToString()})
}

// EmitMintEvent 触发铸造事件
// @param account
// @param amount
func (c *CmdfaContract) EmitMintEvent(account string, amount *safemath.SafeUint256) {
	sdk.Instance.EmitEvent(standard.TopicMint, []string{account, amount.ToString()})
}

// EmitBurnEvent 触发销毁事件
// @param spender
// @param amount
func (c *CmdfaContract) EmitBurnEvent(spender string, amount *safemath.SafeUint256) {
	sdk.Instance.EmitEvent(standard.TopicBurn, []string{spender, amount.ToString()})

}

/////////////////////////Data Access Layer/////////////////////////////////

func createCompositeKey(prefix string, data ...string) string {
	return prefix + "_" + strings.Join(data, "_")
}

// GetUint256 获得DB中的SafeUint256
// @param key
// @param field
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) GetUint256(key, field string) (*safemath.SafeUint256, error) {
	fromBalStr, err := sdk.Instance.GetState(key, processDid4Key(field))
	if err != nil {
		return nil, err
	}

	fromBalance, pass := safemath.ParseSafeUint256(string(fromBalStr))
	if !pass {
		return nil, errors.New("invalid uint256 data")
	}
	return fromBalance, nil
}

// GetBalance 获得DB中的账户余额
// @param account
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) GetBalance(account string) (*safemath.SafeUint256, error) {
	return c.GetUint256(balanceKey, account)
}

// SetBalance 设置DB中的账户余额
// @param account
// @param amount
// @return error
func (c *CmdfaContract) SetBalance(account string, amount *safemath.SafeUint256) error {
	return sdk.Instance.PutState(balanceKey, processDid4Key(account), amount.ToString())
}
func processDid4Key(did string) string {
	return strings.ReplaceAll(did, ":", "_")
}

// SetAllowance 设置DB中的授权额度
// @param owner
// @param spender
// @param amount
// @return error
func (c *CmdfaContract) SetAllowance(owner string, spender string, amount *safemath.SafeUint256) error {
	key := createCompositeKey(allowanceKey, owner, spender)
	return sdk.Instance.PutState(key, "", amount.ToString())
}

// GetAllowance 获得DB中的授权额度
// @param owner
// @param spender
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) GetAllowance(owner string, spender string) (*safemath.SafeUint256, error) {
	key := createCompositeKey(allowanceKey, owner, spender)
	return c.GetUint256(key, "")
}

// GetTotalSupply 获得发行总量
// @return *safemath.SafeUint256
// @return error
func (c *CmdfaContract) GetTotalSupply() (*safemath.SafeUint256, error) {
	return c.GetUint256(totalSupplyKey, "")
}

// SetTotalSupply 设置发行总量
// @param amount
// @return error
func (c *CmdfaContract) SetTotalSupply(amount *safemath.SafeUint256) error {
	return sdk.Instance.PutState(totalSupplyKey, "", amount.ToString())
}

// GetName 获得DB中的Name
// @return string
// @return error
func (c *CmdfaContract) GetName() (string, error) {
	return sdk.Instance.GetState(nameKey, "")
}

// SetName 设置DB中的Name
// @param name
// @return error
func (c *CmdfaContract) SetName(name string) error {
	return sdk.Instance.PutState(nameKey, "", name)
}

// GetSymbol 获得DB中的符号
// @return string
// @return error
func (c *CmdfaContract) GetSymbol() (string, error) {
	return sdk.Instance.GetState(symbolKey, "")
}

// SetSymbol 设置DB中的符号
// @param symbol
// @return error
func (c *CmdfaContract) SetSymbol(symbol string) error {
	return sdk.Instance.PutState(symbolKey, "", symbol)
}

// GetDecimals 获得DB中的小数位数
// @return uint8
// @return error
func (c *CmdfaContract) GetDecimals() (uint8, error) {
	d, err := sdk.Instance.GetState(decimalKey, "")
	if err != nil {
		return 0, err
	}
	decimal, err := strconv.ParseUint(d, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(decimal), nil
}

// SetDecimals 设置DB中的小数位数
// @param decimal
// @return error
func (c *CmdfaContract) SetDecimals(decimal uint8) error {
	return sdk.Instance.PutState(decimalKey, "", strconv.Itoa(int(decimal)))
}

// GetAdmin 获得DB中的Admin
// @return string
// @return error
func (c *CmdfaContract) GetAdmin() (string, error) {
	return sdk.Instance.GetState(adminKey, "")

}

// SetAdmin 设置DB中的Admin
// @param admin
// @return error
func (c *CmdfaContract) SetAdmin(admin string) error {
	return sdk.Instance.PutState(adminKey, "", admin)
}

////////////////////////////CMDFA Core/////////////////////////////

func (c *CmdfaContract) isDid(s string) bool {
	//检查前缀是不是did:
	return strings.HasPrefix(s, "did:")
}

// baseTransfer
// @param from
// @param to
// @param amount
// @return error
func (c *CmdfaContract) baseTransfer(from string, to string, amount *safemath.SafeUint256) error {
	//检查from和to的合法性
	if c.isDid(from) {
		//检查from的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(from)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: transfer from the invalid did")
		}
	} else {
		if !address.IsValidAddress(from) {
			return errors.New("CMDFA: transfer from the invalid address")
		}
		if address.IsZeroAddress(from) {
			return errors.New("CMDFA: transfer from the zero address")
		}
	}
	if c.isDid(to) {
		//检查to的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(to)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: transfer to the invalid did")
		}
	} else {
		if !address.IsValidAddress(to) {
			return errors.New("CMDFA: transfer to the invalid address")
		}
		if address.IsZeroAddress(to) {
			return errors.New("CMDFA: transfer to the zero address")
		}
	}
	//检查from余额充足
	fromBalance, err := c.GetBalance(from)
	if err != nil {
		return err
	}
	if !fromBalance.GTE(amount) {
		return errors.New("CMDFA: transfer amount exceeds balance")
	}
	//更新from和to的余额
	fromNewBalance, _ := safemath.SafeSub(fromBalance, amount)
	err = c.SetBalance(from, fromNewBalance)
	if err != nil {
		return err
	}
	toBalance, err := c.GetBalance(to)
	if err != nil {
		return err
	}
	toNewBalance, ok := safemath.SafeAdd(toBalance, amount)
	if !ok {
		return errors.New("calculate new to balance error")
	}
	err = c.SetBalance(to, toNewBalance)
	if err != nil {
		return err
	}
	//触发事件
	c.EmitTransferEvent(from, to, amount)

	return nil
}

func (c *CmdfaContract) baseApprove(owner string, spender string, amount *safemath.SafeUint256) error {
	//检查owner和spender的合法性
	if c.isDid(owner) {
		//检查owner的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(owner)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: approve owner is invalid did")
		}
	} else {
		if !address.IsValidAddress(owner) {
			return errors.New("CMDFA: approve owner is invalid address")
		}
		if address.IsZeroAddress(owner) {
			return errors.New("CMDFA: approve owner is zero address")
		}
	}
	if c.isDid(spender) {
		//检查spender的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(spender)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: approve spender is invalid did")
		}
	} else {
		if !address.IsValidAddress(spender) {
			return errors.New("CMDFA: approve spender is invalid address")
		}
		if address.IsZeroAddress(spender) {
			return errors.New("CMDFA: approve spender is zero address")
		}
	}
	//设置Allowance
	err := c.SetAllowance(owner, spender, amount)
	if err != nil {
		return err
	}
	//触发事件Approval
	c.EmitApproveEvent(owner, spender, amount)
	return nil
}

func (c *CmdfaContract) baseSpendAllowance(owner string, spender string, amount *safemath.SafeUint256) error {
	//获得授权的额度
	currentAllowance, err := c.GetAllowance(owner, spender)
	if err != nil {
		return err
	}
	// Does not update the allowance amount in case of infinite allowance.
	if currentAllowance.IsMaxSafeUint256() {
		return nil
	}
	//计算额度是否够用
	if !currentAllowance.GTE(amount) {
		return errors.New("CMDFA: insufficient allowance")
	}
	//扣减授权额度
	newCurrentAllowance, ok := safemath.SafeSub(currentAllowance, amount)
	if !ok {
		return errors.New("spend allowance error")
	}
	return c.baseApprove(owner, spender, newCurrentAllowance)
}
func (c *CmdfaContract) baseMint(account string, amount *safemath.SafeUint256) error {
	//检查account的合法性
	if c.isDid(account) {
		//检查account did的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(account)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: mint to the invalid did")
		}
	} else {
		//检查account 地址的合法性
		if !address.IsValidAddress(account) {
			return errors.New("CMDFA: mint to the invalid address")
		}
		if address.IsZeroAddress(account) {
			return errors.New("CMDFA: mint to the zero address")
		}
	}
	//更新TotalSupply
	totalSupply, err := c.GetTotalSupply()
	if err != nil {
		return err
	}
	newTotal, ok := safemath.SafeAdd(totalSupply, amount)
	if !ok {
		return errors.New("calculate totalSupply failed")
	}
	err = c.SetTotalSupply(newTotal)
	if err != nil {
		return err
	}
	//更新余额
	toBalance, err := c.GetBalance(account)
	if err != nil {
		return err
	}
	toNewBalance, ok := safemath.SafeAdd(toBalance, amount)
	if !ok {
		return errors.New("calculate new to balance error")
	}
	err = c.SetBalance(account, toNewBalance)
	if err != nil {
		return err
	}
	//触发事件
	c.EmitMintEvent(account, amount)
	return nil
}

func (c *CmdfaContract) baseBurn(account string, amount *safemath.SafeUint256) error {
	//检查account的合法性
	if c.isDid(account) {
		//检查account did的合法性
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		isValid, err := c.didContract.IsValidDid(account)
		if err != nil {
			return err
		}
		if !isValid {
			return errors.New("CMDFA: burn from the invalid did")
		}
	} else {
		//检查account 地址的合法性
		if !address.IsValidAddress(account) {
			return errors.New("CMDFA: burn from the invalid address")
		}
		if address.IsZeroAddress(account) {
			return errors.New("CMDFA: burn from the zero address")
		}
	}
	//检查用户余额充足
	fromBalance, err := c.GetBalance(account)
	if err != nil {
		return err
	}
	if !fromBalance.GTE(amount) {
		return errors.New("CMDFA: burn amount exceeds balance")
	}
	//更新TotalSupply
	totalSupply, err := c.GetTotalSupply()
	if err != nil {
		return err
	}
	newTotal, ok := safemath.SafeSub(totalSupply, amount)
	if !ok {
		return errors.New("calculate totalSupply failed")
	}
	err = c.SetTotalSupply(newTotal)
	if err != nil {
		return err
	}
	//更新余额
	fromNewBalance, ok := safemath.SafeSub(fromBalance, amount)
	if !ok {
		return errors.New("calculate new to balance error")
	}
	err = c.SetBalance(account, fromNewBalance)
	if err != nil {
		return err
	}
	//触发事件
	c.EmitBurnEvent(account, amount)

	return nil
}

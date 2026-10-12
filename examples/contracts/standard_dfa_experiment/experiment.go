/*
 Copyright (C) BABEC. All rights reserved.
 Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

 SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"errors"
	"fmt"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/address"
	"chainmaker.org/chainmaker/contract-utils/safemath"
)

const (
	pExperimentInitialBalance     = "experimentInitialBalance"
	experimentInitialBalanceKey   = "experimentInitialBalance"
	defaultExperimentInitialValue = "1000000000"
)

// updateExperimentConfig stores the lazy source balance used only by
// TransferExperiment. Standard CMDFA methods never read this setting.
func (c *CmdfaContract) updateExperimentConfig() error {
	args := sdk.Instance.GetArgs()
	value := string(args[pExperimentInitialBalance])
	if value == "" {
		current, err := sdk.Instance.GetState(experimentInitialBalanceKey, "")
		if err != nil {
			return err
		}
		if current != "" {
			return nil
		}
		value = defaultExperimentInitialValue
	}
	initialBalance, ok := safemath.ParseSafeUint256(value)
	if !ok || initialBalance.Equal(safemath.SafeUintZero) {
		return errors.New("CMDFA experiment: invalid experimentInitialBalance")
	}
	return sdk.Instance.PutState(experimentInitialBalanceKey, "", initialBalance.ToString())
}

// TransferExperiment is an explicitly non-standard benchmark extension.
// The caller supplies a unique source address. An unseen source receives a
// virtual initial balance, while an unseen destination starts at zero.
func (c *CmdfaContract) TransferExperiment(from, to string, amount *safemath.SafeUint256) error {
	if from == to {
		return errors.New("CMDFA experiment: from and to must differ")
	}
	if err := c.validateExperimentAddress(from, "from"); err != nil {
		return err
	}
	if err := c.validateExperimentAddress(to, "to"); err != nil {
		return err
	}

	fromRaw, err := sdk.Instance.GetState(balanceKey, processDid4Key(from))
	if err != nil {
		return err
	}
	if fromRaw == "" {
		fromRaw, err = sdk.Instance.GetState(experimentInitialBalanceKey, "")
		if err != nil {
			return err
		}
	}
	fromBalance, ok := safemath.ParseSafeUint256(fromRaw)
	if !ok {
		return errors.New("CMDFA experiment: invalid source balance")
	}
	if !fromBalance.GTE(amount) {
		return errors.New("CMDFA: transfer amount exceeds balance")
	}

	toBalance, err := c.GetBalance(to)
	if err != nil {
		return err
	}
	fromNewBalance, ok := safemath.SafeSub(fromBalance, amount)
	if !ok {
		return errors.New("calculate new from balance error")
	}
	toNewBalance, ok := safemath.SafeAdd(toBalance, amount)
	if !ok {
		return errors.New("calculate new to balance error")
	}
	if err = c.SetBalance(from, fromNewBalance); err != nil {
		return err
	}
	if err = c.SetBalance(to, toNewBalance); err != nil {
		return err
	}
	c.EmitTransferEvent(from, to, amount)
	return nil
}

func (c *CmdfaContract) validateExperimentAddress(account, role string) error {
	if c.isDid(account) {
		if c.didContract == nil {
			return errors.New("CMDFA: did contract not set")
		}
		valid, err := c.didContract.IsValidDid(account)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("CMDFA experiment: %s is an invalid did", role)
		}
		return nil
	}
	if !address.IsValidAddress(account) || address.IsZeroAddress(account) {
		return fmt.Errorf("CMDFA experiment: %s is an invalid address", role)
	}
	return nil
}

package scheduler

import (
	"fmt"
	"strconv"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
)

func VerifyOptimizeChargeGasTx(block *commonPb.Block, snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider, blockVersion uint32) error {

	// maxbft would have empty block
	if len(block.Txs) == 0 {
		return nil
	}

	// gas to charge from validator
	gasCalc := make(map[string]uint64, 24)
	// gas to charge from proposer
	gasNeedToCharge := make(map[string]uint64, 24)

	contractName := syscontract.SystemContract_ACCOUNT_MANAGER.String()
	methodName := syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String()
	found := false
	for _, tx := range block.Txs {
		if tx.Payload.ContractName == contractName && tx.Payload.Method == methodName {
			found = true
			for _, kv := range tx.Payload.Parameters {
				total, err2 := strconv.ParseUint(string(kv.Value), 10, 64)
				if err2 != nil {
					return fmt.Errorf("ParseUint error: %v", err2)
				}
				gasNeedToCharge[kv.Key] = total
			}
		} else {
			needToCharge := true
			if !checkNativeFilter(tx) || tx.GetPayload().GetTxType() != commonPb.TxType_INVOKE_CONTRACT {
				needToCharge = false
			}
			gasUsed := tx.Result.ContractResult.GasUsed
			address, _, err2 := getPayerAddressAndPkFromTx(tx, snapshot, ac)
			if err2 != nil {
				return err2
			}

			if totalGas, exists := gasCalc[address]; exists {
				if needToCharge {
					gasCalc[address] = totalGas + gasUsed
				}
			} else if needToCharge {
				gasCalc[address] = gasUsed
			} else {
				gasCalc[address] = 0
			}
		}
	}

	if !found {
		return fmt.Errorf("charge gas tx is missing")
	}
	// compare gasCalc and gasNeedToCharge
	if len(gasCalc) != len(gasNeedToCharge) {
		return fmt.Errorf("gas need to charging is not correct, expect %v account, got %v account",
			len(gasCalc), len(gasNeedToCharge))
	}

	for addr, totalGasCalc := range gasCalc {
		if totalGasNeedToCharge, exists := gasNeedToCharge[addr]; !exists {
			return fmt.Errorf("missing some account to charge gas => `%v`", addr)
		} else if totalGasCalc != totalGasNeedToCharge {
			return fmt.Errorf("gas to charge error for address `%v`, expect %v, got %v",
				addr, totalGasCalc, totalGasNeedToCharge)
		}
	}

	return nil
}

func getMultiSignEnableManualRun(chainConfig *configPb.ChainConfig) bool {
	if chainConfig.Vm == nil {
		return false
	} else if chainConfig.Vm.Native == nil {
		return false
	} else if chainConfig.Vm.Native.Multisign == nil {
		return false
	}

	return chainConfig.Vm.Native.Multisign.EnableManualRun
}

// checkNativeFilter checks the tx need to charge gas or not, normal native contract need not to charge gas in v240
func checkNativeFilter(tx *commonPb.Transaction) bool {
	// user contracts need to charge gas
	if !utils.IsNativeContract(tx.GetPayload().GetContractName()) {
		return true
	}

	// install & upgrade contracts need to charge gas
	if tx.GetPayload().GetContractName() == syscontract.SystemContract_CONTRACT_MANAGE.String() {
		if tx.GetPayload().GetMethod() == syscontract.ContractManageFunction_INIT_CONTRACT.String() ||
			tx.GetPayload().GetMethod() == syscontract.ContractManageFunction_UPGRADE_CONTRACT.String() {
			return true
		}
	}
	// multi sign contracts need to charge gas
	return tx.GetPayload().GetContractName() == syscontract.SystemContract_MULTI_SIGN.String()
}

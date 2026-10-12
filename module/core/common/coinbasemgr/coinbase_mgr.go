package coinbasemgr

import (
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	consensuspb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
)

// CheckCoinbaseEnable Check if coinbase is enabled
func CheckCoinbaseEnable(chainConf protocol.ChainConf) bool {

	return IsOptimizeChargeGasEnabled(chainConf) ||
		chainConf.ChainConfig().Consensus.Type == consensuspb.ConsensusType_DPOS
}

// IsOptimizeChargeGasEnabled is optimized charge gas enable
func IsOptimizeChargeGasEnabled(chainConf protocol.ChainConf) bool {
	enableGas := false
	enableOptimizeChargeGas := false
	if chainConf.ChainConfig() == nil || chainConf.ChainConfig().AccountConfig == nil {
		return false
	}

	if chainConf.ChainConfig() == nil || chainConf.ChainConfig().Core == nil {
		return false
	}

	enableGas = chainConf.ChainConfig().AccountConfig.EnableGas
	enableOptimizeChargeGas = chainConf.ChainConfig().Core.EnableOptimizeChargeGas

	return enableGas && enableOptimizeChargeGas
}

// IsCoinBaseTx Returns true if it is a coinbase transaction
//func IsCoinBaseTx(tx *commonPb.Transaction) bool {
//	if tx == nil || tx.Payload == nil ||
//		tx.Payload.ContractName != syscontract.SystemContract_COINBASE.String() ||
//		tx.Payload.Method != syscontract.CoinbaseFunction_RUN_COINBASE.String() ||
//		tx.Payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
//		return false
//	}
//
//	return true
//}

// IsGasTx Returns true if it is a gas transaction
func IsGasTx(tx *commonPb.Transaction) bool {
	if tx == nil || tx.Payload == nil ||
		tx.Payload.ContractName != syscontract.SystemContract_ACCOUNT_MANAGER.String() ||
		tx.Payload.Method != syscontract.GasAccountFunction_CHARGE_GAS_FOR_MULTI_ACCOUNT.String() ||
		tx.Payload.TxType != commonPb.TxType_INVOKE_CONTRACT {
		return false
	}

	return true
}

// FilterCoinBaseTxOrGasTx filter coinbase tx or gas tx
func FilterCoinBaseTxOrGasTx(txs []*commonPb.Transaction) []*commonPb.Transaction {

	// 空块场景避免切片溢出
	if len(txs) == 0 {
		return txs
	}

	// 判断最后一笔交易是不是gas交易
	lastTx := txs[len(txs)-1]
	if !IsGasTx(lastTx) {
		// 非coinbase交易或gas交易的情况下直接返回
		return txs
	}
	// 是coinbase交易或gas交易的情况下将其删除
	newBlockTxs := txs[:len(txs)-1]
	return newBlockTxs
}

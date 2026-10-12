package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/vm-native/v2/accountmgr"

	"chainmaker.org/chainmaker/common/v2/crypto"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

// SenderCollection contains:
// key: address
// value: tx collection will address's other data
type SenderCollection struct {
	txsMap         map[string]*TxCollection
	txAddressCache map[string]string
	specialTxTable []*commonPb.Transaction
}

type TxCollection struct {
	// public key to generate address
	publicKey crypto.PublicKey
	// balance of the address saved at SenderCollection
	accountBalance uint64
	// total gas added each tx
	totalGasUsed uint64
	txs          []*commonPb.Transaction
	// the account is normal or not
	accountStatus commonPb.TxStatusCode
	// Mutex for synchronizing concurrent access to accountBalance
	mu sync.Mutex
}

func (g *TxCollection) addTxGas(gas uint64) error {
	totalGasUsed := g.totalGasUsed + gas
	if totalGasUsed < g.totalGasUsed {
		return errors.New("add tx gas in TxCollection overflow")
	}

	g.totalGasUsed = totalGasUsed
	return nil
}

func (g *TxCollection) String() string {
	pubKeyStr, _ := g.publicKey.String()
	return fmt.Sprintf(
		"\nTxsGroup{ \n\tpublicKey: %s, \n\taccountBalance: %v, \n\ttotalGasUsed: %v, \n\ttxs: [%d items] }",
		pubKeyStr, g.accountBalance, g.totalGasUsed, len(g.txs))
}

// NewSenderCollection new sender collection
func (ts *TxScheduler) NewSenderCollection(
	txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider,
	blockVersion uint32,
	log protocol.Logger) *SenderCollection {

	if blockVersion < blockVersion2340 {
		return &SenderCollection{
			txsMap: getSenderTxCollection(txBatch, snapshot, ac, blockVersion, log),
		}
	}

	// v2.3.4 use new collection
	txAddressMap := make(map[string]string, len(txBatch))
	txCollectionMap, specialTxTable := ts.getSenderTxCollection234(
		txBatch, snapshot, ac, txAddressMap, blockVersion)
	return &SenderCollection{
		txsMap:         txCollectionMap,
		txAddressCache: txAddressMap,
		specialTxTable: specialTxTable,
	}
}

func (s *SenderCollection) getParallelTxsNum() int {
	num := 0
	for _, txCollection := range s.txsMap {
		num += len(txCollection.txs)
	}

	return num
}

// getSenderTxCollection split txs in txBatch by sender account
func getSenderTxCollection(
	txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider,
	blockVersion uint32,
	log protocol.Logger) map[string]*TxCollection {
	txCollectionMap := make(map[string]*TxCollection, len(txBatch))

	chainCfg := snapshot.GetLastChainConfig()

	for _, tx := range txBatch {
		address, pk, err := getPayerAddressAndPkFromTx(tx, snapshot, ac)
		if err != nil {
			continue
		}

		txCollection, exists := txCollectionMap[address]
		if !exists {
			txCollection = &TxCollection{
				publicKey:      pk,
				accountBalance: uint64(0),
				totalGasUsed:   uint64(0),
				txs:            make([]*commonPb.Transaction, 0),
			}
			txCollectionMap[address] = txCollection
		}
		txCollection.txs = append(txCollection.txs, tx)
	}

	if chainCfg.GetBlockVersion() < blockVersion2312 {
		preHandleTxCollectionMap2310(txCollectionMap, snapshot, log)
	} else {
		preHandleTxCollectionMap2312(txCollectionMap, snapshot, log)
	}

	return txCollectionMap
}

// getSenderTxCollection split txs in txBatch by sender account
// consistent with v3.0.0
func (ts *TxScheduler) getSenderTxCollection234(
	txBatch []*commonPb.Transaction,
	snapshot protocol.Snapshot,
	ac protocol.AccessControlProvider,
	txAddressCache map[string]string,
	blockVersion uint32) (map[string]*TxCollection, []*commonPb.Transaction) {

	txCollectionMap := make(map[string]*TxCollection)
	var specialTxList []*commonPb.Transaction

	for _, tx := range txBatch {
		address, pk, err := getPayerAddressAndPkFromTx(tx, snapshot, ac)
		if err != nil {
			ts.log.Errorf("Scheduler getPayerPkAndAddress failed, err = %v", err)
			continue
		}
		txAddressCache[tx.Payload.TxId] = address

		txCollection, exists := txCollectionMap[address]
		if !exists {
			ts.log.Debugf("address = %v, blockVersion = %v", address, blockVersion)
			var (
				balance    int64
				err        error
				statusCode commonPb.TxStatusCode
			)
			if blockVersion < blockVersion2310 {
				balance, err = getAccountBalanceFromSnapshot2300(address, snapshot, ts.log)
				if err != nil {
					ts.log.Warnf("get balance failed, err = %+v", err)
				}
			} else {
				balance, statusCode = getAccountBalanceFromSnapshot2312(address, snapshot, ts.log)
				if statusCode != commonPb.TxStatusCode_SUCCESS {
					ts.log.Warnf("get balance failed, err = %s", statusCode.String())
				}
			}
			txCollection = &TxCollection{
				publicKey:      pk,
				accountBalance: uint64(balance),
				totalGasUsed:   0,
				txs:            make([]*commonPb.Transaction, 0, len(txBatch)),
				accountStatus:  statusCode,
			}
			txCollectionMap[address] = txCollection
		}

		// 判断是否需要扣费
		txNeedChargeGas := ts.checkNativeFilter(
			blockVersion,
			tx.GetPayload().ContractName,
			tx.GetPayload().Method,
			tx, snapshot)

		if txNeedChargeGas {
			// account status abnormal
			if txCollection.accountStatus != commonPb.TxStatusCode_SUCCESS {
				txCollection.txs = append(txCollection.txs, tx)
				continue
			}
			if tx.Payload.Limit != nil {
				if err := txCollection.addTxGas(tx.Payload.Limit.GasLimit); err != nil {
					continue
				}
			}
			if txCollection.totalGasUsed > txCollection.accountBalance {
				// 余额不够，加入 special 队列
				specialTxList = append(specialTxList, tx)
			} else {
				// 余额够，加入并行队列
				txCollection.txs = append(txCollection.txs, tx)
			}
		} else {
			// 不需要扣费，加入并行队列
			txCollection.txs = append(txCollection.txs, tx)
		}
	}

	return txCollectionMap, specialTxList
}

func preHandleTxCollectionMap2310(
	txCollectionMap map[string]*TxCollection,
	snapshot protocol.Snapshot,
	log protocol.Logger) {

	for senderAddress, txCollection := range txCollectionMap {
		// get the account balance from snapshot
		balance, err := getAccountBalanceFromSnapshotLt2312(senderAddress, snapshot, log)
		if err != nil {
			errMsg := fmt.Sprintf("get account balance failed: err = %v", err)
			log.Error(errMsg)
			for _, tx := range txCollection.txs {
				tx.Result = &commonPb.Result{
					Code: commonPb.TxStatusCode_CONTRACT_FAIL,
					ContractResult: &commonPb.ContractResult{
						Code:    uint32(1),
						Result:  nil,
						Message: "",
						GasUsed: uint64(0),
					},
					RwSetHash: nil,
					Message:   errMsg,
				}

			}
		}

		txCollection.accountBalance = uint64(balance)
	}
}

func preHandleTxCollectionMap2312(
	txCollectionMap map[string]*TxCollection,
	snapshot protocol.Snapshot,
	log protocol.Logger) {

	for senderAddress, txCollection := range txCollectionMap {
		// get the account balance from snapshot
		balance, errCode := getAccountBalanceFromSnapshot2312(senderAddress, snapshot, log)
		if errCode != commonPb.TxStatusCode_SUCCESS {
			errMsg := fmt.Sprintf("get account balance failed: errCode = %v", errCode)
			log.Error(errMsg)
			for _, tx := range txCollection.txs {
				tx.Result = &commonPb.Result{
					Code: errCode,
					ContractResult: &commonPb.ContractResult{
						Code:    uint32(1),
						Result:  nil,
						Message: errMsg,
						GasUsed: uint64(0),
					},
					RwSetHash: nil,
					Message:   errMsg,
				}

			}
		}

		txCollection.accountBalance = uint64(balance)
	}
}

// Clear clear addr in txs map
func (s *SenderCollection) Clear() {
	s.txsMap = make(map[string]*TxCollection)
}

func (s *SenderCollection) resetTotalGasUsed() {
	for _, txCollection := range s.txsMap {
		txCollection.totalGasUsed = 0
	}
}

func getAccountBalanceFromSnapshotLt2312(
	address string, snapshot protocol.Snapshot, log protocol.Logger) (int64, error) {
	chainConfig := snapshot.GetLastChainConfig()
	blockVersion := chainConfig.GetBlockVersion()
	log.Debugf("address = %v, blockVersion = %v", address, blockVersion)

	if blockVersion < blockVersion2310 {
		return getAccountBalanceFromSnapshot2300(address, snapshot, log)
	}

	return getAccountBalanceFromSnapshot2310(address, snapshot, log)
}

func getAccountBalanceFromSnapshot2300(
	address string, snapshot protocol.Snapshot, log protocol.Logger) (int64, error) {

	var err error
	var balance int64
	balanceData, err := snapshot.GetKey(-1,
		syscontract.SystemContract_ACCOUNT_MANAGER.String(),
		[]byte(accountmgr.AccountPrefix+address))
	if err != nil {
		return -1, err
	}

	if len(balanceData) == 0 {
		balance = int64(0)
	} else {
		balance, err = strconv.ParseInt(string(balanceData), 10, 64)
		if err != nil {
			return 0, err
		}
	}

	return balance, nil
}

func getAccountBalanceFromSnapshot2310(
	address string, snapshot protocol.Snapshot, log protocol.Logger) (int64, error) {
	var err error
	var balance int64
	var frozen bool

	// 查询账户的余额
	balanceData, err := snapshot.GetKey(-1,
		syscontract.SystemContract_ACCOUNT_MANAGER.String(),
		[]byte(accountmgr.AccountPrefix+address))
	if err != nil {
		return -1, err
	}

	if len(balanceData) == 0 {
		balance = int64(0)
	} else {
		balance, err = strconv.ParseInt(string(balanceData), 10, 64)
		if err != nil {
			return 0, err
		}
	}

	// 查询账户的状态
	frozenData, err := snapshot.GetKey(-1,
		syscontract.SystemContract_ACCOUNT_MANAGER.String(),
		[]byte(accountmgr.FrozenPrefix+address))
	if err != nil {
		return -1, err
	}

	if len(frozenData) == 0 {
		frozen = false
	} else {
		if string(frozenData) == "0" {
			frozen = false
		} else if string(frozenData) == "1" {
			frozen = true
		}
	}
	log.Debugf("balance = %v, freeze = %v", balance, frozen)

	if frozen {
		return 0, fmt.Errorf("account `%s` has been locked", address)
	}

	return balance, nil
}

func getAccountBalanceFromSnapshot2312(
	address string, snapshot protocol.Snapshot, log protocol.Logger) (int64, commonPb.TxStatusCode) {

	var err error
	var balance int64
	var frozen bool

	// 查询账户的余额
	balanceData, err := snapshot.GetKey(-1,
		syscontract.SystemContract_ACCOUNT_MANAGER.String(),
		[]byte(accountmgr.AccountPrefix+address))
	if err != nil {
		return -1, commonPb.TxStatusCode_GET_ACCOUNT_BALANCE_FAILED
	}

	if len(balanceData) == 0 {
		balance = int64(0)
	} else {
		balance, err = strconv.ParseInt(string(balanceData), 10, 64)
		if err != nil {
			return 0, commonPb.TxStatusCode_PARSE_ACCOUNT_BALANCE_FAILED
		}
	}

	// 查询账户的状态
	frozenData, err := snapshot.GetKey(-1,
		syscontract.SystemContract_ACCOUNT_MANAGER.String(),
		[]byte(accountmgr.FrozenPrefix+address))
	if err != nil {
		return -1, commonPb.TxStatusCode_GET_ACCOUNT_STATUS_FAILED
	}

	if len(frozenData) == 0 {
		frozen = false
	} else {
		if string(frozenData) == "0" {
			frozen = false
		} else if string(frozenData) == "1" {
			frozen = true
		}
	}
	log.Debugf("balance = %v, freeze = %v", balance, frozen)

	if frozen {
		return 0, commonPb.TxStatusCode_ACCOUNT_STATUS_FROZEN
	}

	return balance, commonPb.TxStatusCode_SUCCESS
}

func (s *SenderCollection) checkBalanceInSenderCollection(tx *commonPb.Transaction, log protocol.Logger) error {

	// 处理需要扣费，但没有设置 gas_limit 的交易
	if tx.Payload.Limit == nil {
		return errors.New("tx need charge gas, but not gas limit was supplied")
	}

	address, exist := s.txAddressCache[tx.Payload.TxId]
	if !exist {
		return fmt.Errorf("cannot find account balance for %v", tx.Payload.TxId)
	}
	senderTxs, exist := s.txsMap[address]
	if !exist {
		return fmt.Errorf("cannot find account balance for %v", tx.Payload.TxId)
	}

	log.Debugf("[%v]: account address = %s, account balance = %v, block totalGasUsed = %v, tx gas_limit = %v",
		tx.Payload.TxId, address, senderTxs.accountBalance, senderTxs.totalGasUsed, tx.Payload.Limit.GasLimit)
	if senderTxs.totalGasUsed >= senderTxs.accountBalance {
		return fmt.Errorf("account balance is not enough for tx `%v`", tx.Payload.TxId)
	}

	// overflow checking
	totalGasUsed, overflow := uint64SafeAdd(senderTxs.totalGasUsed, tx.Payload.Limit.GasLimit)
	if overflow {
		return fmt.Errorf("totalGasUsed is overflow after add tx `%v` gas_limit", tx.Payload.TxId)
	}
	if totalGasUsed > senderTxs.accountBalance {
		return fmt.Errorf("account balance is not enough for tx `%v` gas_limit", tx.Payload.TxId)
	}

	return nil
}

func (s *SenderCollection) chargeGasInSenderCollection(
	tx *commonPb.Transaction, txResult *commonPb.Result, txNeedChargeGas bool) (uint64, error) {

	// 处理不需要扣费的交易
	if !txNeedChargeGas {
		return 0, nil
	}

	// 处理需要扣费，但没有设置 gas_limit 的交易
	if tx.Payload.Limit == nil {
		return 0, errors.New("tx need charge gas, but not gas limit was supplied")
	}

	address, exist := s.txAddressCache[tx.Payload.TxId]
	if !exist {
		return 0, fmt.Errorf("cannot find account balance for %v", tx.Payload.TxId)
	}
	senderTxs, exist := s.txsMap[address]
	if !exist {
		return 0, fmt.Errorf("cannot find payer address for %v", tx.Payload.TxId)
	}

	senderTxs.mu.Lock()
	defer senderTxs.mu.Unlock()
	gasUsed := txResult.ContractResult.GasUsed

	// gasUsed 超过 gasLimit
	if gasUsed > tx.Payload.Limit.GasLimit {
		gasCharged := tx.Payload.Limit.GasLimit
		err := senderTxs.addTxGas(tx.Payload.Limit.GasLimit)
		if err != nil {
			return gasCharged, fmt.Errorf(
				"totalGasUsed is overflow after add gas_limit of tx `%v`", tx.Payload.TxId)
		}
		return gasCharged, fmt.Errorf("gasUsed(%v) is greater than gasLimit(%v) for tx `%v`",
			txResult.ContractResult.GasUsed, tx.Payload.Limit.GasLimit, tx.Payload.TxId)
	}

	// overflow checking
	totalGasUsed, overflow := uint64SafeAdd(senderTxs.totalGasUsed, gasUsed)
	if overflow {
		return gasUsed, fmt.Errorf(
			"totalGasUsed is overflow after add gas_used of tx `%v`", tx.Payload.TxId)
	}

	if totalGasUsed > senderTxs.accountBalance {
		gasAvailable, overflow2 := uint64SafeSub(senderTxs.accountBalance, senderTxs.totalGasUsed)
		if overflow2 {
			// will not execute here, because checking is exec at beginning of executeTx(...)
			gasAvailable = 0
		}
		senderTxs.totalGasUsed = senderTxs.accountBalance
		return gasAvailable, fmt.Errorf("account balance remains %v, is not enough for tx: %v",
			gasAvailable, tx.Payload.TxId)

	}

	senderTxs.totalGasUsed = totalGasUsed
	return gasUsed, nil
}

func uint64SafeAdd(num1 uint64, num2 uint64) (uint64, bool) {
	result := num1 + num2
	if result < num1 {
		return 0, true
	}
	return result, false
}

func uint64SafeSub(num1 uint64, num2 uint64) (uint64, bool) {
	result := num1 - num2
	if int64(result) >= 0 {
		return result, false
	}
	return 0, true
}

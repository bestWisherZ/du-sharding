/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package snapshot

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chainmaker.org/chainmaker/logger/v2"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	vmPb "chainmaker.org/chainmaker/pb-go/v2/vm"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/test"
	uatomic "go.uber.org/atomic"
)

var _ protocol.TxSimContext = (*MockSimContextImpl)(nil)

// Storage interface for smart contracts
type MockSimContextImpl struct {
	txExecSeq    int32
	tx           *commonPb.Transaction
	txRwSet      *commonPb.TxRWSet
	currentDepth int
	txResult     *commonPb.Result
	gasRemaining uint64
}

func (s *MockSimContextImpl) SubtractGas(gasUsed uint64) error {
	if int64(s.gasRemaining) < 0 {
		return errors.New("gas is not enough")
	}
	s.gasRemaining -= gasUsed
	if int64(s.gasRemaining) < 0 {
		return errors.New("gas is not enough")
	}
	return nil
}

func (s *MockSimContextImpl) GetGasRemaining() uint64 {
	//TODO implement me
	panic("implement me")
}

func (s *MockSimContextImpl) GetBlockFingerprint() string {
	//TODO implement me
	panic("implement me")
}

func (s *MockSimContextImpl) GetKeys(keys []*vmPb.BatchKey) ([]*vmPb.BatchKey, error) {
	//TODO implement me
	panic("implement me")
}

func (s *MockSimContextImpl) GetBlockTimestamp() int64 {
	panic("implement me")
}

func (s *MockSimContextImpl) GetHistoryIterForKey(contractName string, key []byte) (protocol.KeyHistoryIterator, error) {
	panic("implement me")
}

func (s *MockSimContextImpl) SetIterHandle(index int32, iter interface{}) {
	panic("implement me")
}

func (s *MockSimContextImpl) GetIterHandle(index int32) (interface{}, bool) {
	panic("implement me")
}

func (s *MockSimContextImpl) PutIntoReadSet(contractName string, key []byte, value []byte) {
	panic("implement me")
}

func (s *MockSimContextImpl) GetContractByName(name string) (*commonPb.Contract, error) {
	panic("implement me")
}

func (s *MockSimContextImpl) GetContractBytecode(name string) ([]byte, error) {
	panic("implement me")
}

const implement_me = "implement me"

func (s *MockSimContextImpl) GetTxExecSeq() int {
	return int(s.txExecSeq)
}

func (s *MockSimContextImpl) GetDepth() int {
	return s.currentDepth
}
func (s *MockSimContextImpl) GetBlockVersion() uint32 {
	return protocol.DefaultBlockVersion
}

func (s *MockSimContextImpl) CallContract(caller *commonPb.Contract, contract *commonPb.Contract, method string, byteCode []byte,
	parameter map[string][]byte, gasUsed uint64, refTxType commonPb.TxType) (*commonPb.ContractResult,
	protocol.ExecOrderTxType, commonPb.TxStatusCode) {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetCurrentResult() []byte {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetCreator(namespace string) *acPb.Member {
	panic(implement_me)
}

func (s *MockSimContextImpl) Select(namespace string, startKey []byte, limit []byte) (protocol.StateIterator, error) {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetBlockHeight() uint64 {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetBlockProposer() *acPb.Member {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetTxResult() *commonPb.Result {
	return s.txResult
}

func (s *MockSimContextImpl) SetTxResult(result *commonPb.Result) {
	s.txResult = result
}

func (s *MockSimContextImpl) GetSender() *acPb.Member {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetBlockchainStore() protocol.BlockchainStore {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetLastChainConfig() *config.ChainConfig {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetAccessControl() (protocol.AccessControlProvider, error) {
	panic(implement_me)
}

func (s *MockSimContextImpl) GetChainNodesInfoProvider() (protocol.ChainNodesInfoProvider, error) {
	panic(implement_me)
}

// StateDB & ReadWriteSet
// 获取合约账户状态、Code
func (s *MockSimContextImpl) Get(contractName string, key []byte) ([]byte, error) {
	return nil, nil
}
func (s *MockSimContextImpl) GetNoRecord(contractName string, key []byte) ([]byte, error) {
	return nil, nil
}

// 写入合约账户状态
func (s *MockSimContextImpl) Put(contractName string, key []byte, value []byte) error {
	return nil
}

func (s *MockSimContextImpl) PutRecord(contractName string, value []byte, sqlType protocol.SqlType) {
}

// 删除合约账户状态
func (s *MockSimContextImpl) Del(contractName string, key []byte) error {
	return nil
}

func (s *MockSimContextImpl) Done() bool {
	return true
}

func (s *MockSimContextImpl) GetTx() *commonPb.Transaction {
	return s.tx
}
func (s *MockSimContextImpl) GetTxRWSet(runVmSuccess bool) *commonPb.TxRWSet {
	return s.txRwSet
}
func (s *MockSimContextImpl) SetTxExecSeq(txExecSeq int) {
	s.txExecSeq = int32(txExecSeq)
}

func (s *MockSimContextImpl) SetStateSqlHandle(index int32, rows protocol.SqlRows) {
	panic("impl me")

}

func (s *MockSimContextImpl) GetStateSqlHandle(index int32) (protocol.SqlRows, bool) {
	panic("impl me")
}

func (s *MockSimContextImpl) GetStateKvHandle(index int32) (protocol.StateIterator, bool) {
	panic("impl me")
}

func (s *MockSimContextImpl) SetStateKvHandle(index int32, rows protocol.StateIterator) {
	panic("impl me")
}

func (s *MockSimContextImpl) GetTxRWMapByContractName(contractName string) (map[string]*commonPb.TxRead, map[string]*commonPb.TxWrite) {
	panic("impl me")
}
func (s *MockSimContextImpl) GetCrossInfo() uint64 {
	panic("impl me")
}
func (s *MockSimContextImpl) HasUsed(runtimeType commonPb.RuntimeType) bool {
	panic("impl me")
}
func (s *MockSimContextImpl) RecordRuntimeTypeIntoCrossInfo(runtimeType commonPb.RuntimeType) {
	panic("impl me")
}
func (s *MockSimContextImpl) RemoveRuntimeTypeFromCrossInfo() {
	panic("impl me")
}

// GetStrAddrFromPbMember calculate string address from pb Member
func (s *MockSimContextImpl) GetStrAddrFromPbMember(pbMember *acPb.Member) (string, error) {
	panic("impl me")
}

// GetSnapshot
func (s *MockSimContextImpl) GetSnapshot() protocol.Snapshot {
	panic("impl me")
}

func TestKey(t *testing.T) {
	s0 := "你好"
	b0 := []byte(s0)
	s1 := string(b0)
	println("s0 equal s1 ", s0 == s1)
	println("len s1 ", len(s1))

	s2 := string([]byte{0x00, 0x01, 0x02})
	s3 := string([]byte{0x00, 0x01, 0x02})

	println("s2 equal s3 ", s2 == s3)
	println("len s2", len(s2))
}

func TestSnapshot(t *testing.T) {
	for i := 0; i < 20; i++ {
		testSnapshot(t, i)
	}
}

func testSnapshot(t *testing.T, i int) {
	snapshot := &SnapshotImpl{
		lock:              sync.RWMutex{},
		blockchainStore:   nil,
		sealed:            uatomic.NewBool(false),
		chainId:           "",
		blockTimestamp:    0,
		blockProposer:     nil,
		blockHeight:       100,
		blockVersion:      210,
		preSnapshot:       nil,
		txRWSetTable:      nil,
		txTable:           make([]*commonPb.Transaction, 0, 2048),
		txResultMap:       make(map[string]*commonPb.Result, 256),
		readTable:         newShardSet(),
		writeTable:        newShardSet(),
		applyConflictTime: uatomic.NewInt64(0),
		applyAddReadTime:  uatomic.NewInt64(0),
		applyAddWriteTime: uatomic.NewInt64(0),
		log:               &test.GoLogger{},
	}

	txCount := 4000
	start := time.Now()

	var count int64
	wg := sync.WaitGroup{}

	for i := 0; i < txCount; i++ {
		wg.Add(1)
		go func(txIndex int) {
			// Each simulated execution owns its mutable read/write context.
			txSimContext := &MockSimContextImpl{
				tx:       &commonPb.Transaction{Payload: &commonPb.Payload{TxId: fmt.Sprintf("tx-%d", txIndex)}},
				txResult: &commonPb.Result{},
			}
			//fmt.Printf("tx:%d\t", i)
			readKey := randKey()
			writeKey := randKey()
			txSimContext.txRwSet = genRwSet(readKey, writeKey)
			// TODO: Use of weak random number generator (math/rand instead of crypto/rand) ?
			txSimContext.txExecSeq = int32(rand.Intn(snapshot.GetSnapshotSize() + 1)) //nolint: gosec

			applyResult, _ := snapshot.ApplyTxSimContext(txSimContext, protocol.ExecOrderTxTypeNormal,
				true, false)
			atomic.AddInt64(&count, 1)
			if !applyResult {
				fmt.Printf("!!!")
				for {

					randNum := snapshot.GetSnapshotSize() - int(txSimContext.txExecSeq) + 1

					if randNum > 0 {
						txSimContext.txRwSet = genRwSet(readKey, writeKey)
						// TODO: Use of weak random number generator (math/rand instead of crypto/rand) ?
						// nolint: gosec
						txSimContext.txExecSeq = txSimContext.txExecSeq +
							int32(
								rand.Intn(
									randNum,
								),
							)
						applyResult, _ = snapshot.ApplyTxSimContext(txSimContext, protocol.ExecOrderTxTypeNormal, true, false)

						atomic.AddInt64(&count, 1)
						if applyResult {
							break
						}
					}

				}
			}
			wg.Done()
		}(i)

		////fmt.Printf("apply read write set %v, size %d, txExecSeq %d, ", applyResult, len(snapshot.txTable), txExecSeq)
		//for _, txRead := range readKey {
		//	fmt.Printf("%s ", txRead)
		//}
		//fmt.Print("\t")
		//for _, txWrite := range writeKey {
		//	fmt.Printf("%s ", txWrite)
		//}
		//fmt.Println("")
	}
	wg.Wait()
	if size := snapshot.GetSnapshotSize(); size != txCount {
		t.Fatalf("snapshot lost successful executions: got %d, want %d", size, txCount)
	}
	timeCost := time.Since(start)
	snapshot.Seal()
	//dump(snapshot)
	//dumpDAG(snapshot.BuildDAG())

	fmt.Printf("Cost:%v, count:%d\n", timeCost, count)
}

func randKey() []string {
	kRange := 1000000000
	// TODO: Use of weak random number generator (math/rand instead of crypto/rand) ?
	size := rand.Intn(5) + 1 //nolint: gosec

	var keySlice []string
	for i := 0; i < size; i++ {
		// TODO: Use of weak random number generator (math/rand instead of crypto/rand) ?
		kId := rand.Intn(kRange) //nolint: gosec
		key := "K" + strconv.Itoa(kId)
		keySlice = append(keySlice, key)
	}
	return keySlice
}

func genRwSet(readKeySet []string, writeKeySet []string) *commonPb.TxRWSet {
	var txReads []*commonPb.TxRead
	var txWrites []*commonPb.TxWrite
	for _, key := range readKeySet {
		txRead := &commonPb.TxRead{
			Key:     []byte(key),
			Value:   []byte("value"),
			Version: nil,
		}
		txReads = append(txReads, txRead)
	}
	for _, key := range writeKeySet {
		txWrite := &commonPb.TxWrite{
			Key:   []byte(key),
			Value: []byte("value"),
		}
		txWrites = append(txWrites, txWrite)
	}

	txRwSet := &commonPb.TxRWSet{
		TxReads:  txReads,
		TxWrites: txWrites,
	}

	return txRwSet
}

func testApply(txSimContext protocol.TxSimContext, snapshot *SnapshotImpl, txExecSeq int, readKeySet []string, writeKeySet []string) (bool, int) {
	return snapshot.ApplyTxSimContext(txSimContext, protocol.ExecOrderTxTypeNormal, true, false)
}

func dump(snapshot *SnapshotImpl) {
	fmt.Printf("tableSize %+v\n", len(snapshot.txTable))
	fmt.Printf("txTable %+v\n", snapshot.txTable)
	fmt.Printf("readTable %+v\n", snapshot.readTable)
	fmt.Printf("writeTable %+v\n", snapshot.writeTable)
}

func dumpDAG(dag *commonPb.DAG) {
	fmt.Println("digraph DAG {")
	for i, ns := range dag.Vertexes {
		if len(ns.Neighbors) == 0 {
			fmt.Printf("tx%d -> begin%d\n", i, 0)
			continue
		}
		for _, n := range ns.Neighbors {
			fmt.Printf("tx%d -> tx%d\n", i, n)
		}
	}
	fmt.Println("}")
}

var snapshot = &SnapshotImpl{
	lock:              sync.RWMutex{},
	blockchainStore:   nil,
	sealed:            uatomic.NewBool(false),
	chainId:           "",
	blockTimestamp:    0,
	blockProposer:     nil,
	blockHeight:       100,
	preSnapshot:       nil,
	txRWSetTable:      nil,
	txTable:           make([]*commonPb.Transaction, 0, 2048),
	txResultMap:       make(map[string]*commonPb.Result, 256),
	readTable:         newShardSet(),
	writeTable:        newShardSet(),
	applyConflictTime: uatomic.NewInt64(0),
	applyAddReadTime:  uatomic.NewInt64(0),
	applyAddWriteTime: uatomic.NewInt64(0),
	log:               &test.GoLogger{},
}

type fields struct {
	//lock            sync.RWMutex
	blockchainStore protocol.BlockchainStore
	log             protocol.Logger
	sealed          *uatomic.Bool
	chainId         string
	blockTimestamp  int64
	blockProposer   *acPb.Member
	blockHeight     uint64
	blockVersion    uint32
	preBlockHash    []byte
	preSnapshot     protocol.Snapshot
	txRWSetTable    []*commonPb.TxRWSet
	txTable         []*commonPb.Transaction
	specialTxTable  []*commonPb.Transaction
	txResultMap     map[string]*commonPb.Result
	txRoot          []byte
	dagHash         []byte
	rwSetHash       []byte
}
type args struct {
	isSql bool
}

func TestSnapshotImpl_BuildDAG(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *commonPb.DAG
	}{
		// TODO: Add test cases.
		{
			name: "txRWSetTable",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "11111111",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key1"),
								Value: []byte("value of key1"),
							},
						},
					},
					{
						TxId: "222222222",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key1"),
								Value: []byte("value of key1"),
							},
						},
					},
					{
						TxId: "333333333",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key2"),
								Value: []byte("value of key2"),
							},
						},
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
				},
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{0, 1},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SnapshotImpl{
				lock:              sync.RWMutex{},
				blockchainStore:   tt.fields.blockchainStore,
				log:               tt.fields.log,
				sealed:            tt.fields.sealed,
				chainId:           tt.fields.chainId,
				blockTimestamp:    tt.fields.blockTimestamp,
				blockProposer:     tt.fields.blockProposer,
				blockHeight:       tt.fields.blockHeight,
				preBlockHash:      tt.fields.preBlockHash,
				preSnapshot:       tt.fields.preSnapshot,
				txRWSetTable:      tt.fields.txRWSetTable,
				txTable:           tt.fields.txTable,
				specialTxTable:    tt.fields.specialTxTable,
				txResultMap:       tt.fields.txResultMap,
				readTable:         newShardSet(),
				writeTable:        newShardSet(),
				applyConflictTime: uatomic.NewInt64(0),
				applyAddReadTime:  uatomic.NewInt64(0),
				applyAddWriteTime: uatomic.NewInt64(0),
				txRoot:            tt.fields.txRoot,
				dagHash:           tt.fields.dagHash,
				rwSetHash:         tt.fields.rwSetHash,
			}
			if got := s.BuildDAG(tt.args.isSql, nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildDAG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReBuildDag(t *testing.T) {
	tests := []struct {
		name               string
		fields             fields
		args               args
		blockDagRwSetTable []*commonPb.TxRWSet
		want               *commonPb.DAG
	}{
		{
			name: "differentOrderOfApply",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "3",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key2"),
								Value: []byte("value of key2"),
							},
						},
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
					{
						TxId: "1",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key1"),
								Value: []byte("value of key1"),
							},
						},
					},
					{
						TxId: "2",
						TxReads: []*commonPb.TxRead{
							{
								Key:   []byte("key1"),
								Value: []byte("value of key1"),
							},
						},
					},
				},
				log:    logger.GetLogger("test"),
				sealed: uatomic.NewBool(false),
			},
			blockDagRwSetTable: []*commonPb.TxRWSet{
				{
					TxId: "1",
					TxReads: []*commonPb.TxRead{
						{
							Key:   []byte("key1"),
							Value: []byte("value of key1"),
						},
					},
				},
				{
					TxId: "2",
					TxReads: []*commonPb.TxRead{
						{
							Key:   []byte("key1"),
							Value: []byte("value of key1"),
						},
					},
				},
				{
					TxId: "3",
					TxReads: []*commonPb.TxRead{
						{
							Key:   []byte("key2"),
							Value: []byte("value of key2"),
						},
					},
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{0, 1},
					},
				},
			},
		},
		{
			name: "differentOrderOfApply2",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "3",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
					{
						TxId: "1",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
					{
						TxId: "2",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
				},
				log:    logger.GetLogger("test"),
				sealed: uatomic.NewBool(false),
			},
			blockDagRwSetTable: []*commonPb.TxRWSet{
				{
					TxId: "1",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "2",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "3",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{0},
					},
					{
						Neighbors: []uint32{1},
					},
				},
			},
		},
		{
			name: "differentOrderOfApply3",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "3",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
					{
						TxId: "2",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
					{
						TxId: "1",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
				},
				log:    logger.GetLogger("test"),
				sealed: uatomic.NewBool(false),
			},
			blockDagRwSetTable: []*commonPb.TxRWSet{
				{
					TxId: "1",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "2",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "3",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{0},
					},
					{
						Neighbors: []uint32{1},
					},
				},
			},
		},
		{
			name: "orderNotMatter",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "3",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key3"),
								Value: []byte("new value of key3"),
							},
						},
					},
					{
						TxId: "1",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key2"),
							},
						},
					},
					{
						TxId: "2",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key2"),
								Value: []byte("new value of key1"),
							},
						},
					},
				},
				log:    logger.GetLogger("test"),
				sealed: uatomic.NewBool(false),
			},
			blockDagRwSetTable: []*commonPb.TxRWSet{
				{
					TxId: "1",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "2",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key2"),
							Value: []byte("new value of key2"),
						},
					},
				},
				{
					TxId: "3",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key3"),
							Value: []byte("new value of key3"),
						},
					},
				},
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
				},
			},
		},
		{
			name: "orderNotMatter2",
			fields: fields{
				txRWSetTable: []*commonPb.TxRWSet{
					{
						TxId: "3",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key3"),
								Value: []byte("new value of key3"),
							},
						},
					},
					{
						TxId: "2",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key2"),
								Value: []byte("new value of key2"),
							},
						},
					},
					{
						TxId: "1",
						TxWrites: []*commonPb.TxWrite{
							{
								Key:   []byte("key1"),
								Value: []byte("new value of key1"),
							},
						},
					},
				},
				log:    logger.GetLogger("test"),
				sealed: uatomic.NewBool(false),
			},
			blockDagRwSetTable: []*commonPb.TxRWSet{
				{
					TxId: "1",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key1"),
							Value: []byte("new value of key1"),
						},
					},
				},
				{
					TxId: "2",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key2"),
							Value: []byte("new value of key2"),
						},
					},
				},
				{
					TxId: "3",
					TxWrites: []*commonPb.TxWrite{
						{
							Key:   []byte("key3"),
							Value: []byte("new value of key3"),
						},
					},
				},
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SnapshotImpl{
				lock:              sync.RWMutex{},
				blockchainStore:   tt.fields.blockchainStore,
				log:               tt.fields.log,
				sealed:            tt.fields.sealed,
				chainId:           tt.fields.chainId,
				blockTimestamp:    tt.fields.blockTimestamp,
				blockProposer:     tt.fields.blockProposer,
				blockHeight:       tt.fields.blockHeight,
				preBlockHash:      tt.fields.preBlockHash,
				preSnapshot:       tt.fields.preSnapshot,
				txRWSetTable:      []*commonPb.TxRWSet{},
				txTable:           []*commonPb.Transaction{},
				specialTxTable:    tt.fields.specialTxTable,
				txResultMap:       map[string]*commonPb.Result{},
				readTable:         newShardSet(),
				writeTable:        newShardSet(),
				applyConflictTime: uatomic.NewInt64(0),
				applyAddReadTime:  uatomic.NewInt64(0),
				applyAddWriteTime: uatomic.NewInt64(0),
				txRoot:            tt.fields.txRoot,
				dagHash:           tt.fields.dagHash,
				rwSetHash:         tt.fields.rwSetHash,
			}
			for i, t := range tt.fields.txRWSetTable {
				txSimContext := &MockSimContextImpl{
					txExecSeq:    int32(i),
					tx:           &commonPb.Transaction{Payload: &commonPb.Payload{TxId: t.GetTxId()}},
					txRwSet:      t,
					currentDepth: 0,
					txResult:     nil,
				}
				s.ApplyTxSimContext(txSimContext, protocol.ExecOrderTxTypeNormal, true, false)
			}
			if got := s.BuildDAG(tt.args.isSql, tt.blockDagRwSetTable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildDAG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstructKey(t *testing.T) {
	contractName := "save"
	key := []byte("name")
	fmt.Println(constructKey(contractName, key))
}

// oneKey scenario
func TestSnapshotImpl_BuildDAGCaseOneKey(t *testing.T) {

	txRWSetTable1 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want1 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
		},
	}

	txRWSetTable2 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
	}
	txRWSetTable3 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
	}
	txRWSetTable4 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
	}
	txRWSetTable5 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want2 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{2},
			},
		},
	}

	txRWSetTable6 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want3 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{2},
			},
		},
	}

	txRWSetTable7 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want4 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1, 2},
			},
		},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *commonPb.DAG
	}{
		// dag: vertexes:<> vertexes:<> vertexes:<> vertexes:<>
		{
			name: "txRWSetTable",
			fields: fields{
				txRWSetTable: txRWSetTable1,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable2,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable3,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable4,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario4",
			fields: fields{
				txRWSetTable: txRWSetTable5,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		// dag: vertexes:<> vertexes:<> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable6,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable7,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SnapshotImpl{
				lock:              sync.RWMutex{},
				blockchainStore:   tt.fields.blockchainStore,
				log:               tt.fields.log,
				sealed:            tt.fields.sealed,
				chainId:           tt.fields.chainId,
				blockTimestamp:    tt.fields.blockTimestamp,
				blockProposer:     tt.fields.blockProposer,
				blockHeight:       tt.fields.blockHeight,
				preBlockHash:      tt.fields.preBlockHash,
				preSnapshot:       tt.fields.preSnapshot,
				txRWSetTable:      tt.fields.txRWSetTable,
				txTable:           tt.fields.txTable,
				specialTxTable:    tt.fields.specialTxTable,
				txResultMap:       tt.fields.txResultMap,
				readTable:         newShardSet(),
				writeTable:        newShardSet(),
				applyConflictTime: uatomic.NewInt64(0),
				applyAddReadTime:  uatomic.NewInt64(0),
				applyAddWriteTime: uatomic.NewInt64(0),
				txRoot:            tt.fields.txRoot,
				dagHash:           tt.fields.dagHash,
				rwSetHash:         tt.fields.rwSetHash,
			}
			got := s.BuildDAG(tt.args.isSql, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildDAG() = %v, want %v", got, tt.want)
			}

			fmt.Println(got)
		})
	}
}

// twoKey scenario
func TestSnapshotImpl_BuildDAGCaseTwoKey(t *testing.T) {

	txRWSetTable1 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable2 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want1 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{1},
			},
		},
	}

	txRWSetTable3 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable4 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	txRWSetTable5 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want2 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1, 2},
			},
		},
	}

	txRWSetTable6 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable7 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable8 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want3 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{0, 2},
			},
		},
	}

	txRWSetTable9 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
		},
	}
	txRWSetTable10 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
		},
	}
	txRWSetTable11 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
		},
	}
	txRWSetTable12 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
		},
	}
	want4 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{0},
			},
		},
	}

	txRWSetTable13 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable14 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	txRWSetTable15 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want5 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{2},
			},
		},
	}

	txRWSetTable16 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	txRWSetTable17 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	txRWSetTable18 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want6 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{1, 2},
			},
		},
	}

	txRWSetTable19 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("5"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want7 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1, 2},
			},
		},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *commonPb.DAG
	}{
		// dag: vertexes:<> vertexes:<> vertexes:<> vertexes:<neighbors:1>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable1,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable2,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable3,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable4,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: &commonPb.DAG{
				Vertexes: []*commonPb.DAG_Neighbor{
					{
						Neighbors: []uint32{},
					},
					{
						Neighbors: []uint32{0},
					},
					{
						Neighbors: []uint32{0},
					},
					{
						Neighbors: []uint32{1, 2},
					},
				},
			},
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable5,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:0 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable6,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable7,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable8,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:0>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable9,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable10,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable11,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		{
			name: "scenario4",
			fields: fields{
				txRWSetTable: txRWSetTable12,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable13,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want5,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable14,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want5,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable15,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want5,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable16,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable17,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable18,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable19,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SnapshotImpl{
				lock:              sync.RWMutex{},
				blockchainStore:   tt.fields.blockchainStore,
				log:               tt.fields.log,
				sealed:            tt.fields.sealed,
				chainId:           tt.fields.chainId,
				blockTimestamp:    tt.fields.blockTimestamp,
				blockProposer:     tt.fields.blockProposer,
				blockHeight:       tt.fields.blockHeight,
				preBlockHash:      tt.fields.preBlockHash,
				preSnapshot:       tt.fields.preSnapshot,
				txRWSetTable:      tt.fields.txRWSetTable,
				txTable:           tt.fields.txTable,
				specialTxTable:    tt.fields.specialTxTable,
				txResultMap:       tt.fields.txResultMap,
				readTable:         newShardSet(),
				writeTable:        newShardSet(),
				applyConflictTime: uatomic.NewInt64(0),
				applyAddReadTime:  uatomic.NewInt64(0),
				applyAddWriteTime: uatomic.NewInt64(0),
				txRoot:            tt.fields.txRoot,
				dagHash:           tt.fields.dagHash,
				rwSetHash:         tt.fields.rwSetHash,
			}
			got := s.BuildDAG(tt.args.isSql, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildDAG() = %v, want %v", got, tt.want)
			}

			fmt.Println(got)
		})
	}
}

// threeKey scenario
func TestSnapshotImpl_BuildDAGCaseThreeKey(t *testing.T) {

	txRWSetTable1 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable2 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	txRWSetTable3 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want1 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
		},
	}

	txRWSetTable4 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable5 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable6 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want2 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
		},
	}

	txRWSetTable7 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable8 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want3 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1, 2},
			},
		},
	}

	txRWSetTable9 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable10 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable11 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want4 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{0, 1, 2},
			},
		},
	}

	txRWSetTable12 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable13 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want5 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{0, 1, 2},
			},
		},
	}

	txRWSetTable14 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable15 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable16 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want6 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{0, 2},
			},
		},
	}

	txRWSetTable17 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable18 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable19 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want7 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1},
			},
			{
				Neighbors: []uint32{2},
			},
		},
	}

	txRWSetTable20 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable21 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want8 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{2},
			},
		},
	}

	txRWSetTable22 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable23 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want9 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{1, 2},
			},
		},
	}

	txRWSetTable24 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable25 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	txRWSetTable26 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("3"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("4"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("4"),
				},
			},
		},
	}
	want10 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0, 2},
			},
		},
	}

	txRWSetTable27 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
			},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("3"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "44444444",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("4"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want11 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0, 1},
			},
			{
				Neighbors: []uint32{0, 1, 2},
			},
			{
				Neighbors: []uint32{1},
			},
		},
	}

	txRWSetTable28 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId:    "11111111",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("1"),
				},
			},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
		},
	}
	txRWSetTable29 := []*commonPb.TxRWSet{
		{
			TxId:    "00000000",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("1"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "22222222",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("1"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId:    "33333333",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
			},
		},
	}
	txRWSetTable30 := []*commonPb.TxRWSet{
		{
			TxId: "00000000",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
		{
			TxId: "11111111",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("1"),
				},
			},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("1"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k3"),
					Value:        []byte("1"),
				},
			},
		},
		{
			TxId:    "22222222",
			TxReads: []*commonPb.TxRead{},
			TxWrites: []*commonPb.TxWrite{
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("1"),
				},
			},
		},
		{
			TxId: "33333333",
			TxReads: []*commonPb.TxRead{
				{
					ContractName: "E_CNY",
					Key:          []byte("k1"),
					Value:        []byte("2"),
				},
				{
					ContractName: "E_CNY",
					Key:          []byte("k2"),
					Value:        []byte("2"),
				},
			},
			TxWrites: []*commonPb.TxWrite{},
		},
	}
	want12 := &commonPb.DAG{
		Vertexes: []*commonPb.DAG_Neighbor{
			{
				Neighbors: []uint32{},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{0},
			},
			{
				Neighbors: []uint32{1, 2},
			},
		},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *commonPb.DAG
	}{
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:0>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable1,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable2,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable3,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want1,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable4,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable5,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable6,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want2,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable7,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable8,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want3,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:0 neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable9,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable10,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable11,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want4,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:0 neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable12,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want5,
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable13,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want5,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:0 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable14,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable15,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable16,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want6,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:1> vertexes:<neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable17,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want7,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable18,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want7,
		},
		{
			name: "scenario3",
			fields: fields{
				txRWSetTable: txRWSetTable19,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want7,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable20,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want8,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable21,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want8,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable22,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want9,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable23,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want9,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<> vertexes:<neighbors:0 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable24,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want10,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable25,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want10,
		},
		{
			name: "scenario2",
			fields: fields{
				txRWSetTable: txRWSetTable26,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want10,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0 neighbors:1> vertexes:<neighbors:0 neighbors:1 neighbors:2> vertexes:<neighbors:1>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable27,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want11,
		},
		// dag: vertexes:<> vertexes:<neighbors:0> vertexes:<neighbors:0> vertexes:<neighbors:1 neighbors:2>
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable28,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want12,
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable29,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want12,
		},
		{
			name: "scenario1",
			fields: fields{
				txRWSetTable: txRWSetTable30,
				blockHeight:  1,
				blockVersion: 221,
				txTable: []*commonPb.Transaction{
					{},
					{},
					{},
					{},
				},
				log: logger.GetLogger("test"),
			},
			args: args{
				isSql: false,
			},
			want: want12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SnapshotImpl{
				lock:              sync.RWMutex{},
				blockchainStore:   tt.fields.blockchainStore,
				log:               tt.fields.log,
				sealed:            tt.fields.sealed,
				chainId:           tt.fields.chainId,
				blockTimestamp:    tt.fields.blockTimestamp,
				blockProposer:     tt.fields.blockProposer,
				blockHeight:       tt.fields.blockHeight,
				preBlockHash:      tt.fields.preBlockHash,
				preSnapshot:       tt.fields.preSnapshot,
				txRWSetTable:      tt.fields.txRWSetTable,
				txTable:           tt.fields.txTable,
				specialTxTable:    tt.fields.specialTxTable,
				txResultMap:       tt.fields.txResultMap,
				readTable:         newShardSet(),
				writeTable:        newShardSet(),
				applyConflictTime: uatomic.NewInt64(0),
				applyAddReadTime:  uatomic.NewInt64(0),
				applyAddWriteTime: uatomic.NewInt64(0),
				txRoot:            tt.fields.txRoot,
				dagHash:           tt.fields.dagHash,
				rwSetHash:         tt.fields.rwSetHash,
			}
			got := s.BuildDAG(tt.args.isSql, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildDAG() = %v, want %v", got, tt.want)
			}

			fmt.Println(got)
		})
	}
}

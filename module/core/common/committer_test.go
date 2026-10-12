package common

import (
	"encoding/hex"
	"reflect"
	"testing"

	mbusmock "chainmaker.org/chainmaker/common/v2/msgbus/mock"
	"chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	consensusPb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func TestCommitBlock_CommitBlock(t *testing.T) {

	ctl := gomock.NewController(t)
	log := logger.GetLoggerByChain(logger.MODULE_CORE, "chain1")
	b0 := createBlock(11)
	block := createNewTestBlock(12)
	hash, err := utils.CalcBlockHash("SHA256", block)
	require.Nil(t, err)
	block.Header.BlockHash = hash

	// snapshotManager
	snapshotManager := mock.NewMockSnapshotManager(ctl)
	//snapshotManager.EXPECT().NotifyBlockCommitted(block).Return(nil)

	// 	ledgerCache
	ledgerCache := mock.NewMockLedgerCache(ctl)
	//ledgerCache.EXPECT().SetLastCommittedBlock(block)

	// msgbus
	msgbus := mbusmock.NewMockMessageBus(ctl)
	msgbus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return()

	// storehelper
	storeHelper := mock.NewMockStoreHelper(ctl)

	// txfilter
	txFilter := mock.NewMockTxFilter(ctl)

	// proposalCache
	proposalCache := mock.NewMockProposalCache(ctl)

	// txpool
	txpool := mock.NewMockTxPool(ctl)

	//chainConf mock
	chainConf := mock.NewMockChainConf(ctl)

	// Mock blockChain Store
	store := mock.NewMockBlockchainStore(ctl)

	log.Infof("init block(%d,%s)", block.Header.BlockHeight, hex.EncodeToString(block.Header.BlockHash))
	//store.EXPECT().PutBlock(block, txRWSets).Return(nil)

	ledgerCache.EXPECT().GetLastCommittedBlock().Return(b0)

	txRWSetMap := make(map[string]*commonpb.TxRWSet)
	tx0 := block.Txs[0]
	contractName := "testContract"
	txRWSetMap[tx0.Payload.TxId] = &commonpb.TxRWSet{
		TxId: tx0.Payload.TxId,
		TxReads: []*commonpb.TxRead{{
			ContractName: contractName,
			Key:          []byte("K1"),
			Value:        []byte("V"),
		}},
		TxWrites: []*commonpb.TxWrite{{
			ContractName: contractName,
			Key:          []byte("K2"),
			Value:        []byte("V"),
		}},
	}

	conEventMap := make(map[string][]*commonpb.ContractEvent)
	proposalCache.EXPECT().GetProposedBlock(gomock.Any()).Return(block, txRWSetMap, conEventMap)

	config := &config.ChainConfig{
		ChainId: "chain1",
		Crypto: &config.CryptoConfig{
			Hash: "SHA256",
		},
		Block: &config.BlockConfig{
			BlockTxCapacity: 1000,
			BlockSize:       1,
			BlockInterval:   DEFAULTDURATION,
		},
		Consensus: &config.ConsensusConfig{
			Type: consensusPb.ConsensusType_RAFT,
		},
		Core: &config.CoreConfig{
			ConsensusTurboConfig: nil,
		},
	}

	chainConf.EXPECT().ChainConfig().AnyTimes().Return(config)

	txRWSets := []*commonpb.TxRWSet{
		txRWSetMap[tx0.Payload.TxId],
	}
	store.EXPECT().PutBlock(block, txRWSets).Return(nil)

	ledgerCache.EXPECT().SetLastCommittedBlock(gomock.Any()).Times(1)

	snapshotManager.EXPECT().NotifyBlockCommitted(gomock.Any()).Times(1)

	proposalCache.EXPECT().GetProposedBlocksAt(gomock.Any()).Return([]*commonpb.Block{block}).Times(1)

	txpool.EXPECT().RetryAndRemoveTxs(gomock.Any(), gomock.Any()).Times(1)

	proposalCache.EXPECT().ClearProposedBlockAt(gomock.Any()).Times(1)

	msgbus.EXPECT().PublishSafe(gomock.Any(), gomock.Any()).Times(1)

	cbConf := BlockCommitterConfig{
		ChainId:         "chain1",
		BlockchainStore: store,
		SnapshotManager: snapshotManager,
		TxPool:          txpool,
		LedgerCache:     ledgerCache,
		ProposedCache:   proposalCache,
		ChainConf:       chainConf,
		MsgBus:          msgbus,
		StoreHelper:     storeHelper,
		TxFilter:        txFilter,
	}

	committer, err := NewBlockCommitter(cbConf, log)
	require.Nil(t, err)

	err = committer.AddBlock(block)
	require.Nil(t, err)
}

func TestCommitBlock_CommitBlock_DPOS(t *testing.T) {

	ctl := gomock.NewController(t)
	log := logger.GetLoggerByChain(logger.MODULE_CORE, "chain1")
	b0 := createBlock(11)
	block := createNewTestBlock(12)
	block.Header.BlockVersion = blockVersion230
	hash, err := utils.CalcBlockHash("SHA256", block)
	require.Nil(t, err)

	block.Header.BlockHash = hash

	// snapshotManager
	snapshotManager := mock.NewMockSnapshotManager(ctl)
	//snapshotManager.EXPECT().NotifyBlockCommitted(block).Return(nil)

	// 	ledgerCache
	ledgerCache := mock.NewMockLedgerCache(ctl)
	//ledgerCache.EXPECT().SetLastCommittedBlock(block)

	// msgbus
	msgbus := mbusmock.NewMockMessageBus(ctl)
	msgbus.EXPECT().Publish(gomock.Any(), gomock.Any()).Return()

	// storehelper
	storeHelper := mock.NewMockStoreHelper(ctl)

	// txfilter
	txFilter := mock.NewMockTxFilter(ctl)

	// proposalCache
	proposalCache := mock.NewMockProposalCache(ctl)

	// txpool
	txpool := mock.NewMockTxPool(ctl)

	//chainConf mock
	chainConf := mock.NewMockChainConf(ctl)

	// Mock blockChain Store
	store := mock.NewMockBlockchainStore(ctl)

	log.Infof("init block(%d,%s)", block.Header.BlockHeight, hex.EncodeToString(block.Header.BlockHash))
	//store.EXPECT().PutBlock(block, txRWSets).Return(nil)

	ledgerCache.EXPECT().GetLastCommittedBlock().Return(b0)

	txRWSetMap := make(map[string]*commonpb.TxRWSet)
	tx0 := block.Txs[0]
	contractName := "testContract"
	txRWSetMap[tx0.Payload.TxId] = &commonpb.TxRWSet{
		TxId: tx0.Payload.TxId,
		TxReads: []*commonpb.TxRead{{
			ContractName: contractName,
			Key:          []byte("K1"),
			Value:        []byte("V"),
		}},
		TxWrites: []*commonpb.TxWrite{{
			ContractName: contractName,
			Key:          []byte("K2"),
			Value:        []byte("V"),
		}},
	}

	conEventMap := make(map[string][]*commonpb.ContractEvent)
	proposalCache.EXPECT().GetProposedBlock(gomock.Any()).Return(block, txRWSetMap, conEventMap)

	config := &config.ChainConfig{
		ChainId: "chain1",
		Crypto: &config.CryptoConfig{
			Hash: "SHA256",
		},
		Block: &config.BlockConfig{
			BlockTxCapacity: 1000,
			BlockSize:       1,
			BlockInterval:   DEFAULTDURATION,
		},
		Consensus: &config.ConsensusConfig{
			Type: consensusPb.ConsensusType_DPOS,
		},
		Core: &config.CoreConfig{
			ConsensusTurboConfig: nil,
		},
	}

	chainConf.EXPECT().ChainConfig().AnyTimes().Return(config)

	txRWSets := []*commonpb.TxRWSet{
		txRWSetMap[tx0.Payload.TxId],
	}
	store.EXPECT().PutBlock(block, txRWSets).Return(nil)

	ledgerCache.EXPECT().SetLastCommittedBlock(gomock.Any()).Times(1)

	snapshotManager.EXPECT().NotifyBlockCommitted(gomock.Any()).Times(1)

	proposalCache.EXPECT().GetProposedBlocksAt(gomock.Any()).Return([]*commonpb.Block{block}).Times(1)

	txpool.EXPECT().RetryAndRemoveTxs(gomock.Any(), gomock.Any()).Times(1)

	proposalCache.EXPECT().ClearProposedBlockAt(gomock.Any()).Times(1)

	msgbus.EXPECT().PublishSafe(gomock.Any(), gomock.Any()).Times(1)

	cbConf := BlockCommitterConfig{
		ChainId:         "chain1",
		BlockchainStore: store,
		SnapshotManager: snapshotManager,
		TxPool:          txpool,
		LedgerCache:     ledgerCache,
		ProposedCache:   proposalCache,
		ChainConf:       chainConf,
		MsgBus:          msgbus,
		StoreHelper:     storeHelper,
		TxFilter:        txFilter,
	}

	committer, err := NewBlockCommitter(cbConf, log)
	require.Nil(t, err)

	err = committer.AddBlock(block)
	require.Nil(t, err)
}

func createNewTestBlock(height uint64) *commonpb.Block {
	var hash = []byte("0123456789")
	var block = &commonpb.Block{
		Header: &commonpb.BlockHeader{
			ChainId:        "Chain1",
			BlockHeight:    height,
			PreBlockHash:   hash,
			BlockHash:      hash,
			PreConfHeight:  0,
			BlockVersion:   1,
			DagHash:        hash,
			RwSetRoot:      hash,
			TxRoot:         hash,
			BlockTimestamp: 0,
			Proposer: &accesscontrol.Member{
				OrgId:      "org1",
				MemberType: 0,
				MemberInfo: nil,
			},
			TxCount:   0,
			Signature: nil,
		},
		Dag: &commonpb.DAG{
			Vertexes: nil,
		},
		Txs:            nil,
		AdditionalData: &commonpb.AdditionalData{ExtraData: map[string][]byte{}},
	}
	tx := createNewTestTx("0123456789")
	txs := make([]*commonpb.Transaction, 1)
	txs[0] = tx
	block.Txs = txs
	return block
}

func Test_rearrangeContractEvent(t *testing.T) {
	type args struct {
		block       *commonpb.Block
		conEventMap map[string][]*commonpb.ContractEvent
	}
	tests := []struct {
		name string
		args args
		want []*commonpb.ContractEvent
	}{
		{
			name: "test0",
			args: args{
				block:       createBlock(0),
				conEventMap: nil,
			},
			want: make([]*commonpb.ContractEvent, 0),
		},
		{
			name: "test1",
			args: args{
				block: func() *commonpb.Block {
					block := createBlock(0)
					block.Txs = []*commonpb.Transaction{
						{
							Payload: &commonpb.Payload{
								TxId: "123456",
							},
						},
					}
					return block
				}(),
				conEventMap: map[string][]*commonpb.ContractEvent{
					"test": {
						{
							TxId: "123456",
						},
					},
				},
			},
			want: make([]*commonpb.ContractEvent, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rearrangeContractEvent(tt.args.block, tt.args.conEventMap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rearrangeContractEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

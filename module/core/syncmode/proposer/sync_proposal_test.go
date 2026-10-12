package proposer

import (
	"sync/atomic"
	"testing"

	"chainmaker.org/chainmaker-go/module/core/common"
	"chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"chainmaker.org/chainmaker/txpool-batch/v2"
	"github.com/golang/mock/gomock"
)

func TestSpecialProposingDoesNotRotateOrdinaryQueueForRawPending(t *testing.T) {
	ctl := gomock.NewController(t)
	pool := mock.NewMockTxPool(ctl)
	cache := mock.NewMockProposalCache(ctl)
	conf := mock.NewMockChainConf(ctl)
	oldType := common.TxPoolType
	common.TxPoolType = "single"
	t.Cleanup(func() { common.TxPoolType = oldType; syncproposer.ClearReadyBatch() })
	scheduler.Init(scheduler.Config{Enable: true, Role: "BUSINESS", ShardId: "business-shard1",
		StartHeight: 10, BusinessBatch: 1, BridgeBatch: 4}, nil, nil)
	scheduler.ShouldProduce(11)
	scheduler.OnBlockCommitted(&commonpb.Block{Header: &commonpb.BlockHeader{BlockHeight: 11}})
	var pending int32 = 1
	syncproposer.RegisterPendingChecker(func() bool { return atomic.LoadInt32(&pending) != 0 })
	t.Cleanup(func() { atomic.StoreInt32(&pending, 0) })
	syncproposer.ClearReadyBatch()
	cache.EXPECT().GetSelfProposedBlockAt(uint64(12)).Return(nil).Times(2)
	conf.EXPECT().ChainConfig().Return(&configpb.ChainConfig{Block: &configpb.BlockConfig{BlockTxCapacity: 1000}}).Times(2)
	// No FetchTxs or RetryAndRemoveTxs expectation: either call fails the test.
	bp := &BlockProposerImpl{idle: true, proposalCache: cache, chainConf: conf, txPool: pool, log: log}
	if block := bp.proposing(12, nil); block != nil {
		t.Fatal("raw message produced a block without an admitted sync transaction")
	}
	t.Run("exact IDs skip ordinary transactions", func(t *testing.T) {
		tx := trackerSyncTx("sync")
		pool.EXPECT().GetTxsInPoolByTxIds([]string{"sync"}).Return([]*commonpb.Transaction{tx}, nil, nil)
		txs, err := bp.getReadySpecialSyncTxs([]string{"sync"})
		if err != nil || len(txs) != 1 || txs[0] != tx {
			t.Fatalf("exact sync selection failed: %v", err)
		}
	})
	t.Run("missing ID cannot become a partial wave", func(t *testing.T) {
		pool.EXPECT().GetTxsInPoolByTxIds([]string{"missing"}).Return(nil, []string{"missing"}, nil)
		if _, err := bp.getReadySpecialSyncTxs([]string{"missing"}); err == nil {
			t.Fatal("missing transaction admitted to sync proposal")
		}
	})
	t.Run("batch pool fallback stops after one ordinary batch", func(t *testing.T) {
		common.TxPoolType = batch.TxPoolType
		ordinary := &commonpb.Transaction{Payload: &commonpb.Payload{TxId: "ordinary", Method: "Transfer"}}
		pool.EXPECT().FetchTxBatches(uint64(12)).Return([]string{"ordinary-batch"}, [][]*commonpb.Transaction{{ordinary}})
		pool.EXPECT().RetryAndRemoveTxBatches([]string{"ordinary-batch"}, gomock.Nil())
		if block := bp.proposing(12, nil); block != nil {
			t.Fatal("ordinary batch produced a special sync block")
		}
	})
}

package globalstate

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/internal/received"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func resetCommittedWaveTest(t *testing.T) {
	t.Helper()
	oldCfg, oldChecker := globalCfg, txCommitted
	oldReceived := receivedCheckpoints
	receivedCheckpoints = received.New(4096)
	oldTasks, oldPositions := pendingBridge.tasks, pendingBridge.positions
	oldCommitted := pendingBridge.committedWaves
	oldCycle, oldSlot, oldTotal := pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal
	oldAnnounced, oldVersion := pendingBridge.waveAnnounced, pendingBridge.version
	oldWrites, oldBytes := pendingBridge.queuedWrites, pendingBridge.queuedBytes
	oldReady := atomic.LoadInt32(&bridgeReady)
	atomic.StoreInt32(&bridgeReady, 0)
	globalCfg = &shardingsync.ShardingConfig{Schedule: shardingsync.ScheduleConfig{
		BusinessShards: []string{"business-shard1", "business-shard2"},
	}}
	txCommitted = nil
	pendingBridge.tasks = make(map[string]*pendingBridgeSync)
	pendingBridge.positions = make(map[string]string)
	pendingBridge.committedWaves = nil
	pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal = 0, 0, 0
	pendingBridge.waveAnnounced = false
	pendingBridge.queuedWrites, pendingBridge.queuedBytes = 0, 0
	t.Cleanup(func() {
		receivedCheckpoints = oldReceived
		globalCfg, txCommitted = oldCfg, oldChecker
		pendingBridge.tasks, pendingBridge.positions = oldTasks, oldPositions
		pendingBridge.committedWaves = oldCommitted
		pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal = oldCycle, oldSlot, oldTotal
		pendingBridge.waveAnnounced, pendingBridge.version = oldAnnounced, oldVersion
		pendingBridge.queuedWrites, pendingBridge.queuedBytes = oldWrites, oldBytes
		atomic.StoreInt32(&bridgeReady, oldReady)
	})
}

func TestBridgeCachedReadinessTracksCompleteWaveAndCommit(t *testing.T) {
	resetCommittedWaveTest(t)
	enqueueBridgeSync("business-shard1", waveTestDelta(1, 1))
	if bridgePendingReady() {
		t.Fatal("incomplete wave released cached gate")
	}
	enqueueBridgeSync("business-shard2", waveTestDelta(1, 1))
	// The gate must not acquire the raw-task lock, even while a flush is active.
	pendingBridge.Lock()
	ready := bridgePendingReady()
	pendingBridge.Unlock()
	if !ready {
		t.Fatal("complete raw wave did not release gate before proposer selection")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 1, 1, 0, 1), waveTestTx("business-shard2", 1, 1, 0, 1),
	}})
	if bridgePendingReady() {
		t.Fatal("committed wave left a stale readiness flag")
	}
}

func waveTestDelta(cycle uint64, slot int) *CompactDelta {
	height := cycle*10 + uint64(slot)
	return &CompactDelta{
		StartHeight: height, Height: height, BlockHash: fmt.Sprintf("block-%d", height),
		Cycle: cycle, Slot: slot, SlotTotal: 2, Checkpoint: true,
	}
}

func waveTestTx(shard string, cycle uint64, slot, chunk, total int) *commonpb.Transaction {
	delta := waveTestDelta(cycle, slot)
	params := map[string]string{
		crossshard.SourceShardParamKey:       shard,
		crossshard.SourceHeightParamKey:      strconv.FormatUint(delta.Height, 10),
		crossshard.SourceStartHeightParamKey: strconv.FormatUint(delta.StartHeight, 10),
		crossshard.SourceBlockHashParamKey:   delta.BlockHash,
		crossshard.SyncCycleParamKey:         strconv.FormatUint(cycle, 10),
		crossshard.SyncSlotParamKey:          strconv.Itoa(slot),
		crossshard.SyncSlotTotalParamKey:     "2",
		crossshard.SyncCheckpointParamKey:    "true",
		crossshard.SyncChunkIndexParamKey:    strconv.Itoa(chunk),
		crossshard.SyncChunkTotalParamKey:    strconv.Itoa(total),
	}
	tx := &commonpb.Transaction{
		Payload: &commonpb.Payload{
			TxId:   fmt.Sprintf("%s:%d:%d:%d", shard, cycle, slot, chunk),
			TxType: commonpb.TxType_INVOKE_CONTRACT, ContractName: "sharding_heartbeat",
			Method: crossshard.SystemBridgeStateSyncApplyMethod,
		},
		Result: &commonpb.Result{Code: commonpb.TxStatusCode_SUCCESS, ContractResult: &commonpb.ContractResult{}},
	}
	for key, value := range params {
		tx.Payload.Parameters = append(tx.Payload.Parameters, &commonpb.KeyValuePair{Key: key, Value: []byte(value)})
	}
	return tx
}

func TestCommittedWaveAdvancesWithoutLocalAnnouncement(t *testing.T) {
	resetCommittedWaveTest(t)
	// One original checkpoint is delayed; all of the next wave is already queued.
	enqueueBridgeSync("business-shard1", waveTestDelta(5, 1))
	enqueueBridgeSync("business-shard1", waveTestDelta(5, 2))
	enqueueBridgeSync("business-shard2", waveTestDelta(5, 2))
	if pendingBridge.waveAnnounced || hasPendingBridgeSync() {
		t.Fatal("incomplete wave unexpectedly released consensus")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 5, 1, 0, 1), waveTestTx("business-shard2", 5, 1, 0, 1),
	}})
	if pendingBridge.waveCycle != 5 || pendingBridge.waveSlot != 2 || !hasPendingBridgeSync() {
		t.Fatalf("committed wave did not release next wave: %d/%d", pendingBridge.waveCycle, pendingBridge.waveSlot)
	}
	if enqueueBridgeSync("business-shard2", waveTestDelta(5, 1)) {
		t.Fatal("late raw checkpoint reopened a committed wave")
	}
	if len(pendingBridge.tasks) != 2 {
		t.Fatalf("stale tasks were not cleaned: %d", len(pendingBridge.tasks))
	}
}

func TestCommittedWaveBootstrapsWithoutAnyRawMessages(t *testing.T) {
	resetCommittedWaveTest(t)
	block := &commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 1, 1, 0, 1), waveTestTx("business-shard2", 1, 1, 0, 1),
	}}
	completePendingBridgeSync(block)
	completePendingBridgeSync(block)
	if pendingBridge.waveCycle != 1 || pendingBridge.waveSlot != 2 || len(pendingBridge.committedWaves) != 0 {
		t.Fatal("missing raw messages or duplicate commits corrupted wave progress")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 1, 2, 0, 1), waveTestTx("business-shard2", 1, 2, 0, 1),
	}})
	if pendingBridge.waveCycle != 2 || pendingBridge.waveSlot != 1 {
		t.Fatal("last committed slot did not advance the cycle")
	}
}

func TestCommittedWaveRequiresAllSuccessfulChunks(t *testing.T) {
	resetCommittedWaveTest(t)
	first := waveTestTx("business-shard1", 1, 1, 0, 2)
	failed := waveTestTx("business-shard2", 1, 1, 0, 1)
	failed.Result.ContractResult.Code = 1
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{first, first, failed}})
	if pendingBridge.waveSlot != 1 {
		t.Fatal("duplicate or failed chunks advanced the wave")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{waveTestTx("business-shard2", 1, 1, 0, 1)}})
	if pendingBridge.waveSlot != 1 {
		t.Fatal("an incomplete chunk set advanced the wave")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{waveTestTx("business-shard1", 1, 1, 1, 2)}})
	if pendingBridge.waveSlot != 2 {
		t.Fatal("complete chunk sets did not advance the wave")
	}
}

func TestCommittedWaveAllowsRemainingRawShards(t *testing.T) {
	resetCommittedWaveTest(t)
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{waveTestTx("business-shard1", 1, 1, 0, 1)}})
	enqueueBridgeSync("business-shard2", waveTestDelta(1, 1))
	if !hasPendingBridgeSync() {
		t.Fatal("a committed shard without raw data blocked the remaining raw checkpoint")
	}
	pendingBridge.Lock()
	keys := bridgeFlushKeysLocked(true)
	pendingBridge.Unlock()
	if len(keys) != 1 || pendingBridge.tasks[keys[0]].sourceShard != "business-shard2" {
		t.Fatalf("incorrect remaining checkpoint batch: %v", keys)
	}
}

func TestCommittedFutureWaveDoesNotSkipMissingSlot(t *testing.T) {
	resetCommittedWaveTest(t)
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 1, 2, 0, 1), waveTestTx("business-shard2", 1, 2, 0, 1),
	}})
	if pendingBridge.waveCycle != 0 {
		t.Fatal("future slot bypassed the missing first slot")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{
		waveTestTx("business-shard1", 1, 1, 0, 1), waveTestTx("business-shard2", 1, 1, 0, 1),
	}})
	if pendingBridge.waveCycle != 2 || pendingBridge.waveSlot != 1 {
		t.Fatal("buffered committed slots did not advance in order")
	}
}

func TestCommittedWaveRejectsInconsistentMetadata(t *testing.T) {
	resetCommittedWaveTest(t)
	first := waveTestTx("business-shard1", 1, 1, 0, 2)
	conflict := waveTestTx("business-shard1", 1, 1, 1, 2)
	for _, param := range conflict.Payload.Parameters {
		if param.Key == crossshard.SourceBlockHashParamKey {
			param.Value = []byte("different-block")
		}
	}
	unknown := waveTestTx("unknown-shard", 1, 1, 0, 1)
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{first, conflict, unknown,
		waveTestTx("business-shard2", 1, 1, 0, 1)}})
	if pendingBridge.waveSlot != 1 {
		t.Fatal("inconsistent checkpoint metadata advanced a wave")
	}
}

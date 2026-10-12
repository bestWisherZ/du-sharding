package globalstate

import (
	"reflect"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
)

func TestBridgeRawSyncReadyUsesThresholdAndForceFlush(t *testing.T) {
	oldCfg := globalCfg
	globalCfg = &shardingsync.ShardingConfig{
		Schedule: shardingsync.ScheduleConfig{
			StateSyncFlushWrites: 10,
			StateSyncFlushBytes:  100,
		},
	}
	syncproposer.SetForceFlush(false)

	pendingBridge.Lock()
	oldWrites := pendingBridge.queuedWrites
	oldBytes := pendingBridge.queuedBytes
	defer func() {
		pendingBridge.queuedWrites = oldWrites
		pendingBridge.queuedBytes = oldBytes
		pendingBridge.Unlock()
		syncproposer.SetForceFlush(false)
		globalCfg = oldCfg
	}()
	pendingBridge.queuedWrites = 9
	pendingBridge.queuedBytes = 99
	if bridgeRawSyncReadyLocked() {
		t.Fatal("batch below both thresholds must remain queued")
	}
	pendingBridge.queuedWrites = 10
	if !bridgeRawSyncReadyLocked() {
		t.Fatal("write threshold did not release batch")
	}
	pendingBridge.queuedWrites = 1
	pendingBridge.queuedBytes = 1
	syncproposer.SetForceFlush(true)
	if !bridgeRawSyncReadyLocked() {
		t.Fatal("phase boundary did not release final partial batch")
	}
}

func TestBridgeFlushKeysPreservePerShardHeightOrder(t *testing.T) {
	pendingBridge.Lock()
	oldTasks := pendingBridge.tasks
	pendingBridge.tasks = map[string]*pendingBridgeSync{
		"s1-h12": {
			sourceShard: "business-shard1",
			delta:       &CompactDelta{Height: 12},
		},
		"s2-h11": {
			sourceShard: "business-shard2",
			delta:       &CompactDelta{Height: 11},
		},
		"s1-h11": {
			sourceShard: "business-shard1",
			delta:       &CompactDelta{Height: 11},
		},
	}
	defer func() {
		pendingBridge.tasks = oldTasks
		pendingBridge.Unlock()
	}()

	if got, want := bridgeFlushKeysLocked(true), []string{"s1-h11", "s2-h11"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("flush keys = %v, want %v", got, want)
	}
	if got := bridgeFlushKeysLocked(false); len(got) != 0 {
		t.Fatalf("non-released raw tasks were selected: %v", got)
	}
	pendingBridge.tasks["s1-h11"].submitted = true
	if got, want := bridgeFlushKeysLocked(false), []string{"s1-h11"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("submitted flush keys = %v, want %v", got, want)
	}
}

func TestBridgeWaveWaitsForEveryConfiguredShard(t *testing.T) {
	oldCfg := globalCfg
	globalCfg = &shardingsync.ShardingConfig{
		Schedule: shardingsync.ScheduleConfig{
			BusinessShards: []string{"business-shard1", "business-shard2", "business-shard3"},
		},
	}

	pendingBridge.Lock()
	oldTasks := pendingBridge.tasks
	oldPositions := pendingBridge.positions
	oldCycle := pendingBridge.waveCycle
	oldSlot := pendingBridge.waveSlot
	oldTotal := pendingBridge.waveTotal
	oldAnnounced := pendingBridge.waveAnnounced
	oldCommitted := pendingBridge.committedWaves
	pendingBridge.tasks = make(map[string]*pendingBridgeSync)
	pendingBridge.positions = make(map[string]string)
	pendingBridge.waveCycle = 0
	pendingBridge.waveSlot = 0
	pendingBridge.waveTotal = 0
	pendingBridge.waveAnnounced = false
	pendingBridge.committedWaves = nil
	defer func() {
		pendingBridge.tasks = oldTasks
		pendingBridge.positions = oldPositions
		pendingBridge.waveCycle = oldCycle
		pendingBridge.waveSlot = oldSlot
		pendingBridge.waveTotal = oldTotal
		pendingBridge.waveAnnounced = oldAnnounced
		pendingBridge.committedWaves = oldCommitted
		pendingBridge.Unlock()
		globalCfg = oldCfg
	}()

	add := func(id, shard string, cycle uint64, slot int) {
		delta := &CompactDelta{Cycle: cycle, Slot: slot, SlotTotal: 2, Height: uint64(10 + slot)}
		pendingBridge.tasks[id] = &pendingBridgeSync{
			sourceShard: shard,
			delta:       delta,
			txIds:       map[string]struct{}{id: {}},
		}
		pendingBridge.positions[bridgeWaveSourceKey(shard, delta)] = id
	}

	// A complete future wave must not overtake a missing first wave.
	add("s1-2", "business-shard1", 1, 2)
	add("s2-2", "business-shard2", 1, 2)
	add("s3-2", "business-shard3", 1, 2)
	if bridgeRawSyncReadyLocked() {
		t.Fatal("slot 2 was released before slot 1")
	}

	add("s1-1", "business-shard1", 1, 1)
	add("s2-1", "business-shard2", 1, 1)
	if bridgeRawSyncReadyLocked() {
		t.Fatal("incomplete slot 1 was released")
	}
	add("s3-1", "business-shard3", 1, 1)
	if !bridgeRawSyncReadyLocked() {
		t.Fatal("complete slot 1 was not released")
	}
	if got, want := bridgeFlushKeysLocked(true), []string{"s1-1", "s2-1", "s3-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wave keys = %v, want %v", got, want)
	}

	pendingBridge.waveAnnounced = true
	deleteBridgeTaskLocked("s1-1")
	deleteBridgeTaskLocked("s2-1")
	deleteBridgeTaskLocked("s3-1")
	if maybeAdvanceBridgeWaveLocked() {
		t.Fatal("empty local queue advanced a wave without committed checkpoints")
	}
	for _, shard := range globalCfg.Schedule.BusinessShards {
		recordCommittedBridgeWaveLocked(crossshard.BridgeStateSyncInfo{
			SourceShard: shard, StartHeight: 11, EndHeight: 11,
			Cycle: 1, Slot: 1, SlotTotal: 2, Checkpoint: true, ChunkTotal: 1,
		}, "block-11")
	}
	if !maybeAdvanceBridgeWaveLocked() {
		t.Fatal("completed wave did not advance")
	}
	if pendingBridge.waveCycle != 1 || pendingBridge.waveSlot != 2 {
		t.Fatalf("next wave = %d/%d, want 1/2", pendingBridge.waveCycle, pendingBridge.waveSlot)
	}
	if !bridgeWaveIsStaleLocked(&CompactDelta{Cycle: 1, Slot: 1, SlotTotal: 2}) {
		t.Fatal("completed slot was not recognized as stale")
	}
	if bridgeWaveIsStaleLocked(&CompactDelta{Cycle: 2, Slot: 1, SlotTotal: 2}) {
		t.Fatal("future cycle was incorrectly recognized as stale")
	}
	if got, want := bridgeFlushKeysLocked(true), []string{"s1-2", "s2-2", "s3-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("next wave keys = %v, want %v", got, want)
	}
}

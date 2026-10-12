package globalstate

import "chainmaker.org/chainmaker-go/module/sharding/crossshard"

type bridgeWavePosition struct {
	cycle uint64
	slot  int
}

type committedBridgeWave struct {
	slotTotal int
	shards    map[string]*committedBridgeCheckpoint
}

type committedBridgeCheckpoint struct {
	startHeight uint64
	endHeight   uint64
	blockHash   string
	chunkTotal  int
	chunks      map[int]struct{}
}

// Commit metadata, not raw-message arrival order, is authoritative for progress.
// Callers hold pendingBridge's lock and only pass successful committed txs.
func recordCommittedBridgeWaveLocked(info crossshard.BridgeStateSyncInfo, blockHash string) {
	if !info.Checkpoint || info.Cycle == 0 || info.Slot <= 0 || info.SlotTotal < info.Slot ||
		info.ChunkTotal <= 0 || info.ChunkIndex < 0 || info.ChunkIndex >= info.ChunkTotal ||
		!isExpectedBusinessShard(info.SourceShard) {
		return
	}
	if info.Cycle < pendingBridge.waveCycle ||
		(info.Cycle == pendingBridge.waveCycle && info.Slot < pendingBridge.waveSlot) {
		return
	}
	if pendingBridge.committedWaves == nil {
		pendingBridge.committedWaves = make(map[bridgeWavePosition]*committedBridgeWave)
	}
	position := bridgeWavePosition{info.Cycle, info.Slot}
	wave := pendingBridge.committedWaves[position]
	if wave == nil {
		wave = &committedBridgeWave{
			slotTotal: info.SlotTotal,
			shards:    make(map[string]*committedBridgeCheckpoint),
		}
		pendingBridge.committedWaves[position] = wave
	}
	if wave.slotTotal != info.SlotTotal {
		globalLog.Errorf("[GlobalState] committed sync wave has inconsistent slot total | cycle=%d slot=%d", info.Cycle, info.Slot)
		return
	}
	checkpoint := wave.shards[info.SourceShard]
	if checkpoint == nil {
		checkpoint = &committedBridgeCheckpoint{
			startHeight: info.StartHeight,
			endHeight:   info.EndHeight,
			blockHash:   blockHash,
			chunkTotal:  info.ChunkTotal,
			chunks:      make(map[int]struct{}),
		}
		wave.shards[info.SourceShard] = checkpoint
	}
	if checkpoint.startHeight != info.StartHeight || checkpoint.endHeight != info.EndHeight ||
		checkpoint.blockHash != blockHash || checkpoint.chunkTotal != info.ChunkTotal {
		globalLog.Errorf("[GlobalState] committed sync checkpoint has inconsistent chunks | cycle=%d slot=%d shard=%s",
			info.Cycle, info.Slot, info.SourceShard)
		return
	}
	checkpoint.chunks[info.ChunkIndex] = struct{}{}
}

func bridgeWaveShardCommittedLocked(shard string) bool {
	wave := pendingBridge.committedWaves[bridgeWavePosition{pendingBridge.waveCycle, pendingBridge.waveSlot}]
	if wave == nil || wave.slotTotal != pendingBridge.waveTotal {
		return false
	}
	checkpoint := wave.shards[shard]
	return checkpoint != nil && len(checkpoint.chunks) == checkpoint.chunkTotal
}

func bridgeWaveCommittedLocked() bool {
	shards := expectedBusinessShards()
	if pendingBridge.waveCycle == 0 || pendingBridge.waveTotal <= 0 || len(shards) == 0 {
		return false
	}
	for _, shard := range shards {
		if !bridgeWaveShardCommittedLocked(shard) {
			return false
		}
	}
	return true
}

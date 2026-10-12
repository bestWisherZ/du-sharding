package globalstate

import (
	"encoding/hex"
	"sort"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	shardingscheduler "chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func NotifyBlockCommitted(block *commonpb.Block, rwSetMap map[string]*commonpb.TxRWSet) {
	completePendingBridgeSync(block)
	if !shouldSend() || block == nil || block.Header == nil {
		return
	}
	if !shardingscheduler.IsAfterStartHeight(block.Header.BlockHeight) {
		return
	}
	if isBridgeWritebackOnlyBlock(block) {
		globalLog.Debugf("[GlobalState] 跳过桥接回写系统块检查点 | shard=%s height=%d txs=%d",
			globalCfg.ShardId, block.Header.BlockHeight, len(block.Txs))
		return
	}
	writes := collectDeltaWrites(block, rwSetMap)
	blockHash := hex.EncodeToString(block.Header.BlockHash)
	cycle, slot, slotTotal, positioned := shardingscheduler.BusinessCheckpointPosition(block.Header.BlockHeight)
	if !positioned {
		globalLog.Warnf("[GlobalState] 业务检查点缺少调度位置 | shard=%s height=%d",
			globalCfg.ShardId, block.Header.BlockHeight)
	}
	sendCompactDelta(&CompactDelta{
		StartHeight: block.Header.BlockHeight,
		Height:      block.Header.BlockHeight,
		BlockHash:   blockHash,
		Timestamp:   block.Header.BlockTimestamp,
		Cycle:       cycle,
		Slot:        slot,
		SlotTotal:   slotTotal,
		Checkpoint:  true,
		Writes:      writes,
	})
}

// Bridge-originated writebacks already have authoritative state on bridge.
// Their business-shard blocks must not re-enter the business-to-bridge
// checkpoint stream; the scheduler advertises the exact normal block heights
// that the bridge barrier must wait for.
func isBridgeWritebackOnlyBlock(block *commonpb.Block) bool {
	if block == nil || len(block.Txs) == 0 {
		return false
	}
	for _, tx := range block.Txs {
		if !crossshard.IsStateSyncTx(tx) {
			return false
		}
	}
	return true
}

func sendCompactDelta(delta *CompactDelta) {
	if delta == nil || delta.Height == 0 {
		return
	}
	chunks, err := encodeBridgeStateSyncChunks(globalCfg.ShardId, delta)
	if err != nil {
		globalLog.Errorf("[GlobalState] encode authorization chunks: %v", err)
		return
	}
	signer, err := getStateSyncSigner()
	if err != nil {
		globalLog.Errorf("[GlobalState] get source signer: %v", err)
		return
	}
	delta.Authorizations = make([][]byte, len(chunks))
	for i, chunk := range chunks {
		payload := bridgeStateSyncPayload(globalCfg.SyncAuth.ChainIDs["bridge"], globalCfg.ShardId, delta, chunk, i, len(chunks))
		delta.Authorizations[i], err = syncauth.Sign(globalCfg.SyncAuth, globalCfg.ShardId, "bridge", payload, signer.certPem, signer.sk)
		if err != nil {
			globalLog.Errorf("[GlobalState] sign source checkpoint: %v", err)
			return
		}
	}
	var encodedWrites []byte
	if len(chunks) == 1 && chunks[0].writeCount == len(delta.Writes) {
		encodedWrites = chunks[0].writesRaw
	}
	msg, err := buildEncodedCompactDeltaMessage(globalCfg.ShardId, delta, encodedWrites)
	if err != nil {
		globalLog.Warnf("[GlobalState] 构建状态增量失败: %v", err)
		return
	}
	if err := shardingsync.BroadcastStateMessageToShard("bridge-shard", msg); err != nil {
		globalLog.Errorf("[GlobalState] 发送状态增量失败: %v", err)
		return
	}
	if len(delta.Writes) == 0 {
		globalLog.Debugf("[GlobalState] 已发送链上同步检查点 | shard=%s height=%d bytes=%d cycle=%d slot=%d/%d",
			globalCfg.ShardId, delta.Height, len(msg), delta.Cycle, delta.Slot, delta.SlotTotal)
		return
	}
	globalLog.Infof("[GlobalState] 已发送链上状态批次 | shard=%s height=%d writes=%d checkpoint=%v bytes=%d cycle=%d slot=%d/%d",
		globalCfg.ShardId, delta.Height, len(delta.Writes), delta.Checkpoint, len(msg),
		delta.Cycle, delta.Slot, delta.SlotTotal)
}

func collectDeltaWrites(block *commonpb.Block, rwSetMap map[string]*commonpb.TxRWSet) []DeltaWrite {
	if block == nil || len(block.Txs) == 0 || len(rwSetMap) == 0 {
		return nil
	}
	byKey := make(map[string]DeltaWrite)
	keys := make([]string, 0)
	for _, tx := range block.Txs {
		if tx == nil || tx.Payload == nil {
			continue
		}
		if tx.Result != nil && tx.Result.Code != commonpb.TxStatusCode_SUCCESS {
			continue
		}
		if !shouldSyncStateTx(tx.Payload.ContractName, tx.Payload.Method) {
			continue
		}
		rw := rwSetMap[tx.Payload.TxId]
		if rw == nil {
			continue
		}
		for _, write := range rw.TxWrites {
			if write == nil || !shouldSyncStateWrite(write.ContractName, write.Key) {
				continue
			}
			aggKey := write.ContractName + "\x00" + string(write.Key)
			if _, exists := byKey[aggKey]; !exists {
				keys = append(keys, aggKey)
			}
			byKey[aggKey] = DeltaWrite{
				Contract: write.ContractName,
				Key:      cloneStateBytes(write.Key),
				Value:    cloneStateBytes(write.Value),
			}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	writes := make([]DeltaWrite, 0, len(keys))
	for _, key := range keys {
		writes = append(writes, byKey[key])
	}
	return writes
}

func shouldSend() bool {
	if globalCfg == nil {
		return false
	}
	if !globalCfg.SyncNetwork.Enable {
		return false
	}
	if strings.ToUpper(globalCfg.ShardType) != "BUSINESS" {
		return false
	}
	return true
}

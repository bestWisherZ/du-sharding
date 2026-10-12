package globalstate

import (
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"encoding/hex"
	"sort"
	"strings"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	storepb "chainmaker.org/chainmaker/pb-go/v2/store"
)

func handleBlockDelta(shardId string, bw *storepb.BlockWithRWSet) {
	if !shouldReceive() || bw == nil || bw.Block == nil || bw.Block.Header == nil {
		return
	}
	txById := make(map[string]bool, len(bw.Block.Txs))
	for _, tx := range bw.Block.Txs {
		if tx == nil || tx.Payload == nil {
			continue
		}
		if tx.Result != nil && tx.Result.Code != commonpb.TxStatusCode_SUCCESS {
			txById[tx.Payload.TxId] = false
			continue
		}
		txById[tx.Payload.TxId] = shouldSyncStateTx(tx.Payload.ContractName, tx.Payload.Method)
	}
	byKey := make(map[string]DeltaWrite)
	keys := make([]string, 0)
	for _, rw := range bw.TxRWSets {
		if rw == nil {
			continue
		}
		if syncTx, ok := txById[rw.TxId]; ok && !syncTx {
			continue
		}
		for _, w := range rw.TxWrites {
			if w == nil || !shouldSyncStateWrite(w.ContractName, w.Key) {
				continue
			}
			aggKey := w.ContractName + "\x00" + string(w.Key)
			if _, exists := byKey[aggKey]; !exists {
				keys = append(keys, aggKey)
			}
			byKey[aggKey] = DeltaWrite{
				Contract: w.ContractName,
				Key:      cloneStateBytes(w.Key),
				Value:    cloneStateBytes(w.Value),
			}
		}
	}
	sort.Strings(keys)
	writes := make([]DeltaWrite, 0, len(keys))
	for _, key := range keys {
		writes = append(writes, byKey[key])
	}
	handleCompactDelta(shardId, &CompactDelta{
		StartHeight: bw.Block.Header.BlockHeight,
		Height:      bw.Block.Header.BlockHeight,
		BlockHash:   hex.EncodeToString(bw.Block.Header.BlockHash),
		Timestamp:   bw.Block.Header.BlockTimestamp,
		Checkpoint:  true,
		Writes:      writes,
	})
}

func handleCompactDelta(shardId string, delta *CompactDelta) bool {
	if !shouldReceive() || delta == nil || delta.Height == 0 {
		return false
	}
	if delta.StartHeight == 0 {
		delta.StartHeight = delta.Height
	}
	chunks, err := encodeBridgeStateSyncChunks(shardId, delta)
	if err != nil || len(delta.Authorizations) != len(chunks) {
		globalLog.Errorf("[GlobalState] missing/invalid source checkpoint authorizations | shard=%s height=%d", shardId, delta.Height)
		return false
	}
	for i, chunk := range chunks {
		p := bridgeStateSyncPayload(globalCfg.SyncAuth.ChainIDs["bridge"], shardId, delta, chunk, i, len(chunks))
		if err := syncauth.Verify(globalCfg.SyncAuth, shardId, "bridge", p); err != nil {
			globalLog.Errorf("[GlobalState] unauthorized source checkpoint | shard=%s height=%d err=%v", shardId, delta.Height, err)
			return false
		}
	}
	if !enqueueEncodedBridgeSync(shardId, delta, chunks) {
		return true
	}
	globalLog.Infof("[GlobalState] bridge 状态同步任务已缓存 | shard=%s heights=%d-%d writes=%d checkpoint=%v",
		shardId, delta.StartHeight, delta.Height, len(delta.Writes), delta.Checkpoint)
	return true
}

func handleDelta(shardId string, bw *storepb.BlockWithRWSet) {
	handleBlockDelta(shardId, bw)
}

func shouldReceive() bool {
	if globalCfg == nil {
		return false
	}
	if !globalCfg.SyncNetwork.Enable {
		return false
	}
	return strings.ToUpper(globalCfg.ShardType) == "BRIDGE"
}

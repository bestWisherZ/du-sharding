package crossshard

import (
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
)

func SendBatch(batch *CrossWriteBatch) {
	if crossCfg == nil || batch == nil {
		return
	}
	if !crossCfg.SyncNetwork.Enable {
		return
	}
	if strings.ToUpper(crossCfg.ShardType) != "BRIDGE" {
		return
	}
	crossLog.Infof("[CrossShard] 准备发送回写批次 | batch=%s shard=%s writes=%d",
		batch.BatchId, batch.TargetShard, len(batch.Writes))
	sendToShard(batch.TargetShard, batch)
}

func sendToShard(targetShard string, batch *CrossWriteBatch) {
	if targetShard == "" {
		return
	}
	chunks, err := buildApplyWriteChunks(batch)
	if err != nil {
		crossLog.Errorf("[CrossShard] encode writeback authorization: %v", err)
		return
	}
	signer, err := getApplySigner()
	if err != nil {
		crossLog.Errorf("[CrossShard] get writeback signer: %v", err)
		return
	}
	batch.Authorizations = make(map[string][]byte, len(chunks))
	for _, chunk := range chunks {
		payload := applyWriteChunkPayload(crossCfg.SyncAuth.ChainIDs[syncauth.NormalizeShard(targetShard)], chunk)
		batch.Authorizations[chunk.txId], err = syncauth.Sign(crossCfg.SyncAuth, "bridge", targetShard, payload, signer.certPem, signer.sk)
		if err != nil {
			crossLog.Errorf("[CrossShard] sign writeback: %v", err)
			return
		}
	}
	msg, err := BuildBatchMessage(targetShard, batch)
	if err != nil {
		crossLog.Warnf("[CrossShard] 构建回写消息失败 | shard=%s batch=%s err=%v",
			targetShard, batch.BatchId, err)
		return
	}
	if err := shardingsync.BroadcastStateMessageToShard(targetShard, msg); err != nil {
		crossLog.Warnf("[CrossShard] 回写发送失败 | shard=%s batch=%s err=%v",
			targetShard, batch.BatchId, err)
	} else {
		crossLog.Infof("[CrossShard] 已发送回写 | shard=%s batch=%s", targetShard, batch.BatchId)
	}
}

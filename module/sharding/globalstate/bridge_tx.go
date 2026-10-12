package globalstate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	commonerrors "chainmaker.org/chainmaker/common/v2/errors"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

const defaultStateSyncMaxWritesPerTx = 4000

// Encoded writes belong to the pending task and remain immutable across retries.
type bridgeStateSyncChunk struct {
	writesRaw  []byte
	writeCount int
	txId       string
	logicalId  string
}

func submitBridgeStateSync(task *pendingBridgeSync) error {
	if !syncproposer.IsLocalProposer() {
		return fmt.Errorf("local node is no longer the bridge sync proposer")
	}
	if submitTx == nil {
		return fmt.Errorf("tx submitter not set")
	}
	if task == nil || task.delta == nil || task.delta.Height == 0 || strings.TrimSpace(task.sourceShard) == "" || len(task.chunks) == 0 {
		return fmt.Errorf("invalid bridge state sync delta")
	}
	sourceShard, delta, chunks := task.sourceShard, task.delta, task.chunks
	chunkTotal := len(chunks)
	chainId, err := pickChainId()
	if err != nil {
		return err
	}
	for chunkIndex, chunk := range chunks {
		txId := chunk.txId
		if txCommitted != nil {
			exists, checkErr := txCommitted(chainId, txId)
			if checkErr != nil {
				return fmt.Errorf("check committed bridge state sync tx failed: %v", checkErr)
			}
			if exists {
				globalLog.Infof("[GlobalState] bridge 状态同步交易已提交 | tx_id=%s shard=%s height=%d chunk=%d/%d",
					txId, sourceShard, delta.Height, chunkIndex+1, chunkTotal)
				continue
			}
		}
		if txInPool != nil {
			exists, checkErr := txInPool(chainId, txId)
			if checkErr != nil {
				return fmt.Errorf("check bridge state sync tx in pool failed: %v", checkErr)
			}
			if exists {
				syncproposer.NotifyTxSubmitted(txId)
				globalLog.Infof("[GlobalState] bridge 状态同步交易已由 proposer 传播 | tx_id=%s shard=%s height=%d chunk=%d/%d",
					txId, sourceShard, delta.Height, chunkIndex+1, chunkTotal)
				continue
			}
		}
		prepared, _, buildErr := task.prepareChunk(chainId, chunkIndex)
		if buildErr != nil {
			return buildErr
		}
		if !syncproposer.IsLocalProposer() {
			return fmt.Errorf("bridge sync proposer changed during preparation")
		}
		tx, buildErr := signPreparedBridgeStateSyncTx(prepared)
		if buildErr != nil {
			return buildErr
		}
		if !syncproposer.IsLocalProposer() {
			return fmt.Errorf("bridge sync proposer changed during signing")
		}
		if submitErr := submitTx(chainId, tx); submitErr != nil {
			if isDuplicateBridgeStateSyncError(submitErr) {
				syncproposer.NotifyTxSubmitted(txId)
				globalLog.Infof("[GlobalState] bridge 状态同步交易已存在 | tx_id=%s shard=%s height=%d chunk=%d/%d",
					tx.Payload.TxId, sourceShard, delta.Height, chunkIndex+1, chunkTotal)
				continue
			}
			return submitErr
		}
		syncproposer.NotifyTxSubmitted(txId)
		globalLog.Infof("[GlobalState] bridge 状态同步交易已入池 | tx_id=%s shard=%s heights=%d-%d chunk=%d/%d writes=%d checkpoint=%v",
			tx.Payload.TxId, sourceShard, delta.StartHeight, delta.Height, chunkIndex+1, chunkTotal,
			chunk.writeCount, delta.Checkpoint)
	}
	return nil
}

func encodeBridgeStateSyncChunks(sourceShard string, delta *CompactDelta) ([]bridgeStateSyncChunk, error) {
	writes := make([]crossshard.BridgeStateWrite, 0, len(delta.Writes))
	for _, write := range delta.Writes {
		if !shouldSyncStateWrite(write.Contract, write.Key) {
			continue
		}
		// The encoder owns its output. Keep nil (delete) distinct from empty.
		writes = append(writes, crossshard.BridgeStateWrite{
			ContractName: write.Contract,
			Key:          write.Key,
			Value:        write.Value,
		})
	}
	maxWrites := globalCfg.Schedule.StateSyncMaxWritesPerTx
	if maxWrites <= 0 {
		maxWrites = defaultStateSyncMaxWritesPerTx
	}
	chunkTotal := (len(writes) + maxWrites - 1) / maxWrites
	if chunkTotal == 0 {
		chunkTotal = 1
	}
	chunks := make([]bridgeStateSyncChunk, 0, chunkTotal)
	for index := 0; index < chunkTotal; index++ {
		start := index * maxWrites
		end := start + maxWrites
		if end > len(writes) {
			end = len(writes)
		}
		writesRaw, err := crossshard.EncodeBridgeStateWrites(writes[start:end])
		if err != nil {
			return nil, err
		}
		logicalId := bridgeStateSyncTxId(sourceShard, delta, index, chunkTotal, writesRaw)
		chunk := bridgeStateSyncChunk{
			writesRaw: writesRaw, writeCount: end - start,
			txId: logicalId, logicalId: logicalId,
		}
		chunk.txId = bridgeStateSyncPayload(globalCfg.SyncAuth.ChainIDs["bridge"], sourceShard, delta, chunk, index, chunkTotal).TxId
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func buildBridgeStateSyncTx(chainId, sourceShard string, delta *CompactDelta,
	chunk bridgeStateSyncChunk, chunkIndex, chunkTotal int) (*commonpb.Transaction, error) {
	prepared, err := prepareBridgeStateSyncTx(chainId, sourceShard, delta, chunk, chunkIndex, chunkTotal)
	if err != nil {
		return nil, err
	}
	return signPreparedBridgeStateSyncTx(prepared)
}

func bridgeStateSyncPayload(chainId, sourceShard string, delta *CompactDelta,
	chunk bridgeStateSyncChunk, chunkIndex, chunkTotal int) *commonpb.Payload {
	// Preparation only borrows immutable chunk bytes. The release-time signer
	// makes an isolated payload before handing the transaction to the pool.
	writesRaw := chunk.writesRaw
	params := []*commonpb.KeyValuePair{
		{Key: syncauth.LogicalIDParameter, Value: []byte(chunk.logicalId)},
		{Key: crossshard.SourceShardParamKey, Value: []byte(sourceShard)},
		{Key: crossshard.SourceStartHeightParamKey, Value: []byte(strconv.FormatUint(delta.StartHeight, 10))},
		{Key: crossshard.SourceHeightParamKey, Value: []byte(strconv.FormatUint(delta.Height, 10))},
		{Key: crossshard.SourceBlockHashParamKey, Value: []byte(delta.BlockHash)},
		{Key: crossshard.SyncCheckpointParamKey, Value: []byte(strconv.FormatBool(delta.Checkpoint))},
		{Key: crossshard.SyncChunkIndexParamKey, Value: []byte(strconv.Itoa(chunkIndex))},
		{Key: crossshard.SyncChunkTotalParamKey, Value: []byte(strconv.Itoa(chunkTotal))},
		{Key: crossshard.SyncCycleParamKey, Value: []byte(strconv.FormatUint(delta.Cycle, 10))},
		{Key: crossshard.SyncSlotParamKey, Value: []byte(strconv.Itoa(delta.Slot))},
		{Key: crossshard.SyncSlotTotalParamKey, Value: []byte(strconv.Itoa(delta.SlotTotal))},
		{Key: crossshard.StateWritesParamKey, Value: writesRaw},
		{Key: crossshard.StateWritesEncodingParamKey, Value: []byte(crossshard.BridgeStateWritesEncodingV1)},
	}
	if chunkIndex < len(delta.Authorizations) && len(delta.Authorizations[chunkIndex]) != 0 {
		params = append(params, &commonpb.KeyValuePair{Key: syncauth.Parameter, Value: delta.Authorizations[chunkIndex]})
	}
	txTimestamp, expirationTime := deterministicPayloadTimes(delta.Timestamp, systemSyncTxExpiration)
	payload := &commonpb.Payload{
		ChainId:        chainId,
		TxType:         commonpb.TxType_INVOKE_CONTRACT,
		TxId:           chunk.txId,
		Timestamp:      txTimestamp,
		ExpirationTime: expirationTime,
		ContractName:   "sharding_heartbeat",
		Method:         crossshard.SystemBridgeStateSyncApplyMethod,
		Parameters:     params,
	}
	syncauth.BindPayloadID(payload)
	return payload
}

func bridgeStateSyncTxId(sourceShard string, delta *CompactDelta, chunkIndex, chunkTotal int,
	writesRaw []byte) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%s|", crossshard.BridgeStateWritesEncodingV1)
	fmt.Fprintf(sum, "%s|%d|%d|%s|%d|%d|%d|%t|%d|%d|", sourceShard, delta.StartHeight,
		delta.Height, delta.BlockHash, delta.Cycle, delta.Slot, delta.SlotTotal,
		delta.Checkpoint, chunkIndex, chunkTotal)
	_, _ = hex.NewEncoder(sum).Write(writesRaw)
	return hex.EncodeToString(sum.Sum(nil))
}

func isDuplicateBridgeStateSyncError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, commonerrors.ErrTxIdExist) || errors.Is(err, commonerrors.ErrTxIdExistDB)
}

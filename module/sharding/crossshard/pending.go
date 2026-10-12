package crossshard

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

type pendingApplyWrite struct {
	chunk     *applyWriteChunk
	idKey     string
	txId      string
	submitted bool
}

var pendingApply = struct {
	sync.Mutex
	tasks    map[string]*pendingApplyWrite
	version  uint64
	flushing bool
}{tasks: make(map[string]*pendingApplyWrite)}

const applySubmitRetryDelay = 500 * time.Millisecond

var applyReady int32

func applyPendingReady() bool { return atomic.LoadInt32(&applyReady) != 0 }

func refreshApplyReadyLocked() {
	var ready int32
	if len(pendingApply.tasks) > 0 {
		ready = 1
	}
	atomic.StoreInt32(&applyReady, ready)
}

func enqueueApplyBatch(batch *CrossWriteBatch) int {
	if batch == nil {
		return 0
	}
	chunks, err := buildApplyWriteChunks(batch)
	if err != nil {
		crossLog.Warnf("[CrossShard] 构造批量回写任务失败 | batch=%s err=%v", batch.BatchId, err)
		return 0
	}
	queued := 0
	known := make(map[string]bool, len(chunks))
	for _, chunk := range chunks {
		pendingApply.Lock()
		_, duplicate := pendingApply.tasks[chunk.idKey]
		pendingApply.Unlock()
		// idKey includes the complete write content and checkpoint metadata;
		// this task was authenticated on its first admission.
		if duplicate {
			known[chunk.idKey] = true
			continue
		}
		payload := applyWriteChunkPayload(crossCfg.SyncAuth.ChainIDs[syncauth.NormalizeShard(crossCfg.ShardId)], chunk)
		if err := syncauth.Verify(crossCfg.SyncAuth, "bridge", crossCfg.ShardId, payload); err != nil {
			crossLog.Errorf("[CrossShard] reject unauthenticated writeback | batch=%s err=%v", batch.BatchId, err)
			return 0
		}
	}
	for _, chunk := range chunks {
		if known[chunk.idKey] {
			continue
		}
		pendingApply.Lock()
		_, duplicate := pendingApply.tasks[chunk.idKey]
		pendingApply.Unlock()
		if duplicate {
			continue
		}
		processed, checkErr := isProcessed(crossCfg.ShardId, "logical:"+chunk.logicalId)
		if checkErr == nil && !processed {
			processed, checkErr = isProcessed(crossCfg.ShardId, chunk.idKey)
		}
		err = checkErr
		if err != nil {
			crossLog.Warnf("[CrossShard] 回写幂等检查失败 | batch=%s err=%v", batch.BatchId, err)
			continue
		}
		if processed {
			continue
		}
		task := &pendingApplyWrite{
			chunk: chunk,
			idKey: chunk.idKey,
			txId:  chunk.txId,
		}
		if applyTaskAlreadyCommitted(task) {
			if err = markProcessed(crossCfg.ShardId, chunk.idKey); err != nil {
				crossLog.Warnf("[CrossShard] 记录已提交回写失败 | tx_id=%s err=%v", task.txId, err)
			}
			continue
		}
		pendingApply.Lock()
		if _, exists := pendingApply.tasks[chunk.idKey]; !exists {
			pendingApply.tasks[chunk.idKey] = task
			pendingApply.version++
			queued++
		}
		refreshApplyReadyLocked()
		pendingApply.Unlock()
	}
	if queued > 0 {
		syncproposer.NotifyPending()
		if syncproposer.IsLocalProposer() {
			go flushPendingApplyWrites()
		}
	}
	return queued
}

func applyTaskAlreadyCommitted(task *pendingApplyWrite) bool {
	if task == nil || txCommitted == nil {
		return false
	}
	chainId, err := pickChainId()
	if err != nil {
		return false
	}
	exists, err := txCommitted(chainId, task.txId)
	if err != nil {
		crossLog.Warnf("[CrossShard] 入队前查询回写交易失败 | tx_id=%s err=%v", task.txId, err)
		return false
	}
	return exists
}

func onApplyProposerStatusChanged(active bool) {
	if active {
		flushPendingApplyWrites()
	}
}

func hasPendingApplyWrites() bool {
	pendingApply.Lock()
	defer pendingApply.Unlock()
	return len(pendingApply.tasks) > 0
}

func flushPendingApplyWrites() {
	if !syncproposer.IsLocalProposer() {
		return
	}
	pendingApply.Lock()
	if pendingApply.flushing {
		// Preserve a proposer-change wakeup received by an in-flight flush.
		pendingApply.version++
		pendingApply.Unlock()
		return
	}
	pendingApply.flushing = true
	snapshotVersion := pendingApply.version
	keys := applyFlushKeysLocked()
	pendingApply.Unlock()

	retryNeeded := false
	progressed := false
	defer func() {
		pendingApply.Lock()
		pendingApply.flushing = false
		changed := pendingApply.version != snapshotVersion
		retry := len(pendingApply.tasks) > 0 && (changed || retryNeeded || progressed)
		pendingApply.Unlock()
		if retry && syncproposer.IsLocalProposer() {
			if changed && !retryNeeded {
				go flushPendingApplyWrites()
				return
			}
			time.AfterFunc(applySubmitRetryDelay, flushPendingApplyWrites)
		}
	}()

	for _, key := range keys {
		if !syncproposer.IsLocalProposer() {
			retryNeeded = true
			return
		}
		pendingApply.Lock()
		task := pendingApply.tasks[key]
		submitted := task != nil && task.submitted
		pendingApply.Unlock()
		if task == nil {
			continue
		}
		if completeApplyTaskIfCommitted(task) {
			progressed = true
			continue
		}
		if submitted && applyTxStillInPool(task) {
			continue
		}
		if err := submitApplyWriteChunk(task.chunk); err != nil {
			crossLog.Warnf("[CrossShard] proposer 提交通用回写失败 | batch=%s tx_id=%s err=%v",
				task.chunk.batch.BatchId, task.txId, err)
			retryNeeded = true
			continue
		}
		if completeApplyTaskIfCommitted(task) {
			progressed = true
			continue
		}
		pendingApply.Lock()
		if current := pendingApply.tasks[key]; current != nil {
			current.submitted = true
		}
		pendingApply.Unlock()
	}
}

func applyFlushKeysLocked() []string {
	var activeHeight uint64
	for _, task := range pendingApply.tasks {
		if task == nil || task.chunk == nil || task.chunk.batch == nil {
			continue
		}
		height := task.chunk.batch.BridgeBlockHeight
		if activeHeight == 0 || height < activeHeight {
			activeHeight = height
		}
	}
	keys := make([]string, 0, len(pendingApply.tasks))
	for key, task := range pendingApply.tasks {
		if task != nil && task.chunk != nil && task.chunk.batch != nil &&
			task.chunk.batch.BridgeBlockHeight == activeHeight {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		left := pendingApply.tasks[keys[i]]
		right := pendingApply.tasks[keys[j]]
		if left == nil || left.chunk == nil || left.chunk.batch == nil {
			return true
		}
		if right == nil || right.chunk == nil || right.chunk.batch == nil {
			return false
		}
		if left.chunk.batch.BridgeBlockHeight != right.chunk.batch.BridgeBlockHeight {
			return left.chunk.batch.BridgeBlockHeight < right.chunk.batch.BridgeBlockHeight
		}
		if left.chunk.batch.BatchId != right.chunk.batch.BatchId {
			return left.chunk.batch.BatchId < right.chunk.batch.BatchId
		}
		if left.chunk.contractName != right.chunk.contractName {
			return left.chunk.contractName < right.chunk.contractName
		}
		return left.chunk.chunkIndex < right.chunk.chunkIndex
	})
	return keys
}

func completeApplyTaskIfCommitted(task *pendingApplyWrite) bool {
	if task == nil || txCommitted == nil {
		return false
	}
	chainId, err := pickChainId()
	if err != nil {
		return false
	}
	exists, err := txCommitted(chainId, task.txId)
	if err != nil {
		crossLog.Warnf("[CrossShard] 查询回写交易提交状态失败 | tx_id=%s err=%v", task.txId, err)
		return false
	}
	if !exists {
		return false
	}
	if err = markProcessed(crossCfg.ShardId, task.idKey); err != nil {
		crossLog.Warnf("[CrossShard] 记录链上已有回写失败 | tx_id=%s err=%v", task.txId, err)
		return false
	}
	pendingApply.Lock()
	if current := pendingApply.tasks[task.idKey]; current == task {
		delete(pendingApply.tasks, task.idKey)
	}
	refreshApplyReadyLocked()
	pendingApply.Unlock()
	crossLog.Infof("[CrossShard] 通用回写交易已在链上存在 | batch=%s tx_id=%s",
		task.chunk.batch.BatchId, task.txId)
	return true
}

func applyTxStillInPool(task *pendingApplyWrite) bool {
	if task == nil || txInPool == nil {
		return false
	}
	chainId, err := pickChainId()
	if err != nil {
		return false
	}
	exists, err := txInPool(chainId, task.txId)
	return err == nil && exists
}

func completePendingApplyWrites(block *commonpb.Block) {
	if block == nil || len(block.Txs) == 0 {
		return
	}
	committed := make(map[string]struct{}, len(block.Txs))
	logicalCommitted := make(map[string]struct{}, len(block.Txs))
	for _, tx := range block.Txs {
		if TransactionSucceeded(tx) && tx.Payload != nil {
			committed[tx.Payload.TxId] = struct{}{}
			if IsStateSyncTx(tx) {
				logical := syncauth.LogicalID(tx.Payload)
				logicalCommitted[logical] = struct{}{}
				if crossCfg != nil {
					if err := markProcessed(crossCfg.ShardId, "logical:"+logical); err != nil {
						crossLog.Warnf("[CrossShard] record committed logical writeback failed | tx_id=%s err=%v", tx.Payload.TxId, err)
					}
				}
			}
		} else if tx != nil && tx.Payload != nil && IsSystemSyncTx(tx) {
			crossLog.Errorf("[CrossShard] sync transaction committed with failure; pending state is retained | tx_id=%s result=%v", tx.Payload.TxId, tx.Result)
		}
	}
	if len(committed) == 0 {
		return
	}
	completed := make([]*pendingApplyWrite, 0)
	pendingApply.Lock()
	for _, task := range pendingApply.tasks {
		_, exact := committed[task.txId]
		var logical bool
		if task.chunk != nil {
			_, logical = logicalCommitted[task.chunk.logicalId]
		}
		if exact || logical {
			completed = append(completed, task)
		}
	}
	pendingApply.Unlock()
	for _, task := range completed {
		if err := markProcessed(crossCfg.ShardId, task.idKey); err != nil {
			crossLog.Warnf("[CrossShard] 记录已提交回写失败 | tx_id=%s err=%v", task.txId, err)
			continue
		}
		pendingApply.Lock()
		if current := pendingApply.tasks[task.idKey]; current == task {
			delete(pendingApply.tasks, task.idKey)
		}
		refreshApplyReadyLocked()
		pendingApply.Unlock()
		crossLog.Infof("[CrossShard] 通用回写交易已提交 | batch=%s tx_id=%s",
			task.chunk.batch.BatchId, task.txId)
	}
	if syncproposer.IsLocalProposer() && hasPendingApplyWrites() {
		go flushPendingApplyWrites()
	}
}

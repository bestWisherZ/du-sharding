package globalstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/gogo/protobuf/proto"
)

type pendingBridgeSync struct {
	sourceShard string
	delta       *CompactDelta
	txIds       map[string]struct{}
	chunks      []bridgeStateSyncChunk
	rawWrites   int
	rawBytes    int
	submitted   bool
	prepareMu   sync.Mutex
	prepared    []*preparedBridgeStateSyncTx
}

var bridgeReady int32

func bridgePendingReady() bool { return atomic.LoadInt32(&bridgeReady) != 0 }

func refreshBridgeReadyLocked() {
	var ready int32
	if hasPendingBridgeSyncLocked() {
		ready = 1
	}
	atomic.StoreInt32(&bridgeReady, ready)
}

const (
	defaultStateSyncFlushWrites = 64000
	defaultStateSyncFlushBytes  = 8 * 1024 * 1024
	stateSyncSubmitRetryDelay   = 500 * time.Millisecond
)

var pendingBridge = struct {
	sync.Mutex
	tasks          map[string]*pendingBridgeSync
	queuedWrites   int
	queuedBytes    int
	version        uint64
	flushing       bool
	positions      map[string]string
	waveCycle      uint64
	waveSlot       int
	waveTotal      int
	waveAnnounced  bool
	committedWaves map[bridgeWavePosition]*committedBridgeWave
}{
	tasks:     make(map[string]*pendingBridgeSync),
	positions: make(map[string]string),
}

func enqueueBridgeSync(sourceShard string, delta *CompactDelta) bool {
	return enqueueEncodedBridgeSync(sourceShard, delta, nil)
}

func enqueueEncodedBridgeSync(sourceShard string, delta *CompactDelta, encoded []bridgeStateSyncChunk) bool {
	if delta == nil {
		return false
	}
	sourceShard = strings.TrimSpace(sourceShard)
	if bridgeWaveBatchingEnabled() {
		if !isExpectedBusinessShard(sourceShard) {
			globalLog.Warnf("[GlobalState] 忽略未知业务分片状态同步 | shard=%s height=%d",
				sourceShard, delta.Height)
			return false
		}
		if !validBridgeWaveDelta(delta) {
			globalLog.Errorf("[GlobalState] 拒绝缺少有效波次标识的状态同步 | shard=%s height=%d cycle=%d slot=%d/%d",
				sourceShard, delta.Height, delta.Cycle, delta.Slot, delta.SlotTotal)
			return false
		}
	}
	task, taskId, err := cloneEncodedBridgeSyncTask(sourceShard, delta, encoded)
	if err != nil {
		globalLog.Errorf("[GlobalState] 缓存 bridge 状态同步失败 | shard=%s height=%d err=%v",
			sourceShard, delta.Height, err)
		return false
	}
	copyDelta := task.delta
	pendingBridge.Lock()
	_, duplicate := pendingBridge.tasks[taskId]
	pendingBridge.Unlock()
	if duplicate {
		return false
	}
	removeAlreadyCommittedBridgeTxIds(task.txIds)
	if len(task.txIds) == 0 {
		globalLog.Infof("[GlobalState] 忽略已提交的 bridge 状态同步消息 | shard=%s heights=%d-%d",
			sourceShard, copyDelta.StartHeight, copyDelta.Height)
		return false
	}
	pendingBridge.Lock()
	if _, exists := pendingBridge.tasks[taskId]; exists {
		pendingBridge.Unlock()
		return false
	}
	if bridgeWaveBatchingEnabled() {
		ensureBridgeWaveLocked()
		if bridgeWaveIsStaleLocked(copyDelta) {
			currentCycle := pendingBridge.waveCycle
			currentSlot := pendingBridge.waveSlot
			pendingBridge.Unlock()
			globalLog.Infof("[GlobalState] 忽略过期 bridge 同步波次 | shard=%s height=%d wave=%d/%d current=%d/%d",
				sourceShard, copyDelta.Height, copyDelta.Cycle, copyDelta.Slot, currentCycle, currentSlot)
			return false
		}
	}
	if pendingBridge.positions == nil {
		pendingBridge.positions = make(map[string]string)
	}
	positionKey := bridgeWaveSourceKey(sourceShard, copyDelta)
	if positionKey != "" {
		if existing := pendingBridge.positions[positionKey]; existing != "" && existing != taskId {
			pendingBridge.Unlock()
			globalLog.Errorf("[GlobalState] 同一同步波次收到冲突检查点 | shard=%s height=%d cycle=%d slot=%d/%d",
				sourceShard, copyDelta.Height, copyDelta.Cycle, copyDelta.Slot, copyDelta.SlotTotal)
			return false
		}
		pendingBridge.positions[positionKey] = taskId
	}
	pendingBridge.tasks[taskId] = task
	pendingBridge.queuedWrites += task.rawWrites
	pendingBridge.queuedBytes += task.rawBytes
	pendingBridge.version++
	ready := bridgeRawSyncReadyLocked()
	refreshBridgeReadyLocked()
	announce := ready
	if bridgeWaveBatchingEnabled() {
		announce = ready && !pendingBridge.waveAnnounced
		if announce {
			pendingBridge.waveAnnounced = true
		}
	}
	queuedWrites := pendingBridge.queuedWrites
	queuedBytes := pendingBridge.queuedBytes
	taskCount := len(pendingBridge.tasks)
	waveCycle := pendingBridge.waveCycle
	waveSlot := pendingBridge.waveSlot
	waveTotal := pendingBridge.waveTotal
	pendingBridge.Unlock()
	notifyBridgePreparation()
	if ready {
		reason := "threshold"
		if bridgeWaveBatchingEnabled() {
			reason = "wave_complete"
		} else if syncproposer.ForceFlushEnabled() {
			reason = "phase_end"
		}
		if announce {
			globalLog.Infof("[GlobalState] bridge 同步批次就绪 | reason=%s cycle=%d slot=%d/%d tasks=%d writes=%d bytes=%d",
				reason, waveCycle, waveSlot, waveTotal, taskCount, queuedWrites, queuedBytes)
			if bridgeWaveBatchingEnabled() {
				syncproposer.SetBatchPreparing(true)
			}
			syncproposer.NotifyPending()
		}
		if syncproposer.IsLocalProposer() {
			go flushPendingBridgeSync()
		}
	}
	return true
}

func removeAlreadyCommittedBridgeTxIds(txIds map[string]struct{}) {
	if txCommitted == nil || len(txIds) == 0 {
		return
	}
	chainId, err := pickChainId()
	if err != nil {
		return
	}
	for txId := range txIds {
		exists, checkErr := txCommitted(chainId, txId)
		if checkErr != nil {
			globalLog.Warnf("[GlobalState] 入队前查询同步交易失败 | tx_id=%s err=%v", txId, checkErr)
			continue
		}
		if exists {
			delete(txIds, txId)
		}
	}
}

func cloneBridgeSyncTask(sourceShard string, delta *CompactDelta) (*pendingBridgeSync, string, error) {
	return cloneEncodedBridgeSyncTask(sourceShard, delta, nil)
}

func cloneEncodedBridgeSyncTask(sourceShard string, delta *CompactDelta, encoded []bridgeStateSyncChunk) (*pendingBridgeSync, string, error) {
	if delta == nil {
		return nil, "", fmt.Errorf("nil compact delta")
	}
	copyDelta := *delta
	if delta.Authorizations != nil {
		copyDelta.Authorizations = make([][]byte, len(delta.Authorizations))
		for i, authorization := range delta.Authorizations {
			copyDelta.Authorizations[i] = append([]byte(nil), authorization...)
		}
	}
	validStrings := utf8.ValidString(delta.BlockHash)
	for _, write := range delta.Writes {
		validStrings = validStrings && utf8.ValidString(write.Contract)
	}
	// JSON used to normalize invalid UTF-8 during the deep copy. Keep that
	// behavior for malformed strings, without decoding ordinary tasks again.
	if !validStrings {
		raw, err := json.Marshal(delta)
		if err != nil {
			return nil, "", err
		}
		copyDelta = CompactDelta{}
		if err = json.Unmarshal(raw, &copyDelta); err != nil {
			return nil, "", err
		}
	}
	chunks := encoded
	if chunks == nil || !validStrings {
		var err error
		chunks, err = encodeBridgeStateSyncChunks(sourceShard, &copyDelta)
		if err != nil {
			return nil, "", err
		}
	}
	txIds := make(map[string]struct{}, len(chunks))
	for _, chunk := range chunks {
		txIds[chunk.txId] = struct{}{}
	}
	// Encoded chunks own an immutable snapshot of all writes. The queue only
	// needs checkpoint metadata afterward, not another copy of the raw writes.
	rawWrites := len(copyDelta.Writes)
	copyDelta.Writes = nil
	metadata, err := proto.Marshal(compactDeltaMetadata(&copyDelta))
	if err != nil {
		return nil, "", err
	}
	sum := sha256.New()
	fmt.Fprintf(sum, "bridge-task-v3|%d:%s|%d:", len(sourceShard), sourceShard, len(metadata))
	_, _ = sum.Write(metadata)
	rawBytes := len(metadata)
	for _, chunk := range chunks {
		_, _ = sum.Write([]byte(chunk.logicalId))
		rawBytes += len(chunk.writesRaw)
	}
	return &pendingBridgeSync{
		sourceShard: sourceShard, delta: &copyDelta, txIds: txIds,
		chunks: chunks, rawWrites: rawWrites, rawBytes: rawBytes,
	}, hex.EncodeToString(sum.Sum(nil)), nil
}

func onProposerStatusChanged(active bool) {
	if active {
		flushPendingBridgeSync()
	}
}

func onForceFlushChanged(active bool) {
	if !active {
		pendingBridge.Lock()
		refreshBridgeReadyLocked()
		pendingBridge.Unlock()
		return
	}
	pendingBridge.Lock()
	taskCount := len(pendingBridge.tasks)
	queuedWrites := pendingBridge.queuedWrites
	queuedBytes := pendingBridge.queuedBytes
	ready := bridgeRawSyncReadyLocked()
	refreshBridgeReadyLocked()
	announce := ready && !pendingBridge.waveAnnounced
	if announce {
		pendingBridge.waveAnnounced = true
	}
	waveCycle := pendingBridge.waveCycle
	waveSlot := pendingBridge.waveSlot
	waveTotal := pendingBridge.waveTotal
	pendingBridge.Unlock()
	if bridgeWaveBatchingEnabled() {
		if !ready {
			globalLog.Infof("[GlobalState] bridge 阶段边界等待完整同步波次 | cycle=%d slot=%d/%d tasks=%d writes=%d bytes=%d",
				waveCycle, waveSlot, waveTotal, taskCount, queuedWrites, queuedBytes)
			return
		}
		if announce {
			syncproposer.SetBatchPreparing(true)
			syncproposer.NotifyPending()
		}
		if syncproposer.IsLocalProposer() {
			go flushPendingBridgeSync()
		}
		return
	}
	globalLog.Infof("[GlobalState] bridge 阶段边界强制刷新 | tasks=%d writes=%d bytes=%d",
		taskCount, queuedWrites, queuedBytes)
	syncproposer.NotifyPending()
	if syncproposer.IsLocalProposer() {
		go flushPendingBridgeSync()
	}
}

func hasPendingBridgeSync() bool {
	pendingBridge.Lock()
	defer pendingBridge.Unlock()
	return hasPendingBridgeSyncLocked()
}

func hasPendingBridgeSyncLocked() bool {
	if bridgeRawSyncReadyLocked() {
		return true
	}
	for _, task := range pendingBridge.tasks {
		if task != nil && task.submitted {
			return true
		}
	}
	return false
}

func bridgeWaveBatchingEnabled() bool {
	return len(expectedBusinessShards()) > 0
}

func expectedBusinessShards() []string {
	if globalCfg == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(globalCfg.Schedule.BusinessShards))
	shards := make([]string, 0, len(globalCfg.Schedule.BusinessShards))
	for _, shard := range globalCfg.Schedule.BusinessShards {
		shard = strings.TrimSpace(shard)
		if shard == "" {
			continue
		}
		if _, exists := seen[shard]; exists {
			continue
		}
		seen[shard] = struct{}{}
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	return shards
}

func isExpectedBusinessShard(shard string) bool {
	shard = strings.TrimSpace(shard)
	for _, expected := range expectedBusinessShards() {
		if shard == expected {
			return true
		}
	}
	return false
}

func validBridgeWaveDelta(delta *CompactDelta) bool {
	return delta != nil && delta.Cycle > 0 && delta.Slot > 0 &&
		delta.SlotTotal > 0 && delta.Slot <= delta.SlotTotal
}

func bridgeWaveSourceKey(sourceShard string, delta *CompactDelta) string {
	if !validBridgeWaveDelta(delta) {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", delta.Cycle, delta.Slot, strings.TrimSpace(sourceShard))
}

func bridgeWaveIsStaleLocked(delta *CompactDelta) bool {
	if !validBridgeWaveDelta(delta) || pendingBridge.waveCycle == 0 {
		return false
	}
	return delta.Cycle < pendingBridge.waveCycle ||
		(delta.Cycle == pendingBridge.waveCycle && delta.Slot < pendingBridge.waveSlot)
}

func ensureBridgeWaveLocked() {
	if pendingBridge.waveCycle > 0 && pendingBridge.waveTotal > 0 {
		return
	}
	var selectedCycle uint64
	var selectedTotal int
	selectWave := func(cycle uint64, total int) {
		if pendingBridge.waveCycle > 0 && cycle != pendingBridge.waveCycle {
			return
		}
		if selectedCycle == 0 || cycle < selectedCycle {
			selectedCycle, selectedTotal = cycle, total
		}
	}
	for _, task := range pendingBridge.tasks {
		if task == nil || task.delta == nil || task.delta.Slot != 1 || !validBridgeWaveDelta(task.delta) {
			continue
		}
		selectWave(task.delta.Cycle, task.delta.SlotTotal)
	}
	for position, wave := range pendingBridge.committedWaves {
		if position.slot == 1 {
			selectWave(position.cycle, wave.slotTotal)
		}
	}
	if selectedCycle == 0 {
		return
	}
	if pendingBridge.waveCycle == 0 {
		pendingBridge.waveCycle = selectedCycle
	}
	pendingBridge.waveSlot = 1
	pendingBridge.waveTotal = selectedTotal
}

func bridgeWaveKeysLocked() ([]string, bool) {
	ensureBridgeWaveLocked()
	if pendingBridge.waveCycle == 0 || pendingBridge.waveSlot <= 0 || pendingBridge.waveTotal <= 0 {
		return nil, false
	}
	shards := expectedBusinessShards()
	if len(shards) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(shards))
	for _, shard := range shards {
		if bridgeWaveShardCommittedLocked(shard) {
			continue
		}
		positionKey := fmt.Sprintf("%d:%d:%s", pendingBridge.waveCycle, pendingBridge.waveSlot, shard)
		taskId := pendingBridge.positions[positionKey]
		task := pendingBridge.tasks[taskId]
		if task == nil || task.delta == nil || task.delta.SlotTotal != pendingBridge.waveTotal {
			return nil, false
		}
		keys = append(keys, taskId)
	}
	sort.Slice(keys, func(i, j int) bool {
		return pendingBridge.tasks[keys[i]].sourceShard < pendingBridge.tasks[keys[j]].sourceShard
	})
	return keys, true
}

func announceReadyBridgeWave() bool {
	pendingBridge.Lock()
	ready := bridgeRawSyncReadyLocked()
	refreshBridgeReadyLocked()
	announce := ready && !pendingBridge.waveAnnounced
	if announce {
		pendingBridge.waveAnnounced = true
	}
	cycle := pendingBridge.waveCycle
	slot := pendingBridge.waveSlot
	total := pendingBridge.waveTotal
	taskCount := len(pendingBridge.tasks)
	writes := pendingBridge.queuedWrites
	bytes := pendingBridge.queuedBytes
	pendingBridge.Unlock()
	if !announce {
		return ready
	}
	globalLog.Infof("[GlobalState] bridge 完整同步波次可提交 | cycle=%d slot=%d/%d tasks=%d writes=%d bytes=%d",
		cycle, slot, total, taskCount, writes, bytes)
	syncproposer.SetBatchPreparing(true)
	syncproposer.NotifyPending()
	return true
}

func bridgeRawSyncReadyLocked() bool {
	if bridgeWaveBatchingEnabled() {
		keys, complete := bridgeWaveKeysLocked()
		if !complete {
			return false
		}
		for _, key := range keys {
			if task := pendingBridge.tasks[key]; task != nil && !task.submitted {
				return true
			}
		}
		return false
	}
	if pendingBridge.queuedWrites == 0 && pendingBridge.queuedBytes == 0 {
		return false
	}
	if syncproposer.ForceFlushEnabled() {
		return true
	}
	maxWrites, maxBytes := stateSyncFlushThresholds()
	return pendingBridge.queuedWrites >= maxWrites || pendingBridge.queuedBytes >= maxBytes
}

func stateSyncFlushThresholds() (int, int) {
	maxWrites := defaultStateSyncFlushWrites
	maxBytes := defaultStateSyncFlushBytes
	if globalCfg != nil {
		if globalCfg.Schedule.StateSyncFlushWrites > 0 {
			maxWrites = globalCfg.Schedule.StateSyncFlushWrites
		}
		if globalCfg.Schedule.StateSyncFlushBytes > 0 {
			maxBytes = globalCfg.Schedule.StateSyncFlushBytes
		}
	}
	return maxWrites, maxBytes
}

func flushPendingBridgeSync() {
	if !syncproposer.IsLocalProposer() {
		return
	}
	pendingBridge.Lock()
	if pendingBridge.flushing {
		pendingBridge.version++
		pendingBridge.Unlock()
		return
	}
	pendingBridge.flushing = true
	snapshotVersion := pendingBridge.version
	releaseRaw := bridgeRawSyncReadyLocked()
	keys := bridgeFlushKeysLocked(releaseRaw)
	cycle, slot := pendingBridge.waveCycle, pendingBridge.waveSlot
	pendingBridge.Unlock()
	started := time.Now()
	if len(keys) > 0 {
		defer func() {
			globalLog.Infof("[GlobalState] SYNC_PIPELINE stage=submit cycle=%d slot=%d tasks=%d elapsed_ms=%.3f",
				cycle, slot, len(keys), float64(time.Since(started).Microseconds())/1000)
		}()
	}

	retryNeeded := false
	progressed := false
	defer func() {
		pendingBridge.Lock()
		pendingBridge.flushing = false
		refreshBridgeReadyLocked()
		changed := pendingBridge.version != snapshotVersion
		retry := hasPendingBridgeSyncLocked() && (changed || retryNeeded || progressed)
		pendingBridge.Unlock()
		if retry && syncproposer.IsLocalProposer() {
			if changed && !retryNeeded {
				go flushPendingBridgeSync()
				return
			}
			time.AfterFunc(stateSyncSubmitRetryDelay, flushPendingBridgeSync)
		}
	}()

	for _, key := range keys {
		if !syncproposer.IsLocalProposer() {
			retryNeeded = true
			return
		}
		pendingBridge.Lock()
		task := pendingBridge.tasks[key]
		submitted := task != nil && task.submitted
		txCount := 0
		if task != nil {
			txCount = len(task.txIds)
		}
		pendingBridge.Unlock()
		if task == nil {
			continue
		}
		if reconcileCommittedBridgeSync(key) {
			progressed = true
			continue
		}
		if submitted && bridgeSyncTxsStillInPool(key) {
			continue
		}
		if err := submitBridgeStateSync(task); err != nil {
			globalLog.Errorf("[GlobalState] proposer 提交 bridge 状态同步失败 | shard=%s heights=%d-%d err=%v",
				task.sourceShard, task.delta.StartHeight, task.delta.Height, err)
			retryNeeded = true
			continue
		}
		if reconcileCommittedBridgeSync(key) {
			progressed = true
			continue
		}
		pendingBridge.Lock()
		if current := pendingBridge.tasks[key]; current != nil && !current.submitted {
			current.submitted = true
			pendingBridge.queuedWrites -= current.rawWrites
			pendingBridge.queuedBytes -= current.rawBytes
			clampPendingBridgeCountersLocked()
		}
		pendingBridge.Unlock()
		globalLog.Infof("[GlobalState] proposer 已提交 bridge 状态同步 | shard=%s heights=%d-%d txs=%d",
			task.sourceShard, task.delta.StartHeight, task.delta.Height, txCount)
	}
	if bridgeWaveBatchingEnabled() {
		pendingBridge.Lock()
		advanced := maybeAdvanceBridgeWaveLocked()
		pendingBridge.Unlock()
		if advanced {
			syncproposer.ClearReadyBatch()
			announceReadyBridgeWave()
			notifyBridgePreparation()
		}
		txIds, cycle, slot, total, shardCount, ready := submittedBridgeWave()
		if ready {
			globalLog.Infof("[GlobalState] bridge 完整同步波次已入池 | cycle=%d slot=%d/%d shards=%d txs=%d",
				cycle, slot, total, shardCount, len(txIds))
			syncproposer.PublishReadyBatch(txIds)
		}
	}
}

func submittedBridgeWave() (txIds []string, cycle uint64, slot, total, shardCount int, ready bool) {
	pendingBridge.Lock()
	defer pendingBridge.Unlock()
	keys, complete := bridgeWaveKeysLocked()
	shardCount = len(expectedBusinessShards())
	if !complete || len(keys) == 0 {
		return nil, pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal, shardCount, false
	}
	for _, key := range keys {
		task := pendingBridge.tasks[key]
		if task == nil || !task.submitted || len(task.txIds) == 0 {
			return nil, pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal, shardCount, false
		}
		for txId := range task.txIds {
			txIds = append(txIds, txId)
		}
	}
	sort.Strings(txIds)
	return txIds, pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal, shardCount, true
}

func bridgeFlushKeysLocked(releaseRaw bool) []string {
	if bridgeWaveBatchingEnabled() {
		keys, complete := bridgeWaveKeysLocked()
		if !complete {
			return nil
		}
		if releaseRaw {
			return keys
		}
		submitted := keys[:0]
		for _, key := range keys {
			if task := pendingBridge.tasks[key]; task != nil && task.submitted {
				submitted = append(submitted, key)
			}
		}
		return submitted
	}
	activeHeight := make(map[string]uint64)
	for _, task := range pendingBridge.tasks {
		if task == nil || task.delta == nil {
			continue
		}
		height, exists := activeHeight[task.sourceShard]
		if !exists || task.delta.Height < height {
			activeHeight[task.sourceShard] = task.delta.Height
		}
	}
	keys := make([]string, 0, len(pendingBridge.tasks))
	for key, task := range pendingBridge.tasks {
		if task != nil && task.delta != nil &&
			(task.submitted || (releaseRaw && task.delta.Height == activeHeight[task.sourceShard])) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		left := pendingBridge.tasks[keys[i]]
		right := pendingBridge.tasks[keys[j]]
		if left == nil || left.delta == nil {
			return true
		}
		if right == nil || right.delta == nil {
			return false
		}
		if left.sourceShard != right.sourceShard {
			return left.sourceShard < right.sourceShard
		}
		if left.delta.Height != right.delta.Height {
			return left.delta.Height < right.delta.Height
		}
		return keys[i] < keys[j]
	})
	return keys
}

func clampPendingBridgeCountersLocked() {
	if pendingBridge.queuedWrites < 0 {
		pendingBridge.queuedWrites = 0
	}
	if pendingBridge.queuedBytes < 0 {
		pendingBridge.queuedBytes = 0
	}
}

func deleteBridgeTaskLocked(taskId string) {
	task := pendingBridge.tasks[taskId]
	if task == nil {
		return
	}
	if positionKey := bridgeWaveSourceKey(task.sourceShard, task.delta); positionKey != "" {
		if pendingBridge.positions[positionKey] == taskId {
			delete(pendingBridge.positions, positionKey)
		}
	}
	delete(pendingBridge.tasks, taskId)
}

func maybeAdvanceBridgeWaveLocked() bool {
	if !bridgeWaveBatchingEnabled() {
		return false
	}
	advanced := false
	for {
		ensureBridgeWaveLocked()
		if !bridgeWaveCommittedLocked() {
			return advanced
		}
		position := bridgeWavePosition{pendingBridge.waveCycle, pendingBridge.waveSlot}
		globalLog.Infof("[GlobalState] bridge 同步波次按落链检查点推进 | cycle=%d slot=%d/%d shards=%d",
			position.cycle, position.slot, pendingBridge.waveTotal, len(expectedBusinessShards()))
		// A raw message may have been queued concurrently with its block commit.
		for id, task := range pendingBridge.tasks {
			if task != nil && task.delta != nil && task.delta.Cycle == position.cycle && task.delta.Slot == position.slot {
				if !task.submitted {
					pendingBridge.queuedWrites -= task.rawWrites
					pendingBridge.queuedBytes -= task.rawBytes
				}
				deleteBridgeTaskLocked(id)
			}
		}
		clampPendingBridgeCountersLocked()
		delete(pendingBridge.committedWaves, position)
		if pendingBridge.waveSlot < pendingBridge.waveTotal {
			pendingBridge.waveSlot++
		} else {
			pendingBridge.waveCycle++
			pendingBridge.waveSlot = 1
			pendingBridge.waveTotal = 0
		}
		pendingBridge.waveAnnounced = false
		pendingBridge.version++
		advanced = true
	}
}

func reconcileCommittedBridgeSync(taskId string) bool {
	if txCommitted == nil {
		return false
	}
	pendingBridge.Lock()
	task := pendingBridge.tasks[taskId]
	if task == nil {
		pendingBridge.Unlock()
		return true
	}
	txIds := make([]string, 0, len(task.txIds))
	for txId := range task.txIds {
		txIds = append(txIds, txId)
	}
	pendingBridge.Unlock()
	chainId, err := pickChainId()
	if err != nil {
		return false
	}
	committed := make([]string, 0)
	for _, txId := range txIds {
		exists, checkErr := txCommitted(chainId, txId)
		if checkErr != nil {
			globalLog.Warnf("[GlobalState] 查询 bridge 状态同步提交状态失败 | tx_id=%s err=%v", txId, checkErr)
			return false
		}
		if exists {
			committed = append(committed, txId)
		}
	}
	if len(committed) == 0 {
		return false
	}
	pendingBridge.Lock()
	task = pendingBridge.tasks[taskId]
	if task == nil {
		pendingBridge.Unlock()
		return true
	}
	for _, txId := range committed {
		delete(task.txIds, txId)
	}
	done := len(task.txIds) == 0
	if done {
		if !task.submitted {
			pendingBridge.queuedWrites -= task.rawWrites
			pendingBridge.queuedBytes -= task.rawBytes
			clampPendingBridgeCountersLocked()
		}
		deleteBridgeTaskLocked(taskId)
	}
	refreshBridgeReadyLocked()
	pendingBridge.Unlock()
	if done {
		globalLog.Infof("[GlobalState] bridge 状态同步任务已在链上存在 | shard=%s heights=%d-%d",
			task.sourceShard, task.delta.StartHeight, task.delta.Height)
	}
	return done
}

func bridgeSyncTxsStillInPool(taskId string) bool {
	if txInPool == nil {
		return false
	}
	pendingBridge.Lock()
	task := pendingBridge.tasks[taskId]
	if task == nil || len(task.txIds) == 0 {
		pendingBridge.Unlock()
		return false
	}
	txIds := make([]string, 0, len(task.txIds))
	for txId := range task.txIds {
		txIds = append(txIds, txId)
	}
	pendingBridge.Unlock()
	chainId, err := pickChainId()
	if err != nil {
		return false
	}
	for _, txId := range txIds {
		exists, checkErr := txInPool(chainId, txId)
		if checkErr != nil || !exists {
			return false
		}
	}
	return true
}

func completePendingBridgeSync(block *commonpb.Block) {
	if block == nil || len(block.Txs) == 0 {
		return
	}
	committed := make(map[string]struct{})
	logicalCommitted := make(map[string]struct{})
	for _, tx := range block.Txs {
		if crossshard.TransactionSucceeded(tx) && tx.Payload != nil {
			committed[tx.Payload.TxId] = struct{}{}
			if crossshard.IsBridgeStateSyncTx(tx) {
				logicalCommitted[syncauth.LogicalID(tx.Payload)] = struct{}{}
			}
		} else if tx != nil && tx.Payload != nil && crossshard.IsBridgeStateSyncTx(tx) {
			globalLog.Errorf("[GlobalState] checkpoint committed with failure; pending wave is retained | tx_id=%s result=%v", tx.Payload.TxId, tx.Result)
		}
	}
	if len(committed) == 0 {
		return
	}
	committedIds := make([]string, 0, len(committed))
	for txId := range committed {
		committedIds = append(committedIds, txId)
	}
	syncproposer.MarkBatchTxsCommitted(committedIds)
	pendingBridge.Lock()
	for _, tx := range block.Txs {
		if tx == nil || tx.Result == nil || tx.Result.Code != commonpb.TxStatusCode_SUCCESS ||
			tx.Result.ContractResult == nil || tx.Result.ContractResult.Code != 0 {
			continue
		}
		info, err := crossshard.ParseBridgeStateSyncInfo(tx)
		if err != nil {
			continue
		}
		var blockHash string
		for _, param := range tx.Payload.Parameters {
			if param != nil && param.Key == crossshard.SourceBlockHashParamKey {
				blockHash = string(param.Value)
				break
			}
		}
		recordCommittedBridgeWaveLocked(info, blockHash)
	}
	for taskId, task := range pendingBridge.tasks {
		for _, chunk := range task.chunks {
			_, exact := committed[chunk.txId]
			_, logical := logicalCommitted[chunk.logicalId]
			if exact || logical {
				delete(task.txIds, chunk.txId)
				committedIds = append(committedIds, chunk.txId)
			}
		}
		if len(task.txIds) == 0 {
			if !task.submitted {
				pendingBridge.queuedWrites -= task.rawWrites
				pendingBridge.queuedBytes -= task.rawBytes
				clampPendingBridgeCountersLocked()
			}
			deleteBridgeTaskLocked(taskId)
			globalLog.Infof("[GlobalState] bridge 状态同步任务已提交 | shard=%s heights=%d-%d",
				task.sourceShard, task.delta.StartHeight, task.delta.Height)
		}
	}
	advanced := maybeAdvanceBridgeWaveLocked()
	refreshBridgeReadyLocked()
	pendingBridge.Unlock()
	syncproposer.MarkBatchTxsCommitted(committedIds)
	if advanced {
		syncproposer.ClearReadyBatch()
		announceReadyBridgeWave()
		notifyBridgePreparation()
	}
	if syncproposer.IsLocalProposer() && hasPendingBridgeSync() {
		go flushPendingBridgeSync()
	}
}

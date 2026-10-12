package globalstate

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
)

const bridgePrepareConcurrency = 4

// No Sender or signature is cached. The proposer at release time signs only
// after checking the chain and pool, including after a proposer change.
type preparedBridgeStateSyncTx struct {
	payload       *commonpb.Payload
	unsignedBytes []byte
}

func prepareBridgeStateSyncTx(chainID, shard string, delta *CompactDelta,
	chunk bridgeStateSyncChunk, index, total int) (*preparedBridgeStateSyncTx, error) {
	payload := bridgeStateSyncPayload(chainID, shard, delta, chunk, index, total)
	unsigned, err := utils.CalcUnsignedTxBytes(&commonpb.Transaction{Payload: payload})
	if err != nil {
		return nil, err
	}
	return &preparedBridgeStateSyncTx{payload: payload, unsignedBytes: unsigned}, nil
}

func (task *pendingBridgeSync) prepareChunk(chainID string, index int) (*preparedBridgeStateSyncTx, bool, error) {
	task.prepareMu.Lock()
	defer task.prepareMu.Unlock()
	if index < 0 || index >= len(task.chunks) {
		return nil, false, fmt.Errorf("invalid bridge sync chunk index %d", index)
	}
	if task.prepared == nil {
		task.prepared = make([]*preparedBridgeStateSyncTx, len(task.chunks))
	}
	if cached := task.prepared[index]; cached != nil {
		if cached.payload.ChainId != chainID {
			return nil, false, fmt.Errorf("prepared bridge sync chain ID changed")
		}
		return cached, true, nil
	}
	prepared, err := prepareBridgeStateSyncTx(chainID, task.sourceShard, task.delta,
		task.chunks[index], index, len(task.chunks))
	if err != nil {
		return nil, false, err
	}
	task.prepared[index] = prepared
	return prepared, false, nil
}

func signPreparedBridgeStateSyncTx(prepared *preparedBridgeStateSyncTx) (*commonpb.Transaction, error) {
	signer, err := getStateSyncSigner()
	if err != nil {
		return nil, err
	}
	// Pool/VM code may mutate a transaction. Never expose the cached payload or
	// its parameter buffers; unsigned bytes remain private and immutable.
	payload := *prepared.payload
	payload.Parameters = make([]*commonpb.KeyValuePair, len(prepared.payload.Parameters))
	for i, param := range prepared.payload.Parameters {
		copyParam := *param
		if param.Value != nil {
			copyParam.Value = make([]byte, len(param.Value))
			copy(copyParam.Value, param.Value)
		}
		payload.Parameters[i] = &copyParam
	}
	sender, err := signer.signBytes(prepared.unsignedBytes)
	if err != nil {
		return nil, err
	}
	return &commonpb.Transaction{Payload: &payload, Sender: sender}, nil
}

var bridgePreparation = struct {
	sync.Mutex
	enabled bool
	running bool
	dirty   bool
}{}

func startBridgePreparation() {
	bridgePreparation.Lock()
	bridgePreparation.enabled = true
	bridgePreparation.Unlock()
	notifyBridgePreparation()
}

func notifyBridgePreparation() {
	bridgePreparation.Lock()
	defer bridgePreparation.Unlock()
	if !bridgePreparation.enabled {
		return
	}
	bridgePreparation.dirty = true
	if bridgePreparation.running {
		return
	}
	chainID, err := pickChainId()
	if err != nil {
		return
	}
	bridgePreparation.running = true
	go func() {
		for {
			bridgePreparation.Lock()
			if !bridgePreparation.dirty {
				bridgePreparation.running = false
				bridgePreparation.Unlock()
				return
			}
			bridgePreparation.dirty = false
			bridgePreparation.Unlock()
			prepareBridgeWindow(chainID)
		}
	}()
}

// The active wave and one lookahead wave may be prepared. Readiness and wave
// advancement are deliberately untouched: preparation cannot release the gate.
func bridgePreparationTasks() ([]*pendingBridgeSync, uint64, int) {
	pendingBridge.Lock()
	defer pendingBridge.Unlock()
	if !bridgeWaveBatchingEnabled() {
		return nil, 0, 0
	}
	ensureBridgeWaveLocked()
	cycle, slot, total := pendingBridge.waveCycle, pendingBridge.waveSlot, pendingBridge.waveTotal
	if cycle == 0 || slot <= 0 || total <= 0 {
		return nil, cycle, slot
	}
	nextCycle, nextSlot := cycle, slot+1
	if slot == total {
		nextCycle, nextSlot = cycle+1, 1
	}
	var tasks []*pendingBridgeSync
	for _, task := range pendingBridge.tasks {
		if task == nil || task.delta == nil || task.submitted {
			continue
		}
		delta := task.delta
		if (delta.Cycle == cycle && delta.Slot == slot) || (delta.Cycle == nextCycle && delta.Slot == nextSlot) {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		left, right := tasks[i], tasks[j]
		if left.delta.Cycle != right.delta.Cycle {
			return left.delta.Cycle < right.delta.Cycle
		}
		if left.delta.Slot != right.delta.Slot {
			return left.delta.Slot < right.delta.Slot
		}
		return left.sourceShard < right.sourceShard
	})
	return tasks, cycle, slot
}

func prepareBridgeWindow(chainID string) {
	tasks, cycle, slot := bridgePreparationTasks()
	if len(tasks) == 0 {
		return
	}
	started := time.Now()
	jobs := make(chan *pendingBridgeSync)
	var wg sync.WaitGroup
	var preparedCount int64
	for i := 0; i < bridgePrepareConcurrency && i < len(tasks); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				for index := range task.chunks {
					_, cached, err := task.prepareChunk(chainID, index)
					if err != nil {
						globalLog.Warnf("[GlobalState] bridge sync preparation failed | shard=%s height=%d err=%v",
							task.sourceShard, task.delta.Height, err)
						break
					}
					if !cached {
						atomic.AddInt64(&preparedCount, 1)
					}
				}
			}
		}()
	}
	for _, task := range tasks {
		jobs <- task
	}
	close(jobs)
	wg.Wait()
	if preparedCount > 0 {
		globalLog.Infof("[GlobalState] SYNC_PIPELINE stage=prepare cycle=%d slot=%d tasks=%d prepared_chunks=%d elapsed_ms=%.3f",
			cycle, slot, len(tasks), preparedCount, float64(time.Since(started).Microseconds())/1000)
	}
}

package syncproposer

import (
	"sort"
	"sync"
)

var state = struct {
	sync.RWMutex
	localProposer  bool
	forceFlush     bool
	listeners      []func(bool)
	flushListeners []func(bool)
	checkers       []func() bool
	wake           func()
	proposalReady  []func()
	txSubmitted    []func(string)
	batchPreparing bool
	readyBatchTxs  []string
}{}

// RegisterTxSubmittedListener indexes locally submitted sync transactions;
// unlike peer transactions, they do not arrive through RecvTxPoolMsg.
func RegisterTxSubmittedListener(listener func(string)) {
	if listener == nil {
		return
	}
	state.Lock()
	state.txSubmitted = append(state.txSubmitted, listener)
	state.Unlock()
}

func NotifyTxSubmitted(txID string) {
	if txID == "" {
		return
	}
	state.RLock()
	listeners := append([]func(string){}, state.txSubmitted...)
	state.RUnlock()
	for _, listener := range listeners {
		listener(txID)
	}
}

// SetLocalProposer updates whether this node is the proposer selected by the
// local consensus engine and notifies state-sync task owners.
func SetLocalProposer(active bool) {
	state.Lock()
	changed := state.localProposer != active
	state.localProposer = active
	listeners := append([]func(bool){}, state.listeners...)
	state.Unlock()
	if !changed {
		return
	}
	for _, listener := range listeners {
		go listener(active)
	}
}

// IsLocalProposer reports the latest proposer state published by TBFT.
func IsLocalProposer() bool {
	state.RLock()
	defer state.RUnlock()
	return state.localProposer
}

// RegisterListener registers a state-sync task owner for proposer changes.
func RegisterListener(listener func(bool)) {
	if listener == nil {
		return
	}
	state.Lock()
	state.listeners = append(state.listeners, listener)
	active := state.localProposer
	state.Unlock()
	if active {
		go listener(true)
	}
}

// RegisterFlushListener registers a state-sync owner that can release a
// partially filled batch when a phase boundary requires all pending state to
// become durable.
func RegisterFlushListener(listener func(bool)) {
	if listener == nil {
		return
	}
	state.Lock()
	state.flushListeners = append(state.flushListeners, listener)
	active := state.forceFlush
	state.Unlock()
	if active {
		go listener(true)
	}
}

// SetForceFlush keeps the phase-boundary drain active until the scheduler
// explicitly clears it after the state-sync barrier has been satisfied.
func SetForceFlush(active bool) {
	state.Lock()
	changed := state.forceFlush != active
	state.forceFlush = active
	listeners := append([]func(bool){}, state.flushListeners...)
	wake := state.wake
	state.Unlock()
	if changed {
		for _, listener := range listeners {
			go listener(active)
		}
	}
	if active && wake != nil {
		wake()
	}
}

// ForceFlushEnabled reports whether the current phase boundary is draining all
// state-sync tasks, including a final batch below the normal threshold.
func ForceFlushEnabled() bool {
	state.RLock()
	defer state.RUnlock()
	return state.forceFlush
}

// RegisterPendingChecker exposes queued raw sync messages to the consensus
// gate before they have been converted into signed transactions.
func RegisterPendingChecker(checker func() bool) {
	if checker == nil {
		return
	}
	state.Lock()
	state.checkers = append(state.checkers, checker)
	state.Unlock()
}

// HasPending reports whether any state-sync receiver has an uncommitted task.
func HasPending() bool {
	state.RLock()
	checkers := append([]func() bool{}, state.checkers...)
	state.RUnlock()
	for _, checker := range checkers {
		if checker() {
			return true
		}
	}
	return false
}

// SetWakeFunc connects raw task arrival to the TBFT BlockInfo gate.
func SetWakeFunc(wake func()) {
	state.Lock()
	state.wake = wake
	state.Unlock()
}

// RegisterProposalReadyListener registers a block-proposer wakeup. Raw state
// synchronization can open the TBFT height first so that the elected proposer
// is known; this callback is fired only after that proposer has put the whole
// synchronization wave into its local transaction pool.
func RegisterProposalReadyListener(listener func()) {
	if listener == nil {
		return
	}
	state.Lock()
	state.proposalReady = append(state.proposalReady, listener)
	state.Unlock()
}

// SetBatchPreparing prevents a special block from being proposed while an
// elected proposer is still constructing or submitting a complete sync wave.
func SetBatchPreparing(active bool) {
	state.Lock()
	state.batchPreparing = active
	if active {
		state.readyBatchTxs = nil
	}
	state.Unlock()
}

// BatchPreparing reports whether a complete raw wave is waiting for all of its
// deterministic system transactions to enter the local transaction pool.
func BatchPreparing() bool {
	state.RLock()
	defer state.RUnlock()
	return state.batchPreparing
}

// PublishReadyBatch exposes one deterministic synchronization wave to the
// block proposer. IDs are sorted so proposer changes build the same tx order.
func PublishReadyBatch(txIds []string) {
	if len(txIds) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(txIds))
	ids := make([]string, 0, len(txIds))
	for _, txId := range txIds {
		if txId == "" {
			continue
		}
		if _, exists := seen[txId]; exists {
			continue
		}
		seen[txId] = struct{}{}
		ids = append(ids, txId)
	}
	if len(ids) == 0 {
		return
	}
	sort.Strings(ids)
	state.Lock()
	state.batchPreparing = false
	state.readyBatchTxs = ids
	listeners := append([]func(){}, state.proposalReady...)
	wake := state.wake
	state.Unlock()
	if wake != nil {
		wake()
	}
	for _, listener := range listeners {
		go listener()
	}
}

// ReadyBatchTxIds returns the exact state-sync transactions that should be
// packed into the next special block.
func ReadyBatchTxIds() []string {
	state.RLock()
	defer state.RUnlock()
	return append([]string(nil), state.readyBatchTxs...)
}

// MarkBatchTxsCommitted removes committed members from the currently exposed
// wave. A partial block remains ready and is drained by the next proposal.
func MarkBatchTxsCommitted(txIds []string) {
	if len(txIds) == 0 {
		return
	}
	committed := make(map[string]struct{}, len(txIds))
	for _, txId := range txIds {
		committed[txId] = struct{}{}
	}
	state.Lock()
	remaining := state.readyBatchTxs[:0]
	for _, txId := range state.readyBatchTxs {
		if _, ok := committed[txId]; !ok {
			remaining = append(remaining, txId)
		}
	}
	state.readyBatchTxs = remaining
	hasRemaining := len(remaining) > 0
	listeners := append([]func(){}, state.proposalReady...)
	state.Unlock()
	if hasRemaining {
		for _, listener := range listeners {
			go listener()
		}
	}
}

// ClearReadyBatch resets local proposal state after a wave has fully drained.
func ClearReadyBatch() {
	state.Lock()
	state.batchPreparing = false
	state.readyBatchTxs = nil
	state.Unlock()
}

// NotifyPending wakes consensus after a raw state-sync task is queued.
func NotifyPending() {
	state.RLock()
	wake := state.wake
	state.RUnlock()
	if wake != nil {
		wake()
	}
}

package crossshard

import (
	"sync/atomic"
	"testing"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/localconf/v2"
)

func TestProposerWakeupDuringApplyFlushIsRetained(t *testing.T) {
	oldLocal, oldCommitted, oldPool := localconf.ChainMakerConfig, txCommitted, txInPool
	oldProposer := syncproposer.IsLocalProposer()
	pendingApply.Lock()
	oldTasks, oldVersion, oldFlushing := pendingApply.tasks, pendingApply.version, pendingApply.flushing
	pendingApply.tasks = map[string]*pendingApplyWrite{
		"task": {idKey: "task", txId: "id", submitted: true,
			chunk: &applyWriteChunk{batch: &CrossWriteBatch{BridgeBlockHeight: 20}}},
	}
	pendingApply.flushing = false
	pendingApply.Unlock()
	localconf.ChainMakerConfig = &localconf.CMConfig{BlockChainConfig: []localconf.BlockchainConfig{{ChainId: "chain1"}}}
	txCommitted = func(_, _ string) (bool, error) { return false, nil }
	var calls int32
	entered, release, rechecked := make(chan struct{}), make(chan struct{}), make(chan struct{})
	txInPool = func(_, _ string) (bool, error) {
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			close(entered)
			<-release
		case 2:
			close(rechecked)
		}
		return true, nil
	}
	syncproposer.SetLocalProposer(true)
	finished := make(chan struct{})
	go func() { flushPendingApplyWrites(); close(finished) }()
	<-entered
	syncproposer.SetLocalProposer(false)
	syncproposer.SetLocalProposer(true)
	onApplyProposerStatusChanged(true)
	close(release)
	<-finished
	select {
	case <-rechecked:
	case <-time.After(2 * time.Second):
		t.Error("proposer-change wakeup was lost")
	}
	// The second lookup signals before the worker exits. Acquire its lock only
	// after it has finished, so package globals are not restored underneath it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		pendingApply.Lock()
		flushing := pendingApply.flushing
		pendingApply.Unlock()
		if !flushing || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	pendingApply.Lock()
	pendingApply.tasks, pendingApply.version, pendingApply.flushing = oldTasks, oldVersion, oldFlushing
	pendingApply.Unlock()
	localconf.ChainMakerConfig, txCommitted, txInPool = oldLocal, oldCommitted, oldPool
	syncproposer.SetLocalProposer(oldProposer)
}

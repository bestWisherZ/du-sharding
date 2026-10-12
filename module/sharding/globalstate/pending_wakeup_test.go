package globalstate

import (
	"sync/atomic"
	"testing"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/localconf/v2"
)

func TestProposerWakeupRechecksSubmittedBridgeTask(t *testing.T) {
	oldLocal, oldCfg := localconf.ChainMakerConfig, globalCfg
	oldCommitted, oldPool := txCommitted, txInPool
	oldProposer := syncproposer.IsLocalProposer()
	oldReady := atomic.LoadInt32(&bridgeReady)
	pendingBridge.Lock()
	oldTasks, oldVersion, oldFlushing := pendingBridge.tasks, pendingBridge.version, pendingBridge.flushing
	oldWrites, oldBytes := pendingBridge.queuedWrites, pendingBridge.queuedBytes
	pendingBridge.tasks = map[string]*pendingBridgeSync{
		"task": {sourceShard: "business-shard1", delta: &CompactDelta{Height: 20},
			txIds: map[string]struct{}{"id": {}}, submitted: true},
	}
	pendingBridge.flushing = false
	pendingBridge.queuedWrites, pendingBridge.queuedBytes = 0, 0
	pendingBridge.Unlock()
	globalCfg = &shardingsync.ShardingConfig{}
	localconf.ChainMakerConfig = &localconf.CMConfig{BlockChainConfig: []localconf.BlockchainConfig{{ChainId: "chain0"}}}
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
	go func() { flushPendingBridgeSync(); close(finished) }()
	<-entered
	syncproposer.SetLocalProposer(false)
	syncproposer.SetLocalProposer(true)
	onProposerStatusChanged(true)
	close(release)
	<-finished
	select {
	case <-rechecked:
	case <-time.After(2 * time.Second):
		t.Error("submitted bridge task lost proposer-change wakeup")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		pendingBridge.Lock()
		flushing := pendingBridge.flushing
		pendingBridge.Unlock()
		if !flushing || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	pendingBridge.Lock()
	pendingBridge.tasks, pendingBridge.version, pendingBridge.flushing = oldTasks, oldVersion, oldFlushing
	pendingBridge.queuedWrites, pendingBridge.queuedBytes = oldWrites, oldBytes
	pendingBridge.Unlock()
	atomic.StoreInt32(&bridgeReady, oldReady)
	globalCfg, localconf.ChainMakerConfig = oldCfg, oldLocal
	txCommitted, txInPool = oldCommitted, oldPool
	syncproposer.SetLocalProposer(oldProposer)
}

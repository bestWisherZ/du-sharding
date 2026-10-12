package globalstate

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/localconf/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
)

func installPreparationTestSigner(t testing.TB, org string) {
	t.Helper()
	old := stateSyncSignerID
	t.Cleanup(func() { stateSyncSignerID = old })
	key, err := asym.GenerateKeyPair(crypto.ECC_NISTP256)
	if err != nil {
		t.Fatal(err)
	}
	stateSyncSignerID = &stateSyncSigner{orgId: org, certPem: []byte(org), sk: key,
		cert: &bcx509.Certificate{SignatureAlgorithm: bcx509.ECDSAWithSHA256}}
}

func preparationTestTask(t *testing.T) *pendingBridgeSync {
	t.Helper()
	delta := waveTestDelta(1, 1)
	delta.Timestamp = 100
	delta.Writes = []DeltaWrite{{Contract: "token", Key: []byte("key"), Value: []byte("value")}}
	task, _, err := cloneBridgeSyncTask("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestPreparedPayloadIsUnsignedImmutableAndReusable(t *testing.T) {
	resetCommittedWaveTest(t)
	installPreparationTestSigner(t, "org1")
	task := preparationTestTask(t)
	var wg sync.WaitGroup
	var misses int64
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prepared, cached, err := task.prepareChunk("chain0", 0)
			if err != nil {
				t.Error(err)
				return
			}
			if !cached {
				atomic.AddInt64(&misses, 1)
			}
			tx, err := signPreparedBridgeStateSyncTx(prepared)
			if err != nil {
				t.Error(err)
				return
			}
			raw, err := utils.CalcUnsignedTxBytes(tx)
			if err != nil || !bytes.Equal(raw, prepared.unsignedBytes) {
				t.Error("cached sign bytes do not match submitted transaction")
			}
			for _, param := range tx.Payload.Parameters {
				if len(param.Value) > 0 {
					param.Value[0] ^= 0xff
				}
			}
			tx.Payload.Timestamp = 999
		}()
	}
	wg.Wait()
	if misses != 1 || task.prepared[0].payload.Timestamp != 100 {
		t.Fatal("preparation was repeated or submitted transaction mutated the cache")
	}
	unsigned, err := utils.CalcUnsignedTxBytes(&commonpb.Transaction{Payload: task.prepared[0].payload})
	if err != nil || !bytes.Equal(unsigned, task.prepared[0].unsignedBytes) {
		t.Fatal("submitted parameter bytes mutated the prepared payload")
	}
	if _, _, err := task.prepareChunk("wrong-chain", 0); err == nil {
		t.Fatal("reused payload on another chain")
	}
	// Only the current signer is used; unsigned preparation must not pin org1.
	installPreparationTestSigner(t, "org2")
	tx, err := signPreparedBridgeStateSyncTx(task.prepared[0])
	if err != nil || tx.Sender.Signer.OrgId != "org2" {
		t.Fatal("cached transaction retained the old proposer's sender")
	}
}

func TestPreparationWindowCannotReleaseFutureWave(t *testing.T) {
	resetCommittedWaveTest(t)
	for _, shard := range globalCfg.Schedule.BusinessShards {
		enqueueBridgeSync(shard, waveTestDelta(1, 2))
	}
	// Slot 2 alone must neither initialize slot 1 nor prepare an unbounded future.
	prepareBridgeWindow("chain0")
	for _, task := range pendingBridge.tasks {
		if task.prepared != nil {
			t.Fatal("prepared a future cycle without the first checkpoint")
		}
	}
	enqueueBridgeSync("business-shard1", waveTestDelta(1, 1))
	for _, shard := range globalCfg.Schedule.BusinessShards {
		enqueueBridgeSync(shard, waveTestDelta(2, 1))
	}
	prepareBridgeWindow("chain0")
	for _, task := range pendingBridge.tasks {
		if got, want := len(task.prepared) > 0, task.delta.Cycle == 1; got != want {
			t.Fatalf("preparation window mismatch for %s %d/%d", task.sourceShard, task.delta.Cycle, task.delta.Slot)
		}
		if task.submitted {
			t.Fatal("preparation submitted a transaction")
		}
	}
	if bridgePendingReady() || pendingBridge.waveSlot != 1 {
		t.Fatal("preparing next wave released the incomplete current wave")
	}
	enqueueBridgeSync("business-shard2", waveTestDelta(1, 1))
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{waveTestTx("business-shard1", 1, 1, 0, 1)}})
	if pendingBridge.waveSlot != 1 {
		t.Fatal("partial current commit released a prepared future wave")
	}
	failed := waveTestTx("business-shard2", 1, 1, 0, 1)
	failed.Result.ContractResult.Code = 1
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{failed}})
	if pendingBridge.waveSlot != 1 {
		t.Fatal("failed checkpoint released a prepared future wave")
	}
	completePendingBridgeSync(&commonpb.Block{Txs: []*commonpb.Transaction{waveTestTx("business-shard2", 1, 1, 0, 1)}})
	if pendingBridge.waveSlot != 2 {
		t.Fatal("successful checkpoints did not advance")
	}
	prepareBridgeWindow("chain0")
	pendingBridge.Lock()
	keys := bridgeFlushKeysLocked(true)
	pendingBridge.Unlock()
	if len(keys) != 2 {
		t.Fatal("prepared next wave was not available after commit")
	}
	for _, key := range keys {
		task := pendingBridge.tasks[key]
		if task.delta.Cycle != 1 || task.delta.Slot != 2 || len(task.prepared) != 1 {
			t.Fatal("wrong prepared wave was released")
		}
	}
	for _, task := range pendingBridge.tasks {
		if task.delta.Cycle == 2 && len(task.prepared) != 1 {
			t.Fatal("lookahead did not cross the cycle boundary")
		}
	}
}

func installPreparationChainConfig(t *testing.T) {
	t.Helper()
	old := localconf.ChainMakerConfig
	localconf.ChainMakerConfig = &localconf.CMConfig{BlockChainConfig: []localconf.BlockchainConfig{{ChainId: "chain0"}}}
	t.Cleanup(func() { localconf.ChainMakerConfig = old })
}

func TestPreparedSubmitRechecksPoolAndChain(t *testing.T) {
	resetCommittedWaveTest(t)
	installPreparationChainConfig(t)
	oldPool, oldSubmit := txInPool, submitTx
	oldProposer := syncproposer.IsLocalProposer()
	t.Cleanup(func() { txInPool, submitTx = oldPool, oldSubmit; syncproposer.SetLocalProposer(oldProposer) })
	syncproposer.SetLocalProposer(true)
	task := preparationTestTask(t)
	if _, _, err := task.prepareChunk("chain0", 0); err != nil {
		t.Fatal(err)
	}
	var submitted int
	submitTx = func(_ string, _ interface{}) error { submitted++; return nil }
	txInPool = func(_, _ string) (bool, error) { return true, nil }
	// No signer is installed: an existing transaction must skip signing entirely.
	if err := submitBridgeStateSync(task); err != nil || submitted != 0 {
		t.Fatalf("did not reuse previous proposer's pooled transaction: %v", err)
	}
	txInPool = nil
	txCommitted = func(_, _ string) (bool, error) { return true, nil }
	if err := submitBridgeStateSync(task); err != nil || submitted != 0 {
		t.Fatalf("resubmitted committed prepared transaction: %v", err)
	}
	txCommitted = func(_, _ string) (bool, error) { return false, fmt.Errorf("store unavailable") }
	if err := submitBridgeStateSync(task); err == nil || submitted != 0 {
		t.Fatal("uncertain commit check did not prevent submission")
	}
	txCommitted = nil
	installPreparationTestSigner(t, "new-proposer")
	submitTx = func(_ string, tx interface{}) error {
		if tx.(*commonpb.Transaction).Sender.Signer.OrgId != "new-proposer" {
			t.Fatal("prepared transaction did not use release-time signer")
		}
		submitted++
		return nil
	}
	if err := submitBridgeStateSync(task); err != nil || submitted != 1 {
		t.Fatalf("missing transaction was not submitted: %v", err)
	}
	txInPool = func(_, _ string) (bool, error) { syncproposer.SetLocalProposer(false); return false, nil }
	if err := submitBridgeStateSync(task); err == nil || submitted != 1 {
		t.Fatal("old proposer submitted after losing leadership")
	}
}

func TestPreparationWorkerCoalescesNotifications(t *testing.T) {
	resetCommittedWaveTest(t)
	installPreparationChainConfig(t)
	// Cleanup order matters: finish preparation before restoring package globals.
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			bridgePreparation.Lock()
			bridgePreparation.enabled = false
			running := bridgePreparation.running
			bridgePreparation.Unlock()
			if !running {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("preparation worker did not finish")
			}
			time.Sleep(time.Millisecond)
		}
	})
	startBridgePreparation()
	for _, shard := range globalCfg.Schedule.BusinessShards {
		for slot := 1; slot <= 2; slot++ {
			enqueueBridgeSync(shard, waveTestDelta(1, slot))
		}
	}
	for i := 0; i < 100; i++ {
		notifyBridgePreparation()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		bridgePreparation.Lock()
		idle := !bridgePreparation.running && !bridgePreparation.dirty
		bridgePreparation.Unlock()
		if idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("preparation notifications were lost or spun indefinitely")
		}
		time.Sleep(time.Millisecond)
	}
	for _, task := range pendingBridge.tasks {
		task.prepareMu.Lock()
		prepared := len(task.prepared)
		task.prepareMu.Unlock()
		if prepared != 1 || task.submitted {
			t.Fatal("worker did not prepare both waves without submitting")
		}
	}
}

func BenchmarkBridgeSyncRelease(b *testing.B) {
	old := globalCfg
	globalCfg = &shardingsync.ShardingConfig{}
	b.Cleanup(func() { globalCfg = old })
	installPreparationTestSigner(b, "org1")
	delta := transportBenchmarkDelta()
	task, _, err := cloneBridgeSyncTask("business-shard1", delta)
	if err != nil {
		b.Fatal(err)
	}
	prepared, _, err := task.prepareChunk("chain0", 0)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("previous_on_demand", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			payload := bridgeStateSyncPayload("chain0", task.sourceShard, task.delta, task.chunks[0], 0, 1)
			// The old release path copied writes, assembled parameters and
			// serialized the entire payload immediately before signing.
			for _, param := range payload.Parameters {
				if param.Key == crossshard.StateWritesParamKey {
					param.Value = append([]byte(nil), param.Value...)
				}
			}
			tx := &commonpb.Transaction{Payload: payload}
			if tx.Sender, err = stateSyncSignerID.sign(tx); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("preprepared_release_only", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := signPreparedBridgeStateSyncTx(prepared); err != nil {
				b.Fatal(err)
			}
		}
	})
}

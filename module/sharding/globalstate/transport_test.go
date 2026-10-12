package globalstate

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"github.com/gogo/protobuf/proto"
)

func legacyCompactDeltaMessage(shard string, delta *CompactDelta) string {
	raw, _ := json.Marshal(delta)
	return compactDeltaPrefix + shard + ":" + base64.StdEncoding.EncodeToString(raw)
}

func TestBinaryTransportPreservesStateAndTransactionIDs(t *testing.T) {
	old := globalCfg
	globalCfg = &shardingsync.ShardingConfig{Schedule: shardingsync.ScheduleConfig{StateSyncMaxWritesPerTx: 2}}
	t.Cleanup(func() { globalCfg = old })
	delta := &CompactDelta{StartHeight: 11, Height: 11, BlockHash: "hash", Timestamp: 100,
		Cycle: 1, Slot: 1, SlotTotal: 6, Checkpoint: true, Writes: []DeltaWrite{
			{Contract: "token", Key: []byte("key"), Value: nil},
			{Contract: "token", Key: []byte("key"), Value: []byte{}},
			{Contract: "other", Key: []byte{0, 255, ':', '\n'}, Value: []byte{0, 255, ':', '\n'}},
			{Contract: "token", Key: []byte("key"), Value: []byte("last")},
		}}
	binary, err := BuildCompactDeltaMessage("business-shard1", delta)
	if err != nil || !strings.HasPrefix(binary, compactDeltaBinaryPrefix) {
		t.Fatalf("binary encoding: %v", err)
	}
	_, decoded, ok := ParseCompactDeltaMessage(binary)
	if !ok || !reflect.DeepEqual(decoded, delta) {
		t.Fatalf("binary round trip changed writes or checkpoint: %#v", decoded)
	}
	again, _ := BuildCompactDeltaMessage("business-shard1", decoded)
	if binary != again {
		t.Fatal("binary encoding is not deterministic")
	}
	_, legacy, ok := ParseCompactDeltaMessage(legacyCompactDeltaMessage("business-shard1", delta))
	if !ok || !reflect.DeepEqual(legacy, decoded) {
		t.Fatal("legacy message cannot be replayed")
	}
	task, id, err := cloneBridgeSyncTask("business-shard1", decoded)
	if err != nil {
		t.Fatal(err)
	}
	oldTask, oldID, err := cloneBridgeSyncTask("business-shard1", legacy)
	if err != nil || oldID != id || !reflect.DeepEqual(task.chunks, oldTask.chunks) {
		t.Fatal("transport version changed on-chain sync IDs or task deduplication")
	}
	decoded.Writes[2].Value[0] = 123
	_, isolated, ok := ParseCompactDeltaMessage(binary)
	if !ok || isolated.Writes[2].Value[0] != 0 {
		t.Fatal("decoded bytes alias the immutable message")
	}
}

func TestBinaryTransportHandlesEmptyCheckpoints(t *testing.T) {
	for _, writes := range [][]DeltaWrite{nil, {}} {
		delta := waveTestDelta(1, 1)
		delta.Writes = writes
		msg, err := BuildCompactDeltaMessage("business-shard1", delta)
		if err != nil {
			t.Fatal(err)
		}
		_, got, ok := ParseCompactDeltaMessage(msg)
		if !ok || !got.Checkpoint || len(got.Writes) != 0 || got.Cycle != 1 || got.Slot != 1 {
			t.Fatal("empty checkpoint was lost")
		}
	}
}

func TestBinaryTransportRejectsMalformedMessages(t *testing.T) {
	wrap := func(wire *compactDeltaWire) string {
		raw, err := proto.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		return compactDeltaBinaryPrefix + "business-shard1:" + string(raw)
	}
	sourceIDWrites, _ := crossshard.EncodeBridgeStateWrites([]crossshard.BridgeStateWrite{{
		ContractName: "token", Key: []byte("k"), SourceTxId: "unexpected",
	}})
	for _, msg := range []string{
		"GLOBAL_STATE_DELTA_V4:business-shard1:anything", compactDeltaBinaryPrefix,
		compactDeltaBinaryPrefix + ":\x10\x01", compactDeltaBinaryPrefix + "business-shard1:\xff",
		wrap(&compactDeltaWire{}), wrap(&compactDeltaWire{Height: 1, Slot: -1}),
		wrap(&compactDeltaWire{Height: 1, SlotTotal: -1}),
		wrap(&compactDeltaWire{Height: 1, Writes: []byte{0xff}}),
		wrap(&compactDeltaWire{Height: 1, Writes: sourceIDWrites}),
	} {
		if _, _, ok := ParseCompactDeltaMessage(msg); ok {
			t.Fatalf("accepted malformed message %q", msg)
		}
	}
	for _, shard := range []string{"", "bad:shard", "bad\xff"} {
		if _, err := BuildCompactDeltaMessage(shard, waveTestDelta(1, 1)); err == nil {
			t.Fatal("accepted invalid envelope")
		}
	}
}

func TestBinaryAndLegacyHandlersDeduplicateSameCheckpoint(t *testing.T) {
	resetCommittedWaveTest(t)
	globalCfg.ShardType = "BRIDGE"
	globalCfg.SyncNetwork.Enable = true
	delta := waveTestDelta(1, 1)
	id := testidentity.New(t)
	globalCfg.SyncAuth = id.Config()
	chunks, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	delta.Authorizations = make([][]byte, len(chunks))
	for i, chunk := range chunks {
		payload := bridgeStateSyncPayload("chain0", "business-shard1", delta, chunk, i, len(chunks))
		delta.Authorizations[i], err = syncauth.Sign(globalCfg.SyncAuth, "business-shard1", "bridge", payload, id.Certificate, id.Key)
		if err != nil {
			t.Fatal(err)
		}
	}
	binary, err := BuildCompactDeltaMessage("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	handleMessage(binary)
	handleMessage(legacyCompactDeltaMessage("business-shard1", delta))
	if len(pendingBridge.tasks) != 1 || bridgePendingReady() {
		t.Fatal("duplicate transport messages changed task count or released an incomplete wave")
	}
}

func TestCheckpointContentKeyExcludesProofButBindsEveryStateField(t *testing.T) {
	base := &compactDeltaWire{Height: 11, StartHeight: 11, BlockHash: "h11", Timestamp: 100,
		Cycle: 1, Slot: 1, SlotTotal: 6, Checkpoint: true, Writes: []byte("writes"),
		Authorizations: [][]byte{[]byte("replica-1")}}
	key := checkpointContentKey("business-shard1", base)
	copy := *base
	copy.Authorizations = [][]byte{[]byte("replica-2")}
	if checkpointContentKey("business-shard1", &copy) != key {
		t.Fatal("replica signatures prevented content deduplication")
	}
	for _, mutate := range []func(*compactDeltaWire){
		func(w *compactDeltaWire) { w.StartHeight++ }, func(w *compactDeltaWire) { w.Height++ },
		func(w *compactDeltaWire) { w.BlockHash += "x" }, func(w *compactDeltaWire) { w.Timestamp++ },
		func(w *compactDeltaWire) { w.Cycle++ }, func(w *compactDeltaWire) { w.Slot++ },
		func(w *compactDeltaWire) { w.SlotTotal++ }, func(w *compactDeltaWire) { w.Checkpoint = false },
		func(w *compactDeltaWire) { w.Writes = []byte("changed") },
	} {
		copy := *base
		mutate(&copy)
		if checkpointContentKey("business-shard1", &copy) == key {
			t.Fatal("changed state metadata was treated as an authenticated duplicate")
		}
	}
	if checkpointContentKey("business-shard2", base) == key {
		t.Fatal("different source shard shares a cache key")
	}
}

func TestInvalidCheckpointCannotPoisonValidRetransmission(t *testing.T) {
	resetCommittedWaveTest(t)
	globalCfg.ShardType = "BRIDGE"
	globalCfg.SyncNetwork.Enable = true
	id := testidentity.New(t)
	globalCfg.SyncAuth = id.Config()
	delta := waveTestDelta(1, 1)
	delta.Writes = []DeltaWrite{{Contract: "token", Key: []byte("a"), Value: []byte("1")}}
	invalid, _ := BuildCompactDeltaMessage("business-shard1", delta)
	handleMessage(invalid)
	if len(pendingBridge.tasks) != 0 {
		t.Fatal("unsigned checkpoint queued")
	}
	chunks, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	delta.Authorizations = make([][]byte, len(chunks))
	for i, chunk := range chunks {
		p := bridgeStateSyncPayload("chain0", "business-shard1", delta, chunk, i, len(chunks))
		delta.Authorizations[i], err = syncauth.Sign(globalCfg.SyncAuth, "business-shard1", "bridge", p, id.Certificate, id.Key)
		if err != nil {
			t.Fatal(err)
		}
	}
	valid, _ := BuildCompactDeltaMessage("business-shard1", delta)
	handleMessage(valid)
	if len(pendingBridge.tasks) != 1 {
		t.Fatal("valid retry suppressed by invalid first message")
	}
	handleMessage(invalid)
	delta.Writes[0].Value = []byte("forged")
	forged, _ := BuildCompactDeltaMessage("business-shard1", delta)
	handleMessage(forged)
	if len(pendingBridge.tasks) != 1 {
		t.Fatal("cached signature authorized modified writes")
	}
	for _, task := range pendingBridge.tasks {
		writes, err := crossshard.DecodeBridgeStateWrites(task.chunks[0].writesRaw, crossshard.BridgeStateWritesEncodingV1)
		if err != nil || len(writes) != 1 || string(writes[0].Value) != "1" {
			t.Fatal("duplicate replaced original authenticated content")
		}
	}
}

func transportBenchmarkDelta() *CompactDelta {
	delta := waveTestDelta(1, 1)
	for i := 0; i < 2000; i++ {
		delta.Writes = append(delta.Writes, DeltaWrite{Contract: "standard_dfa_experiment",
			Key: []byte(fmt.Sprintf("balance_%040x", i)), Value: []byte("999999999")})
	}
	return delta
}

func TestBinaryTransportPayloadSize(t *testing.T) {
	delta := transportBenchmarkDelta()
	old := legacyCompactDeltaMessage("business-shard1", delta)
	new, err := BuildCompactDeltaMessage("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("2000 writes: JSON/base64=%d bytes binary=%d bytes reduction=%.2f%%",
		len(old), len(new), 100*(1-float64(len(new))/float64(len(old))))
	if len(new) >= len(old) {
		t.Fatal("binary transport did not reduce payload size")
	}
}

func BenchmarkCompactDeltaTransport(b *testing.B) {
	delta := transportBenchmarkDelta()
	b.Run("json_base64_roundtrip", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, ok := ParseCompactDeltaMessage(legacyCompactDeltaMessage("business-shard1", delta))
			if !ok {
				b.Fatal("legacy decode failed")
			}
		}
	})
	b.Run("binary_roundtrip", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			msg, err := BuildCompactDeltaMessage("business-shard1", delta)
			_, _, ok := ParseCompactDeltaMessage(msg)
			if err != nil || !ok {
				b.Fatal("binary round trip failed")
			}
		}
	})
}

func FuzzCompactDeltaMessage(f *testing.F) {
	msg, _ := BuildCompactDeltaMessage("business-shard1", waveTestDelta(1, 1))
	f.Add([]byte(msg))
	f.Add([]byte(legacyCompactDeltaMessage("business-shard1", waveTestDelta(1, 1))))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if !bytes.HasPrefix(raw, []byte(compactDeltaBinaryPrefix)) {
			return
		}
		shard, delta, ok := ParseCompactDeltaMessage(string(raw))
		if !ok {
			return
		}
		encoded, err := BuildCompactDeltaMessage(shard, delta)
		if err != nil {
			t.Fatal(err)
		}
		_, again, ok := ParseCompactDeltaMessage(encoded)
		if !ok || !reflect.DeepEqual(delta, again) {
			t.Fatal("valid binary message does not round trip")
		}
	})
}

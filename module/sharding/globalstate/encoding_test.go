package globalstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
)

// Preserve the previous JSON implementation as a state-semantics oracle and
// preparation benchmark baseline; new transaction IDs use a versioned domain.
func legacyBridgeTask(source string, delta *CompactDelta) (*pendingBridgeSync, string) {
	raw, _ := json.Marshal(delta)
	var copied CompactDelta
	_ = json.Unmarshal(raw, &copied)
	chunks := legacyBridgeChunks(&copied)
	task := &pendingBridgeSync{sourceShard: source, delta: &copied,
		txIds: make(map[string]struct{}), rawWrites: len(copied.Writes), rawBytes: len(raw)}
	for i, writes := range chunks {
		encoded, _ := json.Marshal(writes)
		id := legacyBridgeID(source, &copied, i, len(chunks), encoded)
		task.chunks = append(task.chunks, bridgeStateSyncChunk{writesRaw: encoded, writeCount: len(writes), txId: id, logicalId: id})
		task.txIds[id] = struct{}{}
	}
	sum := sha256.Sum256(append([]byte(source+"|"), raw...))
	return task, hex.EncodeToString(sum[:])
}

func legacyBridgeChunks(delta *CompactDelta) [][]crossshard.BridgeStateWrite {
	writes := make([]crossshard.BridgeStateWrite, 0, len(delta.Writes))
	for _, w := range delta.Writes {
		if shouldSyncStateWrite(w.Contract, w.Key) {
			writes = append(writes, crossshard.BridgeStateWrite{ContractName: w.Contract,
				Key: append([]byte(nil), w.Key...), Value: append([]byte(nil), w.Value...)})
		}
	}
	limit := globalCfg.Schedule.StateSyncMaxWritesPerTx
	if limit <= 0 {
		limit = 4000
	}
	if len(writes) == 0 {
		return [][]crossshard.BridgeStateWrite{{}}
	}
	var chunks [][]crossshard.BridgeStateWrite
	for start := 0; start < len(writes); start += limit {
		end := start + limit
		if end > len(writes) {
			end = len(writes)
		}
		chunks = append(chunks, writes[start:end])
	}
	return chunks
}

func legacyBridgeID(source string, delta *CompactDelta, index, total int, raw []byte) string {
	s := fmt.Sprintf("%s|%d|%d|%s|%d|%d|%d|%t|%d|%d|%x", source, delta.StartHeight,
		delta.Height, delta.BlockHash, delta.Cycle, delta.Slot, delta.SlotTotal,
		delta.Checkpoint, index, total, raw)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestBridgeEncodingPreservesLegacyWrites(t *testing.T) {
	old := globalCfg
	t.Cleanup(func() { globalCfg = old })
	globalCfg = &shardingsync.ShardingConfig{}
	for _, limit := range []int{0, 1, 2, 4000} {
		globalCfg.Schedule.StateSyncMaxWritesPerTx = limit
		for _, writes := range [][]DeltaWrite{
			nil, {},
			{{Contract: "token_business", Key: []byte("a"), Value: []byte("999")}},
			{{Contract: "token_business", Key: []byte("a"), Value: nil},
				{Contract: "token_business", Key: []byte{0, 255}, Value: []byte{0, 255, 34}}},
			{{Contract: "token_<>&\u2028", Key: []byte("key"), Value: []byte("value")}},
			{{Contract: "token_\xff\xfe", Key: []byte("key"), Value: []byte("value")}},
		} {
			for _, hash := range []string{"block<&>\u2029", "block\xff"} {
				delta := &CompactDelta{StartHeight: 11, Height: 12, BlockHash: hash,
					Timestamp: 1789529000, Cycle: 2, Slot: 1, SlotTotal: 6, Checkpoint: true, Writes: writes}
				want, wantID := legacyBridgeTask("business-shard1", delta)
				want.delta.Writes = nil
				got, gotID, err := cloneBridgeSyncTask("business-shard1", delta)
				if err != nil || gotID == "" || gotID == wantID || !reflect.DeepEqual(got.delta, want.delta) || len(got.chunks) != len(want.chunks) {
					t.Fatalf("limit=%d hash=%q writes=%v err=%v: checkpoint/chunk boundaries changed", limit, hash, writes, err)
				}
				for i, chunk := range got.chunks {
					decoded, err := crossshard.DecodeBridgeStateWrites(chunk.writesRaw, crossshard.BridgeStateWritesEncodingV1)
					var legacy []crossshard.BridgeStateWrite
					if oldErr := json.Unmarshal(want.chunks[i].writesRaw, &legacy); err != nil || oldErr != nil || !reflect.DeepEqual(decoded, legacy) {
						t.Fatalf("chunk %d changed state writes: err=%v", i, err)
					}
					if chunk.writeCount != len(legacy) || chunk.txId == want.chunks[i].txId {
						t.Fatal("incorrect write count or missing versioned transaction ID")
					}
					raw := fmt.Sprintf("%s|%s|%d|%d|%s|%d|%d|%d|%t|%d|%d|%x", crossshard.BridgeStateWritesEncodingV1,
						got.sourceShard, got.delta.StartHeight, got.delta.Height, got.delta.BlockHash, got.delta.Cycle,
						got.delta.Slot, got.delta.SlotTotal, got.delta.Checkpoint, i, len(got.chunks), chunk.writesRaw)
					sum := sha256.Sum256([]byte(raw))
					if chunk.txId != hex.EncodeToString(sum[:]) {
						t.Fatal("transaction ID does not bind codec version and payload")
					}
				}
			}
		}
	}
}

func TestBridgeEncodingDistinguishesEmptyAndDeletedValues(t *testing.T) {
	old := globalCfg
	globalCfg = &shardingsync.ShardingConfig{}
	t.Cleanup(func() { globalCfg = old })
	delta := &CompactDelta{Height: 11, Writes: []DeltaWrite{
		{Contract: "token", Key: []byte("empty"), Value: []byte{}},
		{Contract: "token", Key: []byte("deleted"), Value: nil},
	}}
	chunks, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	writes, err := crossshard.DecodeBridgeStateWrites(chunks[0].writesRaw, crossshard.BridgeStateWritesEncodingV1)
	if err != nil || len(writes) != 2 || writes[0].Value == nil || len(writes[0].Value) != 0 || writes[1].Value != nil {
		t.Fatalf("empty value was changed to deletion: %v %v", writes, err)
	}
}

func TestBridgeCachedWritesAreIsolatedAcrossRetries(t *testing.T) {
	oldCfg, oldSigner := globalCfg, stateSyncSignerID
	t.Cleanup(func() { globalCfg, stateSyncSignerID = oldCfg, oldSigner })
	globalCfg = &shardingsync.ShardingConfig{}
	key, err := asym.GenerateKeyPair(crypto.ECC_NISTP256)
	if err != nil {
		t.Fatal(err)
	}
	stateSyncSignerID = &stateSyncSigner{orgId: "org1", certPem: []byte("test-cert"), sk: key,
		cert: &bcx509.Certificate{SignatureAlgorithm: bcx509.ECDSAWithSHA256}}
	delta := &CompactDelta{Height: 11, Timestamp: 100, Checkpoint: true,
		Writes: []DeltaWrite{{Contract: "token_business", Key: []byte("key"), Value: []byte("value")}}}
	task, _, err := cloneBridgeSyncTask("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), task.chunks[0].writesRaw...)
	delta.Writes[0].Key[0], delta.Writes[0].Value[0], delta.Height = 'X', 'X', 99
	if task.delta.Writes != nil || task.delta.Height != 11 || !bytes.Equal(task.chunks[0].writesRaw, want) {
		t.Fatal("pending task retained or aliases source writes")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := buildBridgeStateSyncTx("chain0", task.sourceShard, task.delta, task.chunks[0], 0, 1)
			if err != nil {
				t.Error(err)
				return
			}
			versionFound := false
			for _, kv := range tx.Payload.Parameters {
				if kv.Key == crossshard.StateWritesEncodingParamKey {
					versionFound = string(kv.Value) == crossshard.BridgeStateWritesEncodingV1
				}
				if kv.Key == crossshard.StateWritesParamKey {
					if !bytes.Equal(kv.Value, want) || tx.Payload.TxId != task.chunks[0].txId {
						t.Error("retry payload changed")
					}
					kv.Value[0] = 'X'
				}
			}
			if !versionFound {
				t.Error("missing encoding version")
			}
		}()
	}
	wg.Wait()
	if !bytes.Equal(task.chunks[0].writesRaw, want) {
		t.Fatal("submitted transaction mutated cached writes")
	}
}

var bridgeEncodingBenchmarkSink string
var bridgeEncodingBytesSink []byte

func BenchmarkBridgeSyncPreparation(b *testing.B) {
	old := globalCfg
	b.Cleanup(func() { globalCfg = old })
	globalCfg = &shardingsync.ShardingConfig{}
	delta := &CompactDelta{Height: 11, Timestamp: 100, Checkpoint: true, Cycle: 1, Slot: 1, SlotTotal: 6}
	for i := 0; i < 2000; i++ {
		delta.Writes = append(delta.Writes, DeltaWrite{Contract: "standard_dfa_experiment",
			Key: []byte(fmt.Sprintf("balance_%040x", i)), Value: []byte("999999999")})
	}
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			task, id := legacyBridgeTask("business-shard1", delta)
			chunks := legacyBridgeChunks(task.delta)
			for j, writes := range chunks {
				for n := 0; n < 2; n++ {
					raw, _ := json.Marshal(writes)
					id = legacyBridgeID(task.sourceShard, task.delta, j, len(chunks), raw)
					bridgeEncodingBytesSink = raw
				}
			}
			bridgeEncodingBenchmarkSink = id
		}
	})
	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			task, id, err := cloneBridgeSyncTask("business-shard1", delta)
			if err != nil {
				b.Fatal(err)
			}
			for _, chunk := range task.chunks {
				raw := append([]byte(nil), chunk.writesRaw...)
				bridgeEncodingBytesSink = raw
				if len(raw) == 0 {
					b.Fatal("empty payload")
				}
			}
			bridgeEncodingBenchmarkSink = id
		}
	})
}

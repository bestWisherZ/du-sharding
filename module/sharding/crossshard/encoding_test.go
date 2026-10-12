package crossshard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
)

func TestApplyCachedWritesMatchLegacyAndRemainIsolated(t *testing.T) {
	oldCfg, oldSigner := crossCfg, applySignerID
	t.Cleanup(func() { crossCfg, applySignerID = oldCfg, oldSigner })
	crossCfg = &shardingsync.ShardingConfig{Schedule: shardingsync.ScheduleConfig{StateSyncMaxWritesPerTx: 2}}
	key, err := asym.GenerateKeyPair(crypto.ECC_NISTP256)
	if err != nil {
		t.Fatal(err)
	}
	applySignerID = &applySigner{orgId: "org1", certPem: []byte("test-cert"), sk: key,
		cert: &bcx509.Certificate{SignatureAlgorithm: bcx509.ECDSAWithSHA256}}
	batch := &CrossWriteBatch{BatchId: "batch<&>\u2028", TargetShard: "business-shard1",
		BridgeBlockHeight: 20, BridgeBlockHash: "hash", Timestamp: 100,
		Writes: []CrossWrite{
			{ContractName: "token_business", Key: []byte("c"), Value: nil, BridgeTxId: "tx-c"},
			{ContractName: "token_business", Key: []byte("a"), Value: []byte{}, BridgeTxId: "tx-a"},
			{ContractName: "token_business", Key: []byte("b"), Value: []byte{0, 255}, BridgeTxId: "tx-b"},
		}}
	chunks, err := buildApplyWriteChunks(batch)
	if err != nil || len(chunks) != 2 {
		t.Fatalf("chunks=%d err=%v", len(chunks), err)
	}
	for _, chunk := range chunks {
		encoded, err := json.Marshal(chunk.writes)
		if err != nil || !bytes.Equal(encoded, chunk.writesRaw) {
			t.Fatal("cached writes differ from legacy encoding")
		}
		raw := fmt.Sprintf("%s|%s|%d|%s|%s|%d|%d|%x", batch.BatchId,
			normalizeShardId(batch.TargetShard), batch.BridgeBlockHeight, batch.BridgeBlockHash,
			chunk.contractName, chunk.chunkIndex, chunk.chunkTotal, encoded)
		sum := sha256.Sum256([]byte(raw))
		wantKey := "batch:" + hex.EncodeToString(sum[:])
		if chunk.idKey != wantKey || chunk.txId != deterministicTxId(wantKey) {
			t.Fatal("writeback ID changed")
		}
	}
	batch.Writes[2].Key[0], batch.Writes[2].Value[0] = 'X', 'X'
	batch.Timestamp, batch.BatchId = 999, "mutated"
	if string(chunks[0].writes[1].Key) != "b" || chunks[0].writes[1].Value[0] != 0 {
		t.Fatal("chunk aliases incoming batch")
	}
	var wg sync.WaitGroup
	for _, chunk := range chunks {
		for n := 0; n < 8; n++ {
			wg.Add(1)
			go func(chunk *applyWriteChunk) {
				defer wg.Done()
				tx, err := buildApplyWriteChunkTx("chain1", chunk)
				if err != nil {
					t.Error(err)
					return
				}
				if tx.Payload.Timestamp != 100 || tx.Payload.TxId != chunk.txId {
					t.Error("retry metadata changed")
				}
				for _, param := range tx.Payload.Parameters {
					if param.Key == StateWritesParamKey {
						if !bytes.Equal(param.Value, chunk.writesRaw) {
							t.Error("retry changed encoded writes")
						}
						param.Value[0] = 'X'
					}
				}
			}(chunk)
		}
	}
	wg.Wait()
	for _, chunk := range chunks {
		encoded, _ := json.Marshal(chunk.writes)
		if !bytes.Equal(encoded, chunk.writesRaw) {
			t.Fatal("submitted payload mutated retry buffer")
		}
	}
}

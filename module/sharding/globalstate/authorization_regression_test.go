package globalstate

import (
	"bytes"
	"errors"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
)

func TestSourceReplicaAuthorizationsBindDifferentPayloadIDs(t *testing.T) {
	old := globalCfg
	t.Cleanup(func() { globalCfg = old })
	one, two := testidentity.New(t), testidentity.New(t)
	cfg := one.Config()
	cfg.Signers["business-shard1"] = append(cfg.Signers["business-shard1"], two.Pin)
	globalCfg = &shardingsync.ShardingConfig{ShardId: "bridge", ShardType: "BRIDGE", SyncAuth: cfg}
	delta := &CompactDelta{StartHeight: 12, Height: 12, BlockHash: "h12", Timestamp: 1700000000,
		Cycle: 1, Slot: 1, SlotTotal: 6, Checkpoint: true,
		Writes: []DeltaWrite{{Contract: "token", Key: []byte("a"), Value: []byte("1")}}}
	chunks, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	p := bridgeStateSyncPayload("chain0", "business-shard1", delta, chunks[0], 0, 1)
	auth1, err := syncauth.Sign(cfg, "business-shard1", "bridge", p, one.Certificate, one.Key)
	if err != nil {
		t.Fatal(err)
	}
	auth2, err := syncauth.Sign(cfg, "business-shard1", "bridge", p, two.Certificate, two.Key)
	if err != nil {
		t.Fatal(err)
	}
	a, b := *delta, *delta
	a.Authorizations, b.Authorizations = [][]byte{auth1}, [][]byte{auth2}
	ta, idA, err := cloneBridgeSyncTask("business-shard1", &a)
	if err != nil {
		t.Fatal(err)
	}
	tb, idB, err := cloneBridgeSyncTask("business-shard1", &b)
	if err != nil {
		t.Fatal(err)
	}
	pa := bridgeStateSyncPayload("chain0", "business-shard1", ta.delta, ta.chunks[0], 0, 1)
	pb := bridgeStateSyncPayload("chain0", "business-shard1", tb.delta, tb.chunks[0], 0, 1)
	for _, p := range []*commonpb.Payload{pa, pb} {
		if err := syncauth.Verify(cfg, "business-shard1", "bridge", p); err != nil {
			t.Fatal(err)
		}
	}
	if idA != idB || pa.TxId == pb.TxId || syncauth.LogicalID(pa) != syncauth.LogicalID(pb) {
		t.Fatal("replica proofs must have distinct transaction IDs but one checkpoint identity")
	}
	copy := bridgeStateSyncPayload("chain0", "business-shard1", ta.delta, ta.chunks[0], 0, 1)
	rawA, _ := utils.CalcUnsignedTxBytes(&commonpb.Transaction{Payload: pa})
	rawCopy, _ := utils.CalcUnsignedTxBytes(&commonpb.Transaction{Payload: copy})
	if pa.TxId != copy.TxId || !bytes.Equal(rawA, rawCopy) {
		t.Fatal("retry changed payload")
	}
	copy.TxId = pb.TxId
	if err := syncauth.Verify(cfg, "business-shard1", "bridge", copy); err == nil {
		t.Fatal("mismatched ID accepted")
	}
}

func TestBridgeDuplicateErrorDoesNotHideMissingCertificate(t *testing.T) {
	if isDuplicateBridgeStateSyncError(errors.New("required certificate does not exist")) {
		t.Fatal("missing certificate is not a duplicate transaction")
	}
}

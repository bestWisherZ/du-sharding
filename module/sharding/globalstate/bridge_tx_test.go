package globalstate

import (
	"reflect"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
)

func TestBridgeStateSyncTxIdsAreDeterministicAndChunked(t *testing.T) {
	oldCfg := globalCfg
	globalCfg = &shardingsync.ShardingConfig{
		Schedule: shardingsync.ScheduleConfig{StateSyncMaxWritesPerTx: 2},
	}
	defer func() { globalCfg = oldCfg }()

	delta := &CompactDelta{
		StartHeight: 20,
		Height:      20,
		BlockHash:   "block-20",
		Timestamp:   100,
		Checkpoint:  true,
		Writes: []DeltaWrite{
			{Contract: "token_business", Key: []byte("a"), Value: []byte("1")},
			{Contract: "token_business", Key: []byte("b"), Value: []byte("2")},
			{Contract: "token_business", Key: []byte("c"), Value: []byte("3")},
		},
	}
	first, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encodeBridgeStateSyncChunks("business-shard1", delta)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("tx id count = %d, want 2", len(first))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("tx ids are not deterministic: %v != %v", first, second)
	}
	if first[0].txId == first[1].txId {
		t.Fatal("different chunks must have different tx ids")
	}
}

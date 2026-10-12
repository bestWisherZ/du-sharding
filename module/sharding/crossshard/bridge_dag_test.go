package crossshard

import (
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func TestIsBridgeStateSyncBatch(t *testing.T) {
	old := crossCfg
	defer func() { crossCfg = old }()
	syncTx := &commonPb.Transaction{Payload: &commonPb.Payload{
		TxType: commonPb.TxType_INVOKE_CONTRACT, Method: SystemBridgeStateSyncApplyMethod,
	}}
	ordinary := &commonPb.Transaction{Payload: &commonPb.Payload{Method: "TransferExperiment"}}
	writeback := &commonPb.Transaction{Payload: &commonPb.Payload{Method: SystemStateSyncApplyMethod}}
	for _, role := range []string{"", "BUSINESS", "BRIDGE", " bridge "} {
		crossCfg = &shardingsync.ShardingConfig{ShardType: role}
		want := role == "BRIDGE" || role == " bridge "
		if IsBridgeStateSyncBatch([]*commonPb.Transaction{syncTx}) != want {
			t.Fatalf("role %q", role)
		}
		for _, txs := range [][]*commonPb.Transaction{nil, {nil}, {syncTx, ordinary}, {writeback}} {
			if IsBridgeStateSyncBatch(txs) {
				t.Fatalf("unexpected eligible batch for role %q", role)
			}
		}
	}
	crossCfg = nil
	if IsBridgeStateSyncBatch([]*commonPb.Transaction{syncTx}) {
		t.Fatal("unconfigured node eligible")
	}
}

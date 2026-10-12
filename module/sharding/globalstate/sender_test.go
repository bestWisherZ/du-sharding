package globalstate

import (
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func TestWritebackDoesNotProduceStateDelta(t *testing.T) {
	tx := &commonpb.Transaction{
		Payload: &commonpb.Payload{
			TxType:       commonpb.TxType_INVOKE_CONTRACT,
			TxId:         "business-writeback-1",
			ContractName: "token_business",
			Method:       crossshard.SystemStateSyncApplyMethod,
			Parameters: []*commonpb.KeyValuePair{
				{Key: crossshard.SourceShardParamKey, Value: []byte("bridge")},
				{Key: crossshard.SourceHeightParamKey, Value: []byte("21")},
				{Key: crossshard.SourceTxIdParamKey, Value: []byte("bridge-tx-1")},
				{Key: crossshard.SourceBlockHashParamKey, Value: []byte("bridge-block-hash")},
				{Key: crossshard.TargetShardParamKey, Value: []byte("business-shard1")},
				{Key: crossshard.TargetContractParamKey, Value: []byte("token_business")},
				{Key: crossshard.StateKeyParamKey, Value: []byte("alice")},
			},
		},
		Result: &commonpb.Result{Code: commonpb.TxStatusCode_SUCCESS},
	}
	block := &commonpb.Block{
		Header: &commonpb.BlockHeader{BlockHeight: 22, BlockHash: []byte{1, 2, 3}},
		Txs:    []*commonpb.Transaction{tx},
	}
	rwSets := map[string]*commonpb.TxRWSet{
		"business-writeback-1": {
			TxId: "business-writeback-1",
			TxWrites: []*commonpb.TxWrite{{
				ContractName: "token_business",
				Key:          []byte("alice"),
				Value:        []byte("99"),
			}},
		},
	}
	if writes := collectDeltaWrites(block, rwSets); len(writes) != 0 {
		t.Fatalf("writeback state echoed to bridge: %+v", writes)
	}
	if !isBridgeWritebackOnlyBlock(block) {
		t.Fatal("writeback-only block was not excluded from checkpoint stream")
	}
}

func TestMixedBlockStillProducesCheckpoint(t *testing.T) {
	block := &commonpb.Block{Txs: []*commonpb.Transaction{
		{Payload: &commonpb.Payload{
			TxType: commonpb.TxType_INVOKE_CONTRACT,
			Method: crossshard.SystemStateSyncApplyBatchMethod,
		}},
		{Payload: &commonpb.Payload{
			TxType:       commonpb.TxType_INVOKE_CONTRACT,
			ContractName: "token_business",
			Method:       "transfer",
		}},
	}}
	if isBridgeWritebackOnlyBlock(block) {
		t.Fatal("mixed user/writeback block was excluded from checkpoint stream")
	}
}

func TestCollectDeltaWritesKeepsLatestValueWithinBlock(t *testing.T) {
	block := &commonpb.Block{Txs: []*commonpb.Transaction{
		{Payload: &commonpb.Payload{TxId: "tx-1", ContractName: "token_business", Method: "transfer"},
			Result: &commonpb.Result{Code: commonpb.TxStatusCode_SUCCESS}},
		{Payload: &commonpb.Payload{TxId: "tx-2", ContractName: "token_business", Method: "transfer"},
			Result: &commonpb.Result{Code: commonpb.TxStatusCode_SUCCESS}},
	}}
	rwSets := map[string]*commonpb.TxRWSet{
		"tx-1": {TxId: "tx-1", TxWrites: []*commonpb.TxWrite{{
			ContractName: "token_business", Key: []byte("alice"), Value: []byte("100"),
		}}},
		"tx-2": {TxId: "tx-2", TxWrites: []*commonpb.TxWrite{{
			ContractName: "token_business", Key: []byte("alice"), Value: []byte("99"),
		}}},
	}
	writes := collectDeltaWrites(block, rwSets)
	if len(writes) != 1 || string(writes[0].Value) != "99" {
		t.Fatalf("unexpected block writes: %+v", writes)
	}
}

package crossshard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	protocolmock "chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

func TestStateSyncWriteFootprint(t *testing.T) {
	oldCfg := crossCfg
	t.Cleanup(func() { crossCfg = oldCfg })
	for _, tc := range []struct {
		method string
		count  int
	}{
		{SystemBridgeStateSyncApplyMethod, 2000},
		{SystemStateSyncApplyBatchMethod, 2000},
		{SystemStateSyncApplyMethod, 1},
	} {
		t.Run(tc.method, func(t *testing.T) {
			writes := make([]BridgeStateWrite, tc.count)
			expected := make(map[string][]byte, tc.count+1)
			for i := range writes {
				writes[i] = BridgeStateWrite{ContractName: "token", Key: []byte(fmt.Sprintf("account-%d", i)), Value: []byte("99")}
				expected["token/"+string(writes[i].Key)] = writes[i].Value
			}
			raw, err := json.Marshal(writes)
			if err != nil {
				t.Fatal(err)
			}
			crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS"}
			tx := &commonpb.Transaction{Payload: &commonpb.Payload{
				TxType: commonpb.TxType_INVOKE_CONTRACT, ContractName: "token", Method: tc.method,
				Parameters: []*commonpb.KeyValuePair{
					{Key: SourceShardParamKey, Value: []byte("bridge")},
					{Key: TargetShardParamKey, Value: []byte("business-shard1")},
					{Key: SourceHeightParamKey, Value: []byte("15")},
					{Key: SourceBlockHashParamKey, Value: []byte("block-hash-15")},
					{Key: StateWritesParamKey, Value: raw},
					{Key: StateKeyParamKey, Value: writes[0].Key},
					{Key: StateValueParamKey, Value: writes[0].Value},
				},
			}}
			apply := ApplyStateSyncTx
			if tc.method == SystemBridgeStateSyncApplyMethod {
				crossCfg.ShardId, crossCfg.ShardType = "bridge", "BRIDGE"
				tx = bridgeStateSyncTestTx(raw)
				authorizeTestTx(t, tx, "business-shard1", "bridge")
				expected[bridgeSyncMetadataContract+"/__bridge_state_sync:business-shard1:15:0:1"] = []byte("block-hash-15")
				apply = ApplyBridgeStateSyncTx
			} else {
				authorizeTestTx(t, tx, "bridge", "business-shard1")
			}
			// Any state read or additional write fails this expectation.
			ctx := stateSyncMockNoWrites(t)
			ctx.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(contract string, key, value []byte) error {
				identity := contract + "/" + string(key)
				want, exists := expected[identity]
				if !exists || !bytes.Equal(value, want) {
					t.Fatalf("unexpected state write: %s", identity)
				}
				delete(expected, identity)
				return nil
			}).Times(len(expected))
			result, err := apply(tx, ctx)
			if err != nil || result == nil || result.Code != commonpb.TxStatusCode_SUCCESS {
				t.Fatalf("apply failed: result=%v err=%v", result, err)
			}
			if len(expected) != 0 {
				t.Fatalf("missing state writes: %d", len(expected))
			}
		})
	}
}

func TestApplyBridgeStateSyncTx(t *testing.T) {
	oldCfg := crossCfg
	crossCfg = &shardingsync.ShardingConfig{
		ShardId:   "bridge-shard",
		ShardType: "BRIDGE",
		Schedule: shardingsync.ScheduleConfig{
			BusinessShards: []string{"business-shard1"},
		},
	}
	defer func() { crossCfg = oldCfg }()

	writesRaw, err := json.Marshal([]BridgeStateWrite{{
		ContractName: "token_business",
		Key:          []byte("alice"),
		Value:        []byte("99"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	tx := bridgeStateSyncTestTx(writesRaw)
	authorizeTestTx(t, tx, "business-shard1", "bridge")
	ctx, state := stateSyncMock(t)

	result, err := ApplyBridgeStateSyncTx(tx, ctx)
	if err != nil {
		t.Fatalf("ApplyBridgeStateSyncTx() error = %v", err)
	}
	if result.Code != commonpb.TxStatusCode_SUCCESS {
		t.Fatalf("result code = %s", result.Code.String())
	}
	if string(state["token_business/alice"]) != "99" ||
		string(state[bridgeSyncMetadataContract+"/__bridge_state_sync:business-shard1:15:0:1"]) != "block-hash-15" {
		t.Fatal("business value or checkpoint missing")
	}
	info, err := ParseBridgeStateSyncInfo(tx)
	if err != nil {
		t.Fatal(err)
	}
	if info.SourceShard != "business-shard1" || info.StartHeight != 15 || info.EndHeight != 15 || !info.Checkpoint {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestApplyBridgeStateSyncTxRejectsUnknownShard(t *testing.T) {
	oldCfg := crossCfg
	crossCfg = &shardingsync.ShardingConfig{
		ShardType: "BRIDGE",
		Schedule: shardingsync.ScheduleConfig{
			BusinessShards: []string{"business-shard1"},
		},
	}
	defer func() { crossCfg = oldCfg }()

	tx := bridgeStateSyncTestTx([]byte("[]"))
	for _, param := range tx.Payload.Parameters {
		if param.Key == SourceShardParamKey {
			param.Value = []byte("business-shard2")
		}
	}
	ctrl := gomock.NewController(t)
	ctx := protocolmock.NewMockTxSimContext(ctrl)
	if _, err := ApplyBridgeStateSyncTx(tx, ctx); err == nil {
		t.Fatal("expected unknown source shard error")
	}
}

func TestParseBridgeStateSyncInfoWithWavePosition(t *testing.T) {
	tx := bridgeStateSyncTestTx([]byte("[]"))
	tx.Payload.Parameters = append(tx.Payload.Parameters,
		&commonpb.KeyValuePair{Key: SyncCycleParamKey, Value: []byte("7")},
		&commonpb.KeyValuePair{Key: SyncSlotParamKey, Value: []byte("3")},
		&commonpb.KeyValuePair{Key: SyncSlotTotalParamKey, Value: []byte("5")},
	)

	info, err := ParseBridgeStateSyncInfo(tx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Cycle != 7 || info.Slot != 3 || info.SlotTotal != 5 {
		t.Fatalf("wave position = %d/%d/%d, want 7/3/5", info.Cycle, info.Slot, info.SlotTotal)
	}
}

func TestParseBridgeStateSyncInfoRejectsPartialWavePosition(t *testing.T) {
	tx := bridgeStateSyncTestTx([]byte("[]"))
	tx.Payload.Parameters = append(tx.Payload.Parameters,
		&commonpb.KeyValuePair{Key: SyncSlotParamKey, Value: []byte("1")},
		&commonpb.KeyValuePair{Key: SyncSlotTotalParamKey, Value: []byte("5")},
	)

	if _, err := ParseBridgeStateSyncInfo(tx); err == nil {
		t.Fatal("partial wave position was accepted")
	}
}

func TestApplyStateSyncBatchTx(t *testing.T) {
	oldCfg := crossCfg
	crossCfg = &shardingsync.ShardingConfig{
		ShardId:   "business-shard1",
		ShardType: "BUSINESS",
	}
	defer func() { crossCfg = oldCfg }()

	writesRaw, err := json.Marshal([]BridgeStateWrite{
		{ContractName: "standard_dfa_experiment", Key: []byte("b#alice"), Value: []byte("10")},
		{ContractName: "standard_dfa_experiment", Key: []byte("b#bob"), Value: []byte("20")},
	})
	if err != nil {
		t.Fatal(err)
	}
	tx := &commonpb.Transaction{Payload: &commonpb.Payload{
		TxType:       commonpb.TxType_INVOKE_CONTRACT,
		ContractName: "standard_dfa_experiment",
		Method:       SystemStateSyncApplyBatchMethod,
		Parameters: []*commonpb.KeyValuePair{
			{Key: TargetContractParamKey, Value: []byte("standard_dfa_experiment")},
			{Key: TargetShardParamKey, Value: []byte("business-shard1")},
			{Key: SourceShardParamKey, Value: []byte("bridge")},
			{Key: SourceHeightParamKey, Value: []byte("15")},
			{Key: SourceBlockHashParamKey, Value: []byte("hash-15")},
			{Key: StateWritesParamKey, Value: writesRaw},
		},
	}}

	authorizeTestTx(t, tx, "bridge", "business-shard1")
	ctx, state := stateSyncMock(t)
	result, err := ApplyStateSyncTx(tx, ctx)
	if err != nil {
		t.Fatalf("ApplyStateSyncTx() error = %v", err)
	}
	if result.Code != commonpb.TxStatusCode_SUCCESS {
		t.Fatalf("result code = %s", result.Code.String())
	}
	if string(state["standard_dfa_experiment/b#alice"]) != "10" || string(state["standard_dfa_experiment/b#bob"]) != "20" {
		t.Fatal("batch values missing")
	}
}

func TestBuildApplyWriteChunksCanonicalAndChunked(t *testing.T) {
	oldCfg := crossCfg
	crossCfg = &shardingsync.ShardingConfig{
		Schedule: shardingsync.ScheduleConfig{StateSyncMaxWritesPerTx: 2},
	}
	defer func() { crossCfg = oldCfg }()

	batch := &CrossWriteBatch{
		BatchId:           "bridge-20-business-shard1",
		TargetShard:       "business-shard1",
		BridgeBlockHeight: 20,
		BridgeBlockHash:   "block-20",
		Timestamp:         100,
		Writes: []CrossWrite{
			{ContractName: "token", Key: []byte("c"), Value: []byte("3")},
			{ContractName: "token", Key: []byte("a"), Value: []byte("1")},
			{ContractName: "token", Key: []byte("b"), Value: []byte("2")},
		},
	}
	first, err := buildApplyWriteChunks(batch)
	if err != nil {
		t.Fatal(err)
	}
	batch.Writes[0], batch.Writes[2] = batch.Writes[2], batch.Writes[0]
	second, err := buildApplyWriteChunks(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("chunk counts = %d/%d, want 2/2", len(first), len(second))
	}
	for i := range first {
		if first[i].txId != second[i].txId {
			t.Fatalf("chunk %d tx id is not canonical: %s != %s", i, first[i].txId, second[i].txId)
		}
	}
	if string(first[0].writes[0].Key) != "a" || string(first[0].writes[1].Key) != "b" ||
		string(first[1].writes[0].Key) != "c" {
		t.Fatalf("unexpected canonical chunk order: %+v", first)
	}
}

func bridgeStateSyncTestTx(writesRaw []byte) *commonpb.Transaction {
	return &commonpb.Transaction{Payload: &commonpb.Payload{
		TxType:       commonpb.TxType_INVOKE_CONTRACT,
		ContractName: "sharding_heartbeat",
		Method:       SystemBridgeStateSyncApplyMethod,
		Parameters: []*commonpb.KeyValuePair{
			{Key: SourceShardParamKey, Value: []byte("business-shard1")},
			{Key: SourceStartHeightParamKey, Value: []byte("15")},
			{Key: SourceHeightParamKey, Value: []byte("15")},
			{Key: SourceBlockHashParamKey, Value: []byte("block-hash-15")},
			{Key: SyncCheckpointParamKey, Value: []byte("true")},
			{Key: SyncChunkIndexParamKey, Value: []byte("0")},
			{Key: SyncChunkTotalParamKey, Value: []byte("1")},
			{Key: StateWritesParamKey, Value: writesRaw},
		},
	}}
}

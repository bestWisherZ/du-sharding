package crossshard

import (
	"encoding/json"
	"strings"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	protocolmock "chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

func authorizeTestTx(t *testing.T, tx *commonpb.Transaction, source, target string) {
	t.Helper()
	id := testidentity.New(t)
	crossCfg.SyncAuth = id.Config()
	if len(crossCfg.Schedule.BusinessShards) == 0 {
		crossCfg.Schedule.BusinessShards = []string{"business-shard1"}
	}
	crossCfg.Schedule.CrossWriteRouting.Strategy = "key_hash"
	tx.Payload.ChainId = crossCfg.SyncAuth.ChainIDs[syncauth.NormalizeShard(target)]
	auth, err := syncauth.Sign(crossCfg.SyncAuth, source, target, tx.Payload, id.Certificate, id.Key)
	if err != nil {
		t.Fatal(err)
	}
	tx.Payload.Parameters = append(tx.Payload.Parameters, &commonpb.KeyValuePair{Key: syncauth.Parameter, Value: auth})
}

func stateSyncMock(t *testing.T) (*protocolmock.MockTxSimContext, map[string][]byte) {
	t.Helper()
	ctx := protocolmock.NewMockTxSimContext(gomock.NewController(t))
	state := make(map[string][]byte)
	ctx.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(c string, k, v []byte) error { state[c+"/"+string(k)] = v; return nil }).AnyTimes()
	return ctx, state
}

func stateSyncMockNoWrites(t *testing.T) *protocolmock.MockTxSimContext {
	t.Helper()
	return protocolmock.NewMockTxSimContext(gomock.NewController(t))
}

func TestWritebackChunksDoNotAliasSourceOrProof(t *testing.T) {
	batch := &CrossWriteBatch{BatchId: "b", TargetShard: "business-shard1", BridgeBlockHeight: 12, BridgeBlockHash: "h12", Writes: []CrossWrite{
		{ContractName: "token", Key: []byte("a"), Value: []byte{}}, {ContractName: "token", Key: []byte("b"), Value: nil}}, Authorizations: map[string][]byte{"proof": []byte("original")}}
	chunks, err := buildApplyWriteChunks(batch)
	if err != nil {
		t.Fatal(err)
	}
	batch.Authorizations["proof"][0] = 'X'
	batch.Writes[0].Key[0] = 'X'
	if string(chunks[0].batch.Authorizations["proof"]) != "original" || string(chunks[0].writes[0].Key) != "a" || chunks[0].writes[0].Value == nil || chunks[0].writes[1].Value != nil {
		t.Fatal("chunk aliases source data or changed empty/deletion semantics")
	}
}

func TestUnauthorizedStateSyncCannotWrite(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS"}
	ctx := protocolmock.NewMockTxSimContext(gomock.NewController(t))
	tx := &commonpb.Transaction{Payload: &commonpb.Payload{TxType: commonpb.TxType_INVOKE_CONTRACT, ContractName: "token", Method: SystemStateSyncApplyBatchMethod}}
	if _, err := ApplyStateSyncTx(tx, ctx); err == nil {
		t.Fatal("unauthorized write accepted")
	}
}

func TestFailedWritebackIsNotCompleted(t *testing.T) {
	pendingApply.Lock()
	old := pendingApply.tasks
	pendingApply.tasks = map[string]*pendingApplyWrite{"test": {txId: "failed"}}
	pendingApply.Unlock()
	t.Cleanup(func() { pendingApply.Lock(); pendingApply.tasks = old; pendingApply.Unlock() })
	completePendingApplyWrites(&commonpb.Block{Txs: []*commonpb.Transaction{{Payload: &commonpb.Payload{TxId: "failed", Method: SystemStateSyncApplyBatchMethod}, Result: &commonpb.Result{Code: commonpb.TxStatusCode_CONTRACT_FAIL}}}})
	if !hasPendingApplyWrites() {
		t.Fatal("failed writeback was treated as applied")
	}
}

func TestSignedWritebackRoundTrip(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	id := testidentity.New(t)
	crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS", SyncAuth: id.Config(), Schedule: shardingsync.ScheduleConfig{
		BusinessShards: []string{"business-shard1"}, CrossWriteRouting: shardingsync.CrossWriteRoutingConfig{Strategy: "key_hash"}}}
	batch := &CrossWriteBatch{BatchId: "bridge-12-business-shard1", TargetShard: "business-shard1", BridgeBlockHeight: 12, BridgeBlockHash: "h12", Timestamp: 100,
		Writes: []CrossWrite{{ContractName: "token", Key: []byte("a"), Value: []byte("1")}}, Authorizations: map[string][]byte{}}
	chunks, err := buildApplyWriteChunks(batch)
	if err != nil {
		t.Fatal(err)
	}
	p := applyWriteChunkPayload("chain1", chunks[0])
	auth, err := syncauth.Sign(crossCfg.SyncAuth, "bridge", "business-shard1", p, id.Certificate, id.Key)
	if err != nil {
		t.Fatal(err)
	}
	batch.Authorizations[chunks[0].txId] = auth
	msg, err := BuildBatchMessage(batch.TargetShard, batch)
	if err != nil {
		t.Fatal(err)
	}
	_, decoded, ok := ParseBatchMessage(msg)
	if !ok {
		t.Fatal("decode failed")
	}
	chunks, err = buildApplyWriteChunks(decoded)
	if err != nil {
		t.Fatal(err)
	}
	ctx, state := stateSyncMock(t)
	result, err := ApplyStateSyncTx(&commonpb.Transaction{Payload: applyWriteChunkPayload("chain1", chunks[0])}, ctx)
	if err != nil || result.Code != commonpb.TxStatusCode_SUCCESS {
		t.Fatalf("apply: %v %v", result, err)
	}
	if string(state["token/a"]) != "1" {
		t.Fatal("write missing")
	}
	batch.Writes[0].Value = []byte("forged")
	chunks, _ = buildApplyWriteChunks(batch)
	if _, err := ApplyStateSyncTx(&commonpb.Transaction{Payload: applyWriteChunkPayload("chain1", chunks[0])}, ctx); err == nil {
		t.Fatal("tampered write accepted")
	}
}

func TestStateSyncRejectsReservedNamespace(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	names := []string{bridgeSyncMetadataContract}
	for name := range syscontract.SystemContract_value {
		names = append(names, name)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal([]BridgeStateWrite{{ContractName: name, Key: []byte("a"), Value: []byte("1")}})
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{SystemStateSyncApplyMethod, SystemStateSyncApplyBatchMethod} {
				crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS"}
				tx := &commonpb.Transaction{Payload: &commonpb.Payload{TxType: commonpb.TxType_INVOKE_CONTRACT, ContractName: name, Method: method,
					Parameters: []*commonpb.KeyValuePair{{Key: SourceShardParamKey, Value: []byte("bridge")}, {Key: StateWritesParamKey, Value: raw}}}}
				authorizeTestTx(t, tx, "bridge", "business-shard1")
				if _, err := ApplyStateSyncTx(tx, stateSyncMockNoWrites(t)); err == nil || !strings.Contains(err.Error(), "forbidden") {
					t.Fatalf("%s did not reject reserved namespace: %v", method, err)
				}
			}
			crossCfg = &shardingsync.ShardingConfig{ShardId: "bridge", ShardType: "BRIDGE", Schedule: shardingsync.ScheduleConfig{BusinessShards: []string{"business-shard1"}}}
			tx := bridgeStateSyncTestTx(raw)
			authorizeTestTx(t, tx, "business-shard1", "bridge")
			if _, err := ApplyBridgeStateSyncTx(tx, stateSyncMockNoWrites(t)); err == nil || !strings.Contains(err.Error(), "forbidden") {
				t.Fatalf("bridge sync did not reject reserved namespace: %v", err)
			}
		})
	}
}

func TestStateSyncRejectsInvalidDeleteFlag(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS"}
	tx := &commonpb.Transaction{Payload: &commonpb.Payload{TxType: commonpb.TxType_INVOKE_CONTRACT, ContractName: "token", Method: SystemStateSyncApplyMethod,
		Parameters: []*commonpb.KeyValuePair{{Key: StateKeyParamKey, Value: []byte("a")}, {Key: StateValueParamKey, Value: []byte{}}, {Key: StateDeleteParamKey, Value: []byte("invalid")}}}}
	authorizeTestTx(t, tx, "bridge", "business-shard1")
	if _, err := ApplyStateSyncTx(tx, stateSyncMockNoWrites(t)); err == nil || !strings.Contains(err.Error(), "delete flag") {
		t.Fatalf("invalid delete flag was not rejected: %v", err)
	}
}

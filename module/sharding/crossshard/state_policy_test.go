package crossshard

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	common "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/spf13/viper"
)

func TestBusinessStatePolicyRejectsNonlocalAndSharedWrites(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{ShardId: "business-shard1", ShardType: "BUSINESS", Schedule: shardingsync.ScheduleConfig{
		BusinessShards: []string{"business-shard1", "business-shard2"}, CrossWriteRouting: shardingsync.CrossWriteRoutingConfig{Rules: []shardingsync.CrossWriteRoutingRule{
			{ContractName: "token", KeyPrefix: "local", TargetShard: "business-shard1"},
			{ContractName: "token", KeyPrefix: "remote", TargetShard: "business-shard2"},
			{ContractName: "token", KeyPrefix: "shared", TargetShard: "bridge"},
			{ContractName: "token", KeyPrefix: "constant", Immutable: true},
		}}}}
	tx := &common.Transaction{Payload: &common.Payload{ContractName: "token", Method: "Transfer"}}
	check := func(key string, write bool) error {
		rw := &common.TxRWSet{}
		if write {
			rw.TxWrites = []*common.TxWrite{{ContractName: "token", Key: []byte(key)}}
		} else {
			rw.TxReads = []*common.TxRead{{ContractName: "token", Key: []byte(key)}}
		}
		return ValidateBusinessRWSet(tx, rw)
	}
	for _, key := range []string{"remote-balance", "shared-total", "unknown"} {
		if err := check(key, true); err == nil {
			t.Fatalf("nonlocal write accepted: %s", key)
		}
		if err := check(key, false); err == nil {
			t.Fatalf("nonlocal read accepted: %s", key)
		}
	}
	if err := check("local-balance", true); err != nil {
		t.Fatal(err)
	}
	if err := check("constant-name", false); err != nil {
		t.Fatal(err)
	}
	if err := check("constant-name", true); err == nil {
		t.Fatal("replicated constant was mutable")
	}
	crossCfg.ShardType = "BRIDGE"
	crossCfg.ShardId = "bridge"
	if err := check("shared-total", true); err != nil {
		t.Fatal("bridge-owned shared state rejected", err)
	}
}

func TestDefaultExperimentPolicyAllowsBalanceAndInitialValueReads(t *testing.T) {
	v := viper.New()
	v.SetConfigFile(filepath.Join("..", "..", "..", "config", "sharding", "state_routing.yml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	var policy shardingsync.CrossWriteRoutingConfig
	if err := v.UnmarshalKey("cross_write_routing", &policy); err != nil {
		t.Fatal(err)
	}
	shards := []string{"business-shard1", "business-shard2"}
	if err := policy.Validate(shards); err != nil {
		t.Fatal(err)
	}
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	account := "0x0000000000000000000000000000000000000001"
	crossCfg = &shardingsync.ShardingConfig{ShardId: routeByHash([]byte(account), shards), ShardType: "BUSINESS",
		Schedule: shardingsync.ScheduleConfig{BusinessShards: shards, CrossWriteRouting: policy}}
	contract := "standard_dfa_experiment"
	tx := &common.Transaction{Payload: &common.Payload{ContractName: contract, Method: "TransferExperiment"}}
	rw := &common.TxRWSet{
		TxReads: []*common.TxRead{{ContractName: contract, Key: []byte("b#" + account)},
			{ContractName: contract, Key: []byte("experimentInitialBalance")}},
		TxWrites: []*common.TxWrite{{ContractName: contract, Key: []byte("b#" + account), Value: []byte("1")}},
	}
	if err := ValidateBusinessRWSet(tx, rw); err != nil {
		t.Fatal("default policy rejected experiment balance access", err)
	}
	rw.TxWrites = append(rw.TxWrites, &common.TxWrite{ContractName: contract, Key: []byte("experimentInitialBalance"), Value: []byte("2")})
	if err := ValidateBusinessRWSet(tx, rw); err == nil {
		t.Fatal("experiment initial value became writable")
	}
}

func TestStateCheckpointCannotWriteAnotherShardsKeys(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{ShardId: "bridge", ShardType: "BRIDGE", Schedule: shardingsync.ScheduleConfig{
		BusinessShards: []string{"business-shard1", "business-shard2"}, CrossWriteRouting: shardingsync.CrossWriteRoutingConfig{Rules: []shardingsync.CrossWriteRoutingRule{
			{ContractName: "token", TargetShard: "business-shard2"}}}}}
	raw, err := json.Marshal([]BridgeStateWrite{{ContractName: "token", Key: []byte("a"), Value: []byte("forged")}})
	if err != nil {
		t.Fatal(err)
	}
	tx := bridgeStateSyncTestTx(raw)
	authorizeTestTx(t, tx, "business-shard1", "bridge")
	if _, err := ApplyBridgeStateSyncTx(tx, stateSyncMockNoWrites(t)); err == nil {
		t.Fatal("trusted signer was allowed to modify another shard's key")
	}
}

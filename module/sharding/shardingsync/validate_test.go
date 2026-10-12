package shardingsync

import (
	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"testing"
)

func validTestConfig(t *testing.T) *ShardingConfig {
	return &ShardingConfig{ShardId: "bridge-shard", ShardType: "BRIDGE", SyncNetwork: SyncNetworkConfig{Enable: true, Port: 20001},
		SyncAuth: testidentity.New(t).Config(), Schedule: ScheduleConfig{BusinessBatch: 6, BridgeBatch: 4, BusinessShards: []string{"business-shard1", "business-shard2"},
			CrossWriteRouting: CrossWriteRoutingConfig{Strategy: "none", Rules: []CrossWriteRoutingRule{{ContractName: "token", KeyPattern: "^b#(.+)$", Strategy: "key_hash"}}}}}
}

func TestConfigValidation(t *testing.T) {
	if err := validTestConfig(t).Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ShardingConfig){
		"missing trust":      func(c *ShardingConfig) { c.SyncAuth.NetworkID = "" },
		"missing signer":     func(c *ShardingConfig) { delete(c.SyncAuth.Signers, "business-shard2") },
		"duplicate shard":    func(c *ShardingConfig) { c.Schedule.BusinessShards[1] = c.Schedule.BusinessShards[0] },
		"invalid regex":      func(c *ShardingConfig) { c.Schedule.CrossWriteRouting.Rules[0].KeyPattern = "[" },
		"two capture groups": func(c *ShardingConfig) { c.Schedule.CrossWriteRouting.Rules[0].KeyPattern = "^(b)#(.+)$" },
		"typo strategy":      func(c *ShardingConfig) { c.Schedule.CrossWriteRouting.Strategy = "key-hash" },
		"empty state policy": func(c *ShardingConfig) { c.Schedule.CrossWriteRouting.Rules = nil },
		"unknown target": func(c *ShardingConfig) {
			c.Schedule.CrossWriteRouting.Rules[0] = CrossWriteRoutingRule{ContractName: "token", TargetShard: "missing"}
		},
		"wrong role": func(c *ShardingConfig) { c.ShardType = "BUSINESS" },
	} {
		t.Run(name, func(t *testing.T) {
			c := validTestConfig(t)
			mutate(c)
			if err := c.Validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}

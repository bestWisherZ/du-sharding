package crossshard

import (
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func TestRouteWriteTargetIgnoresUntrustedShardParameters(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{Schedule: shardingsync.ScheduleConfig{
		BusinessShards: []string{"business-shard1", "business-shard2"},
		CrossWriteRouting: shardingsync.CrossWriteRoutingConfig{Strategy: "none", Rules: []shardingsync.CrossWriteRoutingRule{
			{ContractName: "standard_dfa_experiment", KeyPattern: "^(?:b#)?([0-9a-f]{40})$", Strategy: "key_hash"},
		}},
	}}
	tx := &commonpb.Transaction{Payload: &commonpb.Payload{Parameters: []*commonpb.KeyValuePair{
		{Key: "from", Value: []byte("1111111111111111111111111111111111111111")},
		{Key: "to", Value: []byte("2222222222222222222222222222222222222222")},
		{Key: "from_shard", Value: []byte("business-3")},
		{Key: "to_shard", Value: []byte("business-9")},
	}}}

	tests := []struct {
		name    string
		key     string
		account string
	}{
		{name: "direct source key", key: "1111111111111111111111111111111111111111", account: "1111111111111111111111111111111111111111"},
		{name: "DFA source balance key", key: "b#1111111111111111111111111111111111111111", account: "1111111111111111111111111111111111111111"},
		{name: "DFA destination balance key", key: "b#2222222222222222222222222222222222222222", account: "2222222222222222222222222222222222222222"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := routeByHash([]byte(tt.account), crossCfg.Schedule.BusinessShards)
			if got := routeWriteTargetForTx(tx, "standard_dfa_experiment", []byte(tt.key)); got != want {
				t.Fatalf("routeWriteTargetForTx() = %q, want %q", got, want)
			}
		})
	}
	if got := routeWriteTargetForTx(tx, "unconfigured", []byte("balance")); got != "" {
		t.Fatalf("unconfigured state was implicitly routed to %s", got)
	}
}

package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	common "chainmaker.org/chainmaker/pb-go/v2/common"
)

const routingTestTopology = `listen_address: "127.0.0.1:14301"
default_backend: business-shard1
bridge_backend: bridge
business_shards: [business-shard1, business-shard2]
backends:
  business-shard1: {address: "127.0.0.1:1", chain_id: chain1}
  business-shard2: {address: "127.0.0.1:2", chain_id: chain2}
  bridge: {address: "127.0.0.1:3", chain_id: chain0}
`

func writeRoutingConfig(t *testing.T, path, rules string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(routingTestTopology+"routing_rules:\n"+rules), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNewContractRoutingConfiguredInYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yml")
	fixed := `  - contract_name: order_book
    method: "*"
    strategy: fixed
    target_backend: business-shard2
`
	wildcard := `  - contract_name: asset_ledger
    method: "*"
    strategy: fixed
    target_backend: bridge
`
	hash := `  - contract_name: asset_ledger
    method: move
    strategy: hash
    account_parameters: [source, destination]
`
	writeRoutingConfig(t, path, fixed)
	original, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	req := &common.TxRequest{Payload: &common.Payload{
		TxType: common.TxType_INVOKE_CONTRACT, ContractName: "asset_ledger", Method: "move",
		Parameters: []*common.KeyValuePair{{Key: "source", Value: []byte("account-0")},
			{Key: "destination", Value: []byte("account-0")}},
	}}
	if _, _, err := ResolveRoute(original, req); err == nil {
		t.Fatal("unconfigured contract accepted")
	}
	target := shardForAccount(original.BusinessShards, "account-0")
	other := ""
	for i := 1; i <= 100; i++ {
		candidate := fmt.Sprintf("account-%d", i)
		if shardForAccount(original.BusinessShards, candidate) != target {
			other = candidate
			break
		}
	}
	if other == "" {
		t.Fatal("no test account found on the other shard")
	}

	for _, rules := range []string{fixed + wildcard + hash, fixed + hash + wildcard} {
		writeRoutingConfig(t, path, rules)
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			contract, method, destination, want string
		}{
			{"asset_ledger", "move", "account-0", target},
			{"asset_ledger", "move", other, "bridge"},
			{"asset_ledger", "other_method", "account-0", "bridge"},
			{"order_book", "place", "account-0", "business-shard2"},
		} {
			req.Payload.ContractName, req.Payload.Method = tc.contract, tc.method
			req.Payload.Parameters[1].Value = []byte(tc.destination)
			backend, chainID, err := ResolveRoute(cfg, req)
			if err != nil || backend != tc.want || chainID != cfg.Backends[tc.want].ChainID {
				t.Fatalf("%s/%s: backend=%s chain=%s err=%v", tc.contract, tc.method, backend, chainID, err)
			}
		}
	}
	req.Payload.ContractName, req.Payload.Method = "asset_ledger", "move"
	if _, _, err := ResolveRoute(original, req); err == nil {
		t.Fatal("config file edit mutated an already loaded configuration")
	}
}

func TestRoutingStrategyConfigurationValidation(t *testing.T) {
	for _, tc := range []struct {
		name, fields, wantError string
	}{
		{"hash", "strategy: hash\n    account_parameters: [owner]", ""},
		{"fixed", "strategy: fixed\n    target_backend: business-shard1", ""},
		{"implicit hash", "account_parameters: [owner]", ""},
		{"implicit fixed", "target_backend: business-shard1", ""},
		{"unknown strategy", "strategy: has\n    account_parameters: [owner]", "unsupported routing strategy"},
		{"unknown field", "stratgey: hash\n    account_parameters: [owner]", "stratgey"},
		{"fixed without target", "strategy: fixed", "fixed route"},
		{"unknown target", "strategy: fixed\n    target_backend: absent", "fixed route"},
		{"fixed with accounts", "strategy: fixed\n    target_backend: business-shard1\n    account_parameters: [owner]", "fixed route"},
		{"hash without inputs", "strategy: hash", "no routing inputs"},
		{"hash with target", "strategy: hash\n    target_backend: bridge\n    account_parameters: [owner]", "cannot set target_backend"},
		{"duplicate parameters", "strategy: hash\n    account_parameters: [owner, owner]", "duplicate or empty account_parameters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "gateway.yml")
			writeRoutingConfig(t, path, "  - contract_name: asset_ledger\n    method: move\n    "+tc.fields+"\n")
			_, err := LoadConfig(path)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("want error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}

func TestRoutingRejectsUnsupportedStrategy(t *testing.T) {
	cfg := routeTestConfig()
	cfg.RoutingRules[1].Strategy = "unsupported"
	req := &common.TxRequest{Payload: &common.Payload{
		TxType: common.TxType_INVOKE_CONTRACT, ContractName: "custom", Method: "move",
		Parameters: []*common.KeyValuePair{{Key: "src", Value: []byte("a")}, {Key: "dst", Value: []byte("a")}},
	}}
	if _, _, err := ResolveRoute(cfg, req); err == nil {
		t.Fatal("unsupported strategy silently used hash routing")
	}
}

func TestGatewayExampleConfiguration(t *testing.T) {
	cfg, err := LoadConfig("gateway.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RoutingRules) != 7 || cfg.RoutingRules[0].Strategy != routingStrategyHash {
		t.Fatal("example routing configuration was not loaded")
	}
}

func TestRuntimeGatewayConfiguration(t *testing.T) {
	cfg, err := LoadConfig("gateway.yml")
	if err != nil {
		t.Fatal(err)
	}
	req := &common.TxRequest{Payload: &common.Payload{
		TxType: common.TxType_INVOKE_CONTRACT, ContractName: "standard_dfa_experiment", Method: "TransferExperiment",
		Parameters: []*common.KeyValuePair{{Key: "from", Value: []byte("test-account")},
			{Key: "to", Value: []byte("test-account")}},
	}}
	want := shardForAccount(cfg.BusinessShards, "test-account")
	backend, chain, err := ResolveRoute(cfg, req)
	if err != nil || backend != want || chain != cfg.Backends[want].ChainID {
		t.Fatalf("runtime config route: backend=%s chain=%s err=%v", backend, chain, err)
	}
}

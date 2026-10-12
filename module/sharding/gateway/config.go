package gateway

import (
	"fmt"
	"path/filepath"
	"strings"

	"chainmaker.org/chainmaker/common/v2/crypto"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"github.com/spf13/viper"
)

type TLSConfig struct {
	Enable             bool   `mapstructure:"enable"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	CaFile             string `mapstructure:"ca_file"`
	ServerName         string `mapstructure:"server_name"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

type BackendConfig struct {
	Address string    `mapstructure:"address"`
	ChainID string    `mapstructure:"chain_id"`
	TLS     TLSConfig `mapstructure:"tls"`
}

const (
	routingStrategyHash  = "hash"
	routingStrategyFixed = "fixed"
)

// RoutingRule names the routing inputs; business parameters remain untouched.
type RoutingRule struct {
	ContractName      string   `mapstructure:"contract_name"`
	Method            string   `mapstructure:"method"`
	Strategy          string   `mapstructure:"strategy"`
	TargetBackend     string   `mapstructure:"target_backend"`
	AccountParameters []string `mapstructure:"account_parameters"`
	IncludeSender     bool     `mapstructure:"include_sender"`
}

func (r RoutingRule) effectiveStrategy() string {
	if r.Strategy != "" {
		return r.Strategy
	}
	if r.TargetBackend != "" {
		return routingStrategyFixed
	}
	return routingStrategyHash
}

type GatewayConfig struct {
	ListenAddress  string                    `mapstructure:"listen_address"`
	ListenTLS      TLSConfig                 `mapstructure:"listen_tls"`
	DefaultBackend string                    `mapstructure:"default_backend"`
	BridgeBackend  string                    `mapstructure:"bridge_backend"`
	BusinessShards []string                  `mapstructure:"business_shards"`
	AddressType    string                    `mapstructure:"address_type"`
	HashType       string                    `mapstructure:"hash_type"`
	RoutingRules   []RoutingRule             `mapstructure:"routing_rules"`
	Backends       map[string]*BackendConfig `mapstructure:"backends"`
}

func LoadConfig(path string) (*GatewayConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	if v.IsSet("user_config_path") || v.IsSet("shard_credentials") {
		return nil, fmt.Errorf("SDK proxy credentials are no longer supported; configure routing_rules and forward client-signed requests")
	}
	cfg := &GatewayConfig{}
	if err := v.UnmarshalExact(cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	base := filepath.Dir(path)
	resolveTLSPaths(base, &cfg.ListenTLS)
	for _, backend := range cfg.Backends {
		resolveTLSPaths(base, &backend.TLS)
	}
	return cfg, nil
}

func (c *GatewayConfig) validate() error {
	if c == nil || c.ListenAddress == "" || len(c.Backends) == 0 {
		return fmt.Errorf("listen_address and backends are required")
	}
	chainIDs := make(map[string]bool)
	for name, b := range c.Backends {
		if name == "" || b == nil || b.Address == "" || b.ChainID == "" || chainIDs[b.ChainID] {
			return fmt.Errorf("backend %q requires an address and a unique chain_id", name)
		}
		chainIDs[b.ChainID] = true
	}
	if c.Backends[c.DefaultBackend] == nil || c.Backends[c.BridgeBackend] == nil {
		return fmt.Errorf("default_backend and bridge_backend must exist")
	}
	seen := make(map[string]bool)
	for _, shard := range c.BusinessShards {
		if c.Backends[shard] == nil || shard == c.BridgeBackend || seen[shard] {
			return fmt.Errorf("invalid or duplicate business shard: %s", shard)
		}
		seen[shard] = true
	}
	seen = make(map[string]bool)
	for _, r := range c.RoutingRules {
		key := r.ContractName + "\x00" + r.Method
		if r.ContractName == "" || r.Method == "" || seen[key] {
			return fmt.Errorf("routing rules require unique contract_name/method pairs")
		}
		seen[key] = true
		parameters := make(map[string]bool)
		for _, name := range r.AccountParameters {
			if name == "" || parameters[name] {
				return fmt.Errorf("duplicate or empty account_parameters in %s/%s", r.ContractName, r.Method)
			}
			parameters[name] = true
		}
		if r.IncludeSender {
			if _, ok := configpb.AddrType_value[strings.ToUpper(c.AddressType)]; !ok {
				return fmt.Errorf("include_sender requires a valid address_type")
			}
			if _, ok := crypto.HashAlgoMap[strings.ToUpper(c.HashType)]; !ok {
				return fmt.Errorf("include_sender requires a valid hash_type")
			}
		}
		switch r.effectiveStrategy() {
		case routingStrategyFixed:
			if c.Backends[r.TargetBackend] == nil || r.IncludeSender || len(r.AccountParameters) != 0 {
				return fmt.Errorf("fixed route %s/%s must name only target_backend", r.ContractName, r.Method)
			}
		case routingStrategyHash:
			if r.TargetBackend != "" {
				return fmt.Errorf("hash route %s/%s cannot set target_backend", r.ContractName, r.Method)
			}
			if len(c.BusinessShards) == 0 || (!r.IncludeSender && len(r.AccountParameters) == 0) {
				return fmt.Errorf("partitioned route %s/%s has no routing inputs", r.ContractName, r.Method)
			}
		default:
			return fmt.Errorf("unsupported routing strategy %q for %s/%s", r.Strategy, r.ContractName, r.Method)
		}
	}
	return nil
}

func resolveTLSPaths(base string, tls *TLSConfig) {
	for _, value := range []*string{&tls.CertFile, &tls.KeyFile, &tls.CaFile} {
		if *value != "" && !filepath.IsAbs(*value) {
			*value = filepath.Join(base, *value)
		}
	}
}

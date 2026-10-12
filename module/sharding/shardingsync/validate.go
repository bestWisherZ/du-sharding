package shardingsync

import (
	"fmt"
	"regexp"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
)

// Validate rejects ambiguous ownership and incomplete trust configuration.
// These execution rules must be identical on every validator in a deployment.
func (c *ShardingConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil sharding config")
	}
	role := strings.ToUpper(c.ShardType)
	if role != "BUSINESS" && role != "BRIDGE" {
		return fmt.Errorf("invalid shard_type: %s", c.ShardType)
	}
	local := syncauth.NormalizeShard(c.ShardId)
	shards := map[string]bool{"bridge": true}
	if len(c.Schedule.BusinessShards) == 0 {
		return fmt.Errorf("schedule.business_shards is required")
	}
	for _, s := range c.Schedule.BusinessShards {
		name := syncauth.NormalizeShard(s)
		if name == "" || shards[name] {
			return fmt.Errorf("empty or duplicate business shard: %s", s)
		}
		shards[name] = true
	}
	if !shards[local] || ((role == "BRIDGE") != (local == "bridge")) {
		return fmt.Errorf("shard_id and shard_type do not match the topology")
	}
	if c.Schedule.BusinessBatch <= 0 || c.Schedule.BridgeBatch <= 0 || c.Schedule.TotalBatch < 0 {
		return fmt.Errorf("schedule requires positive business_batch/bridge_batch and nonnegative total_batch")
	}
	if !c.SyncNetwork.Enable || c.SyncNetwork.Port <= 0 || c.SyncNetwork.Port > 65535 {
		return fmt.Errorf("sharding requires an enabled sync network and a valid port")
	}
	if err := c.SyncAuth.Validate(append(append([]string{}, c.Schedule.BusinessShards...), "bridge")); err != nil {
		return err
	}
	return c.Schedule.CrossWriteRouting.Validate(c.Schedule.BusinessShards)
}

// Validate checks state ownership without requiring generated node identities.
func (routing CrossWriteRoutingConfig) Validate(businessShards []string) error {
	shards := map[string]bool{"bridge": true}
	if len(businessShards) == 0 {
		return fmt.Errorf("business shards are required to validate state routing")
	}
	for _, shard := range businessShards {
		name := syncauth.NormalizeShard(shard)
		if name == "" || shards[name] {
			return fmt.Errorf("empty or duplicate business shard: %s", shard)
		}
		shards[name] = true
	}
	if len(routing.Rules) == 0 && (routing.Strategy == "" || routing.Strategy == "none") {
		return fmt.Errorf("cross_write_routing has no state ownership rules; configure a state policy before preparing or starting shards")
	}
	validStrategy := func(s string) bool {
		switch s {
		case "", "none", "key_hash", "contract_key_hash", "first_business":
			return true
		}
		return false
	}
	if !validStrategy(routing.Strategy) {
		return fmt.Errorf("unknown cross_write_routing.strategy: %s", routing.Strategy)
	}
	seen := make(map[string]bool)
	for i, r := range routing.Rules {
		label := fmt.Sprintf("cross_write_routing.rules[%d]", i)
		if r.ContractName == "" || strings.TrimSpace(r.ContractName) != r.ContractName {
			return fmt.Errorf("%s requires contract_name; wildcard ownership is not supported", label)
		}
		identity := r.ContractName + "\x00" + r.KeyPrefix + "\x00" + r.KeyPattern
		if seen[identity] {
			return fmt.Errorf("%s duplicates an earlier state selector", label)
		}
		seen[identity] = true
		if !validStrategy(r.Strategy) {
			return fmt.Errorf("%s has unknown strategy", label)
		}
		if r.TargetShard != "" && (r.Strategy != "" || !shards[syncauth.NormalizeShard(r.TargetShard)]) {
			return fmt.Errorf("%s must select a known target_shard or a strategy, not both", label)
		}
		if r.KeyPattern != "" {
			p, err := regexp.Compile(r.KeyPattern)
			if err != nil {
				return fmt.Errorf("%s invalid key_pattern: %w", label, err)
			}
			if r.TargetShard == "" && (r.Strategy == "key_hash" || r.Strategy == "contract_key_hash") && p.NumSubexp() != 1 {
				return fmt.Errorf("%s hashed key_pattern needs exactly one capture group", label)
			}
			if r.TargetShard == "" && r.Strategy == "" {
				return fmt.Errorf("%s key_pattern requires an explicit strategy or target_shard", label)
			}
		}
		if r.TargetShard == "" && (r.Strategy == "" || r.Strategy == "none") && !r.Immutable {
			return fmt.Errorf("%s needs explicit state ownership", label)
		}
	}
	return nil
}

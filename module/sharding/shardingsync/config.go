package shardingsync

import "chainmaker.org/chainmaker-go/module/sharding/syncauth"

// ShardingConfig 对应 sharding.yml 的配置结构
type ShardingConfig struct {
	ShardId     string            `mapstructure:"shard_id"`
	ShardType   string            `mapstructure:"shard_type"`
	Schedule    ScheduleConfig    `mapstructure:"schedule"`
	SyncNetwork SyncNetworkConfig `mapstructure:"sync_network"`
	Heartbeat   HeartbeatConfig   `mapstructure:"heartbeat"`
	SyncAuth    syncauth.Config   `mapstructure:"sync_auth"`
}

// HeartbeatConfig 心跳交易配置
type HeartbeatConfig struct {
	// IdleThresholdMs 交易池持续为空多少毫秒后注入心跳交易；优先级高于 IdleThresholdSeconds
	IdleThresholdMs int `mapstructure:"idle_threshold_ms"`
	// IdleThresholdSeconds 兼容旧配置：交易池持续为空多少秒后注入心跳交易
	IdleThresholdSeconds int `mapstructure:"idle_threshold_seconds"`
	// ContractName 心跳合约名，默认 "sharding_heartbeat"
	ContractName string `mapstructure:"contract_name"`
	// Method 心跳方法名，默认 "ping"
	Method string `mapstructure:"method"`
}

type ScheduleConfig struct {
	StartHeight             uint64                  `mapstructure:"start_height"`
	BusinessBatch           int                     `mapstructure:"business_batch"`
	BridgeBatch             int                     `mapstructure:"bridge_batch"`
	BusinessShards          []string                `mapstructure:"business_shards"`
	TotalBatch              int                     `mapstructure:"total_batch"` // 动态调度总批次 N+M；0 表示禁用动态调度
	StateSyncMaxWritesPerTx int                     `mapstructure:"state_sync_max_writes_per_tx"`
	StateSyncFlushWrites    int                     `mapstructure:"state_sync_flush_writes"`
	StateSyncFlushBytes     int                     `mapstructure:"state_sync_flush_bytes"`
	CrossWriteRouting       CrossWriteRoutingConfig `mapstructure:"cross_write_routing"`
}

type CrossWriteRoutingConfig struct {
	// Strategy controls the default target-shard routing for bridge-generated writes.
	// Supported values: key_hash, contract_key_hash, first_business, none.
	Strategy string                  `mapstructure:"strategy"`
	Rules    []CrossWriteRoutingRule `mapstructure:"rules"`
}

type CrossWriteRoutingRule struct {
	ContractName string `mapstructure:"contract_name"`
	KeyPrefix    string `mapstructure:"key_prefix"`
	TargetShard  string `mapstructure:"target_shard"`
	Strategy     string `mapstructure:"strategy"`
	KeyPattern   string `mapstructure:"key_pattern"`
	Immutable    bool   `mapstructure:"immutable"`
}

type SyncNetworkConfig struct {
	Enable bool     `mapstructure:"enable"`
	Port   int      `mapstructure:"port"`
	Seeds  []string `mapstructure:"seeds"`
}

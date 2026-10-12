package scheduler

// Config 定义分时调度模块的基础配置。
// 目前仅支持业务分片与桥接分片双向轮换：
// - 业务分片连续出 businessBatch 个块
// - 桥接分片连续出 bridgeBatch 个块
// - 从 startHeight 之后开始分时调度
type Config struct {
	Enable         bool
	Role           string   // "BUSINESS" 或 "BRIDGE"
	ShardId        string   // 分片ID，例如 "business-shard1"
	StartHeight    uint64   // 分时模式的起始高度（不含），默认 10 表示高度 > 10 时启用分时共识
	BusinessBatch  int      // 业务分片连续出块数量，默认 3
	BridgeBatch    int      // 桥接分片连续出块数量，默认 2
	BusinessShards []string // 业务分片列表（桥接分片需等待全部完成信号）
	TotalBatch     int      // 动态调度总批次数 N+M，默认 6；为 0 时禁用动态调度
	GetPendingFn   func() int // 获取本地 txpool 待处理交易数的回调（业务/桥接均注入）
}

// fillDefaults 填充缺省值。
func (c *Config) fillDefaults() {
	if c.StartHeight == 0 {
		c.StartHeight = 10
	}
	if c.BusinessBatch == 0 {
		c.BusinessBatch = 3
	}
	if c.BridgeBatch == 0 {
		c.BridgeBatch = 2
	}
	// TotalBatch 为 0 时表示禁用动态调度，不填充默认值
}

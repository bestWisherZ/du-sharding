package globalstate

import (
	"strings"
	"sync"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/logger/v2"
)

var (
	once        sync.Once
	globalCfg   *shardingsync.ShardingConfig
	globalLog   = logger.GetLogger(logger.MODULE_CONSENSUS)
	submitTx    func(chainId string, tx interface{}) error
	txInPool    func(chainId, txId string) (bool, error)
	txCommitted func(chainId, txId string) (bool, error)
)

func InitWithConfig(cfg *shardingsync.ShardingConfig) {
	if cfg == nil {
		return
	}
	globalCfg = cfg
	once.Do(func() {
		if strings.EqualFold(cfg.ShardType, "BRIDGE") {
			startBridgePreparation()
		}
		shardingsync.RegisterMessageHandler(handleMessage)
		syncproposer.RegisterListener(onProposerStatusChanged)
		syncproposer.RegisterFlushListener(onForceFlushChanged)
		syncproposer.RegisterPendingChecker(bridgePendingReady)
		globalLog.Infof("[GlobalState] 全局状态同步已启动 | shard=%s type=%s",
			cfg.ShardId, cfg.ShardType)
	})
}

// SetTxSubmitter injects the local transaction submit function. Bridge nodes
// use it to submit deterministic system state-sync transactions.
func SetTxSubmitter(fn func(chainId string, tx interface{}) error) {
	submitTx = fn
}

// SetTxPoolChecker avoids rebuilding a sync transaction already propagated by
// a previous proposer.
func SetTxPoolChecker(fn func(chainId, txId string) (bool, error)) {
	txInPool = fn
}

// SetTxCommittedChecker detects late sync messages whose deterministic tx id
// has already been committed.
func SetTxCommittedChecker(fn func(chainId, txId string) (bool, error)) {
	txCommitted = fn
}

func handleMessage(msg string) {
	if strings.HasPrefix(msg, compactDeltaBinaryPrefix) && receiveBinaryCompactDelta(msg) {
		return
	}
	if shardId, delta, ok := ParseCompactDeltaMessage(msg); ok {
		handleCompactDelta(shardId, delta)
		return
	}
	shardId, bw, ok := ParseDeltaMessage(msg)
	if !ok {
		prefix := msg
		if idx := strings.Index(msg, ":"); idx >= 0 {
			prefix = msg[:idx]
		}
		if prefix != strings.TrimSuffix(deltaPrefix, ":") && prefix != strings.TrimSuffix(compactDeltaPrefix, ":") &&
			prefix != strings.TrimSuffix(compactDeltaBinaryPrefix, ":") {
			return
		}
		parsedShard := ""
		payload := strings.TrimPrefix(msg, prefix+":")
		if parts := strings.SplitN(payload, ":", 2); len(parts) > 0 {
			parsedShard = parts[0]
		}
		globalLog.Warnf("[GlobalState] 解析状态增量失败 | prefix=%s shard=%s len=%d",
			prefix, parsedShard, len(msg))
		return
	}
	handleBlockDelta(shardId, bw)
}

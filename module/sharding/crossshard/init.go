package crossshard

import (
	"sync"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/logger/v2"
)

var (
	once        sync.Once
	crossCfg    *shardingsync.ShardingConfig
	crossLog    = logger.GetLogger(logger.MODULE_CONSENSUS)
	submitTx    func(chainId string, tx interface{}) error
	txInPool    func(chainId, txId string) (bool, error)
	txCommitted func(chainId, txId string) (bool, error)
)

func InitWithConfig(cfg *shardingsync.ShardingConfig) {
	if cfg == nil {
		return
	}
	once.Do(func() {
		crossCfg = cfg
		if cfg.SyncNetwork.Enable {
			shardingsync.RegisterMessageHandler(handleMessage)
		}
		syncproposer.RegisterListener(onApplyProposerStatusChanged)
		syncproposer.RegisterPendingChecker(applyPendingReady)
		crossLog.Infof("[CrossShard] 回写模块已启动 | shard=%s type=%s", cfg.ShardId, cfg.ShardType)
	})
}

// SetTxSubmitter injects the tx submit function to avoid blockchain import cycles.
func SetTxSubmitter(fn func(chainId string, tx interface{}) error) {
	submitTx = fn
}

// SetTxPoolChecker avoids rebuilding a writeback transaction already
// propagated by a previous proposer.
func SetTxPoolChecker(fn func(chainId, txId string) (bool, error)) {
	txInPool = fn
}

// SetTxCommittedChecker detects late writeback messages whose deterministic
// tx id has already been committed.
func SetTxCommittedChecker(fn func(chainId, txId string) (bool, error)) {
	txCommitted = fn
}

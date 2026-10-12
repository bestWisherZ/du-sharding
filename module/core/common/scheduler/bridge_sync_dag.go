package scheduler

import (
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
)

type disjointWriteDAGBuilder interface {
	TryBuildDisjointWriteDAG([]*commonPb.TxRWSet, []*commonPb.Transaction) (*commonPb.DAG, int, bool)
}

// buildDAG is shared by proposal creation and proposal verification.
func (ts *TxScheduler) buildDAG(snapshot protocol.Snapshot, rwsets []*commonPb.TxRWSet,
	txs []*commonPb.Transaction) *commonPb.DAG {
	isSQL := ts.chainConf.ChainConfig().Contract.EnableSqlSupport
	if isSQL || !crossshard.IsBridgeStateSyncBatch(txs) {
		return snapshot.BuildDAG(isSQL, rwsets)
	}
	builder, supported := snapshot.(disjointWriteDAGBuilder)
	if !supported {
		return snapshot.BuildDAG(isSQL, rwsets)
	}
	started := time.Now()
	stage := "verify"
	if rwsets == nil {
		stage = "propose"
	}
	dag, writes, ok := builder.TryBuildDisjointWriteDAG(rwsets, txs)
	if ok {
		ts.log.Infof("SYNC_DAG stage=%s path=disjoint height=%d txs=%d writes=%d elapsed_ms=%.3f",
			stage, snapshot.GetBlockHeight(), len(dag.Vertexes), writes, float64(time.Since(started).Microseconds())/1000)
		return dag
	}
	ts.log.Infof("SYNC_DAG stage=%s path=generic height=%d txs=%d writes=%d check_ms=%.3f",
		stage, snapshot.GetBlockHeight(), len(txs), writes, float64(time.Since(started).Microseconds())/1000)
	return snapshot.BuildDAG(isSQL, rwsets)
}

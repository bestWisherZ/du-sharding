package snapshot

import commonPb "chainmaker.org/chainmaker/pb-go/v2/common"

// TryBuildDisjointWriteDAG keeps the same snapshot read lock as BuildDAG.
// Seal alone does not wait for an in-flight ApplyTxSimContext to release it.
func (s *SnapshotImpl) TryBuildDisjointWriteDAG(rwsets []*commonPb.TxRWSet,
	txs []*commonPb.Transaction) (*commonPb.DAG, int, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if s.sealed == nil || !s.IsSealed() {
		return nil, 0, false
	}
	if rwsets == nil {
		rwsets, txs = s.txRWSetTable, s.txTable
	}
	if len(txs) == 0 || len(txs) != len(rwsets) {
		return nil, 0, false
	}
	writes := 0
	for i, tx := range txs {
		if tx == nil || tx.Payload == nil || rwsets[i] == nil || tx.Payload.TxId == "" ||
			tx.Payload.TxId != rwsets[i].TxId {
			return nil, 0, false
		}
		writes += len(rwsets[i].TxWrites)
	}
	dag, ok := BuildDisjointWriteDAG(rwsets)
	return dag, writes, ok
}

// BuildDisjointWriteDAG avoids the general dependency indexes only after
// checking every key. Callers must independently restrict the eligible blocks.
func BuildDisjointWriteDAG(rwsets []*commonPb.TxRWSet) (*commonPb.DAG, bool) {
	if len(rwsets) == 0 {
		return nil, false
	}
	// Bound the initial allocation for unusually large or overlapping batches.
	const maxInitialKeys = 1 << 20
	capacity := 0
	for _, rwset := range rwsets {
		if rwset == nil {
			return nil, false
		}
		if remaining := maxInitialKeys - capacity; len(rwset.TxWrites) < remaining {
			capacity += len(rwset.TxWrites)
		} else {
			capacity = maxInitialKeys
		}
	}
	seen := make(map[string]int, capacity)
	for index, rwset := range rwsets {
		for _, write := range rwset.TxWrites {
			if write == nil || len(write.Key) == 0 {
				return nil, false
			}
			// Match BuildDAG's existing raw-key identity, including its
			// conservative conflicts between equal keys in different contracts.
			key := string(write.Key)
			if _, exists := seen[key]; exists {
				return nil, false
			}
			seen[key] = index
		}
	}
	for index, rwset := range rwsets {
		for _, read := range rwset.TxReads {
			if read == nil || len(read.Key) == 0 {
				return nil, false
			}
			// Reads of a transaction's own writes are safe; cross-tx
			// read/write dependencies must use the general DAG builder.
			if writer, exists := seen[string(read.Key)]; exists && writer != index {
				return nil, false
			}
		}
	}
	dag := &commonPb.DAG{Vertexes: make([]*commonPb.DAG_Neighbor, len(rwsets))}
	for i := range dag.Vertexes {
		dag.Vertexes[i] = &commonPb.DAG_Neighbor{Neighbors: make([]uint32, 0)}
	}
	return dag, true
}

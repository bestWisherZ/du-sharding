package crossshard

const (
	SourceShardParamKey  = "__source_shard"
	SourceHeightParamKey = "__source_height"
	StateKeyParamKey     = "key"
	StateValueParamKey   = "value"
)

// CrossWrite is a generic state update produced by bridge-shard execution.
// It deliberately carries only storage-level intent, so crossshard does not
// need to understand token transfer, order matching, or any other business rule.
type CrossWrite struct {
	TargetShard       string `json:"target_shard"`
	ContractName      string `json:"contract_name"`
	Key               []byte `json:"key"`
	Value             []byte `json:"value"`
	BridgeTxId        string `json:"bridge_tx_id,omitempty"`
	BridgeBlockHeight uint64 `json:"bridge_block_height,omitempty"`
	BridgeBlockHash   string `json:"bridge_block_hash,omitempty"`
}

// CrossWriteBatch is sent from the bridge shard to one target business shard.
// Writes are already aggregated by target_shard + contract + key.
type CrossWriteBatch struct {
	BatchId           string            `json:"batch_id"`
	TargetShard       string            `json:"target_shard"`
	BridgeBlockHeight uint64            `json:"bridge_block_height"`
	BridgeBlockHash   string            `json:"bridge_block_hash"`
	Timestamp         int64             `json:"timestamp"`
	Writes            []CrossWrite      `json:"writes"`
	Authorizations    map[string][]byte `json:"authorizations,omitempty"`
}

func cloneStateBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

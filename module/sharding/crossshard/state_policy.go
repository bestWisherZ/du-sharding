package crossshard

import (
	"fmt"
	"strings"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

// ValidateBusinessRWSet makes key ownership an execution constraint, not merely
// a gateway hint. Unsupported shared state fails instead of silently diverging.
func ValidateBusinessRWSet(tx *commonpb.Transaction, rw *commonpb.TxRWSet) error {
	if crossCfg == nil || tx == nil || tx.Payload == nil || rw == nil || IsSystemSyncTx(tx) {
		return nil
	}
	if isForbiddenStateSyncContract(tx.Payload.ContractName) {
		return nil
	}
	bridge := strings.EqualFold(crossCfg.ShardType, "BRIDGE")
	local := normalizeShardId(crossCfg.ShardId)
	check := func(contract string, key []byte, write bool) error {
		if contract == bridgeSyncMetadataContract {
			if write && strings.HasPrefix(string(key), "__") {
				return fmt.Errorf("reserved sharding metadata is not writable by contracts")
			}
			return nil
		}
		if isForbiddenStateSyncContract(contract) || contract == "poke_block" {
			return nil
		}
		rule := matchRoutingRule(contract, key, crossCfg.Schedule.CrossWriteRouting.Rules)
		if rule != nil && rule.Immutable {
			if write {
				return fmt.Errorf("immutable replicated key is not writable: %s/%x", contract, key)
			}
			return nil
		}
		owner := routeWriteTarget(contract, key)
		if owner == "" {
			return fmt.Errorf("no state owner for %s/%x", contract, key)
		}
		if !bridge && owner != local {
			return fmt.Errorf("state belongs to %s, not %s: %s/%x", owner, local, contract, key)
		}
		return nil
	}
	for _, read := range rw.TxReads {
		if read != nil {
			if err := check(read.ContractName, read.Key, false); err != nil {
				return err
			}
		}
	}
	for _, write := range rw.TxWrites {
		if write != nil {
			if err := check(write.ContractName, write.Key, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSyncOwnership(source string, writes []BridgeStateWrite) error {
	for _, w := range writes {
		owner := routeWriteTarget(w.ContractName, w.Key)
		if owner == "" {
			return fmt.Errorf("sync key has no configured owner: %s/%x", w.ContractName, w.Key)
		}
		if normalizeShardId(source) == "bridge" {
			if owner != normalizeShardId(crossCfg.ShardId) {
				return fmt.Errorf("writeback key does not belong to target shard: %s/%x", w.ContractName, w.Key)
			}
		} else if owner != normalizeShardId(source) {
			return fmt.Errorf("checkpoint source does not own key: %s/%x", w.ContractName, w.Key)
		}
	}
	return nil
}

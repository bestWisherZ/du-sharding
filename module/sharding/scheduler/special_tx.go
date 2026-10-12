package scheduler

import (
	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

// Enabled is read after sharding initialization, before block proposers start.
func Enabled() bool {
	return global != nil && global.cfg.Enable
}

// MayProduceSpecial reports whether this shard may produce a special sync block
// while it is not the normal time-sharing producer. It intentionally does not
// advance scheduler counters; these blocks are a sync side-channel.
func MayProduceSpecial(height uint64) bool {
	if global == nil || !global.cfg.Enable {
		return false
	}

	global.mu.Lock()
	defer global.mu.Unlock()

	if height <= global.cfg.StartHeight {
		return false
	}
	if !global.active {
		return false
	}
	if global.canProduce && global.phase == global.cfg.Role {
		return false
	}
	return global.cfg.Role == roleBridge || global.cfg.Role == roleBusiness
}

// IsSpecialSyncCandidate recognizes sync payloads before the scheduler is
// initialized. It is only an indexing hint, not permission to enter consensus.
func IsSpecialSyncCandidate(tx *commonpb.Transaction) bool {
	if tx == nil || tx.Payload == nil || tx.Payload.TxType != commonpb.TxType_INVOKE_CONTRACT {
		return false
	}
	if crossshard.IsStateSyncTx(tx) || crossshard.IsBridgeStateSyncTx(tx) {
		return true
	}
	return false
}

// IsSpecialSyncTx applies the local shard-role rule after pool validation.
func IsSpecialSyncTx(tx *commonpb.Transaction) bool {
	if tx == nil || tx.Payload == nil {
		return false
	}
	if tx.Payload.TxType != commonpb.TxType_INVOKE_CONTRACT {
		return false
	}

	role := ""
	if global != nil {
		global.mu.Lock()
		role = global.cfg.Role
		global.mu.Unlock()
	}

	if crossshard.IsStateSyncTx(tx) {
		return role == roleBusiness
	}
	if crossshard.IsBridgeStateSyncTx(tx) {
		return role == roleBridge
	}

	return false
}

// IsSyncOnlyBlock reports whether every transaction in a committed block is a
// state-sync transaction for the local shard. Such blocks do not count toward
// the normal time-sharing block quota.
func IsSyncOnlyBlock(txs []*commonpb.Transaction) bool {
	if len(txs) == 0 {
		return false
	}
	for _, tx := range txs {
		if !IsSpecialSyncTx(tx) {
			return false
		}
	}
	return true
}

// IsSpecialSyncBlock verifies that the block can be produced through the
// non-current-round sync side-channel.
func IsSpecialSyncBlock(height uint64, txs []*commonpb.Transaction) bool {
	if len(txs) == 0 || !MayProduceSpecial(height) {
		return false
	}
	for _, tx := range txs {
		if !IsSpecialSyncTx(tx) {
			return false
		}
	}
	return true
}

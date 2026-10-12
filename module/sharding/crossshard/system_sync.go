package crossshard

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	syscontract "chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/protocol/v2"
)

const (
	// SystemStateSyncApplyMethod is intercepted by the scheduler before VM
	// execution. The payload still names the target business contract for ACL
	// and storage namespace compatibility, but the business contract is not
	// invoked and does not need to implement this method.
	SystemStateSyncApplyMethod = "__system_state_sync_apply"
	// SystemStateSyncApplyBatchMethod applies several writes for one target
	// contract atomically in one system transaction.
	SystemStateSyncApplyBatchMethod = "__system_state_sync_apply_batch"
	// SystemBridgeStateSyncApplyMethod applies business-shard state deltas to
	// the bridge ledger without invoking the mirrored business contract.
	SystemBridgeStateSyncApplyMethod = "__system_bridge_state_sync_apply"

	TargetContractParamKey    = "target_contract"
	TargetShardParamKey       = "target_shard"
	BatchIdParamKey           = "batch_id"
	SourceTxIdParamKey        = "__source_tx_id"
	SourceBlockHashParamKey   = "__source_block_hash"
	SourceStartHeightParamKey = "__source_start_height"
	StateWritesParamKey       = "__state_writes"
	StateDeleteParamKey       = "__state_delete"
	SyncCheckpointParamKey    = "__sync_checkpoint"
	SyncChunkIndexParamKey    = "__sync_chunk_index"
	SyncChunkTotalParamKey    = "__sync_chunk_total"
	SyncCycleParamKey         = "__sync_cycle"
	SyncSlotParamKey          = "__sync_slot"
	SyncSlotTotalParamKey     = "__sync_slot_total"

	bridgeSyncMetadataContract = "sharding_heartbeat"
)

// BridgeStateWrite is one business-contract state value mirrored on bridge.
type BridgeStateWrite struct {
	ContractName string `json:"contract_name"`
	Key          []byte `json:"key"`
	Value        []byte `json:"value"`
	SourceTxId   string `json:"source_tx_id,omitempty"`
}

// BridgeStateSyncInfo identifies one chunk of a business-to-bridge state
// checkpoint. The scheduler uses committed checkpoint chunks as a phase gate.
type BridgeStateSyncInfo struct {
	SourceShard string
	StartHeight uint64
	EndHeight   uint64
	Cycle       uint64
	Slot        int
	SlotTotal   int
	Checkpoint  bool
	ChunkIndex  int
	ChunkTotal  int
}

// IsStateSyncTx reports whether tx is a system-level cross-shard state write.
func IsStateSyncTx(tx *commonpb.Transaction) bool {
	if tx == nil || tx.Payload == nil {
		return false
	}
	if tx.Payload.TxType != commonpb.TxType_INVOKE_CONTRACT {
		return false
	}
	switch strings.TrimSpace(tx.Payload.Method) {
	case SystemStateSyncApplyMethod, SystemStateSyncApplyBatchMethod:
		return true
	default:
		return false
	}
}

// IsBridgeStateSyncTx reports whether tx mirrors business state into the
// bridge ledger.
func IsBridgeStateSyncTx(tx *commonpb.Transaction) bool {
	if tx == nil || tx.Payload == nil || tx.Payload.TxType != commonpb.TxType_INVOKE_CONTRACT {
		return false
	}
	return strings.TrimSpace(tx.Payload.Method) == SystemBridgeStateSyncApplyMethod
}

// IsBridgeStateSyncBatch restricts bridge-only execution optimizations to pure
// business-to-bridge sync batches. Business nodes and mixed blocks stay generic.
func IsBridgeStateSyncBatch(txs []*commonpb.Transaction) bool {
	if crossCfg == nil || !strings.EqualFold(strings.TrimSpace(crossCfg.ShardType), "BRIDGE") || len(txs) == 0 {
		return false
	}
	for _, tx := range txs {
		if !IsBridgeStateSyncTx(tx) {
			return false
		}
	}
	return true
}

// IsSystemSyncTx covers both state-sync directions.
func IsSystemSyncTx(tx *commonpb.Transaction) bool {
	return IsStateSyncTx(tx) || IsBridgeStateSyncTx(tx)
}

func TransactionSucceeded(tx *commonpb.Transaction) bool {
	return tx != nil && tx.Result != nil && tx.Result.Code == commonpb.TxStatusCode_SUCCESS &&
		tx.Result.ContractResult != nil && tx.Result.ContractResult.Code == 0
}

// ApplyStateSyncTx applies a cross-shard write directly to the target contract
// namespace. It creates a normal RWSet through TxSimContext.Put, so consensus
// verification and block commit still validate and persist the write through
// ChainMaker's standard RWSet path.
func ApplyStateSyncTx(tx *commonpb.Transaction, txSimContext protocol.TxSimContext) (*commonpb.Result, error) {
	if !IsStateSyncTx(tx) {
		return stateSyncFailResult("not a state sync tx"), fmt.Errorf("not a state sync tx")
	}
	if txSimContext == nil {
		return stateSyncFailResult("nil tx simulation context"), fmt.Errorf("nil tx simulation context")
	}
	if crossCfg == nil {
		return stateSyncFailResult("sharding config is not initialized"), fmt.Errorf("sharding config is not initialized")
	}
	if err := syncauth.Verify(crossCfg.SyncAuth, "bridge", crossCfg.ShardId, tx.Payload); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if strings.TrimSpace(tx.Payload.Method) == SystemStateSyncApplyBatchMethod {
		return applyStateSyncBatchTx(tx, txSimContext)
	}

	payloadContract := strings.TrimSpace(tx.Payload.ContractName)
	targetContract := strings.TrimSpace(stateSyncParamString(tx, TargetContractParamKey))
	if targetContract != "" && payloadContract != "" && targetContract != payloadContract {
		err := fmt.Errorf("target contract mismatch: payload=%s target=%s", payloadContract, targetContract)
		return stateSyncFailResult(err.Error()), err
	}
	if targetContract == "" {
		targetContract = payloadContract
	}
	key, keyOK := stateSyncParamBytes(tx, StateKeyParamKey)
	value, valueOK := stateSyncParamBytes(tx, StateValueParamKey)
	if targetContract == "" {
		return stateSyncFailResult("target contract is empty"), fmt.Errorf("target contract is empty")
	}
	if isForbiddenStateSyncContract(targetContract) || targetContract == bridgeSyncMetadataContract {
		return stateSyncFailResult("reserved contract state sync is forbidden"), fmt.Errorf("reserved contract state sync is forbidden")
	}
	if !keyOK || len(key) == 0 {
		return stateSyncFailResult("state key is empty"), fmt.Errorf("state key is empty")
	}
	if !valueOK {
		return stateSyncFailResult("state value is missing"), fmt.Errorf("state value is missing")
	}
	// Protobuf bytes alone cannot distinguish an empty value from deletion.
	switch stateSyncParamString(tx, StateDeleteParamKey) {
	case "true":
		value = nil
	case "", "false":
		if value == nil {
			value = []byte{}
		}
	default:
		err := fmt.Errorf("invalid state delete flag")
		return stateSyncFailResult(err.Error()), err
	}
	if err := verifyStateSyncSourceShard(tx); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err := verifyStateSyncTargetShard(tx); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err := validateSyncOwnership("bridge",
		[]BridgeStateWrite{{ContractName: targetContract, Key: key, Value: value}}); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err := txSimContext.Put(targetContract, key, value); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	return &commonpb.Result{
		Code:    commonpb.TxStatusCode_SUCCESS,
		Message: "state sync applied",
		ContractResult: &commonpb.ContractResult{
			Code:    0,
			Message: "state sync applied",
		},
	}, nil
}

func applyStateSyncBatchTx(tx *commonpb.Transaction,
	txSimContext protocol.TxSimContext) (*commonpb.Result, error) {
	payloadContract := strings.TrimSpace(tx.Payload.ContractName)
	targetContract := strings.TrimSpace(stateSyncParamString(tx, TargetContractParamKey))
	if targetContract != "" && payloadContract != "" && targetContract != payloadContract {
		err := fmt.Errorf("target contract mismatch: payload=%s target=%s", payloadContract, targetContract)
		return stateSyncFailResult(err.Error()), err
	}
	if targetContract == "" {
		targetContract = payloadContract
	}
	if targetContract == "" {
		return stateSyncFailResult("target contract is empty"), fmt.Errorf("target contract is empty")
	}
	if isForbiddenStateSyncContract(targetContract) || targetContract == bridgeSyncMetadataContract {
		err := fmt.Errorf("system contract state sync is forbidden: %s", targetContract)
		return stateSyncFailResult(err.Error()), err
	}
	if err := verifyStateSyncSourceShard(tx); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err := verifyStateSyncTargetShard(tx); err != nil {
		return stateSyncFailResult(err.Error()), err
	}

	raw, ok := stateSyncParamBytes(tx, StateWritesParamKey)
	if !ok || len(raw) == 0 {
		return stateSyncFailResult("state writes are missing"), fmt.Errorf("state writes are missing")
	}
	var writes []BridgeStateWrite
	if err := json.Unmarshal(raw, &writes); err != nil {
		return stateSyncFailResult("invalid state writes"), fmt.Errorf("invalid state writes: %v", err)
	}
	if len(writes) == 0 {
		return stateSyncFailResult("state writes are empty"), fmt.Errorf("state writes are empty")
	}
	for i := range writes {
		write := &writes[i]
		contract := strings.TrimSpace(write.ContractName)
		if contract == "" {
			contract = targetContract
		}
		if contract != targetContract {
			err := fmt.Errorf("mixed target contracts in state sync batch: want=%s got=%s",
				targetContract, contract)
			return stateSyncFailResult(err.Error()), err
		}
		if len(write.Key) == 0 {
			return stateSyncFailResult("state key is empty"), fmt.Errorf("state key is empty")
		}
		write.ContractName = targetContract
	}
	if err := validateSyncOwnership("bridge", writes); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	for _, write := range writes {
		if err := txSimContext.Put(targetContract, write.Key, write.Value); err != nil {
			return stateSyncFailResult(err.Error()), err
		}
	}
	return stateSyncSuccessResult(fmt.Sprintf("state sync batch applied: writes=%d", len(writes))), nil
}

// ApplyBridgeStateSyncTx writes a committed business-shard checkpoint into the
// bridge ledger. Values use their original contract namespace, so bridge-side
// contract execution can read them through ChainMaker's ordinary snapshot.
func ApplyBridgeStateSyncTx(tx *commonpb.Transaction, txSimContext protocol.TxSimContext) (*commonpb.Result, error) {
	if !IsBridgeStateSyncTx(tx) {
		return stateSyncFailResult("not a bridge state sync tx"), fmt.Errorf("not a bridge state sync tx")
	}
	if txSimContext == nil {
		return stateSyncFailResult("nil tx simulation context"), fmt.Errorf("nil tx simulation context")
	}
	info, err := ParseBridgeStateSyncInfo(tx)
	if err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err = verifyBridgeStateSyncSource(info.SourceShard); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	if err = syncauth.Verify(crossCfg.SyncAuth, info.SourceShard, "bridge", tx.Payload); err != nil {
		return stateSyncFailResult(err.Error()), err
	}

	var writes []BridgeStateWrite
	encoding := stateSyncParamString(tx, StateWritesEncodingParamKey)
	if raw, ok := stateSyncParamBytes(tx, StateWritesParamKey); ok && (len(raw) > 0 || encoding != "") {
		if writes, err = DecodeBridgeStateWrites(raw, encoding); err != nil {
			return stateSyncFailResult("invalid bridge state writes"), fmt.Errorf("invalid bridge state writes: %v", err)
		}
	} else if encoding != "" {
		return stateSyncFailResult("state writes are missing"), fmt.Errorf("state writes are missing")
	}
	if len(writes) == 0 && !info.Checkpoint {
		return stateSyncFailResult("empty bridge state sync tx"), fmt.Errorf("empty bridge state sync tx")
	}

	for _, write := range writes {
		contract := strings.TrimSpace(write.ContractName)
		if contract == "" || len(write.Key) == 0 {
			return stateSyncFailResult("invalid bridge state write"), fmt.Errorf("invalid bridge state write")
		}
		if isForbiddenStateSyncContract(contract) || contract == bridgeSyncMetadataContract {
			return stateSyncFailResult("system contract state sync is forbidden"),
				fmt.Errorf("system contract state sync is forbidden: %s", contract)
		}
	}
	if err = validateSyncOwnership(info.SourceShard, writes); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	for _, write := range writes {
		if err = txSimContext.Put(write.ContractName, write.Key, write.Value); err != nil {
			return stateSyncFailResult(err.Error()), err
		}
	}

	marker := fmt.Sprintf("%s:%d:%d:%d", normalizeShardId(info.SourceShard), info.EndHeight,
		info.ChunkIndex, info.ChunkTotal)
	if err = txSimContext.Put(bridgeSyncMetadataContract, []byte("__bridge_state_sync:"+marker),
		[]byte(stateSyncParamString(tx, SourceBlockHashParamKey))); err != nil {
		return stateSyncFailResult(err.Error()), err
	}
	return stateSyncSuccessResult(fmt.Sprintf("bridge state sync applied: writes=%d", len(writes))), nil
}

// ParseBridgeStateSyncInfo parses checkpoint metadata without executing the tx.
func ParseBridgeStateSyncInfo(tx *commonpb.Transaction) (BridgeStateSyncInfo, error) {
	var info BridgeStateSyncInfo
	if !IsBridgeStateSyncTx(tx) {
		return info, fmt.Errorf("not a bridge state sync tx")
	}
	info.SourceShard = normalizeShardId(stateSyncParamString(tx, SourceShardParamKey))
	if info.SourceShard == "" {
		return info, fmt.Errorf("source shard is empty")
	}
	var err error
	info.EndHeight, err = parseUintStateSyncParam(tx, SourceHeightParamKey)
	if err != nil || info.EndHeight == 0 {
		return info, fmt.Errorf("invalid source end height")
	}
	info.StartHeight, err = parseUintStateSyncParam(tx, SourceStartHeightParamKey)
	if err != nil || info.StartHeight == 0 {
		info.StartHeight = info.EndHeight
	}
	if info.StartHeight > info.EndHeight {
		return info, fmt.Errorf("source height range is invalid")
	}
	if rawCycle := strings.TrimSpace(stateSyncParamString(tx, SyncCycleParamKey)); rawCycle != "" {
		info.Cycle, err = strconv.ParseUint(rawCycle, 10, 64)
		if err != nil {
			return info, fmt.Errorf("invalid sync cycle")
		}
	}
	info.Slot, err = parseIntStateSyncParam(tx, SyncSlotParamKey, 0)
	if err != nil || info.Slot < 0 {
		return info, fmt.Errorf("invalid sync slot")
	}
	info.SlotTotal, err = parseIntStateSyncParam(tx, SyncSlotTotalParamKey, 0)
	if err != nil || info.SlotTotal < 0 || (info.SlotTotal > 0 && info.Slot > info.SlotTotal) {
		return info, fmt.Errorf("invalid sync slot total")
	}
	hasWavePosition := info.Cycle > 0 || info.Slot > 0 || info.SlotTotal > 0
	if hasWavePosition && (info.Cycle == 0 || info.Slot <= 0 || info.SlotTotal <= 0) {
		return info, fmt.Errorf("incomplete sync wave position")
	}
	info.Checkpoint = strings.EqualFold(strings.TrimSpace(stateSyncParamString(tx, SyncCheckpointParamKey)), "true")
	info.ChunkIndex, err = parseIntStateSyncParam(tx, SyncChunkIndexParamKey, 0)
	if err != nil || info.ChunkIndex < 0 {
		return info, fmt.Errorf("invalid sync chunk index")
	}
	info.ChunkTotal, err = parseIntStateSyncParam(tx, SyncChunkTotalParamKey, 1)
	if err != nil || info.ChunkTotal <= 0 || info.ChunkIndex >= info.ChunkTotal {
		return info, fmt.Errorf("invalid sync chunk total")
	}
	if info.Checkpoint && strings.TrimSpace(stateSyncParamString(tx, SourceBlockHashParamKey)) == "" {
		return info, fmt.Errorf("checkpoint source block hash is empty")
	}
	return info, nil
}

func verifyStateSyncSourceShard(tx *commonpb.Transaction) error {
	sourceShard := strings.TrimSpace(stateSyncParamString(tx, SourceShardParamKey))
	if sourceShard == "" {
		return fmt.Errorf("state sync source shard is empty")
	}
	if normalizeShardId(sourceShard) != "bridge" {
		return fmt.Errorf("state sync source shard must be bridge: source=%s", sourceShard)
	}
	return nil
}

func verifyBridgeStateSyncSource(sourceShard string) error {
	if crossCfg == nil {
		return fmt.Errorf("cross-shard config is not initialized")
	}
	if strings.ToUpper(strings.TrimSpace(crossCfg.ShardType)) != "BRIDGE" {
		return fmt.Errorf("bridge state sync tx can only be executed on bridge shard")
	}
	normalized := normalizeShardId(sourceShard)
	for _, shard := range crossCfg.Schedule.BusinessShards {
		if normalizeShardId(shard) == normalized {
			return nil
		}
	}
	return fmt.Errorf("unknown business source shard: %s", sourceShard)
}

func verifyStateSyncTargetShard(tx *commonpb.Transaction) error {
	if crossCfg == nil {
		return nil
	}
	if strings.ToUpper(strings.TrimSpace(crossCfg.ShardType)) != "BUSINESS" {
		return fmt.Errorf("state sync tx can only be executed on business shard")
	}
	targetShard := strings.TrimSpace(stateSyncParamString(tx, TargetShardParamKey))
	if targetShard == "" {
		return nil
	}
	if normalizeShardId(targetShard) != normalizeShardId(crossCfg.ShardId) {
		return fmt.Errorf("state sync target shard mismatch: target=%s local=%s", targetShard, crossCfg.ShardId)
	}
	return nil
}

func stateSyncParamString(tx *commonpb.Transaction, key string) string {
	value, ok := stateSyncParamBytes(tx, key)
	if !ok {
		return ""
	}
	return string(value)
}

func stateSyncParamBytes(tx *commonpb.Transaction, key string) ([]byte, bool) {
	if tx == nil || tx.Payload == nil {
		return nil, false
	}
	for _, param := range tx.Payload.Parameters {
		if param != nil && param.Key == key {
			return cloneStateBytes(param.Value), true
		}
	}
	return nil, false
}

func stateSyncFailResult(message string) *commonpb.Result {
	return &commonpb.Result{
		Code:    commonpb.TxStatusCode_CONTRACT_FAIL,
		Message: message,
		ContractResult: &commonpb.ContractResult{
			Code:    1,
			Message: message,
		},
	}
}

func stateSyncSuccessResult(message string) *commonpb.Result {
	return &commonpb.Result{
		Code:    commonpb.TxStatusCode_SUCCESS,
		Message: message,
		ContractResult: &commonpb.ContractResult{
			Code:    0,
			Message: message,
		},
	}
}

func parseUintStateSyncParam(tx *commonpb.Transaction, key string) (uint64, error) {
	raw := strings.TrimSpace(stateSyncParamString(tx, key))
	if raw == "" {
		return 0, fmt.Errorf("%s is empty", key)
	}
	return strconv.ParseUint(raw, 10, 64)
}

func parseIntStateSyncParam(tx *commonpb.Transaction, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(stateSyncParamString(tx, key))
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func isForbiddenStateSyncContract(contract string) bool {
	_, reserved := syscontract.SystemContract_value[contract]
	return reserved
}

package crossshard

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

var stateKeyPatterns sync.Map

func compiledStatePattern(pattern string) (*regexp.Regexp, error) {
	if cached, ok := stateKeyPatterns.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	actual, _ := stateKeyPatterns.LoadOrStore(pattern, compiled)
	return actual.(*regexp.Regexp), nil
}

// NotifyBlockCommitted inspects bridge-shard cross transactions and emits
// generic aggregated state writes to business shards.
func NotifyBlockCommitted(block *commonpb.Block, rwSetMap map[string]*commonpb.TxRWSet) {
	completePendingApplyWrites(block)
	if crossCfg == nil || block == nil || block.Header == nil {
		return
	}
	if strings.ToUpper(crossCfg.ShardType) != "BRIDGE" {
		return
	}
	if len(rwSetMap) == 0 {
		return
	}

	aggregated := make(map[string]map[string]*CrossWrite)
	for _, tx := range block.Txs {
		if !isCrossShardTx(tx) {
			continue
		}
		rw := rwSetMap[tx.Payload.TxId]
		if rw == nil {
			continue
		}
		for _, write := range rw.TxWrites {
			if write == nil || write.ContractName == "" || len(write.Key) == 0 {
				continue
			}
			targetShard := routeWriteTargetForTx(tx, write.ContractName, write.Key)
			if targetShard == "" {
				crossLog.Warnf("[CrossShard] 无法路由跨片写集 | tx=%s contract=%s key=%s",
					tx.Payload.TxId, write.ContractName, string(write.Key))
				continue
			}
			byKey := aggregated[targetShard]
			if byKey == nil {
				byKey = make(map[string]*CrossWrite)
				aggregated[targetShard] = byKey
			}
			aggKey := aggregateKey(targetShard, write.ContractName, write.Key)
			byKey[aggKey] = &CrossWrite{
				TargetShard:  targetShard,
				ContractName: write.ContractName,
				Key:          cloneStateBytes(write.Key),
				Value:        cloneStateBytes(write.Value),
				BridgeTxId:   tx.Payload.TxId,
			}
		}
	}
	sendWriteBatches(block, aggregated)
}

func isCrossShardTx(tx *commonpb.Transaction) bool {
	if tx == nil || tx.Payload == nil {
		return false
	}
	if !TransactionSucceeded(tx) || IsSystemSyncTx(tx) || isForbiddenStateSyncContract(tx.Payload.ContractName) ||
		tx.Payload.ContractName == bridgeSyncMetadataContract || tx.Payload.ContractName == "poke_block" {
		return false
	}
	return crossCfg != nil && strings.EqualFold(crossCfg.ShardType, "BRIDGE") &&
		tx.Payload.TxType == commonpb.TxType_INVOKE_CONTRACT
}

func routeWriteTargetForTx(tx *commonpb.Transaction, contract string, key []byte) string {
	return routeWriteTarget(contract, key)
}

func routeWriteTarget(contract string, key []byte) string {
	if crossCfg == nil || len(crossCfg.Schedule.BusinessShards) == 0 {
		return ""
	}
	rule := matchRoutingRule(contract, key, crossCfg.Schedule.CrossWriteRouting.Rules)
	if rule != nil {
		if target := normalizeShardId(rule.TargetShard); target != "" {
			if target == "bridge" || target == "bridge-shard" {
				return "bridge"
			}
			for _, shard := range crossCfg.Schedule.BusinessShards {
				if normalizeShardId(shard) == target {
					return target
				}
			}
			return ""
		}
		if strategy := strings.TrimSpace(rule.Strategy); strategy != "" {
			if rule.KeyPattern != "" {
				pattern, err := compiledStatePattern(rule.KeyPattern)
				if err != nil {
					return ""
				}
				groups := pattern.FindSubmatch(key)
				if len(groups) != 2 || len(groups[1]) == 0 {
					return ""
				}
				key = groups[1]
			}
			return routeByStrategy(strategy, contract, key)
		}
	}
	return routeByStrategy(crossCfg.Schedule.CrossWriteRouting.Strategy, contract, key)
}

func aggregateKey(targetShard, contract string, key []byte) string {
	return targetShard + "|" + contract + "|" + string(key)
}

func matchRoutingRule(contract string, key []byte, rules []shardingsync.CrossWriteRoutingRule) *shardingsync.CrossWriteRoutingRule {
	keyStr := string(key)
	for i := range rules {
		rule := &rules[i]
		if rule.ContractName != "" && rule.ContractName != contract {
			continue
		}
		if rule.KeyPrefix != "" && !strings.HasPrefix(keyStr, rule.KeyPrefix) {
			continue
		}
		if rule.KeyPattern != "" {
			pattern, err := compiledStatePattern(rule.KeyPattern)
			if err != nil || !pattern.Match(key) {
				continue
			}
		}
		return rule
	}
	return nil
}

func routeByStrategy(strategy, contract string, key []byte) string {
	shards := crossCfg.Schedule.BusinessShards
	if len(shards) == 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "key_hash":
		return routeByHash(key, shards)
	case "contract_key_hash":
		raw := append([]byte(strings.TrimSpace(contract)+":"), key...)
		return routeByHash(raw, shards)
	case "first_business":
		return normalizeShardId(strings.TrimSpace(shards[0]))
	case "none", "":
		return ""
	default:
		crossLog.Errorf("[CrossShard] unknown state routing strategy: %s", strategy)
		return ""
	}
}

func routeByHash(raw []byte, shards []string) string {
	if len(raw) == 0 || len(shards) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	idx := int(binary.BigEndian.Uint64(sum[:8]) % uint64(len(shards)))
	return normalizeShardId(strings.TrimSpace(shards[idx]))
}

func sendWriteBatches(block *commonpb.Block, aggregated map[string]map[string]*CrossWrite) {
	if block == nil || block.Header == nil {
		return
	}
	for shard, byKey := range aggregated {
		if shard == "bridge" || shard == "bridge-shard" {
			continue
		}
		if len(byKey) == 0 {
			continue
		}
		writes := make([]CrossWrite, 0, len(byKey))
		for _, write := range byKey {
			if write != nil {
				writes = append(writes, *write)
			}
		}
		if len(writes) == 0 {
			continue
		}
		sort.Slice(writes, func(i, j int) bool {
			if writes[i].ContractName != writes[j].ContractName {
				return writes[i].ContractName < writes[j].ContractName
			}
			if cmp := bytes.Compare(writes[i].Key, writes[j].Key); cmp != 0 {
				return cmp < 0
			}
			if cmp := bytes.Compare(writes[i].Value, writes[j].Value); cmp != 0 {
				return cmp < 0
			}
			return writes[i].BridgeTxId < writes[j].BridgeTxId
		})
		batch := &CrossWriteBatch{
			BatchId:           fmt.Sprintf("bridge-%d-%s", block.Header.BlockHeight, shard),
			TargetShard:       shard,
			BridgeBlockHeight: block.Header.BlockHeight,
			BridgeBlockHash:   hex.EncodeToString(block.Header.BlockHash),
			Timestamp:         block.Header.BlockTimestamp,
			Writes:            writes,
		}
		crossLog.Infof("[CrossShard] 发送通用跨片写集 | batch=%s shard=%s writes=%d",
			batch.BatchId, shard, len(writes))
		SendBatch(batch)
	}
}

func normalizeShardId(shardId string) string {
	shardId = strings.TrimSpace(shardId)
	if shardId == "" {
		return shardId
	}
	if strings.HasPrefix(shardId, "business-shard") || strings.HasPrefix(shardId, "bridge") {
		return shardId
	}
	if strings.HasPrefix(shardId, "business-") {
		suffix := strings.TrimPrefix(shardId, "business-")
		if suffix != "" {
			return "business-shard" + suffix
		}
	}
	return shardId
}

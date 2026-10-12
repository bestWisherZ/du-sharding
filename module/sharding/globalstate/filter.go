package globalstate

import (
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	syscontract "chainmaker.org/chainmaker/pb-go/v2/syscontract"
)

func shouldSyncStateWrite(contractName string, key []byte) bool {
	contractName = strings.TrimSpace(contractName)
	if contractName == "" || len(key) == 0 {
		return false
	}
	if isSystemContract(contractName) {
		return false
	}
	switch contractName {
	case "poke_block", "sharding_heartbeat":
		return false
	default:
		return true
	}
}

func shouldSyncStateTx(contractName, method string) bool {
	contractName = strings.TrimSpace(contractName)
	method = strings.TrimSpace(method)
	if contractName == "" {
		return false
	}
	if isSystemContract(contractName) {
		return false
	}
	switch method {
	case crossshard.SystemStateSyncApplyMethod,
		crossshard.SystemStateSyncApplyBatchMethod:
		return false
	}
	return true
}

func isSystemContract(contractName string) bool {
	_, exists := syscontract.SystemContract_value[contractName]
	return exists
}

func cloneStateBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

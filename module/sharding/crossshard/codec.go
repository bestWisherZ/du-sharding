package crossshard

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	batchPrefix = "CROSS_WRITE_BATCH:"
)

func BuildBatchMessage(targetShard string, batch *CrossWriteBatch) (string, error) {
	if targetShard == "" || batch == nil {
		return "", fmt.Errorf("invalid cross write batch")
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	return fmt.Sprintf("%s%s:%s", batchPrefix, targetShard, encoded), nil
}

func ParseBatchMessage(msg string) (string, *CrossWriteBatch, bool) {
	if !strings.HasPrefix(msg, batchPrefix) {
		return "", nil, false
	}
	payload := strings.TrimPrefix(msg, batchPrefix)
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	targetShard := parts[0]
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, false
	}
	var batch CrossWriteBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return "", nil, false
	}
	return targetShard, &batch, true
}

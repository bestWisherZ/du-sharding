package globalstate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	storepb "chainmaker.org/chainmaker/pb-go/v2/store"
	"github.com/gogo/protobuf/proto"
)

const (
	deltaPrefix        = "GLOBAL_STATE_DELTA:"
	compactDeltaPrefix = "GLOBAL_STATE_DELTA_V2:"
)

type CompactDelta struct {
	StartHeight    uint64       `json:"sh,omitempty"`
	Height         uint64       `json:"h"`
	BlockHash      string       `json:"bh,omitempty"`
	Timestamp      int64        `json:"ts,omitempty"`
	Cycle          uint64       `json:"cy,omitempty"`
	Slot           int          `json:"sl,omitempty"`
	SlotTotal      int          `json:"st,omitempty"`
	Checkpoint     bool         `json:"cp,omitempty"`
	Writes         []DeltaWrite `json:"w,omitempty"`
	Authorizations [][]byte     `json:"auth,omitempty"`
}

type DeltaWrite struct {
	Contract string `json:"c"`
	Key      []byte `json:"k"`
	Value    []byte `json:"v"`
}

func BuildCompactDeltaMessage(shardId string, delta *CompactDelta) (string, error) {
	return buildBinaryCompactDeltaMessage(shardId, delta)
}

func ParseCompactDeltaMessage(msg string) (string, *CompactDelta, bool) {
	if strings.HasPrefix(msg, compactDeltaBinaryPrefix) {
		return parseBinaryCompactDeltaMessage(msg)
	}
	if !strings.HasPrefix(msg, compactDeltaPrefix) {
		return "", nil, false
	}
	payload := strings.TrimPrefix(msg, compactDeltaPrefix)
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	shardId := parts[0]
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, false
	}
	var delta CompactDelta
	if err := json.Unmarshal(raw, &delta); err != nil {
		return "", nil, false
	}
	return shardId, &delta, true
}

func BuildDeltaMessage(shardId string, blockWithRWSet *storepb.BlockWithRWSet) (string, error) {
	if shardId == "" || blockWithRWSet == nil {
		return "", fmt.Errorf("invalid delta")
	}
	raw, err := proto.Marshal(blockWithRWSet)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	return fmt.Sprintf("%s%s:%s", deltaPrefix, shardId, encoded), nil
}

func ParseDeltaMessage(msg string) (string, *storepb.BlockWithRWSet, bool) {
	if !strings.HasPrefix(msg, deltaPrefix) {
		return "", nil, false
	}
	payload := strings.TrimPrefix(msg, deltaPrefix)
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return "", nil, false
	}
	shardId := parts[0]
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, false
	}
	var bw storepb.BlockWithRWSet
	if err := proto.Unmarshal(raw, &bw); err != nil {
		return "", nil, false
	}
	return shardId, &bw, true
}

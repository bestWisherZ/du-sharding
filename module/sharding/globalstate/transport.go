package globalstate

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/internal/received"
	"github.com/gogo/protobuf/proto"
)

const compactDeltaBinaryPrefix = "GLOBAL_STATE_DELTA_V3:"

var receivedCheckpoints = received.New(4096)

// Streams are already message-framed by EOF. The body is raw protobuf, not
// base64; the existing string transport preserves arbitrary bytes unchanged.
type compactDeltaWire struct {
	StartHeight    uint64   `protobuf:"varint,1,opt,name=start_height,json=startHeight,proto3"`
	Height         uint64   `protobuf:"varint,2,opt,name=height,proto3"`
	BlockHash      string   `protobuf:"bytes,3,opt,name=block_hash,json=blockHash,proto3"`
	Timestamp      int64    `protobuf:"varint,4,opt,name=timestamp,proto3"`
	Cycle          uint64   `protobuf:"varint,5,opt,name=cycle,proto3"`
	Slot           int64    `protobuf:"varint,6,opt,name=slot,proto3"`
	SlotTotal      int64    `protobuf:"varint,7,opt,name=slot_total,json=slotTotal,proto3"`
	Checkpoint     bool     `protobuf:"varint,8,opt,name=checkpoint,proto3"`
	Writes         []byte   `protobuf:"bytes,9,opt,name=writes,proto3"`
	Authorizations [][]byte `protobuf:"bytes,10,rep,name=authorizations,proto3"`
}

func (m *compactDeltaWire) Reset()         { *m = compactDeltaWire{} }
func (m *compactDeltaWire) String() string { return proto.CompactTextString(m) }
func (*compactDeltaWire) ProtoMessage()    {}

func compactDeltaMetadata(delta *CompactDelta) *compactDeltaWire {
	return &compactDeltaWire{
		StartHeight: delta.StartHeight, Height: delta.Height, BlockHash: delta.BlockHash,
		Timestamp: delta.Timestamp, Cycle: delta.Cycle, Slot: int64(delta.Slot),
		SlotTotal: int64(delta.SlotTotal), Checkpoint: delta.Checkpoint,
	}
}

func buildBinaryCompactDeltaMessage(shard string, delta *CompactDelta) (string, error) {
	return buildEncodedCompactDeltaMessage(shard, delta, nil)
}

func buildEncodedCompactDeltaMessage(shard string, delta *CompactDelta, encodedWrites []byte) (string, error) {
	if shard == "" || strings.Contains(shard, ":") || !utf8.ValidString(shard) || delta == nil {
		return "", fmt.Errorf("invalid compact delta envelope")
	}
	if !utf8.ValidString(delta.BlockHash) {
		return "", fmt.Errorf("invalid UTF-8 in compact delta block hash")
	}
	wire := compactDeltaMetadata(delta)
	wire.Authorizations = delta.Authorizations
	var err error
	if encodedWrites == nil {
		writes := make([]crossshard.BridgeStateWrite, len(delta.Writes))
		for i, write := range delta.Writes {
			writes[i] = crossshard.BridgeStateWrite{ContractName: write.Contract, Key: write.Key, Value: write.Value}
		}
		encodedWrites, err = crossshard.EncodeBridgeStateWrites(writes)
		if err != nil {
			return "", err
		}
	}
	wire.Writes = encodedWrites
	raw, err := proto.Marshal(wire)
	if err != nil {
		return "", err
	}
	return compactDeltaBinaryPrefix + shard + ":" + string(raw), nil
}

func parseBinaryCompactDeltaMessage(msg string) (string, *CompactDelta, bool) {
	shard, wire, ok := parseCompactDeltaEnvelope(msg)
	if !ok {
		return "", nil, false
	}
	delta, ok := decodeCompactDelta(wire)
	return shard, delta, ok
}

func parseCompactDeltaEnvelope(msg string) (string, *compactDeltaWire, bool) {
	parts := strings.SplitN(strings.TrimPrefix(msg, compactDeltaBinaryPrefix), ":", 2)
	if len(parts) != 2 || parts[0] == "" || !utf8.ValidString(parts[0]) {
		return "", nil, false
	}
	wire := &compactDeltaWire{}
	if err := proto.Unmarshal([]byte(parts[1]), wire); err != nil || wire.Height == 0 ||
		!utf8.ValidString(wire.BlockHash) || int64(int(wire.Slot)) != wire.Slot ||
		int64(int(wire.SlotTotal)) != wire.SlotTotal || wire.Slot < 0 || wire.SlotTotal < 0 {
		return "", nil, false
	}
	return parts[0], wire, true
}

func decodeCompactDelta(wire *compactDeltaWire) (*CompactDelta, bool) {
	writes, err := crossshard.DecodeBridgeStateWrites(wire.Writes, crossshard.BridgeStateWritesEncodingV1)
	if err != nil {
		return nil, false
	}
	delta := &CompactDelta{
		StartHeight: wire.StartHeight, Height: wire.Height, BlockHash: wire.BlockHash,
		Timestamp: wire.Timestamp, Cycle: wire.Cycle, Slot: int(wire.Slot),
		SlotTotal: int(wire.SlotTotal), Checkpoint: wire.Checkpoint,
		Authorizations: wire.Authorizations,
	}
	if len(writes) > 0 {
		delta.Writes = make([]DeltaWrite, len(writes))
	}
	for i, write := range writes {
		if write.SourceTxId != "" {
			return nil, false
		}
		delta.Writes[i] = DeltaWrite{Contract: write.ContractName, Key: write.Key, Value: write.Value}
	}
	return delta, true
}

func checkpointContentKey(shard string, wire *compactDeltaWire) [32]byte {
	metadata := *wire
	metadata.Writes, metadata.Authorizations = nil, nil
	raw, _ := proto.Marshal(&metadata)
	sum := sha256.New()
	fmt.Fprintf(sum, "%d:%s|%d:", len(shard), shard, len(raw))
	_, _ = sum.Write(raw)
	_, _ = sum.Write(wire.Writes)
	var key [32]byte
	copy(key[:], sum.Sum(nil))
	return key
}

func receiveBinaryCompactDelta(msg string) bool {
	if !shouldReceive() {
		return true
	}
	shard, wire, ok := parseCompactDeltaEnvelope(msg)
	if !ok {
		return false
	}
	// All source replicas authenticate the same checkpoint content. Do not
	// decode and re-verify it again after one replica has passed verification.
	receivedCheckpoints.Do(checkpointContentKey(shard, wire), func() bool {
		delta, ok := decodeCompactDelta(wire)
		return ok && handleCompactDelta(shard, delta)
	})
	return true
}

package crossshard

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/gogo/protobuf/proto"
)

const (
	StateWritesEncodingParamKey = "__state_writes_encoding"
	BridgeStateWritesEncodingV1 = "protobuf-v1"
)

// The private wire schema is independent of ChainMaker's transaction/RWSet
// schema. Dictionary order is first occurrence; write order is never changed.
type bridgeWritesWire struct {
	Contracts []string           `protobuf:"bytes,1,rep,name=contracts"`
	Writes    []*bridgeWriteWire `protobuf:"bytes,2,rep,name=writes"`
}

type bridgeWriteWire struct {
	ContractIndex uint32 `protobuf:"varint,1,opt,name=contract_index,json=contractIndex,proto3"`
	Key           []byte `protobuf:"bytes,2,opt,name=key,proto3"`
	Value         []byte `protobuf:"bytes,3,opt,name=value,proto3"`
	SourceTxID    string `protobuf:"bytes,4,opt,name=source_tx_id,json=sourceTxId,proto3"`
	EmptyKey      bool   `protobuf:"varint,5,opt,name=empty_key,json=emptyKey,proto3"`
	EmptyValue    bool   `protobuf:"varint,6,opt,name=empty_value,json=emptyValue,proto3"`
}

func (m *bridgeWritesWire) Reset()         { *m = bridgeWritesWire{} }
func (m *bridgeWritesWire) String() string { return proto.CompactTextString(m) }
func (*bridgeWritesWire) ProtoMessage()    {}
func (m *bridgeWriteWire) Reset()          { *m = bridgeWriteWire{} }
func (m *bridgeWriteWire) String() string  { return proto.CompactTextString(m) }
func (*bridgeWriteWire) ProtoMessage()     {}

// EncodeBridgeStateWrites returns an owned, deterministic protobuf payload.
// Explicit empty flags retain the nil-versus-empty semantics of JSON bytes.
func EncodeBridgeStateWrites(writes []BridgeStateWrite) ([]byte, error) {
	wire := &bridgeWritesWire{Writes: make([]*bridgeWriteWire, 0, len(writes))}
	indices := make(map[string]uint32)
	for _, write := range writes {
		if !utf8.ValidString(write.ContractName) || !utf8.ValidString(write.SourceTxId) {
			return nil, fmt.Errorf("invalid UTF-8 in bridge state write")
		}
		index, ok := indices[write.ContractName]
		if !ok {
			index = uint32(len(wire.Contracts))
			indices[write.ContractName] = index
			wire.Contracts = append(wire.Contracts, write.ContractName)
		}
		wire.Writes = append(wire.Writes, &bridgeWriteWire{
			ContractIndex: index, Key: write.Key, Value: write.Value, SourceTxID: write.SourceTxId,
			EmptyKey:   write.Key != nil && len(write.Key) == 0,
			EmptyValue: write.Value != nil && len(write.Value) == 0,
		})
	}
	return proto.Marshal(wire)
}

// DecodeBridgeStateWrites keeps old committed JSON transactions replayable.
// Unknown versions fail closed instead of interpreting bytes in another format.
func DecodeBridgeStateWrites(raw []byte, encoding string) ([]BridgeStateWrite, error) {
	if encoding == "" {
		var writes []BridgeStateWrite
		err := json.Unmarshal(raw, &writes)
		return writes, err
	}
	if encoding != BridgeStateWritesEncodingV1 {
		return nil, fmt.Errorf("unsupported bridge state writes encoding %q", encoding)
	}
	wire := &bridgeWritesWire{}
	if err := proto.Unmarshal(raw, wire); err != nil {
		return nil, fmt.Errorf("invalid bridge state writes protobuf: %w", err)
	}
	for _, contract := range wire.Contracts {
		if !utf8.ValidString(contract) {
			return nil, fmt.Errorf("invalid UTF-8 in bridge contract dictionary")
		}
	}
	writes := make([]BridgeStateWrite, 0, len(wire.Writes))
	for _, write := range wire.Writes {
		if write == nil || uint64(write.ContractIndex) >= uint64(len(wire.Contracts)) {
			return nil, fmt.Errorf("invalid bridge contract dictionary index")
		}
		if !utf8.ValidString(write.SourceTxID) ||
			(write.EmptyKey && len(write.Key) > 0) || (write.EmptyValue && len(write.Value) > 0) {
			return nil, fmt.Errorf("invalid bridge state write fields")
		}
		if write.EmptyKey {
			write.Key = []byte{}
		}
		if write.EmptyValue {
			write.Value = []byte{}
		}
		writes = append(writes, BridgeStateWrite{
			ContractName: wire.Contracts[write.ContractIndex], Key: write.Key,
			Value: write.Value, SourceTxId: write.SourceTxID,
		})
	}
	return writes, nil
}

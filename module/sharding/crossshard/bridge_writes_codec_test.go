package crossshard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	protocolmock "chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
)

func TestBridgeWritesCodecRoundTrip(t *testing.T) {
	writes := []BridgeStateWrite{
		{ContractName: "token_a", Key: []byte{0, 255}, Value: []byte{0, 255, 34}},
		{ContractName: "token_b", Key: []byte("b"), Value: nil, SourceTxId: "tx-b"},
		{ContractName: "token_a", Key: []byte("a"), Value: []byte{}},
		{ContractName: "token_<>&\u2028", Key: nil, Value: []byte("v")},
		{ContractName: "token_a", Key: []byte{}, Value: nil},
		{ContractName: "token_a", Key: []byte("a"), Value: []byte("last-write")},
	}
	for _, input := range [][]BridgeStateWrite{{}, writes} {
		raw, err := EncodeBridgeStateWrites(input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeBridgeStateWrites(raw, BridgeStateWritesEncodingV1)
		if err != nil || !reflect.DeepEqual(input, got) {
			t.Fatalf("round trip: got=%#v err=%v", got, err)
		}
		for i := 0; i < 20; i++ {
			again, err := EncodeBridgeStateWrites(input)
			if err != nil || !bytes.Equal(raw, again) {
				t.Fatal("nondeterministic wire encoding")
			}
		}
		legacy, _ := json.Marshal(input)
		old, err := DecodeBridgeStateWrites(legacy, "")
		if err != nil || !reflect.DeepEqual(input, old) {
			t.Fatal("legacy JSON replay changed")
		}
	}
}

func TestBridgeWritesCodecDictionaryAndIsolation(t *testing.T) {
	writes := []BridgeStateWrite{{ContractName: "token", Key: []byte("a"), Value: []byte("v")},
		{ContractName: "token", Key: []byte("b"), Value: []byte("w")}}
	raw, err := EncodeBridgeStateWrites(writes)
	if err != nil {
		t.Fatal(err)
	}
	wire := &bridgeWritesWire{}
	if err = proto.Unmarshal(raw, wire); err != nil || !reflect.DeepEqual(wire.Contracts, []string{"token"}) {
		t.Fatalf("dictionary: %+v %v", wire.Contracts, err)
	}
	writes[0].Key[0], writes[0].Value[0] = 'X', 'X'
	got, err := DecodeBridgeStateWrites(raw, BridgeStateWritesEncodingV1)
	if err != nil || string(got[0].Key) != "a" || string(got[0].Value) != "v" {
		t.Fatal("encoding aliases source writes")
	}
	got[0].Key[0], got[0].Value[0] = 'X', 'X'
	again, err := DecodeBridgeStateWrites(raw, BridgeStateWritesEncodingV1)
	if err != nil || string(again[0].Key) != "a" || string(again[0].Value) != "v" {
		t.Fatal("decoding aliases encoded payload")
	}
}

func TestBridgeWritesCodecRejectsMalformed(t *testing.T) {
	badIndex, _ := proto.Marshal(&bridgeWritesWire{Contracts: []string{"token"}, Writes: []*bridgeWriteWire{{ContractIndex: 1}}})
	badEmpty, _ := proto.Marshal(&bridgeWritesWire{Contracts: []string{"token"}, Writes: []*bridgeWriteWire{{Value: []byte("v"), EmptyValue: true}}})
	for _, raw := range [][]byte{{0xff}, {0x0a, 0xff}, badIndex, badEmpty} {
		if _, err := DecodeBridgeStateWrites(raw, BridgeStateWritesEncodingV1); err == nil {
			t.Fatalf("accepted malformed payload %x", raw)
		}
	}
	if _, err := DecodeBridgeStateWrites(nil, "protobuf-v999"); err == nil {
		t.Fatal("accepted unknown codec version")
	}
	if _, err := EncodeBridgeStateWrites([]BridgeStateWrite{{ContractName: "token\xff"}}); err == nil {
		t.Fatal("accepted invalid contract UTF-8")
	}
}

func TestApplyCompactBridgeWritesPreservesStateAndCheckpoint(t *testing.T) {
	old := crossCfg
	t.Cleanup(func() { crossCfg = old })
	crossCfg = &shardingsync.ShardingConfig{ShardType: "BRIDGE",
		Schedule: shardingsync.ScheduleConfig{BusinessShards: []string{"business-shard1"}}}
	writes := []BridgeStateWrite{{ContractName: "token", Key: []byte("a"), Value: nil},
		{ContractName: "token", Key: []byte("b"), Value: []byte{}},
		{ContractName: "token", Key: []byte("c"), Value: []byte("99")}}
	for _, encoding := range []string{"", BridgeStateWritesEncodingV1} {
		raw, err := json.Marshal(writes)
		if encoding != "" {
			raw, err = EncodeBridgeStateWrites(writes)
		}
		if err != nil {
			t.Fatal(err)
		}
		tx := bridgeStateSyncTestTx(raw)
		tx.Payload.Parameters = append(tx.Payload.Parameters, &commonpb.KeyValuePair{Key: StateWritesEncodingParamKey, Value: []byte(encoding)})
		authorizeTestTx(t, tx, "business-shard1", "bridge")
		ctx, state := stateSyncMock(t)
		result, err := ApplyBridgeStateSyncTx(tx, ctx)
		if err != nil || result.Code != commonpb.TxStatusCode_SUCCESS {
			t.Fatalf("apply %s: %v %v", encoding, result, err)
		}
		if state["token/a"] != nil || state["token/b"] == nil || len(state["token/b"]) != 0 || string(state["token/c"]) != "99" {
			t.Fatal("nil, empty or ordinary value changed")
		}
		if string(state[bridgeSyncMetadataContract+"/__bridge_state_sync:business-shard1:15:0:1"]) != "block-hash-15" {
			t.Fatal("checkpoint missing")
		}
	}
	for _, encoding := range []string{BridgeStateWritesEncodingV1, "unknown"} {
		raw, _ := EncodeBridgeStateWrites(nil)
		tx := bridgeStateSyncTestTx(raw)
		tx.Payload.Parameters = append(tx.Payload.Parameters, &commonpb.KeyValuePair{Key: StateWritesEncodingParamKey, Value: []byte(encoding)})
		authorizeTestTx(t, tx, "business-shard1", "bridge")
		ctrl := gomock.NewController(t)
		ctx := protocolmock.NewMockTxSimContext(ctrl)
		if encoding == BridgeStateWritesEncodingV1 {
			ctx.EXPECT().Put(bridgeSyncMetadataContract, []byte("__bridge_state_sync:business-shard1:15:0:1"), []byte("block-hash-15")).Return(nil)
		}
		_, err := ApplyBridgeStateSyncTx(tx, ctx)
		if (err == nil) != (encoding == BridgeStateWritesEncodingV1) {
			t.Fatalf("empty checkpoint %s: %v", encoding, err)
		}
		ctrl.Finish()
	}
}

func bridgeCodecBenchmarkWrites() []BridgeStateWrite {
	writes := make([]BridgeStateWrite, 2000)
	for i := range writes {
		writes[i] = BridgeStateWrite{ContractName: "standard_dfa_experiment",
			Key: []byte(fmt.Sprintf("balance_%040x", i)), Value: []byte("999999999")}
	}
	return writes
}

func TestCompactBridgeWritesSize(t *testing.T) {
	writes := bridgeCodecBenchmarkWrites()
	legacy, _ := json.Marshal(writes)
	compact, err := EncodeBridgeStateWrites(writes)
	if err != nil || len(compact)*100 >= len(legacy)*60 {
		t.Fatalf("compact=%d JSON=%d err=%v", len(compact), len(legacy), err)
	}
	t.Logf("writes=%d JSON=%d protobuf=%d reduction=%.2f%%", len(writes), len(legacy), len(compact), 100*(1-float64(len(compact))/float64(len(legacy))))
}

func BenchmarkBridgeWritesCodec(b *testing.B) {
	writes := bridgeCodecBenchmarkWrites()
	for _, encoding := range []string{"", BridgeStateWritesEncodingV1} {
		encode := func() ([]byte, error) { return json.Marshal(writes) }
		if encoding != "" {
			encode = func() ([]byte, error) { return EncodeBridgeStateWrites(writes) }
		}
		raw, err := encode()
		if err != nil {
			b.Fatal(err)
		}
		b.Run(encoding+"/encode", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := encode(); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(encoding+"/decode", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := DecodeBridgeStateWrites(raw, encoding); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func FuzzBridgeWritesCodec(f *testing.F) {
	raw, _ := EncodeBridgeStateWrites(bridgeCodecBenchmarkWrites()[:2])
	f.Add(raw)
	f.Add([]byte{})
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, raw []byte) {
		writes, err := DecodeBridgeStateWrites(raw, BridgeStateWritesEncodingV1)
		if err != nil {
			return
		}
		encoded, err := EncodeBridgeStateWrites(writes)
		if err != nil {
			t.Fatal(err)
		}
		again, err := DecodeBridgeStateWrites(encoded, BridgeStateWritesEncodingV1)
		if err != nil || !reflect.DeepEqual(writes, again) {
			t.Fatalf("round trip failed: %v", err)
		}
	})
}

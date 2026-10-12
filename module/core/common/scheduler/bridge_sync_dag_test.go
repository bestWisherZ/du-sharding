package scheduler

import (
	"reflect"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

type bridgeDAGTestSnapshot struct {
	*mock.MockSnapshot
	build func([]*commonPb.TxRWSet, []*commonPb.Transaction) (*commonPb.DAG, int, bool)
}

func (s bridgeDAGTestSnapshot) TryBuildDisjointWriteDAG(rwsets []*commonPb.TxRWSet,
	txs []*commonPb.Transaction) (*commonPb.DAG, int, bool) {
	return s.build(rwsets, txs)
}

func TestBridgeSyncDAGProposalAndVerification(t *testing.T) {
	cfg := &shardingsync.ShardingConfig{}
	crossshard.InitWithConfig(cfg)
	defer func() { *cfg = shardingsync.ShardingConfig{} }()
	for _, name := range []string{"propose", "verify", "business", "mixed", "sql", "read", "overlap", "order", "unsupported"} {
		t.Run(name, func(t *testing.T) {
			role := "BRIDGE"
			if name == "business" {
				role = "BUSINESS"
			}
			cfg.ShardType = role
			ctrl := gomock.NewController(t)
			chain := mock.NewMockChainConf(ctrl)
			chain.EXPECT().ChainConfig().Return(&configPb.ChainConfig{
				Contract: &configPb.ContractConfig{EnableSqlSupport: name == "sql"},
			})
			log := mock.NewMockLogger(ctrl)
			log.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
			snap := mock.NewMockSnapshot(ctrl)
			snap.EXPECT().GetBlockHeight().Return(uint64(21)).AnyTimes()
			txs := []*commonPb.Transaction{
				{Payload: &commonPb.Payload{TxId: "a", Method: crossshard.SystemBridgeStateSyncApplyMethod}},
				{Payload: &commonPb.Payload{TxId: "b", Method: crossshard.SystemBridgeStateSyncApplyMethod}},
			}
			sets := []*commonPb.TxRWSet{
				{TxId: "a", TxWrites: []*commonPb.TxWrite{{Key: []byte("alice")}}},
				{TxId: "b", TxWrites: []*commonPb.TxWrite{{Key: []byte("bob")}}},
			}
			switch name {
			case "mixed":
				txs[1].Payload.Method = "TransferExperiment"
			case "read":
				sets[1].TxReads = []*commonPb.TxRead{{Key: []byte("alice")}}
			case "overlap":
				sets[1].TxWrites[0].Key = []byte("alice")
			case "order":
				sets[0], sets[1] = sets[1], sets[0]
			}
			input := sets
			want := &commonPb.DAG{Vertexes: []*commonPb.DAG_Neighbor{{Neighbors: []uint32{}}, {Neighbors: []uint32{}}}}
			if name == "propose" {
				input = nil
			} else if name != "verify" {
				want = &commonPb.DAG{Vertexes: []*commonPb.DAG_Neighbor{{Neighbors: []uint32{0}}}}
				snap.EXPECT().BuildDAG(name == "sql", input).Return(want)
			}
			ts := &TxScheduler{chainConf: chain, log: log}
			called := false
			var snapshot protocol.Snapshot = bridgeDAGTestSnapshot{snap, func(rw []*commonPb.TxRWSet,
				transactions []*commonPb.Transaction) (*commonPb.DAG, int, bool) {
				called = true
				if !reflect.DeepEqual(rw, input) || !reflect.DeepEqual(transactions, txs) {
					t.Fatal("builder inputs changed")
				}
				return want, 2, name == "propose" || name == "verify"
			}}
			if name == "unsupported" {
				snapshot = snap
			}
			if got := ts.buildDAG(snapshot, input, txs); !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected DAG: %+v", got)
			}
			if called != (name == "propose" || name == "verify" || name == "read" || name == "overlap" || name == "order") {
				t.Fatal("unexpected fast-path selection")
			}
		})
	}
}

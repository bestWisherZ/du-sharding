package snapshot

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	protocolmock "chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"go.uber.org/atomic"
)

func disjointDAGFixture(txs, writes int) []*commonPb.TxRWSet {
	sets := make([]*commonPb.TxRWSet, txs)
	for i := range sets {
		sets[i] = &commonPb.TxRWSet{TxId: fmt.Sprint(i)}
		for j := 0; j < writes; j++ {
			sets[i].TxWrites = append(sets[i].TxWrites, &commonPb.TxWrite{
				ContractName: "token", Key: []byte(fmt.Sprintf("balance_%d_%d", i, j)), Value: []byte("100"),
			})
		}
	}
	return sets
}

func TestDisjointWriteDAGSnapshotBoundary(t *testing.T) {
	sets := disjointDAGFixture(2, 2)
	txs := []*commonPb.Transaction{
		{Payload: &commonPb.Payload{TxId: "0"}}, {Payload: &commonPb.Payload{TxId: "1"}},
	}
	s := &SnapshotImpl{sealed: atomic.NewBool(true), txRWSetTable: sets, txTable: txs}
	proposal, count, ok := s.TryBuildDisjointWriteDAG(nil, nil)
	if !ok || count != 4 {
		t.Fatal("proposal inputs rejected")
	}
	verified, _, ok := s.TryBuildDisjointWriteDAG(sets, txs)
	if !ok || !reflect.DeepEqual(proposal, verified) {
		t.Fatal("verification differs")
	}
	if _, _, ok = s.TryBuildDisjointWriteDAG([]*commonPb.TxRWSet{sets[1], sets[0]}, txs); ok {
		t.Fatal("mismatched transaction order accepted")
	}
	s.sealed.Store(false)
	if _, _, ok = s.TryBuildDisjointWriteDAG(nil, nil); ok {
		t.Fatal("unsealed snapshot accepted")
	}
	s.sealed.Store(true)
	s.lock.Lock()
	done := make(chan struct{})
	go func() { s.TryBuildDisjointWriteDAG(nil, nil); close(done) }()
	select {
	case <-done:
		s.lock.Unlock()
		t.Fatal("builder did not acquire snapshot read lock")
	case <-time.After(20 * time.Millisecond):
	}
	s.lock.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("builder did not resume after writer released the lock")
	}
}

func TestDisjointWriteDAGMatchesGeneric(t *testing.T) {
	ctrl := gomock.NewController(t)
	log := protocolmock.NewMockLogger(ctrl)
	log.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	s := &SnapshotImpl{log: log}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 200; i++ {
		sets := disjointDAGFixture(1+rng.Intn(64), rng.Intn(40))
		for _, set := range sets {
			for _, write := range set.TxWrites {
				set.TxReads = append(set.TxReads, &commonPb.TxRead{ContractName: write.ContractName, Key: write.Key})
			}
			set.TxReads = append(set.TxReads, &commonPb.TxRead{Key: []byte("shared_read_only")})
		}
		got, ok := BuildDisjointWriteDAG(sets)
		if !ok {
			t.Fatal("disjoint writes rejected")
		}
		want := s.BuildDAG(false, sets)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("DAG differs at trial %d", i)
		}
		gotBytes, err := proto.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		wantBytes, err := proto.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotBytes, wantBytes) {
			t.Fatal("serialized DAG differs")
		}
	}
}

func TestDisjointWriteDAGFallback(t *testing.T) {
	cases := []struct {
		name string
		edit func([]*commonPb.TxRWSet) []*commonPb.TxRWSet
	}{
		{"empty", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet { return nil }},
		{"nil_rwset", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet { s[0] = nil; return s }},
		{"read_dependency", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet {
			s[1].TxReads = []*commonPb.TxRead{{Key: s[0].TxWrites[0].Key}}
			return s
		}},
		{"nil_write", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet { s[0].TxWrites[0] = nil; return s }},
		{"nil_read", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet { s[0].TxReads = []*commonPb.TxRead{nil}; return s }},
		{"write_after_read", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet {
			s[0].TxReads = []*commonPb.TxRead{{Key: s[1].TxWrites[0].Key}}
			return s
		}},
		{"empty_key", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet { s[0].TxWrites[0].Key = nil; return s }},
		{"overlap", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet {
			s[1].TxWrites[0].Key = s[0].TxWrites[0].Key
			return s
		}},
		{"duplicate_inside_tx", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet {
			s[0].TxWrites[1].Key = s[0].TxWrites[0].Key
			return s
		}},
		{"different_contract_same_key", func(s []*commonPb.TxRWSet) []*commonPb.TxRWSet {
			s[1].TxWrites[0].Key = s[0].TxWrites[0].Key
			s[1].TxWrites[0].ContractName = "other"
			return s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if dag, ok := BuildDisjointWriteDAG(tc.edit(disjointDAGFixture(2, 2))); ok || dag != nil {
				t.Fatal("ineligible inputs must fall back")
			}
		})
	}
}

func BenchmarkBridgeSyncDAG(b *testing.B) {
	sets := disjointDAGFixture(64, 2000)
	ctrl := gomock.NewController(b)
	log := protocolmock.NewMockLogger(ctrl)
	log.EXPECT().Infof(gomock.Any(), gomock.Any()).AnyTimes()
	s := &SnapshotImpl{log: log}
	b.Run("generic", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			s.BuildDAG(false, sets)
		}
	})
	b.Run("disjoint", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, ok := BuildDisjointWriteDAG(sets); !ok {
				b.Fatal("not disjoint")
			}
		}
	})
}

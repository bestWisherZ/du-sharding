package proposer

import (
	"bytes"
	"sync/atomic"
	"testing"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	acpb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	netpb "chainmaker.org/chainmaker/pb-go/v2/net"
	txpoolpb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

type trackerTestPool struct {
	protocol.TxPool
	txs         map[string]*commonpb.Transaction
	lookups     int
	listCalls   int
	afterLookup func()
}

func TestLocalSyncIndexRequiresAdmissionAndRevalidatesAfterCommit(t *testing.T) {
	p := &trackerTestPool{txs: make(map[string]*commonpb.Transaction)}
	var height uint64 = 10
	s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, nil)
	s.currentHeight = func() uint64 { return atomic.LoadUint64(&height) }
	s.observeID("sync-2")
	s.observeID("sync-1")
	s.refresh()
	if len(s.readyIDs()) != 0 {
		t.Fatal("local submission hint bypassed admission")
	}
	p.txs["sync-1"], p.txs["sync-2"] = trackerSyncTx("sync-1"), trackerSyncTx("sync-2")
	s.refresh()
	if ids := s.readyIDs(); len(ids) != 2 || ids[0] != "sync-1" || ids[1] != "sync-2" {
		t.Fatalf("ready IDs = %v", ids)
	}
	atomic.StoreUint64(&height, 11)
	s.OnMessage(&msgbus.Message{Topic: msgbus.BlockInfo, Payload: &commonpb.BlockInfo{
		Block: &commonpb.Block{Txs: []*commonpb.Transaction{p.txs["sync-1"]}},
	}})
	if len(s.wake) != 1 || len(s.readyIDs()) != 0 {
		t.Fatal("height advance did not invalidate and request an index refresh")
	}
	s.refresh()
	if ids := s.readyIDs(); len(ids) != 1 || ids[0] != "sync-2" {
		t.Fatalf("remaining IDs = %v", ids)
	}
}

func TestHintProjectionSkipsLargeOrdinaryPayloads(t *testing.T) {
	tx := &commonpb.Transaction{Payload: &commonpb.Payload{TxId: "ordinary", Method: "Transfer",
		Parameters: []*commonpb.KeyValuePair{{Key: "data", Value: bytes.Repeat([]byte("x"), 100000)}}},
		Sender: &commonpb.EndorsementEntry{Signer: &acpb.Member{MemberInfo: bytes.Repeat([]byte("c"), 10000)}, Signature: []byte("signature")}}
	raw, _ := proto.Marshal(tx)
	var hint syncTxHint
	if err := proto.Unmarshal(raw, &hint); err != nil || hint.Payload == nil || hint.Payload.TxID != "ordinary" {
		t.Fatalf("projection decode failed: %v", err)
	}
	projected, _ := proto.Marshal(&hint)
	if len(projected) > 100 {
		t.Fatal("projection retained signatures or business parameters")
	}
	s := newSpecialSyncTracker(nil, nil, nil, nil, nil)
	s.observeHints([]*syncTxHint{&hint})
	if len(s.ids) != 0 {
		t.Fatal("ordinary transaction was indexed")
	}
}

func (p *trackerTestPool) GetTxIdsByTypeAndStage(_, _ int32) []string {
	p.listCalls++
	ids := make([]string, 0, len(p.txs))
	for id := range p.txs {
		ids = append(ids, id)
	}
	return ids
}

func (p *trackerTestPool) GetTxsInPoolByTxIds(ids []string) ([]*commonpb.Transaction, []string, error) {
	p.lookups++
	var txs []*commonpb.Transaction
	var missing []string
	for _, id := range ids {
		if tx := p.txs[id]; tx != nil {
			txs = append(txs, tx)
		} else {
			missing = append(missing, id)
		}
	}
	if p.afterLookup != nil {
		p.afterLookup()
	}
	return txs, missing, nil
}

func trackerSyncTx(id string) *commonpb.Transaction {
	return &commonpb.Transaction{Payload: &commonpb.Payload{TxId: id, TxType: commonpb.TxType_INVOKE_CONTRACT,
		ContractName: "token_business", Method: "__system_state_sync_apply"}}
}

func TestOrdinaryProposeSignalsNeverInspectPool(t *testing.T) {
	// Any access to the embedded nil pool/ledger panics, including a full scan.
	bp := &BlockProposerImpl{txPool: &trackerTestPool{}, txPoolSignalC: make(chan *txpoolpb.TxPoolSignal, 1)}
	for i := 0; i < 100000; i++ {
		bp.OnReceiveTxPoolSignal(&txpoolpb.TxPoolSignal{SignalType: txpoolpb.SignalType_BLOCK_PROPOSE})
	}
	if len(bp.txPoolSignalC) != 1 {
		t.Fatal("propose requests were not coalesced")
	}
	<-bp.txPoolSignalC
	bp.OnReceiveTxPoolSignal(&txpoolpb.TxPoolSignal{SignalType: txpoolpb.SignalType_BLOCK_PROPOSE})
	if len(bp.txPoolSignalC) != 1 {
		t.Fatal("new arrival after drain was lost")
	}
}

func TestSyncHintRequiresActualPoolAdmissionAndSurvivesRetry(t *testing.T) {
	p := &trackerTestPool{txs: make(map[string]*commonpb.Transaction)}
	var wakes int32
	s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, func() { atomic.AddInt32(&wakes, 1) })
	tx := trackerSyncTx("sync-1")
	s.observe([]*commonpb.Transaction{tx, tx, {Payload: &commonpb.Payload{TxId: "ordinary", Method: "transfer"}}})
	s.refresh()
	if s.hasReady() || wakes != 0 || len(s.ids) != 1 {
		t.Fatal("unvalidated or ordinary transaction released gate")
	}
	p.txs[tx.Payload.TxId] = tx
	s.refresh()
	if !s.hasReady() || wakes != 1 {
		t.Fatal("delayed admission failed to wake a follower")
	}
	delete(p.txs, tx.Payload.TxId)
	s.refresh()
	if s.hasReady() || len(s.ids) != 1 {
		t.Fatal("removed transaction lost retry identity or remained ready")
	}
	p.txs[tx.Payload.TxId] = tx
	s.refresh()
	if !s.hasReady() || wakes != 2 {
		t.Fatal("new proposer retry did not wake consensus")
	}
}

func TestCommitCannotBeOverwrittenByInflightRefresh(t *testing.T) {
	tx := trackerSyncTx("sync-1")
	p := &trackerTestPool{txs: map[string]*commonpb.Transaction{"sync-1": tx}}
	s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, nil)
	s.observe([]*commonpb.Transaction{tx})
	s.refresh()
	p.afterLookup = func() {
		s.OnMessage(&msgbus.Message{Topic: msgbus.BlockInfo, Payload: &commonpb.BlockInfo{Block: &commonpb.Block{Txs: []*commonpb.Transaction{tx}}}})
	}
	s.refresh()
	if s.hasReady() || len(s.ids) != 0 {
		t.Fatal("stale pool query resurrected committed sync transaction")
	}
}

func TestPeerSyncTransactionIsIndexedWithoutRawMessage(t *testing.T) {
	tx := trackerSyncTx("peer-sync")
	for _, kind := range []txpoolpb.TxPoolMsgType{txpoolpb.TxPoolMsgType_SINGLE_TX, txpoolpb.TxPoolMsgType_BATCH_TX, txpoolpb.TxPoolMsgType_RECOVER_RESP} {
		var payload []byte
		switch kind {
		case txpoolpb.TxPoolMsgType_SINGLE_TX:
			payload, _ = proto.Marshal(tx)
		case txpoolpb.TxPoolMsgType_BATCH_TX:
			payload, _ = proto.Marshal(&txpoolpb.TxBatch{BatchId: "batch", Size_: 1, Txs: []*commonpb.Transaction{tx},
				TxIdsMap: map[string]int32{tx.Payload.TxId: 0}, Endorsement: &commonpb.EndorsementEntry{Signature: []byte("sig")}})
		case txpoolpb.TxPoolMsgType_RECOVER_RESP:
			payload, _ = proto.Marshal(&txpoolpb.TxRecoverResponse{NodeId: "node", Height: 11, Txs: []*commonpb.Transaction{tx}})
		}
		raw, _ := proto.Marshal(&txpoolpb.TxPoolMsg{Type: kind, Payload: payload})
		p := &trackerTestPool{txs: map[string]*commonpb.Transaction{"peer-sync": tx}}
		s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, nil)
		s.OnMessage(&msgbus.Message{Topic: msgbus.RecvTxPoolMsg, Payload: &netpb.NetMsg{Payload: raw}})
		s.refresh()
		if !s.hasReady() {
			t.Fatalf("peer sync %s was missed", kind)
		}
	}
}

func TestPoolRecoveryIndexesOnlySyncAndDoesNotRepeatFullScan(t *testing.T) {
	p := &trackerTestPool{txs: map[string]*commonpb.Transaction{
		"sync":     trackerSyncTx("sync"),
		"ordinary": {Payload: &commonpb.Payload{TxId: "ordinary", Method: "transfer"}},
	}}
	s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, nil)
	s.rebuild()
	for i := 0; i < 100; i++ {
		s.refresh()
	}
	if p.listCalls != 1 || len(s.ids) != 1 || !s.hasReady() {
		t.Fatal("pool recovery index or bounded lookup failed")
	}
}

func TestLedgerAdvanceInvalidatesReadyIndexBeforeCommitNotification(t *testing.T) {
	tx := trackerSyncTx("sync")
	p := &trackerTestPool{txs: map[string]*commonpb.Transaction{"sync": tx}}
	s := newSpecialSyncTracker(p, nil, nil, func(*commonpb.Transaction) bool { return true }, nil)
	var height uint64 = 10
	s.currentHeight = func() uint64 { return atomic.LoadUint64(&height) }
	s.observe([]*commonpb.Transaction{tx})
	s.refresh()
	if !s.hasReady() {
		t.Fatal("sync not ready at height 10")
	}
	atomic.StoreUint64(&height, 11)
	if s.hasReady() {
		t.Fatal("stale index authorized another consensus height")
	}
	delete(p.txs, "sync")
	s.refresh()
	if s.hasReady() {
		t.Fatal("committed sync stayed ready")
	}
	p.txs["sync"] = tx
	p.afterLookup = func() { atomic.StoreUint64(&height, 12) }
	s.refresh()
	if s.hasReady() {
		t.Fatal("query overlapping a commit authorized the new height")
	}
}

package proposer

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker/common/v2/msgbus"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	netpb "chainmaker.org/chainmaker/pb-go/v2/net"
	txpoolpb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

// Peer messages only supply candidate IDs. An ID may wake consensus only after
// the real transaction pool has accepted it and its transaction is eligible.
type specialSyncTracker struct {
	pool          protocol.TxPool
	store         protocol.BlockchainStore
	bus           msgbus.MessageBus
	eligible      func(*commonpb.Transaction) bool
	notify        func()
	mu            sync.Mutex
	ids           map[string]bool
	ready         atomic.Value
	readyHeight   uint64
	currentHeight func() uint64
	wake          chan struct{}
	done          chan struct{}
	stopped       sync.Once
}

func newSpecialSyncTracker(pool protocol.TxPool, store protocol.BlockchainStore, bus msgbus.MessageBus,
	eligible func(*commonpb.Transaction) bool, notify func()) *specialSyncTracker {
	return &specialSyncTracker{pool: pool, store: store, bus: bus, eligible: eligible, notify: notify,
		ids: make(map[string]bool), wake: make(chan struct{}, 1), done: make(chan struct{})}
}

func (s *specialSyncTracker) height() uint64 {
	if s.currentHeight != nil {
		return s.currentHeight()
	}
	return 0
}

func (s *specialSyncTracker) hasReady() bool {
	snapshot, ok := s.ready.Load().(syncReadySnapshot)
	return ok && snapshot.ready && snapshot.height == s.height()
}

func (s *specialSyncTracker) readyIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasReady() {
		return nil
	}
	ids := make([]string, 0, len(s.ids))
	for id, accepted := range s.ids {
		if accepted {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

type syncReadySnapshot struct {
	ready  bool
	height uint64
}

func (s *specialSyncTracker) start() {
	if s.bus != nil {
		s.bus.Register(msgbus.RecvTxPoolMsg, s)
		s.bus.Register(msgbus.BlockInfo, s)
	}
	go s.run()
}

func (s *specialSyncTracker) OnQuit() {
	s.stopped.Do(func() {
		close(s.done)
		if s.bus != nil {
			s.bus.UnRegister(msgbus.RecvTxPoolMsg, s)
			s.bus.UnRegister(msgbus.BlockInfo, s)
		}
	})
}

func (s *specialSyncTracker) request() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *specialSyncTracker) observe(txs []*commonpb.Transaction) {
	changed := false
	s.mu.Lock()
	for _, tx := range txs {
		if !scheduler.IsSpecialSyncCandidate(tx) || tx.Payload.TxId == "" {
			continue
		}
		if _, exists := s.ids[tx.Payload.TxId]; !exists {
			s.ids[tx.Payload.TxId] = false
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.request()
	}
}

func (s *specialSyncTracker) observeID(id string) {
	select {
	case <-s.done:
		return
	default:
	}
	if id == "" {
		return
	}
	s.mu.Lock()
	if _, exists := s.ids[id]; !exists {
		s.ids[id] = false
	}
	s.mu.Unlock()
	s.request()
}

func (s *specialSyncTracker) OnMessage(msg *msgbus.Message) {
	select {
	case <-s.done:
		return
	default:
	}
	if msg == nil {
		return
	}
	if msg.Topic == msgbus.BlockInfo {
		info, ok := msg.Payload.(*commonpb.BlockInfo)
		if !ok || info == nil || info.Block == nil {
			return
		}
		s.mu.Lock()
		for _, tx := range info.Block.Txs {
			if tx != nil && tx.Payload != nil {
				delete(s.ids, tx.Payload.TxId)
			}
		}
		s.updateReadyLocked()
		s.mu.Unlock()
		s.request()
		return
	}
	if msg.Topic != msgbus.RecvTxPoolMsg {
		return
	}
	netMsg, ok := msg.Payload.(*netpb.NetMsg)
	if !ok || netMsg == nil {
		return
	}
	var envelope txpoolpb.TxPoolMsg
	if proto.Unmarshal(netMsg.Payload, &envelope) != nil {
		return
	}
	switch envelope.Type {
	case txpoolpb.TxPoolMsgType_SINGLE_TX:
		var tx syncTxHint
		if proto.Unmarshal(envelope.Payload, &tx) == nil {
			s.observeHints([]*syncTxHint{&tx})
		}
	case txpoolpb.TxPoolMsgType_BATCH_TX:
		var batch syncBatchHint
		if proto.Unmarshal(envelope.Payload, &batch) == nil {
			s.observeHints(batch.Txs)
		}
	case txpoolpb.TxPoolMsgType_RECOVER_RESP:
		var response syncRecoverHint
		if proto.Unmarshal(envelope.Payload, &response) == nil {
			s.observeHints(response.Txs)
		}
	}
}

// Protobuf projections skip certificates, signatures, parameters and results.
// Hints still require a subsequent lookup of the validated pool transaction.
type syncPayloadHint struct {
	TxType commonpb.TxType `protobuf:"varint,2,opt,name=tx_type,proto3"`
	TxID   string          `protobuf:"bytes,3,opt,name=tx_id,proto3"`
	Method string          `protobuf:"bytes,7,opt,name=method,proto3"`
}

func (m *syncPayloadHint) Reset()         { *m = syncPayloadHint{} }
func (m *syncPayloadHint) String() string { return proto.CompactTextString(m) }
func (*syncPayloadHint) ProtoMessage()    {}

type syncTxHint struct {
	Payload *syncPayloadHint `protobuf:"bytes,1,opt,name=payload,proto3"`
}

func (m *syncTxHint) Reset()         { *m = syncTxHint{} }
func (m *syncTxHint) String() string { return proto.CompactTextString(m) }
func (*syncTxHint) ProtoMessage()    {}

type syncBatchHint struct {
	Txs []*syncTxHint `protobuf:"bytes,4,rep,name=txs,proto3"`
}

func (m *syncBatchHint) Reset()         { *m = syncBatchHint{} }
func (m *syncBatchHint) String() string { return proto.CompactTextString(m) }
func (*syncBatchHint) ProtoMessage()    {}

type syncRecoverHint struct {
	Txs []*syncTxHint `protobuf:"bytes,3,rep,name=txs,proto3"`
}

func (m *syncRecoverHint) Reset()         { *m = syncRecoverHint{} }
func (m *syncRecoverHint) String() string { return proto.CompactTextString(m) }
func (*syncRecoverHint) ProtoMessage()    {}

func (s *specialSyncTracker) observeHints(hints []*syncTxHint) {
	for _, hint := range hints {
		if hint == nil || hint.Payload == nil {
			continue
		}
		p := hint.Payload
		tx := &commonpb.Transaction{Payload: &commonpb.Payload{TxType: p.TxType, Method: p.Method}}
		if scheduler.IsSpecialSyncCandidate(tx) {
			s.observeID(p.TxID)
		}
	}
}

func (s *specialSyncTracker) rebuild() {
	// A one-time rebuild covers transactions restored from the pool's dump.
	if s.pool != nil {
		ids := s.pool.GetTxIdsByTypeAndStage(int32(txpoolpb.TxType_COMMON_TX), int32(txpoolpb.TxStage_ALL_STAGE))
		for start := 0; start < len(ids); start += 1000 {
			end := start + 1000
			if end > len(ids) {
				end = len(ids)
			}
			txs, _, err := s.pool.GetTxsInPoolByTxIds(ids[start:end])
			if err == nil {
				s.observe(txs)
			}
		}
	}
}

func (s *specialSyncTracker) run() {
	s.rebuild()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-s.wake:
			s.refresh()
		case <-ticker.C:
			s.refresh()
		}
	}
}

func (s *specialSyncTracker) refresh() {
	height := s.height()
	s.mu.Lock()
	ids := make([]string, 0, len(s.ids))
	for id := range s.ids {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	if len(ids) == 0 || s.pool == nil {
		return
	}
	accepted := make(map[string]bool, len(ids))
	committed := make(map[string]bool)
	for start := 0; start < len(ids); start += 1000 {
		end := start + 1000
		if end > len(ids) {
			end = len(ids)
		}
		txs, missing, err := s.pool.GetTxsInPoolByTxIds(ids[start:end])
		if err != nil {
			return
		}
		for _, tx := range txs {
			if tx != nil && tx.Payload != nil && s.eligible(tx) {
				accepted[tx.Payload.TxId] = true
			}
		}
		if s.store != nil {
			for _, id := range missing {
				exists, err := s.store.TxExists(id)
				if err == nil && exists {
					committed[id] = true
				}
			}
		}
	}
	s.mu.Lock()
	wasReady := s.hasReady()
	for _, id := range ids {
		if committed[id] {
			delete(s.ids, id)
			continue
		}
		// A concurrent commit may already have removed this ID.
		if _, exists := s.ids[id]; exists {
			s.ids[id] = accepted[id]
		}
	}
	// Pool queries from an older ledger height must never authorize the next
	// height before the just-committed transactions have been removed.
	s.readyHeight = height
	s.updateReadyLocked()
	ready := s.hasReady()
	s.mu.Unlock()
	if ready && !wasReady && s.notify != nil {
		s.notify()
	}
}

func (s *specialSyncTracker) updateReadyLocked() {
	ready := false
	for _, accepted := range s.ids {
		if accepted {
			ready = true
			break
		}
	}
	s.ready.Store(syncReadySnapshot{ready: ready, height: s.readyHeight})
}

package scheduler

import (
	"sync"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

// NewTBFTBlockInfoGateMessageBus wraps the real message bus for TBFT only.
// Other modules still subscribe to the real bus directly. The wrapper delays
// TBFT's BlockInfo notification while the local shard is not allowed to enter
// the next consensus height, which avoids non-current-round proposing timeouts.
func NewTBFTBlockInfoGateMessageBus(real msgbus.MessageBus) msgbus.MessageBus {
	if real == nil {
		return nil
	}
	gate := &tbftBlockInfoGate{
		real:     real,
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	RegisterReadyListener(gate.Flush)
	go gate.run()
	return &tbftBlockInfoGateMessageBus{
		real: real,
		gate: gate,
	}
}

type tbftBlockInfoGateMessageBus struct {
	real msgbus.MessageBus
	gate *tbftBlockInfoGate
}

func (b *tbftBlockInfoGateMessageBus) Register(topic msgbus.Topic, sub msgbus.Subscriber) {
	if topic == msgbus.BlockInfo {
		b.gate.register(sub)
		return
	}
	b.real.Register(topic, sub)
}

func (b *tbftBlockInfoGateMessageBus) UnRegister(topic msgbus.Topic, sub msgbus.Subscriber) {
	if topic == msgbus.BlockInfo {
		b.gate.unregister(sub)
		return
	}
	b.real.UnRegister(topic, sub)
}

func (b *tbftBlockInfoGateMessageBus) Publish(topic msgbus.Topic, payload interface{}) {
	b.real.Publish(topic, payload)
}

func (b *tbftBlockInfoGateMessageBus) PublishSafe(topic msgbus.Topic, payload interface{}) {
	b.real.PublishSafe(topic, payload)
}

func (b *tbftBlockInfoGateMessageBus) PublishSync(topic msgbus.Topic, payload interface{}) {
	b.real.PublishSync(topic, payload)
}

func (b *tbftBlockInfoGateMessageBus) Close() {
	b.real.Close()
}

type tbftBlockInfoGate struct {
	real msgbus.MessageBus

	mu         sync.Mutex
	registered bool
	subs       []msgbus.Subscriber
	pending    []*msgbus.Message
	wake       chan struct{}
	done       chan struct{}
	finished   chan struct{}
	stopOnce   sync.Once
}

func (g *tbftBlockInfoGate) register(sub msgbus.Subscriber) {
	if sub == nil {
		return
	}
	g.mu.Lock()
	if !g.registered {
		g.real.Register(msgbus.BlockInfo, g)
		g.registered = true
	}
	for _, existing := range g.subs {
		if existing == sub {
			g.mu.Unlock()
			return
		}
	}
	g.subs = append(g.subs, sub)
	g.mu.Unlock()
	g.Flush()
}

func (g *tbftBlockInfoGate) unregister(sub msgbus.Subscriber) {
	if sub == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, existing := range g.subs {
		if existing == sub {
			g.subs = append(g.subs[:i], g.subs[i+1:]...)
			return
		}
	}
}

func (g *tbftBlockInfoGate) OnMessage(msg *msgbus.Message) {
	if msg == nil || msg.Topic != msgbus.BlockInfo {
		return
	}
	g.mu.Lock()
	g.pending = append(g.pending, msg)
	g.mu.Unlock()
	g.Flush()
}

func (g *tbftBlockInfoGate) OnQuit() {
	g.stopOnce.Do(func() { close(g.done) })
	g.mu.Lock()
	subs := append([]msgbus.Subscriber(nil), g.subs...)
	g.subs = nil
	g.pending = nil
	g.mu.Unlock()
	for _, sub := range subs {
		sub.OnQuit()
	}
}

func (g *tbftBlockInfoGate) Flush() {
	select {
	case <-g.done:
		return
	default:
	}
	select {
	case g.wake <- struct{}{}:
	default:
	}
}

func (g *tbftBlockInfoGate) run() {
	defer close(g.finished)
	for {
		select {
		case <-g.done:
			return
		case <-g.wake:
			g.drain()
		}
	}
}

func (g *tbftBlockInfoGate) drain() {
	for {
		msg, subs, forwarded, nextHeight, reason := g.nextForwardable()
		if !forwarded {
			if nextHeight > 0 && log != nil {
				log.Debugf("[Scheduler] TBFT BlockInfo gate paused | next_height=%d reason=%s", nextHeight, reason)
			}
			return
		}
		if nextHeight > 0 && log != nil {
			log.Infof("[Scheduler] TBFT BlockInfo gate forward | next_height=%d reason=%s", nextHeight, reason)
		}
		for _, sub := range subs {
			sub.OnMessage(msg)
		}
	}
}

func (g *tbftBlockInfoGate) nextForwardable() (*msgbus.Message, []msgbus.Subscriber, bool, uint64, string) {
	g.mu.Lock()
	if len(g.pending) == 0 || len(g.subs) == 0 {
		g.mu.Unlock()
		return nil, nil, false, 0, "empty"
	}

	msg := g.pending[0]
	g.mu.Unlock()
	// Only the drain worker removes messages. Never hold the queue lock while
	// consulting another module, and recheck the head after shutdown/unregister.
	allowed, nextHeight, reason := allowTBFTBlockInfo(msg)
	if !allowed {
		return nil, nil, false, nextHeight, reason
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.pending) == 0 || g.pending[0] != msg || len(g.subs) == 0 {
		return nil, nil, false, 0, "empty"
	}
	g.pending = g.pending[1:]
	subs := append([]msgbus.Subscriber(nil), g.subs...)
	return msg, subs, true, nextHeight, reason
}

func allowTBFTBlockInfo(msg *msgbus.Message) (bool, uint64, string) {
	nextHeight, ok := nextHeightFromBlockInfo(msg)
	if !ok {
		return true, 0, "unknown_blockinfo"
	}
	if ShouldProduce(nextHeight) {
		return true, nextHeight, "scheduled_phase"
	}
	if MayProduceSpecial(nextHeight) && HasSpecialPending() {
		return true, nextHeight, "special_sync_pending"
	}
	return false, nextHeight, "waiting_phase_or_special_sync"
}

func nextHeightFromBlockInfo(msg *msgbus.Message) (uint64, bool) {
	if msg == nil {
		return 0, false
	}
	blockInfo, ok := msg.Payload.(*commonpb.BlockInfo)
	if !ok || blockInfo == nil || blockInfo.Block == nil || blockInfo.Block.Header == nil {
		return 0, false
	}
	return blockInfo.Block.Header.BlockHeight + 1, true
}

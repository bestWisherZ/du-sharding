package scheduler

import (
	"sync/atomic"
	"testing"
	"time"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

type gateTestSubscriber struct {
	msgbus.DefaultSubscriber
	onMessage func(*msgbus.Message)
}

func (s *gateTestSubscriber) OnMessage(m *msgbus.Message) { s.onMessage(m) }

func gateTestMessage(h uint64) *msgbus.Message {
	return &msgbus.Message{Topic: msgbus.BlockInfo, Payload: &commonpb.BlockInfo{
		Block: &commonpb.Block{Header: &commonpb.BlockHeader{BlockHeight: h}},
	}}
}

func newTestGate(t *testing.T, checker func() bool, receive func(*msgbus.Message)) *tbftBlockInfoGate {
	t.Helper()
	oldGlobal, oldCheckers := global, specialPendingFns
	global = &Scheduler{cfg: Config{Enable: true, Role: roleBridge, StartHeight: 10},
		active: true, phase: roleBusiness}
	specialPendingFns = []func() bool{checker}
	g := &tbftBlockInfoGate{wake: make(chan struct{}, 1), done: make(chan struct{}), finished: make(chan struct{}),
		subs: []msgbus.Subscriber{&gateTestSubscriber{onMessage: receive}}}
	go g.run()
	t.Cleanup(func() {
		g.OnQuit()
		select {
		case <-g.finished:
		case <-time.After(time.Second):
			t.Error("gate worker did not stop")
		}
		global, specialPendingFns = oldGlobal, oldCheckers
	})
	return g
}

func TestGateDoesNotHoldQueueLockDuringReadinessCheck(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls int32
	var ready int32
	received := make(chan uint64, 4)
	g := newTestGate(t, func() bool {
		if atomic.AddInt32(&calls, 1) == 1 {
			close(entered)
			<-release
			return false
		}
		return atomic.LoadInt32(&ready) == 1
	}, func(m *msgbus.Message) { received <- m.Payload.(*commonpb.BlockInfo).Block.Header.BlockHeight })
	g.OnMessage(gateTestMessage(10))
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("check did not start")
	}
	enqueued := make(chan struct{})
	go func() {
		g.OnMessage(gateTestMessage(11))
		for i := 0; i < 10000; i++ {
			g.Flush()
		}
		close(enqueued)
	}()
	select {
	case <-enqueued:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("readiness check held queue lock")
	}
	atomic.StoreInt32(&ready, 1)
	g.Flush()
	close(release)
	for _, want := range []uint64{10, 11} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("got height %d, want %d", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("coalescing lost a readiness notification")
		}
	}
	select {
	case h := <-received:
		t.Fatalf("duplicate block %d", h)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestGateSerializesForwardingAndPreservesPausedMessages(t *testing.T) {
	var ready int32
	entered, release := make(chan struct{}), make(chan struct{})
	received := make(chan uint64, 4)
	g := newTestGate(t, func() bool { return atomic.LoadInt32(&ready) == 1 }, func(m *msgbus.Message) {
		h := m.Payload.(*commonpb.BlockInfo).Block.Header.BlockHeight
		if h == 10 {
			close(entered)
			<-release
		}
		received <- h
	})
	g.OnMessage(gateTestMessage(10))
	select {
	case <-entered:
		t.Fatal("paused gate forwarded")
	case <-time.After(20 * time.Millisecond):
	}
	atomic.StoreInt32(&ready, 1)
	g.Flush()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("gate did not resume")
	}
	g.OnMessage(gateTestMessage(11))
	for i := 0; i < 1000; i++ {
		g.Flush()
	}
	select {
	case h := <-received:
		close(release)
		t.Fatalf("out-of-order forward %d", h)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	for _, want := range []uint64{10, 11} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("missing BlockInfo")
		}
	}
}

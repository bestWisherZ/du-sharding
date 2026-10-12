package syncproposer

import (
	"sync/atomic"
	"testing"
	"time"
)

func resetCoordinatorForTest() {
	state.Lock()
	state.localProposer = false
	state.forceFlush = false
	state.listeners = nil
	state.flushListeners = nil
	state.checkers = nil
	state.wake = nil
	state.proposalReady = nil
	state.txSubmitted = nil
	state.batchPreparing = false
	state.readyBatchTxs = nil
	state.Unlock()
}

func TestLocalSubmittedTransactionIsIndexed(t *testing.T) {
	resetCoordinatorForTest()
	defer resetCoordinatorForTest()
	var got []string
	RegisterTxSubmittedListener(func(id string) { got = append(got, id) })
	NotifyTxSubmitted("")
	NotifyTxSubmitted("sync-1")
	if len(got) != 1 || got[0] != "sync-1" {
		t.Fatalf("submitted IDs = %v", got)
	}
}

func TestProposerListenerOnlyRunsOnStateChange(t *testing.T) {
	resetCoordinatorForTest()
	defer resetCoordinatorForTest()

	statusC := make(chan bool, 2)
	RegisterListener(func(active bool) {
		statusC <- active
	})

	SetLocalProposer(true)
	select {
	case active := <-statusC:
		if !active {
			t.Fatal("expected active proposer notification")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proposer notification")
	}

	SetLocalProposer(true)
	select {
	case <-statusC:
		t.Fatal("unchanged proposer state must not notify listeners")
	case <-time.After(20 * time.Millisecond):
	}

	SetLocalProposer(false)
	select {
	case active := <-statusC:
		if active {
			t.Fatal("expected inactive proposer notification")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for non-proposer notification")
	}
}

func TestPendingTaskWakesConsensusGate(t *testing.T) {
	resetCoordinatorForTest()
	defer resetCoordinatorForTest()

	var pending int32
	var wakes int32
	RegisterPendingChecker(func() bool { return atomic.LoadInt32(&pending) == 1 })
	SetWakeFunc(func() { atomic.AddInt32(&wakes, 1) })

	if HasPending() {
		t.Fatal("unexpected pending task")
	}
	atomic.StoreInt32(&pending, 1)
	if !HasPending() {
		t.Fatal("pending task was not exposed")
	}
	NotifyPending()
	if atomic.LoadInt32(&wakes) != 1 {
		t.Fatalf("wake count = %d, want 1", atomic.LoadInt32(&wakes))
	}
}

func TestForceFlushStaysEnabledUntilSchedulerClearsIt(t *testing.T) {
	resetCoordinatorForTest()
	defer resetCoordinatorForTest()

	statusC := make(chan bool, 2)
	var wakes int32
	RegisterFlushListener(func(active bool) {
		statusC <- active
	})
	SetWakeFunc(func() { atomic.AddInt32(&wakes, 1) })

	SetForceFlush(true)
	select {
	case active := <-statusC:
		if !active {
			t.Fatal("expected force flush notification")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for force flush notification")
	}
	if !ForceFlushEnabled() {
		t.Fatal("force flush was not kept active")
	}
	if atomic.LoadInt32(&wakes) != 1 {
		t.Fatalf("wake count = %d, want 1", atomic.LoadInt32(&wakes))
	}

	SetForceFlush(false)
	select {
	case active := <-statusC:
		if active {
			t.Fatal("expected force flush clear notification")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for force flush clear notification")
	}
	if ForceFlushEnabled() {
		t.Fatal("force flush remained enabled")
	}
}

func TestReadyBatchIsSortedAndDrained(t *testing.T) {
	resetCoordinatorForTest()
	defer resetCoordinatorForTest()

	readyC := make(chan struct{}, 2)
	RegisterProposalReadyListener(func() { readyC <- struct{}{} })
	SetBatchPreparing(true)
	if !BatchPreparing() {
		t.Fatal("batch preparation was not exposed")
	}
	PublishReadyBatch([]string{"tx-c", "tx-a", "tx-b", "tx-a", ""})
	select {
	case <-readyC:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ready-batch notification")
	}
	if BatchPreparing() {
		t.Fatal("batch remained in preparing state")
	}
	want := []string{"tx-a", "tx-b", "tx-c"}
	got := ReadyBatchTxIds()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ready ids = %v, want %v", got, want)
		}
	}

	MarkBatchTxsCommitted([]string{"tx-a", "tx-c"})
	got = ReadyBatchTxIds()
	if len(got) != 1 || got[0] != "tx-b" {
		t.Fatalf("remaining ids = %v, want [tx-b]", got)
	}
	MarkBatchTxsCommitted([]string{"tx-b"})
	if got = ReadyBatchTxIds(); len(got) != 0 {
		t.Fatalf("ready batch not drained: %v", got)
	}
}

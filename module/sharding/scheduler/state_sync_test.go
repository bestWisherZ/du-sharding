package scheduler

import (
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
)

func TestBridgePhaseWaitsForAllStateCheckpoints(t *testing.T) {
	s := &Scheduler{
		cfg:   Config{Role: roleBridge},
		phase: roleBusiness,
		expectedBusinessShards: map[string]struct{}{
			"business-shard1": {},
			"business-shard2": {},
		},
		businessDone: map[string]bool{
			"business-shard1": true,
			"business-shard2": true,
		},
		requiredSyncHeight: map[string]uint64{
			"business-shard1": 15,
			"business-shard2": 15,
		},
		appliedSyncHeight: map[string]uint64{
			"business-shard1": 15,
			"business-shard2": 14,
		},
	}
	if s.tryEnterBridgePhaseLocked() {
		t.Fatal("bridge phase started before every state checkpoint committed")
	}
	s.appliedSyncHeight["business-shard2"] = 15
	syncproposer.SetForceFlush(true)
	if !s.tryEnterBridgePhaseLocked() {
		t.Fatal("bridge phase did not start after every state checkpoint committed")
	}
	if s.phase != roleBridge || !s.canProduce {
		t.Fatalf("unexpected scheduler state: phase=%s canProduce=%v", s.phase, s.canProduce)
	}
	if syncproposer.ForceFlushEnabled() {
		t.Fatal("force flush remained enabled after state-sync barrier completed")
	}
}

func TestStateCheckpointHeightAdvancesOnlyContiguously(t *testing.T) {
	s := &Scheduler{
		appliedSyncHeight: map[string]uint64{
			"business-shard1": 10,
		},
		completedSyncHeights: map[string]map[uint64]struct{}{
			"business-shard1": {},
		},
	}
	s.markStateCheckpointCompleteLocked("business-shard1", 12)
	if got := s.appliedSyncHeight["business-shard1"]; got != 10 {
		t.Fatalf("out-of-order checkpoint advanced height to %d", got)
	}
	s.markStateCheckpointCompleteLocked("business-shard1", 11)
	if got := s.appliedSyncHeight["business-shard1"]; got != 12 {
		t.Fatalf("contiguous checkpoints advanced height to %d, want 12", got)
	}
}

func TestBridgePhaseWaitsForExactNormalCheckpoints(t *testing.T) {
	s := &Scheduler{
		cfg:   Config{Role: roleBridge, BusinessBatch: 3},
		phase: roleBusiness,
		expectedBusinessShards: map[string]struct{}{
			"business-shard1": {},
		},
		businessDone: map[string]bool{
			"business-shard1": true,
		},
		requiredSyncHeight: map[string]uint64{
			"business-shard1": 16,
		},
		requiredSyncHeights: map[string]map[uint64]struct{}{
			"business-shard1": heightSet([]uint64{11, 14, 16}),
		},
		appliedSyncHeight: map[string]uint64{
			"business-shard1": 10,
		},
		completedSyncHeights: map[string]map[uint64]struct{}{
			"business-shard1": heightSet([]uint64{11, 16}),
		},
	}
	if s.tryEnterBridgePhaseLocked() {
		t.Fatal("bridge phase started with required checkpoint 14 missing")
	}
	s.completedSyncHeights["business-shard1"][14] = struct{}{}
	syncproposer.SetForceFlush(true)
	if !s.tryEnterBridgePhaseLocked() {
		t.Fatal("bridge phase did not start after exact normal checkpoints committed")
	}
	if got := s.appliedSyncHeight["business-shard1"]; got != 16 {
		t.Fatalf("covered physical height = %d, want 16", got)
	}
	if len(s.completedSyncHeights["business-shard1"]) != 0 {
		t.Fatalf("consumed checkpoints were not pruned: %v", s.completedSyncHeights["business-shard1"])
	}
}

func TestParseBusinessDoneWithSyncHeights(t *testing.T) {
	shard, height, pending, heights, ok := parseBusinessDone(
		"SCHED_BUSINESS_DONE:business-shard1:16:pending=320:sync_heights=11,14,16")
	if !ok {
		t.Fatal("new business-done format was not parsed")
	}
	if shard != "business-shard1" || height != 16 || pending != 320 {
		t.Fatalf("unexpected parsed values: shard=%s height=%d pending=%d", shard, height, pending)
	}
	want := []uint64{11, 14, 16}
	if len(heights) != len(want) {
		t.Fatalf("sync heights = %v, want %v", heights, want)
	}
	for i := range want {
		if heights[i] != want[i] {
			t.Fatalf("sync heights = %v, want %v", heights, want)
		}
	}
}

func TestParseBusinessDoneLegacyFormats(t *testing.T) {
	tests := []struct {
		message string
		shard   string
		height  uint64
		pending int
	}{
		{
			message: "SCHED_BUSINESS_DONE:business-shard1:16:pending=320",
			shard:   "business-shard1",
			height:  16,
			pending: 320,
		},
		{
			message: "SCHED_BUSINESS_DONE:16:pending=320",
			height:  16,
			pending: 320,
		},
	}
	for _, test := range tests {
		shard, height, pending, heights, ok := parseBusinessDone(test.message)
		if !ok {
			t.Fatalf("legacy business-done message was not parsed: %s", test.message)
		}
		if shard != test.shard || height != test.height || pending != test.pending {
			t.Fatalf("unexpected parsed values for %s: shard=%s height=%d pending=%d",
				test.message, shard, height, pending)
		}
		if len(heights) != 0 {
			t.Fatalf("legacy message unexpectedly produced sync heights: %v", heights)
		}
	}
}

func TestParseBusinessDoneRejectsInvalidSyncHeights(t *testing.T) {
	invalid := []string{
		"SCHED_BUSINESS_DONE:business-shard1:16:sync_heights=11,11,16",
		"SCHED_BUSINESS_DONE:business-shard1:16:sync_heights=14,15",
		"SCHED_BUSINESS_DONE:business-shard1:16:sync_heights=",
	}
	for _, msg := range invalid {
		if _, _, _, _, ok := parseBusinessDone(msg); ok {
			t.Fatalf("invalid business-done message accepted: %s", msg)
		}
	}
}

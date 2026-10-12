package shardingsync

import "testing"

func TestBinaryDeltaLogSummaryDoesNotExposePayload(t *testing.T) {
	msg := "GLOBAL_STATE_DELTA_V3:business-shard1:\x00\xff:\nsecret"
	kind, shard, ok := messageSummary(msg)
	if !ok || kind != "GLOBAL_STATE_DELTA_V3" || shard != "business-shard1" {
		t.Fatalf("invalid binary message summary: %q %q %v", kind, shard, ok)
	}
}

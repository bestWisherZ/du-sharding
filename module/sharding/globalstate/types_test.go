package globalstate

import "testing"

func TestCompactDeltaMessagePreservesWavePosition(t *testing.T) {
	delta := &CompactDelta{
		StartHeight: 41,
		Height:      41,
		BlockHash:   "block-hash",
		Timestamp:   12345,
		Cycle:       7,
		Slot:        3,
		SlotTotal:   5,
		Checkpoint:  true,
		Writes: []DeltaWrite{{
			Contract: "token",
			Key:      []byte("alice"),
			Value:    []byte("100"),
		}},
	}
	msg, err := BuildCompactDeltaMessage("business-shard1", delta)
	if err != nil {
		t.Fatalf("build compact delta: %v", err)
	}
	shard, parsed, ok := ParseCompactDeltaMessage(msg)
	if !ok {
		t.Fatal("parse compact delta failed")
	}
	if shard != "business-shard1" {
		t.Fatalf("shard = %s", shard)
	}
	if parsed.Cycle != 7 || parsed.Slot != 3 || parsed.SlotTotal != 5 {
		t.Fatalf("wave position = %d/%d/%d, want 7/3/5",
			parsed.Cycle, parsed.Slot, parsed.SlotTotal)
	}
}

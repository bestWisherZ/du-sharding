package scheduler

import (
	"testing"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

func TestBusinessCheckpointPositionAdvancesByCountedBlocks(t *testing.T) {
	oldGlobal := global
	global = &Scheduler{
		cfg: Config{
			Enable:        true,
			Role:          roleBusiness,
			StartHeight:   10,
			BusinessBatch: 2,
		},
		canProduce:          true,
		active:              true,
		phase:               roleBusiness,
		businessCycle:       4,
		businessSyncHeights: make([]uint64, 0, 2),
	}
	defer func() { global = oldGlobal }()

	OnBlockCommitted(&commonpb.Block{Header: &commonpb.BlockHeader{BlockHeight: 11}})
	cycle, slot, total, ok := BusinessCheckpointPosition(11)
	if !ok || cycle != 4 || slot != 1 || total != 2 {
		t.Fatalf("first checkpoint = %d/%d/%d ok=%v, want 4/1/2",
			cycle, slot, total, ok)
	}
	OnBlockCommitted(&commonpb.Block{Header: &commonpb.BlockHeader{BlockHeight: 12}})
	cycle, slot, total, ok = BusinessCheckpointPosition(12)
	if !ok || cycle != 4 || slot != 2 || total != 2 {
		t.Fatalf("second checkpoint = %d/%d/%d ok=%v, want 4/2/2",
			cycle, slot, total, ok)
	}
}

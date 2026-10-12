package crossshard

import (
	"reflect"
	"testing"
)

func TestApplyFlushKeysPreserveBridgeHeightOrder(t *testing.T) {
	pendingApply.Lock()
	oldTasks := pendingApply.tasks
	pendingApply.tasks = map[string]*pendingApplyWrite{
		"h21":    applyPendingTestTask("bridge-21-business-shard1", 21, 0),
		"h20-c1": applyPendingTestTask("bridge-20-business-shard1", 20, 1),
		"h20-c0": applyPendingTestTask("bridge-20-business-shard1", 20, 0),
	}
	defer func() {
		pendingApply.tasks = oldTasks
		pendingApply.Unlock()
	}()

	got := applyFlushKeysLocked()
	want := []string{"h20-c0", "h20-c1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flush keys = %v, want %v", got, want)
	}
}

func applyPendingTestTask(batchId string, height uint64, chunkIndex int) *pendingApplyWrite {
	return &pendingApplyWrite{
		chunk: &applyWriteChunk{
			batch: &CrossWriteBatch{
				BatchId:           batchId,
				BridgeBlockHeight: height,
			},
			contractName: "token",
			chunkIndex:   chunkIndex,
			chunkTotal:   2,
		},
	}
}

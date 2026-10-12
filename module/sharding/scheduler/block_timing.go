/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"strconv"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

const (
	blockProduceStartMillisKey = "__sharding_produce_start_millis"
	blockProduceSpecialKey     = "__sharding_produce_special"
)

// AttachBlockProduceTiming carries the proposer-side packaging start to every
// validator so each node can calculate the same block timing boundary locally.
func AttachBlockProduceTiming(block *commonpb.Block, startMillis int64, special bool) {
	if block == nil || startMillis <= 0 {
		return
	}
	if block.AdditionalData == nil {
		block.AdditionalData = &commonpb.AdditionalData{}
	}
	if block.AdditionalData.ExtraData == nil {
		block.AdditionalData.ExtraData = make(map[string][]byte)
	}
	block.AdditionalData.ExtraData[blockProduceStartMillisKey] =
		[]byte(strconv.FormatInt(startMillis, 10))
	block.AdditionalData.ExtraData[blockProduceSpecialKey] =
		[]byte(strconv.FormatBool(special))
}

func readBlockProduceTiming(block *commonpb.Block) (int64, bool, bool) {
	if block == nil || block.AdditionalData == nil || block.AdditionalData.ExtraData == nil {
		return 0, false, false
	}
	rawStart := block.AdditionalData.ExtraData[blockProduceStartMillisKey]
	startMillis, err := strconv.ParseInt(string(rawStart), 10, 64)
	if err != nil || startMillis <= 0 {
		return 0, false, false
	}
	special, _ := strconv.ParseBool(string(block.AdditionalData.ExtraData[blockProduceSpecialKey]))
	return startMillis, special, true
}

// RecordBlockCommitComplete emits a complete timing pair on every validator.
// The start comes from the proposed block; the commit point uses this node's
// clock after storage, txpool cleanup, and BlockInfo publication have finished.
func RecordBlockCommitComplete(block *commonpb.Block, commitMillis int64) {
	if global == nil || !global.cfg.Enable || block == nil || block.Header == nil || commitMillis <= 0 {
		return
	}
	startMillis, special, ok := readBlockProduceTiming(block)
	if !ok || block.Header.BlockHeight <= global.cfg.StartHeight {
		return
	}
	durationMillis := commitMillis - startMillis

	global.mu.Lock()
	defer global.mu.Unlock()
	global.logMetricLocked("block_commit_complete",
		"height", block.Header.BlockHeight,
		"start_ms", startMillis,
		"commit_ms", commitMillis,
		"duration_ms", durationMillis,
		"special", special,
	)
}

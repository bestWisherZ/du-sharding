/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scheduler

import (
	"testing"

	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/gogo/protobuf/proto"
)

func TestAttachBlockProduceTiming(t *testing.T) {
	block := &commonpb.Block{}
	AttachBlockProduceTiming(block, 123456789, true)

	startMillis, special, ok := readBlockProduceTiming(block)
	if !ok {
		t.Fatal("expected block timing metadata")
	}
	if startMillis != 123456789 {
		t.Fatalf("unexpected start millis: %d", startMillis)
	}
	if !special {
		t.Fatal("expected special block marker")
	}
}

func TestReadBlockProduceTimingRejectsMissingMetadata(t *testing.T) {
	if _, _, ok := readBlockProduceTiming(&commonpb.Block{}); ok {
		t.Fatal("missing timing metadata must not be accepted")
	}
}

func TestBlockProduceTimingSurvivesProposalSerialization(t *testing.T) {
	block := &commonpb.Block{Header: &commonpb.BlockHeader{BlockHeight: 11}}
	AttachBlockProduceTiming(block, 123456789, false)

	raw, err := proto.Marshal(block)
	if err != nil {
		t.Fatalf("marshal block: %v", err)
	}
	received := &commonpb.Block{}
	if err := proto.Unmarshal(raw, received); err != nil {
		t.Fatalf("unmarshal block: %v", err)
	}
	startMillis, special, ok := readBlockProduceTiming(received)
	if !ok || startMillis != 123456789 || special {
		t.Fatalf("unexpected received timing: start=%d special=%v ok=%v", startMillis, special, ok)
	}
}

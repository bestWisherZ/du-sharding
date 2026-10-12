/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package sync

import (
	"encoding/hex"
	"testing"
	"time"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	netPb "chainmaker.org/chainmaker/pb-go/v2/net"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func newMockChainConf(ctrl *gomock.Controller, version string) protocol.ChainConf {
	conf := &config.ChainConfig{
		Version: version,
	}
	chainConf := mock.NewMockChainConf(ctrl)
	chainConf.EXPECT().ChainConfig().Return(conf).AnyTimes()
	return chainConf
}

func TestBlockChainSyncAggregator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	block := &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 10}}
	mockNet := newMockNet(ctrl)
	mockMsgBus := newMockMessageBus(ctrl)
	mockVerify := newMockVerifier(ctrl)
	mockStore := newMockBlockChainStore(ctrl)
	mockLedger := newMockLedgerCache(ctrl, block)
	mockCommit := newMockCommitter(ctrl, mockLedger)
	chainConf := newMockChainConf(ctrl, "2030100")
	mockStore.PutBlock(block, nil)
	log := &test.GoLogger{}
	aggregator := NewBlockChainSyncServer(
		"chain1",
		mockNet,
		mockMsgBus,
		mockStore,
		mockLedger,
		chainConf,
		mockVerify,
		mockCommit,
		log,
	)
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(100 * time.Millisecond)
			version := "2030200"
			if i == 2 {
				version = "2040001"
			}
			newConf := &config.ChainConfig{
				Version: version,
			}
			ss := []string(nil)
			bz, _ := proto.Marshal(newConf)
			ss = append(ss, hex.EncodeToString(bz))
			mockMsgBus.Publish(msgbus.ChainConfig, ss)
		}
	}()
	err := aggregator.Start()
	require.Nil(t, err)
	aggregator.StopBlockSync()
	time.Sleep(500 * time.Millisecond)
	aggregator.Stop()
}

func TestBlockChainSyncAggregatorV1ToV2(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockNet := newMockNet(ctrl)
	mockMsgBus := newMockMessageBus(ctrl)
	mockVerify := newMockVerifier(ctrl)

	block240 := &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 10}}
	mockStore240 := newMockBlockChainStore(ctrl)
	mockLedger240 := newMockLedgerCache(ctrl, block240)
	mockCommit240 := newMockCommitter(ctrl, mockLedger240)
	chainConf240 := newMockChainConf(ctrl, "2040001")
	mockStore240.PutBlock(block240, nil)
	log := &test.GoLogger{}
	syncSvc := NewBlockChainSyncServer(
		"chain1",
		mockNet,
		mockMsgBus,
		mockStore240,
		mockLedger240,
		chainConf240,
		mockVerify,
		mockCommit240,
		log,
	)
	err := syncSvc.Start()
	require.Nil(t, err)
	aggregator := syncSvc.(*ServerAggregator)
	syncV2 := aggregator.SyncService.(*BlockSyncServiceV2)
	err = syncV2.netHandler.netMessageHandle("node2", getNodeStatusReq(t), netPb.NetMsg_SYNC_BLOCK_MSG)
	require.Nil(t, err)
	err = syncV2.netHandler.netMessageHandle("node2", getNodeStatusResp(t, 21), netPb.NetMsg_SYNC_BLOCK_MSG)
	require.Nil(t, err)
	err = syncV2.netHandler.netMessageHandle("node2", getBlockReq(t, 10, 1), netPb.NetMsg_SYNC_BLOCK_MSG)
	require.Nil(t, err)
}

func TestA(t *testing.T) {
	t.Log(utils.GetBlockVersion("v2.4.0"))
}

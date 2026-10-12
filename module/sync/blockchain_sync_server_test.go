/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sync

import (
	"fmt"
	"testing"
	"time"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/net"
	netPb "chainmaker.org/chainmaker/pb-go/v2/net"
	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
	"chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"chainmaker.org/chainmaker/protocol/v2/test"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func getNodeStatusReq(t *testing.T) []byte {
	msg := &syncPb.SyncMsg{Type: syncPb.SyncMsg_NODE_STATUS_REQ}
	bz, err := msg.Marshal()
	require.NoError(t, err)
	return bz
}

func getNodeStatusResp(t *testing.T, height uint64) []byte {
	msg := &syncPb.BlockHeightBCM{BlockHeight: height}
	bz, err := msg.Marshal()
	require.NoError(t, err)
	msg2 := &syncPb.SyncMsg{Type: syncPb.SyncMsg_NODE_STATUS_RESP, Payload: bz}
	bz, err = msg2.Marshal()
	require.NoError(t, err)
	return bz
}

func getBlockReq(t *testing.T, height, batchSize uint64) []byte {
	msg := &syncPb.BlockSyncReq{BlockHeight: height, BatchSize: batchSize}
	bz, err := msg.Marshal()
	require.NoError(t, err)
	msg2 := &syncPb.SyncMsg{Type: syncPb.SyncMsg_BLOCK_SYNC_REQ, Payload: bz}
	bz, err = msg2.Marshal()
	require.NoError(t, err)
	return bz
}

func getBlockResp(t *testing.T, height uint64) []byte {
	msg := &syncPb.SyncBlockBatch{
		Data: &syncPb.SyncBlockBatch_BlockinfoBatch{
			BlockinfoBatch: &syncPb.BlockInfoBatch{
				Batch: []*commonPb.BlockInfo{
					{Block: &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: height}}},
				},
			},
		},
		WithRwset: false,
	}
	bz, err := msg.Marshal()
	require.NoError(t, err)
	msg2 := &syncPb.SyncMsg{Type: syncPb.SyncMsg_BLOCK_SYNC_RESP, Payload: bz}
	bz, err = msg2.Marshal()
	require.NoError(t, err)
	return bz
}

func getBlockRespWithRWset(height uint64) ([]byte, error) {
	msg := &syncPb.SyncBlockBatch{
		Data: &syncPb.SyncBlockBatch_BlockinfoBatch{
			BlockinfoBatch: &syncPb.BlockInfoBatch{
				Batch: []*commonPb.BlockInfo{
					{Block: &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: height}}},
				},
			},
		},
		WithRwset: true,
	}
	bz, err := msg.Marshal()
	if err != nil {
		return nil, err
	}
	msg2 := &syncPb.SyncMsg{Type: syncPb.SyncMsg_BLOCK_SYNC_RESP, Payload: bz}
	bz, err = msg2.Marshal()
	if err != nil {
		return nil, err
	}
	return bz, nil
}

type serverV1 struct {
	*BlockChainSyncServer
	netHandler *syncNetHandler
}

func (s *serverV1) Start() error {
	if err := s.netHandler.Start(); err != nil {
		return err
	}
	return s.BlockChainSyncServer.Start()
}

func (s *serverV1) Stop() {
	s.BlockChainSyncServer.Stop()
	if err := s.netHandler.Stop(); err != nil {
		fmt.Printf("stop net handler error: %v", err)
	}
}

func initTestSync(t *testing.T) (protocol.SyncService, func()) {
	ctrl := gomock.NewController(t)
	mockNet := newMockNet(ctrl)
	mockMsgBus := newMockMessageBus(ctrl)
	mockVerify := newMockVerifier(ctrl)
	mockStore := newMockBlockChainStore(ctrl)
	mockLedger := newMockLedgerCache(ctrl, &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 10}})
	mockCommit := newMockCommitter(ctrl, mockLedger)
	log := &test.GoLogger{}
	// localconf.ChainMakerConfig.SyncConfig.SchedulerTick = 10
	// localconf.ChainMakerConfig.SyncConfig.ProcessBlockTick = 10
	netHandler := newSyncNetHandler(mockNet, log)
	service := &serverV1{
		BlockChainSyncServer: newBlockChainSyncServerV1(
			"chain1", "localhost", mockNet, mockMsgBus,
			mockStore, mockLedger, mockVerify,
			mockCommit, netHandler, log).(*BlockChainSyncServer),
		netHandler: netHandler,
	}
	// service := newBlockChainSyncServerV1("chain1", mockNet, mockMsgBus, mockStore, mockLedger, mockVerify, mockCommit, mockTxPool, log)
	require.NoError(t, service.Start())
	return service, func() {
		service.Stop()
		ctrl.Finish()
	}
}

func TestBlockChainSyncServer_Start(t *testing.T) {
	service, fn := initTestSync(t)
	defer fn()

	// consume message
	implSync := service.(*serverV1)
	bz := getNodeStatusResp(t, 110)
	require.NoError(t, implSync.blockSyncMsgHandler("node1", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	bz = getNodeStatusResp(t, 120)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(3 * time.Millisecond)
	require.EqualValues(t, "pendingRecvHeight: 11, peers num: 2, blockStates num: 110, "+
		"pendingBlocks num: 0, receivedBlocks num: 0", implSync.scheduler.getServiceState())
	require.EqualValues(t, "pendingBlockHeight: 11, queue num: 0", implSync.processor.getServiceState())

	bz = getBlockResp(t, 11)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG)) // TODO
	//require.EqualValues(t, "pendingRecvHeight: 11, peers num: 2, blockStates num: 110, pendingBlocks num: 0, receivedBlocks num: 1",
	//	implSync.scheduler.getServiceState())
	//time.Sleep(time.Second)
	//require.EqualValues(t, "pendingBlockHeight: 12, queue num: 0", implSync.processor.getServiceState())
}

func TestSyncMsg_NODE_STATUS_REQ(t *testing.T) {
	service, fn := initTestSync(t)
	defer fn()
	implSync := service.(*serverV1)

	// 1. req node status
	require.NoError(t, implSync.blockSyncMsgHandler("node1", getNodeStatusReq(t), netPb.NetMsg_SYNC_BLOCK_MSG))
	//require.EqualValues(t, 1, len(implSync.net.(*MockNet).sendMsgs))
	//require.EqualValues(t, "msgType: 6, to: [node1]", implSync.net.(*MockNet).sendMsgs[0])

	require.NoError(t, implSync.blockSyncMsgHandler("node2", getNodeStatusReq(t), netPb.NetMsg_SYNC_BLOCK_MSG))
	//require.EqualValues(t, 2, len(implSync.net.(*MockNet).sendMsgs))
	//require.EqualValues(t, "msgType: 6, to: [node2]", implSync.net.(*MockNet).sendMsgs[1])
}

func TestSyncBlock_Req(t *testing.T) {
	service, fn := initTestSync(t)
	defer fn()
	implSync := service.(*serverV1)

	_ = implSync.blockChainStore.PutBlock(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 99}}, nil)
	_ = implSync.blockChainStore.PutBlock(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 100}}, nil)
	_ = implSync.blockChainStore.PutBlock(&commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 101}}, nil)

	require.NoError(t, implSync.blockSyncMsgHandler("node1", getBlockReq(t, 99, 1), netPb.NetMsg_SYNC_BLOCK_MSG))
	//require.EqualValues(t, 1, len(implSync.net.(*MockNet).sendMsgs))
	//require.EqualValues(t, "msgType: 6, to: [node1]", implSync.net.(*MockNet).sendMsgs[0])

	require.NoError(t, implSync.blockSyncMsgHandler("node2", getBlockReq(t, 100, 2), netPb.NetMsg_SYNC_BLOCK_MSG))
	//require.EqualValues(t, 3, len(implSync.net.(*MockNet).sendMsgs))
	//require.EqualValues(t, "msgType: 6, to: [node2]", implSync.net.(*MockNet).sendMsgs[1])

	require.Error(t, implSync.blockSyncMsgHandler("node2", getBlockReq(t, 110, 2), netPb.NetMsg_SYNC_BLOCK_MSG))
}

func TestSyncMsg_NODE_STATUS_RESP(t *testing.T) {
	service, fn := initTestSync(t)
	defer fn()
	implSync := service.(*serverV1)

	bz := getNodeStatusResp(t, 120)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(3 * time.Second)
	require.EqualValues(t, "pendingRecvHeight: 11, peers num: 1, blockStates num: 110, "+
		"pendingBlocks num: 110, receivedBlocks num: 0", implSync.scheduler.getServiceState())
	require.EqualValues(t, "pendingBlockHeight: 11, queue num: 0", implSync.processor.getServiceState())
}

func TestSyncMsg_BLOCK_SYNC_RESP(t *testing.T) {
	service, fn := initTestSync(t)
	defer fn()
	implSync := service.(*serverV1)
	// modify config for a stable unit test result
	implSync.conf.livenessTick = 10 * time.Second
	// 1. add peer status
	bz := getNodeStatusResp(t, 120)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	//测试概率失败：收到netPb.NetMsg_SYNC_BLOCK_MSG消息后会添加一个NodeStatusMsg的任务，此任务优先级较低，netPb.NetMsg_SYNC_BLOCK_MSG消息会添加一个SyncedBlockMsg类型的任务,优先级为中级
	//如果NodeStatusMsg的任务在消费前添加了一个SyncedBlockMsg类型的任务, 则SyncedBlockMsg的任务会优先被消费，此时scheduler中updateSchedulerBySyncBlockBatch函数中sch.blockStates为空，即needToProcess会被判定为false，消息不能被处理
	//sleep为了避免此情况发生
	time.Sleep(200 * time.Microsecond)
	// 2. receive block
	blkBz := getBlockResp(t, 11)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", blkBz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(4 * time.Second)
	require.EqualValues(t, "pendingRecvHeight: 12, peers num: 1, blockStates num: 109, "+
		"pendingBlocks num: 109, receivedBlocks num: 0", implSync.scheduler.getServiceState())
	require.EqualValues(t, "pendingBlockHeight: 12, queue num: 0", implSync.processor.getServiceState())
}

//func TestStopSyncBlock(t *testing.T) {
//	localconf.ChainMakerConfig.NodeConfig.FastSyncConfig.Enable = true
//	service, fn := initTestSync(t)
//	defer fn()
//	implSync := service.(*serverV1)
//	// modify config for a stable unit test result
//	implSync.conf.livenessTick = 10 * time.Second
//	// 1. add peer status
//	bz := getNodeStatusResp(t, 21)
//	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
//	time.Sleep(200 * time.Microsecond)
//	// 2. receive block
//	blkBz, err := getBlockRespWithRWset(11)
//	require.NoError(t, err)
//	require.NoError(t, implSync.blockSyncMsgHandler("node2", blkBz, netPb.NetMsg_SYNC_BLOCK_MSG))
//	time.Sleep(1 * time.Second)
//	require.EqualValues(t, "pendingRecvHeight: 12, peers num: 1, blockStates num: 10, pendingBlocks num: 10, receivedBlocks num: 0", implSync.scheduler.getServiceState())
//	//3.StopBlockSync
//	service.StopBlockSync()
//	time.Sleep(100 * time.Microsecond)
//	//4.sync node status again
//	bz = getNodeStatusResp(t, 41)
//	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
//	time.Sleep(1 * time.Second)
//	require.EqualValues(t, "pendingRecvHeight: 12, peers num: 1, blockStates num: 30, pendingBlocks num: 10, receivedBlocks num: 10", implSync.scheduler.getServiceState())
//}

func TestStopSyncBlock(t *testing.T) {
	localconf.ChainMakerConfig.NodeConfig.FastSyncConfig.Enable = true
	service, fn := initTestSync(t)
	defer fn()
	implSync := service.(*serverV1)
	// modify config for a stable unit test result
	implSync.conf.livenessTick = 10 * time.Second
	// 1. add peer status
	bz := getNodeStatusResp(t, 21)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(200 * time.Microsecond)
	// 2. receive block
	blkBz, err := getBlockRespWithRWset(11)
	require.NoError(t, err)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", blkBz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(3 * time.Second)
	require.EqualValues(t, "pendingRecvHeight: 12, peers num: 1, blockStates num: 10, pendingBlocks num: 10, receivedBlocks num: 0", implSync.scheduler.getServiceState())
	//3.StopBlockSync
	service.StopBlockSync()
	time.Sleep(100 * time.Microsecond)
	//4.sync node status again
	bz = getNodeStatusResp(t, 41)
	require.NoError(t, implSync.blockSyncMsgHandler("node2", bz, netPb.NetMsg_SYNC_BLOCK_MSG))
	time.Sleep(1 * time.Second)
	require.EqualValues(t, "pendingRecvHeight: 12, peers num: 1, blockStates num: 30, pendingBlocks num: 0, receivedBlocks num: 0", implSync.scheduler.getServiceState())
}

var broadcastCount int

func newMockNet2(ctrl *gomock.Controller) protocol.NetService {
	mockNet := mock.NewMockNetService(ctrl)
	mockNet.EXPECT().BroadcastMsg(gomock.Any(), gomock.Any()).DoAndReturn(
		func(msg []byte, msgType netPb.NetMsg_MsgType) error {
			if msgType == net.NetMsg_SYNC_BLOCK_MSG {
				message := syncPb.SyncMsg{}
				proto.Unmarshal(msg, &message)
				if message.Type == syncPb.SyncMsg_NODE_STATUS_RESP {
					broadcastCount++
				}
			}
			return nil
		}).AnyTimes()
	mockNet.EXPECT().SendMsg(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(msg []byte, msgType netPb.NetMsg_MsgType, to ...string) error {
			return nil
		}).AnyTimes()
	mockNet.EXPECT().ConsensusSubscribe(gomock.Any(), gomock.Any()).AnyTimes()
	mockNet.EXPECT().Subscribe(gomock.Any(), gomock.Any()).AnyTimes()
	mockNet.EXPECT().ReceiveMsg(gomock.Any(), gomock.Any()).AnyTimes()
	mockNet.EXPECT().CancelSubscribe(gomock.Any()).AnyTimes()
	mockNet.EXPECT().CancelReceiveMsg(gomock.Any()).AnyTimes()
	mockNet.EXPECT().CancelConsensusSubscribe(gomock.Any()).AnyTimes()
	return mockNet
}

func initTestSync2(t *testing.T) (protocol.SyncService, func()) {
	ctrl := gomock.NewController(t)
	mockNet := newMockNet2(ctrl)
	mockMsgBus := newMockMessageBus(ctrl)
	mockVerify := newMockVerifier(ctrl)
	mockStore := newMockBlockChainStore(ctrl)
	mockLedger := newMockLedgerCache(ctrl, &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: 10}})
	mockCommit := newMockCommitter(ctrl, mockLedger)
	log := &test.GoLogger{}
	// localconf.ChainMakerConfig.SyncConfig.SchedulerTick = 10
	// localconf.ChainMakerConfig.SyncConfig.ProcessBlockTick = 10
	// service := newBlockChainSyncServer("chain1", mockNet, mockMsgBus, mockStore, mockLedger, mockVerify, mockCommit, mockTxPool, log)
	netHandler := newSyncNetHandler(mockNet, log)
	service := &serverV1{
		BlockChainSyncServer: newBlockChainSyncServerV1(
			"chain1", "localhost", mockNet, mockMsgBus,
			mockStore, mockLedger, mockVerify,
			mockCommit, netHandler, log).(*BlockChainSyncServer),
		netHandler: netHandler,
	}
	require.NoError(t, service.Start())
	return service, func() {
		service.Stop()
		ctrl.Finish()
	}
}

func genBlockMessage(height uint64) *msgbus.Message {
	return &msgbus.Message{
		Topic: msgbus.BlockInfo,
		Payload: &commonPb.BlockInfo{
			Block: &commonPb.Block{Header: &commonPb.BlockHeader{BlockHeight: height}},
		},
	}

}

func TestBlockChainSyncServer_OnMessage(t *testing.T) {
	type args struct {
		totalHeight int
		perBlocks   int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "test1",
			args: args{
				totalHeight: 10,
				perBlocks:   0,
			},
			want: 3,
		},
		{
			name: "test2",
			args: args{
				totalHeight: 10,
				perBlocks:   2,
			},
			want: 5,
		},
		{
			name: "test3",
			args: args{
				totalHeight: 10,
				perBlocks:   -1,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localconf.ChainMakerConfig.SyncConfig.BroadcastStatusPerBlocksCommitted = tt.args.perBlocks
			svc, stop := initTestSync2(t)
			sync := svc.(*serverV1)
			broadcastCount = 0
			for i := 1; i <= tt.args.totalHeight; i++ {
				sync.OnMessage(genBlockMessage(uint64(i)))
			}
			stop()
			require.Equal(t, tt.want, broadcastCount)
		})
	}
}

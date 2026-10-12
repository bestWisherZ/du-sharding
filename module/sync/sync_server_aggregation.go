/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package sync

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"chainmaker.org/chainmaker/common/v2/msgbus"
	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	netPb "chainmaker.org/chainmaker/pb-go/v2/net"
	storePb "chainmaker.org/chainmaker/pb-go/v2/store"
	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
	"chainmaker.org/chainmaker/protocol/v2"
	blockSync "chainmaker.org/chainmaker/sync/v2"
	_ "chainmaker.org/chainmaker/vm-native/v2"
	"github.com/gogo/protobuf/proto"
)

var (
	v2Version = protocol.BlockVersion240
)

// BlockSyncServiceV2 version 2.0 of the Sync Service
// This service holds the new sync and retains the ability to provide sync data for v1 sync.
type BlockSyncServiceV2 struct {
	*blockSync.BlockSyncService
	// receive/broadcast messages from net module
	net protocol.NetService
	log protocol.Logger
	// The module that provides blocks storage/query
	blockChainStore protocol.BlockchainStore
	// Provides the latest chain state for the node
	ledgerCache protocol.LedgerCache
	// ignore repeat block sync request when in process
	requestCache sync.Map
	// Identification of module close
	close chan struct{}

	netHandler *syncNetHandler
}

func newBlockChainSyncServerV2(chainID string,
	net protocol.NetService,
	msgBus msgbus.MessageBus,
	chainConf protocol.ChainConf,
	blockchainStore protocol.BlockchainStore,
	ledgerCache protocol.LedgerCache,
	blockVerifier protocol.BlockVerifier,
	blockCommitter protocol.BlockCommitter,
	netHandler *syncNetHandler,
	log protocol.Logger) *BlockSyncServiceV2 {
	syncServer, _ := blockSync.NewSyncServer(
		localconf.ChainMakerConfig.NodeConfig.NodeId,
		&localconf.ChainMakerConfig.SyncConfig,
		chainConf,
		blockchainStore,
		ledgerCache,
		log,
		msgBus,
		blockVerifier,
		blockCommitter,
	)

	return &BlockSyncServiceV2{
		BlockSyncService: syncServer,
		net:              net,
		log:              log,
		blockChainStore:  blockchainStore,
		ledgerCache:      ledgerCache,
		close:            make(chan struct{}, 1),
		netHandler:       netHandler,
	}
}

// Start start the sync service.
// When the v2 version of the sync service is started, tability to provide synchronous response to
// v1 sync service needs to be enabled at the same time, because there may be v1 sync nodes
// on the chain sending sync requests to the current node.
func (s *BlockSyncServiceV2) Start() error {
	if err := s.enableV1DataProvision(); err != nil {
		return err
	}
	return s.BlockSyncService.Start()
}

// Stop stop the sync service.
func (s *BlockSyncServiceV2) Stop() {
	if err := s.disableV1DataProvision(); err != nil {
		s.log.Errorf("disabling V1 data provision error: %v", err)
	}
	s.BlockSyncService.Stop()
	close(s.close)
}

func (s *BlockSyncServiceV2) enableV1DataProvision() error {
	if err := s.netHandler.RegisterHandler(s); err != nil {
		return err
	}
	go s.blockRequestEntrance()
	return nil
}

func (s *BlockSyncServiceV2) disableV1DataProvision() error {
	return s.netHandler.UnregisterHandler(s)
}

// HandleSyncMsg process the v1 sync request message to v2 version.
func (s *BlockSyncServiceV2) HandleSyncMsg(from string, syncMsg *syncPb.SyncMsg) error {
	switch syncMsg.Type {
	case syncPb.SyncMsg_NODE_STATUS_REQ:
		//received a request to get own state from other nodes
		return s.handleNodeStatusReq(from)
	case syncPb.SyncMsg_NODE_STATUS_RESP:
		return nil
	case syncPb.SyncMsg_BLOCK_SYNC_REQ:
		//received a request to sync blocks from other nodes
		return s.handleBlockReq(syncMsg, from)
	case syncPb.SyncMsg_BLOCK_SYNC_RESP:
		//received a response with block data from other nodes
		return nil
	}
	return fmt.Errorf("not support the syncPb.SyncMsg.Type as %d", syncMsg.Type)
}

// handleNodeStatusReq get own block state information and send it to where the request is from
func (s *BlockSyncServiceV2) handleNodeStatusReq(from string) error {
	var (
		height uint64
		bz     []byte
		err    error
	)
	if height, err = s.ledgerCache.CurrentHeight(); err != nil {
		return err
	}
	archivedHeight := s.blockChainStore.GetArchivedPivot()
	s.log.Debugf("receive node status request from node [%s]", from)
	if bz, err = proto.Marshal(&syncPb.BlockHeightBCM{BlockHeight: height, ArchivedHeight: archivedHeight}); err != nil {
		return err
	}
	return s.sendMsg(syncPb.SyncMsg_NODE_STATUS_RESP, bz, from)
}

// handleBlockReq to avoid repeated requests for the same block data in a short period of time
// cache requests using 'requestCache', key consists of the request source and block height
// value is current time
// firstly check if the request already exists in the cache，if yes, reject
// otherwise, get the corresponding block data from the local ledger and send it back
func (s *BlockSyncServiceV2) handleBlockReq(syncMsg *syncPb.SyncMsg, from string) error {
	var (
		err error
		req syncPb.BlockSyncReq
	)

	if err = proto.Unmarshal(syncMsg.Payload, &req); err != nil {
		s.log.Errorf("fail to proto.Unmarshal the syncPb.SyncMsg:%s", err.Error())
		return err
	}
	// 针对 `SyncMsg_BLOCK_SYNC_REQ` 消息处理函数，添加处理状态检查，要求同一个 `请求来源 + 高度` 不会重复返回多次数据
	// create a key-value pair when receive block request, ignore repeat request
	processKey := fmt.Sprintf("%s_%d", from, req.BlockHeight)
	if _, loaded := s.requestCache.LoadOrStore(processKey, time.Now()); loaded {
		s.log.Warnf("received duplicate request to get block [height: %d, batch_size: %d] from "+
			"node [%s]", req.BlockHeight, req.BatchSize, from)
		return nil
	}

	s.log.Infof("receive request to get block [height: %d, batch_size: %d] from "+
		"node [%s]"+"WithRwset [%v]", req.BlockHeight, req.BatchSize, from, req.WithRwset)
	return s.sendInfos(&req, from)
}

func (s *BlockSyncServiceV2) sendMsg(msgType syncPb.SyncMsg_MsgType, msg []byte, to string) error {
	var (
		bs  []byte
		err error
	)
	if bs, err = proto.Marshal(&syncPb.SyncMsg{
		Type:    msgType,
		Payload: msg,
	}); err != nil {
		s.log.Error(err)
		return err
	}
	if err = s.net.SendMsg(bs, netPb.NetMsg_SYNC_BLOCK_MSG, to); err != nil {
		s.log.Warnf("send [%s] message to [%s] error: %v", netPb.NetMsg_SYNC_BLOCK_MSG.String(), to, err)
		return err
	}
	return nil
}

// sendInfos send block data to 'from'.
// `req
// BlockHeight: get block data starting from the this height
// BatchSize: the number of blocks to be acquired at one time
// WithRwset: the block data has a read-write set or not
// `
// we should get block data whose height is in [req.BlockHeight, req.BlockHeight+req.BatchSize)
// the request height may exceed the block height in the local ledger
// if so, blockChainStore will return a nil block without a error, need to skip it instead of sending it
// since a block data will be large, we send it one by one instead of all at once
// to reduce errors during network transmission.
func (s *BlockSyncServiceV2) sendInfos(req *syncPb.BlockSyncReq, from string) error {
	var (
		bz        []byte
		err       error
		blk       *commonPb.Block
		blkRwInfo *storePb.BlockWithRWSet
	)
	for i := uint64(0); i < req.BatchSize; i++ {
		if req.WithRwset {
			if blkRwInfo, err = s.blockChainStore.GetBlockWithRWSets(req.BlockHeight + i); err != nil {
				return err
			}
			if blkRwInfo == nil {
				s.log.Warnf("GetBlockWithRWSets get block height: [%d] is nil", req.BlockHeight+i)
				continue
			}
		} else {
			if blk, err = s.blockChainStore.GetBlock(req.BlockHeight + i); err != nil {
				s.log.Debugf("[SyncMsg_BLOCK_SYNC_RESP] get block without reset with err: %s", err.Error())
				return err
			}
			if blk == nil {
				s.log.Warnf("GetBlock get block height: [%d] is nil", req.BlockHeight+i)
				continue
			}
			blkRwInfo = &storePb.BlockWithRWSet{
				Block:    blk,
				TxRWSets: nil,
			}
		}
		info := &commonPb.BlockInfo{Block: blkRwInfo.Block, RwsetList: blkRwInfo.TxRWSets}
		if bz, err = proto.Marshal(&syncPb.SyncBlockBatch{
			Data: &syncPb.SyncBlockBatch_BlockinfoBatch{BlockinfoBatch: &syncPb.BlockInfoBatch{
				Batch: []*commonPb.BlockInfo{info}}}, WithRwset: req.WithRwset,
		}); err != nil {
			return err
		}
		if err := s.sendMsg(syncPb.SyncMsg_BLOCK_SYNC_RESP, bz, from); err != nil {
			return err
		}
	}
	return nil
}

// auto check block request from other node
// regularly check whether the cached request information has expired
// if it expires, remove it from the cache
func (s *BlockSyncServiceV2) blockRequestEntrance() {
	delay := 5 * time.Second
	ticker := time.NewTicker(delay)
	dealFunc := func(key, value interface{}) bool {
		if value == nil {
			return true
		}
		if t, ok := value.(time.Time); ok {
			if time.Since(t) > delay {
				s.requestCache.Delete(key)
			}
			return true
		}
		return true
	}
	for {
		select {
		case <-s.close:
			return

		case <-ticker.C:
			s.requestCache.Range(dealFunc)
		}
	}
}

// ServerAggregator is a sync service aggregator based on the chain version
type ServerAggregator struct {
	protocol.SyncService
	mu                 sync.Mutex
	version            int32
	syncRequestStopped bool
	minLagReachC       chan struct{}
	switchCh           chan struct{}
	closeCh            chan struct{}
	createErr          error
	netHandler         *syncNetHandler

	chainID         string
	net             protocol.NetService
	msgBus          msgbus.MessageBus
	chainConf       protocol.ChainConf
	blockchainStore protocol.BlockchainStore
	ledgerCache     protocol.LedgerCache
	blockVerifier   protocol.BlockVerifier
	blockCommitter  protocol.BlockCommitter
	log             protocol.Logger
}

// NewBlockChainSyncServer create a new sync service based on the chain version.
// Use the v1 version of the sync service if the chain version is less than v2Version.
// Otherwise, use the new sync service.
func NewBlockChainSyncServer(
	chainID string,
	net protocol.NetService,
	msgBus msgbus.MessageBus,
	blockchainStore protocol.BlockchainStore,
	ledgerCache protocol.LedgerCache,
	chainConf protocol.ChainConf,
	blockVerifier protocol.BlockVerifier,
	blockCommitter protocol.BlockCommitter,
	log protocol.Logger) protocol.SyncService {

	srv := &ServerAggregator{
		chainID:         chainID,
		net:             net,
		msgBus:          msgBus,
		chainConf:       chainConf,
		blockchainStore: blockchainStore,
		ledgerCache:     ledgerCache,
		blockVerifier:   blockVerifier,
		blockCommitter:  blockCommitter,
		log:             log,
		minLagReachC:    make(chan struct{}, 1),
		switchCh:        make(chan struct{}, 1),
		closeCh:         make(chan struct{}),
		netHandler:      newSyncNetHandler(net, log),
	}
	version, err := strconv.ParseUint(chainConf.ChainConfig().Version, 10, 64)
	if err != nil {
		srv.createErr = fmt.Errorf("failed to parse chain version: %v", err)
		return srv
	}
	if uint32(version) < v2Version {
		srv.SyncService = newBlockChainSyncServerV1(
			chainID,
			localconf.ChainMakerConfig.NodeConfig.NodeId,
			net,
			msgBus,
			blockchainStore,
			ledgerCache,
			blockVerifier,
			blockCommitter,
			srv.netHandler,
			log,
		)
		srv.version = 1
	} else {
		srv.SyncService = newBlockChainSyncServerV2(
			chainID,
			net,
			msgBus,
			chainConf,
			blockchainStore,
			ledgerCache,
			blockVerifier,
			blockCommitter,
			srv.netHandler,
			log,
		)
		srv.version = 2
	}
	return srv
}

// Start start the sync service.
// If the version is 1, register the message bus to listen for chain configuration changes to
// switch sync server version when the chain version is upgraded to 2400 or later.
func (s *ServerAggregator) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.log.Infof("startup sync server version [%d]", s.version)
	if err := s.netHandler.Start(); err != nil {
		return err
	}
	if err := s.SyncService.Start(); err != nil {
		return err
	}
	if s.version == 1 {
		s.msgBus.Register(msgbus.ChainConfig, s)
	}

	go s.listen()
	return nil
}

// Stop stop the sync service.
func (s *ServerAggregator) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version == 0 {
		return
	}
	if s.version == 1 {
		s.msgBus.UnRegister(msgbus.ChainConfig, s)
	}

	s.SyncService.Stop()
	if err := s.netHandler.Stop(); err != nil {
		s.log.Errorf("stop net handler error: %v", err)
	}
	s.version = 0
	close(s.closeCh)
}

// ListenSyncToIdealHeight listen local block height has synced to ideal height
func (s *ServerAggregator) ListenSyncToIdealHeight() <-chan struct{} {
	return s.minLagReachC
}

func (s *ServerAggregator) notifyReachIdealHeight() {
	select {
	case s.minLagReachC <- struct{}{}:
	default:
	}
}

func (s *ServerAggregator) listen() {
	ch := s.SyncService.ListenSyncToIdealHeight()
	for {
		select {
		case _, ok := <-ch:
			if ok {
				s.notifyReachIdealHeight()
			}
		case <-s.closeCh:
			return
		case <-s.switchCh:
			s.log.Infof("sync server version switching causes listener switching")
			ch = s.SyncService.ListenSyncToIdealHeight()
		}
	}
}

// StopBlockSync syncing blocks from other nodes,
// but still process other nodes synchronizing blocks from itself.
func (s *ServerAggregator) StopBlockSync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncRequestStopped = true
	s.SyncService.StopBlockSync()
}

// StartBlockSync start request service
func (s *ServerAggregator) StartBlockSync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncRequestStopped = false
	s.SyncService.StartBlockSync()
}

func (s *ServerAggregator) switchVersion() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version != 1 {
		return
	}
	s.log.Infof("begin to switch sync server version")
	s.SyncService.Stop()

	v2 := newBlockChainSyncServerV2(
		s.chainID,
		s.net,
		s.msgBus,
		s.chainConf,
		s.blockchainStore,
		s.ledgerCache,
		s.blockVerifier,
		s.blockCommitter,
		s.netHandler,
		s.log,
	)
	if err := v2.Start(); err != nil {
		s.log.Panicf("failed to switch v2 sync server %v", err)
	}
	s.SyncService = v2
	if s.syncRequestStopped {
		s.log.Infof("switch sync server stop block request function")
		s.SyncService.StopBlockSync()
	}
	s.msgBus.UnRegister(msgbus.ChainConfig, s)
	s.version = 2
	s.log.Infof("switch sync server version successfully")
	select {
	case s.switchCh <- struct{}{}:
	default:
	}
}

// OnMessage procrss chain config message for version change.
func (s *ServerAggregator) OnMessage(msg *msgbus.Message) {
	switch msg.Topic {
	case msgbus.ChainConfig:
		dataStr, _ := msg.Payload.([]string)
		dataBytes, err := hex.DecodeString(dataStr[0])
		if err != nil {
			s.log.Errorf("receive ChainConfig message DecodeString error: %v", err)
			return
		}
		chainConfig := &config.ChainConfig{}
		if err = proto.Unmarshal(dataBytes, chainConfig); err != nil {
			s.log.Errorf("receive ChainConfig message Unmarshal error: %v", err)
			return
		}
		blockVersion, err := strconv.ParseUint(chainConfig.Version, 10, 64)
		if err != nil {
			return
		}
		if uint32(blockVersion) >= v2Version {
			s.switchVersion()
		}
	}
}

func (bc *ServerAggregator) OnQuit() {
	// nothing for implement interface msgbus.Subscriber
}

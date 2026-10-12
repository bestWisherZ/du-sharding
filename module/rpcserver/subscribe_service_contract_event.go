/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"chainmaker.org/chainmaker/utils/v2"

	"chainmaker.org/chainmaker/localconf/v2"

	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	protocol "chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *ApiService) checkDealContractEventSubscriptionParams(tx *commonPb.Transaction) (
	startBlock int64, endBlock int64, contractName string, topic string, err error) {

	for _, kv := range tx.Payload.Parameters {
		if kv.Key == syscontract.SubscribeContractEvent_START_BLOCK.String() {
			startBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeContractEvent_END_BLOCK.String() {
			endBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeContractEvent_CONTRACT_NAME.String() {
			contractName = string(kv.Value)

			/**
			change from v2.3.6
			Previously, an empty contract name was supported, which would return all events related to all contracts.
			However, due to the potential for a large amount of data being returned, this approach is no longer supported.
			Now, a contract name must be specified.
			*/
			if contractName == "" {
				errCode := commonErr.ERR_CODE_SYSTEM_CONTRACT_UNSUPPORT_CONTRACT_NAME
				errMsg := s.getErrMsg(errCode, fmt.Errorf("contract name should not be empty"))
				err = status.Error(codes.InvalidArgument, errMsg)
				return
			}

		} else if kv.Key == syscontract.SubscribeContractEvent_TOPIC.String() {
			if kv.Value != nil {
				topic = string(kv.Value)
			}
		}

		if err != nil {
			errCode := commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_CONTRACT_EVENT
			errMsg := s.getErrMsg(errCode, err)
			err = status.Error(codes.InvalidArgument, errMsg)
			return
		}
	}

	return
}

// dealContractEventSubscription - deal contract event subscribe request
func (s *ApiService) dealContractEventSubscription(tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer) (retErr error) {

	var (
		err           error
		errMsg        string
		errCode       commonErr.ErrCode
		db            protocol.BlockchainStore
		txId          = tx.Payload.TxId
		chainId       = tx.Payload.ChainId
		startBlock    int64
		endBlock      int64
		subscribeType string
		senderAddr    string
		contractName  string
		topic         string
	)

	defer func() {
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			//metric subscribe active counter
			s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, topic).Dec()
			// if the function returns an error, count the number of subscription interruptions
			if retErr != nil {
				s.log.Warnf("dealContractEventSubscription encountered:%v [txId:%s, sender:%s]",
					retErr, txId, senderAddr)
				s.metricSubscribeInterruptedCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, topic).Inc()
			}
		}
	}()

	subscribeType = syscontract.SubscribeFunction_SUBSCRIBE_CONTRACT_EVENT.String()
	chainId = tx.Payload.ChainId
	if db, err = s.chainMakerServer.GetStore(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_STORE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s]", txId))
		return status.Error(codes.Internal, errMsg)
	}
	senderAddr, err = s.getTxSenderAddress(db, tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("[txId:%s]", txId))
		return err
	}

	startBlock, endBlock, contractName, topic, err = s.checkDealContractEventSubscriptionParams(tx)
	if err != nil {
		s.log.Warnf(fmt.Sprintf("check deal contract event subscription params failed, err:%s. [txId:%s]",
			err, txId))
		return err
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		//metric subscribe total counter
		s.metricSubscribeTotalCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, topic).Inc()
		//metric subscribe active counter
		s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, topic).Inc()
	}

	if err = s.checkSubscribeContractEventPayload(startBlock, endBlock); err != nil {
		errCode = commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_CONTRACT_EVENT
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s]", txId))
		return status.Error(codes.InvalidArgument, errMsg)
	}

	s.log.Infof(
		"Recv contract event subscribe request: [start:%d]/[end:%d]/[contractName:%s]/[topic:%s]/[txId:%s]",
		startBlock, endBlock, contractName, topic, txId)

	return s.doSendContractEvent(tx, db, server, startBlock, endBlock, contractName, topic)
}

func (s *ApiService) checkSubscribeContractEventPayload(startBlockHeight, endBlockHeight int64) error {

	if startBlockHeight < -1 || endBlockHeight < -1 ||
		(endBlockHeight != -1 && startBlockHeight > endBlockHeight) {

		return errors.New("invalid start block height or end block height")
	}

	return nil
}

func (s *ApiService) doSendContractEvent(tx *commonPb.Transaction, db protocol.BlockchainStore,
	server apiPb.RpcNode_SubscribeServer, startBlock, endBlock int64,
	contractName string, topic string) error {

	var (
		alreadySendHistoryBlockHeight int64
		err                           error
		txId                          = tx.Payload.TxId
	)

	senderAddr, err := s.getTxSenderAddress(db, tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("txId:%s", txId))
		return err
	}

	if startBlock == -1 && endBlock == 0 {
		s.log.Infof("send contract event: [sender:%s] [contractName:%s] [topic:%s] "+
			"[startBlock:%d] [endBlock:%d] [txId:%s, addr:%s]",
			senderAddr, contractName, topic, startBlock, endBlock, txId, senderAddr)
		return status.Error(codes.OK, "OK")
	}

	// just send realtime contract event
	// == 0 for compatibility
	if (startBlock == -1 && endBlock == -1) || (startBlock == 0 && endBlock == 0) {
		return s.sendNewContractEvent(db, tx, server, startBlock, endBlock,
			contractName, topic, -1, senderAddr)
	}

	if startBlock != -1 {
		if alreadySendHistoryBlockHeight, err = s.doSendHistoryContractEvent(db, server, startBlock, endBlock,
			contractName, topic, txId, senderAddr); err != nil {
			s.log.Warnf(err.Error() + fmt.Sprintf("[txId:%s, addr:%s, contractName:%s, topic:%s]",
				txId, senderAddr, contractName, topic))
			return err
		}
	}

	if startBlock == -1 {
		alreadySendHistoryBlockHeight = -1
	}

	if alreadySendHistoryBlockHeight == 0 {
		return status.Error(codes.OK, "OK")
	}

	return s.sendNewContractEvent(db, tx, server, startBlock, endBlock, contractName, topic,
		alreadySendHistoryBlockHeight, senderAddr)
}

func (s *ApiService) doSendHistoryContractEvent(db protocol.BlockchainStore, server apiPb.RpcNode_SubscribeServer,
	startBlock, endBlock int64, contractName, topic, txId, senderAddr string) (int64, error) {

	var (
		err             error
		errMsg          string
		errCode         commonErr.ErrCode
		lastBlockHeight int64
	)
	if startBlock < 0 {
		startBlock = 0
	}
	if lastBlockHeight, err = s.checkAndGetLastBlockHeight(db, startBlock); err != nil {
		if lastBlockHeight > 0 {
			startBlock = lastBlockHeight
			s.log.Warn("Set startBlock to the latestBlockHeight")
		} else {
			errCode = commonErr.ERR_CODE_GET_LAST_BLOCK
			errMsg = s.getErrMsg(errCode, err)
			s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, senderAddr:%s, contractName:%s, topic:%s]",
				txId, senderAddr, contractName, topic))
			return -1, status.Error(codes.Internal, errMsg)
		}
	}

	// only send history contract event
	if endBlock > 0 && endBlock <= lastBlockHeight {
		_, err = s.sendHistoryContractEvent(db, server, startBlock, endBlock,
			contractName, topic, txId, senderAddr)

		if err != nil {
			s.log.Warnf(
				"sendHistoryContractEvent failed:%s. [txId:%s, senderAddr:%s, contractName:%s, topic:%s]",
				err, txId, senderAddr, contractName, topic)
			return -1, err
		}

		return 0, status.Error(codes.OK, "OK")
	}

	alreadySendHistoryBlockHeight, err := s.sendHistoryContractEvent(db, server, startBlock, endBlock,
		contractName, topic, txId, senderAddr)

	if err != nil {
		s.log.Warnf("sendHistoryContractEvent failed:%s, [txId:%s, senderAddr:%s, contractName:%s, topic:%s]",
			err, txId, senderAddr, contractName, topic)
		return -1, err
	}

	s.log.Debugf("after sendHistoryContractEvent, alreadySendHistoryBlockHeight is %d",
		alreadySendHistoryBlockHeight)

	return alreadySendHistoryBlockHeight, nil
}

// sendHistoryContractEvent - send history contract event to subscriber
func (s *ApiService) sendHistoryContractEvent(store protocol.BlockchainStore,
	server apiPb.RpcNode_SubscribeServer,
	startBlockHeight, endBlockHeight int64,
	contractName, topic, txId, senderAddr string) (int64, error) {

	var (
		err    error
		errMsg string
		block  *commonPb.Block
	)

	i := startBlockHeight
	for {
		select {
		case <-server.Context().Done():
			s.log.Infof("client server context done[txId:%s, sender:%s, contractName:%s, topic:%s].",
				txId, senderAddr, contractName, topic)
			return -1, status.Error(codes.Internal, "client close subscribe, please check it")
		case <-s.ctx.Done():
			s.log.Warnf("chain server context done[txId:%s, sender:%s, contractName:%s, topic:%s].",
				txId, senderAddr, contractName, topic)
			return -1, status.Error(codes.Internal, "chainmaker is restarting, please retry later")
		default:
			getTokenStick := utils.CurrentTimeMillisSeconds()
			if err = s.getRateLimitToken(senderAddr); err != nil {
				s.log.Warnf(err.Error() + fmt.Sprintf("[txId:%s, sender:%s]", txId, senderAddr))
				return -1, status.Error(codes.Internal, err.Error())
			}
			getTokenCost := utils.CurrentTimeMillisSeconds() - getTokenStick

			if endBlockHeight > 0 && i > endBlockHeight {
				s.log.Infof("sendHistoryContractEvent done[height:%d, endBlockHeight:%d]."+
					"[txId:%s, sender:%s, contractName:%s, topic:%s]",
					i, endBlockHeight, txId, senderAddr, contractName, topic)
				return i - 1, nil
			}

			getBlockStick := utils.CurrentTimeMillisSeconds()
			block, err = store.GetBlock(uint64(i))
			if err != nil {
				errMsg = fmt.Sprintf("get block failed, at [height:%d], %s", i, err)
				s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, sender:%s, contractName:%s, topic:%s]",
					txId, senderAddr, contractName, topic))
				return -1, status.Error(codes.Internal, errMsg)
			}
			getBlockCost := utils.CurrentTimeMillisSeconds() - getBlockStick

			if block == nil {
				return i - 1, nil
			}

			sendSubscribeContractEventStick := utils.CurrentTimeMillisSeconds()
			if err = s.sendSubscribeContractEvent(server, block, contractName, topic); err != nil {
				errMsg = fmt.Sprintf("send subscribe tx failed, %s", err)
				s.log.Warnf(errMsg + fmt.Sprintf("[txId:%s, sender:%s, contractName:%s, topic:%s]",
					txId, senderAddr, contractName, topic))
				return -1, status.Error(codes.Internal, errMsg)
			}
			sendSubscribeContractEventCost := utils.CurrentTimeMillisSeconds() - sendSubscribeContractEventStick

			s.log.Infof("sendHistoryContractEvent[height:%d]. [txId:%s, sender:%s, "+
				"contractName:%s, topic:%s, getTokenCost:%d, getBlockCost:%d, sendSubscribeContractEventCost:%d]",
				i, txId, senderAddr, contractName, topic, getTokenCost, getBlockCost, sendSubscribeContractEventCost)
			i++
		}
	}
}

// sendSubscribeContractEvent send contract even by subscribe request.
/*
For example:
  - If both `contract_name` and `event_name` are provided, only events matching both will be returned.
  - If `contract_name` is provided and `event_name` is empty, all events for the specified contract will be returned.
  - If `contract_name` is empty, an error will be raised, as it is no longer supported.
*/
func (s *ApiService) sendSubscribeContractEvent(server apiPb.RpcNode_SubscribeServer,
	block *commonPb.Block, contractName, topic string) error {

	var (
		contractEvents []*commonPb.ContractEventInfo
	)

	for _, tx := range block.Txs {
		for idx, event := range tx.Result.ContractResult.ContractEvent {
			if contractName == event.ContractName {
				if topic == "" || topic == event.Topic {
					eventInfo := commonPb.ContractEventInfo{
						BlockHeight:     block.Header.BlockHeight,
						ChainId:         block.Header.ChainId,
						Topic:           event.Topic,
						TxId:            tx.Payload.TxId,
						EventIndex:      uint32(idx),
						ContractName:    event.ContractName,
						ContractVersion: event.ContractVersion,
						EventData:       event.EventData,
					}

					if eventInfo.BlockHeight != 0 {
						contractEvents = append(contractEvents, &eventInfo)
					}
				}
			}
		}
	}

	// If no matching events are found, send an empty contract event with block height and chain id.
	if len(contractEvents) == 0 {
		return s.doSendSubscribeContractEvent(server, []*commonPb.ContractEventInfo{{
			BlockHeight: block.Header.BlockHeight,
			ChainId:     block.Header.ChainId,
		}})
	}

	// Send all collected contract events in a single batch.
	return s.doSendSubscribeContractEvent(server, contractEvents)
}

func (s *ApiService) doSendSubscribeContractEvent(server apiPb.RpcNode_SubscribeServer,
	contractEvents []*commonPb.ContractEventInfo) error {

	var (
		err    error
		errMsg string
		result *commonPb.SubscribeResult
	)

	if len(contractEvents) == 0 {
		return nil
	}

	if result, err = s.getContractEventSubscribeResult(contractEvents); err != nil {
		errMsg = fmt.Sprintf("get contract event subscribe result failed, %s", err)
		s.log.Error(errMsg)
		return errors.New(errMsg)
	}

	if err := server.Send(result); err != nil {
		errMsg = fmt.Sprintf("send subscribe contract event result failed, %s", err)
		s.log.Error(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func (s *ApiService) sendNewContractEvent(store protocol.BlockchainStore, tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer, startBlock, endBlock int64,
	contractName string, topic string, alreadySendHistoryBlockHeight int64, senderAddr string) error {

	var (
		errCode         commonErr.ErrCode
		err             error
		errMsg          string
		lastBlockHeight int64
		chainId         = tx.Payload.ChainId
		txId            = tx.Payload.TxId
	)

	contractEventC := make(chan model.NewContractEvent, 1)
	updaterCtx, cancelUpdater := context.WithCancel(context.Background())
	defer cancelUpdater()
	err = s.startSubscribeContractEvent(updaterCtx, &lastBlockHeight, chainId, contractEventC)
	if err != nil {
		errCode = commonErr.ERR_CODE_GET_SUBSCRIBER
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s, senderAddr:%s, contractName:%s, topic:%s]",
			txId, senderAddr, contractName, topic))
		return status.Error(codes.Internal, errMsg)
	}

	if alreadySendHistoryBlockHeight == -1 {
		alreadySendHistoryBlockHeight = atomic.LoadInt64(&lastBlockHeight)
	}
	for {
		select {
		case <-contractEventC:
			// 首先判断是否结束发送数据。
			// 注意：当且仅当 endBlockHeight != -1 时，才有可能结束发送数据。
			// 当 endBlockHeight == -1 时，永不结束。
			if endBlock != -1 && alreadySendHistoryBlockHeight >= endBlock {
				s.log.Infof("send contract event finish: alreadySendHistoryBlockHeight:%d, "+
					"endBlock:%d,[txId:%s, sender:%s, contractName:%s, topic:%s].",
					alreadySendHistoryBlockHeight, endBlock, txId, senderAddr, contractName, topic)
				return status.Error(codes.OK, "OK")
			}

			if alreadySendHistoryBlockHeight < atomic.LoadInt64(&lastBlockHeight) {
				alreadySendHistoryBlockHeight, err = s.sendHistoryContractEvent(store, server,
					alreadySendHistoryBlockHeight+1, endBlock, contractName, topic, txId, senderAddr)
				if err != nil {
					s.log.Warnf("send history contract event failed:%s,[txId:%s, sender:%s, "+
						"contractName:%s, topic:%s].", err.Error(), txId, senderAddr, contractName, topic)
					return err
				}
			}
		case <-server.Context().Done():
			s.log.Infof("client server context done[txId:%s, sender:%s, contractName:%s, topic:%s].",
				txId, senderAddr, contractName, topic)
			return status.Error(codes.Internal, "client close subscribe, please check it")
		case <-s.ctx.Done():
			s.log.Warnf("chain server context done[txId:%s, sender:%s, contractName:%s, topic:%s].",
				txId, senderAddr, contractName, topic)
			return status.Error(codes.Internal, "chainmaker is restarting, please retry later")
		}
	}
}

// func (s *ApiService) getContractEventSubscribeResult(contractEventsInfoList *commonPb.ContractEventInfoList) (
func (s *ApiService) getContractEventSubscribeResult(contractEvents []*commonPb.ContractEventInfo) (
	*commonPb.SubscribeResult, error) {

	eventBytes, err := proto.Marshal(&commonPb.ContractEventInfoList{
		ContractEvents: contractEvents,
	})

	if err != nil {
		errMsg := fmt.Sprintf("marshal contract event info failed:%s", err)
		s.log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	result := &commonPb.SubscribeResult{
		Data: eventBytes,
	}

	return result, nil
}

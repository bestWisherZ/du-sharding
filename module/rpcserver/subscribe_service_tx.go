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
	"strings"
	"sync/atomic"

	"chainmaker.org/chainmaker/localconf/v2"

	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"

	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"chainmaker.org/chainmaker/common/v2/bytehelper"
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	protocol "chainmaker.org/chainmaker/protocol/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *ApiService) checkDealTxSubscriptionParams(tx *commonPb.Transaction) (startBlock int64, endBlock int64,
	contractName string, txIds []string, preAlias string, preTxId string, preOrgId string, err error) {
	for _, kv := range tx.Payload.Parameters {
		if kv.Key == syscontract.SubscribeTx_START_BLOCK.String() {
			startBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeTx_END_BLOCK.String() {
			endBlock, err = bytehelper.BytesToInt64(kv.Value)
		} else if kv.Key == syscontract.SubscribeTx_CONTRACT_NAME.String() {
			contractName = string(kv.Value)
		} else if kv.Key == syscontract.SubscribeTx_TX_IDS.String() {
			if kv.Value != nil {
				txIds = strings.Split(string(kv.Value), ",")
			}
		} else if kv.Key == syscontract.SubscribeTx_PRE_ALIAS.String() {
			preAlias = string(kv.Value)
		} else if kv.Key == syscontract.SubscribeTx_PRE_TX_ID.String() {
			preTxId = string(kv.Value)
		} else if kv.Key == syscontract.SubscribeTx_PRE_ORG_ID.String() {
			preOrgId = string(kv.Value)
		}

		if err != nil {
			errCode := commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_TX
			errMsg := s.getErrMsg(errCode, err)
			err = status.Error(codes.InvalidArgument, errMsg)
			return
		}
	}

	return
}

// dealTxSubscription - deal tx subscribe request
func (s *ApiService) dealTxSubscription(tx *commonPb.Transaction, server apiPb.RpcNode_SubscribeServer) (retErr error) {
	var (
		err           error
		errMsg        string
		errCode       commonErr.ErrCode
		db            protocol.BlockchainStore
		payload       = tx.Payload
		startBlock    int64
		endBlock      int64
		contractName  string
		txIds         []string
		reqSender     protocol.Role
		preAlias      string
		preTxId       string
		preOrgId      string
		txId          = tx.Payload.TxId
		chainId       = tx.Payload.ChainId
		subscribeType string
		senderAddr    string
	)

	defer func() {
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			//metric subscribe active counter
			s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, "").Dec()
			// if the function returns an error, count the number of subscription interruptions
			if retErr != nil {
				s.log.Errorf("dealTxSubscription encountered an error: %v [txId:%s, sender:%s]", retErr, txId, senderAddr)
				s.metricSubscribeInterruptedCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, "").Inc()
			}
		}
	}()

	s.log.Debugf("payload.Parameters=%s", payload.Parameters)

	subscribeType = syscontract.SubscribeFunction_SUBSCRIBE_TX.String()
	chainId = tx.Payload.ChainId
	if db, err = s.chainMakerServer.GetStore(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_STORE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s]", txId))
		return status.Error(codes.Internal, errMsg)
	}
	senderAddr, err = s.getTxSenderAddress(db, tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("txId:%s", txId))
		return err
	}

	startBlock, endBlock, contractName, txIds, preAlias, preTxId, preOrgId, err = s.checkDealTxSubscriptionParams(tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("[reqTxId:%s]", txId))
		return err
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		//metric subscribe total counter
		s.metricSubscribeTotalCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, "").Inc()
		//metric subscribe active counter
		s.metricSubscribeActiveCounter.WithLabelValues(chainId, senderAddr, subscribeType, contractName, "").Inc()
	}

	if err = s.checkSubscribeBlockHeight(startBlock, endBlock); err != nil {
		errCode = commonErr.ERR_CODE_CHECK_PAYLOAD_PARAM_SUBSCRIBE_TX
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s]", txId))
		return status.Error(codes.InvalidArgument, errMsg)
	}

	s.log.Infof("Recv tx subscribe request: [start:%d]/[end:%d]/[contractName:%s]/[txIds:%+v]/[reqTxId:%s]",
		startBlock, endBlock, contractName, txIds, txId)

	reqSender, err = s.getRoleFromTx(tx)
	if err != nil {
		s.log.Warnf(err.Error() + fmt.Sprintf("[reqTxId:%s, sender:%s]", txId, senderAddr))
		return err
	}
	reqSenderOrgId := tx.Sender.Signer.OrgId
	return s.doSendTx(tx, db, server, startBlock, endBlock, contractName, txIds,
		preAlias, preTxId, preOrgId,
		reqSender, reqSenderOrgId, senderAddr)
}

func (s *ApiService) doSendTx(tx *commonPb.Transaction, db protocol.BlockchainStore,
	server apiPb.RpcNode_SubscribeServer, startBlock, endBlock int64, contractName string,
	txIds []string, preAlias string, preTxId string, preOrgId string,
	reqSender protocol.Role, reqSenderOrgId, senderAddr string) error {

	var (
		txIdsMap                      = make(map[string]struct{})
		alreadySendHistoryBlockHeight int64
		err                           error
	)

	for _, txId := range txIds {
		txIdsMap[txId] = struct{}{}
	}

	if startBlock == -1 && endBlock == -1 {
		return s.sendNewTx(db, tx, server, startBlock, endBlock, contractName, txIds,
			preAlias, preTxId, preOrgId,
			txIdsMap, -1, reqSender, reqSenderOrgId, senderAddr)

	}

	if alreadySendHistoryBlockHeight, err = s.doSendHistoryTx(db, server, startBlock, endBlock,
		contractName, txIds,
		preAlias, preTxId, preOrgId,
		txIdsMap, reqSender, reqSenderOrgId, tx.Payload.TxId, senderAddr); err != nil {
		return err
	}

	if alreadySendHistoryBlockHeight == 0 {
		return status.Error(codes.OK, "OK")
	}

	return s.sendNewTx(db, tx, server, startBlock, endBlock, contractName, txIds,
		preAlias, preTxId, preOrgId, txIdsMap,
		alreadySendHistoryBlockHeight, reqSender, reqSenderOrgId, senderAddr)
}

func (s *ApiService) doSendHistoryTx(db protocol.BlockchainStore, server apiPb.RpcNode_SubscribeServer,
	startBlock, endBlock int64, contractName string, txIds []string,
	preAlias string, preTxId string, preOrgId string,
	txIdsMap map[string]struct{}, reqSender protocol.Role, reqSenderOrgId, reqTxId, senderAddr string) (int64, error) {

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
			s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s]", reqTxId))
			return -1, status.Error(codes.Internal, errMsg)
		}
	}

	if endBlock != -1 && endBlock <= lastBlockHeight {
		_, err = s.sendHistoryTx(db, server, startBlock, endBlock, contractName,
			txIds, preAlias, preTxId, preOrgId,
			txIdsMap, reqSender, reqSenderOrgId, reqTxId, senderAddr)

		if err != nil {
			s.log.Warnf("sendHistoryTx failed, %s. [reqTxId:%s, sender:%s]",
				err, reqTxId, senderAddr)
			return -1, err
		}

		s.log.Infof("sendHistoryTx success,endBlock:%d, lastBlockHeight:%d，reqTxId:%s, sender:%s",
			endBlock, lastBlockHeight, reqTxId, senderAddr)
		return 0, status.Error(codes.OK, "OK")
	}

	if len(txIds) > 0 && len(txIdsMap) == 0 {
		return 0, status.Error(codes.OK, "OK")
	}

	alreadySendHistoryBlockHeight, err := s.sendHistoryTx(db, server, startBlock, endBlock, contractName,
		txIds, preAlias, preTxId, preOrgId, txIdsMap, reqSender, reqSenderOrgId, reqTxId, senderAddr)

	if err != nil {
		s.log.Warnf("sendHistoryTx failed, %s. [reqTxId:%s, sender:%s]", err, reqTxId, senderAddr)
		return -1, err
	}

	if len(txIds) > 0 && len(txIdsMap) == 0 {
		s.log.Infof("txIds is not empty, but txIdsMap is empty, so return OK. [reqTxId:%s, sender:%s]",
			reqTxId, senderAddr)
		return 0, status.Error(codes.OK, "OK")
	}

	s.log.Infof("after sendHistoryBlock, alreadySendHistoryBlockHeight is %d. [txId:%s, sender:%s]",
		alreadySendHistoryBlockHeight, reqTxId, senderAddr)

	return alreadySendHistoryBlockHeight, nil
}

// sendNewTx - send new tx to subscriber
func (s *ApiService) sendNewTx(store protocol.BlockchainStore, tx *commonPb.Transaction,
	server apiPb.RpcNode_SubscribeServer, startBlock, endBlock int64, contractName string,
	txIds []string, preAlias string, preTxId string, preOrgId string,
	txIdsMap map[string]struct{}, alreadySendHistoryBlockHeight int64,
	reqSender protocol.Role, reqSenderOrgId, senderAddr string) error {

	var (
		errCode         commonErr.ErrCode
		err             error
		errMsg          string
		lastBlockHeight int64
		chainId         = tx.Payload.ChainId
		txId            = tx.Payload.TxId
	)

	blockC := make(chan model.NewBlockEvent, 1)
	updaterCtx, cancelUpdater := context.WithCancel(context.Background())
	defer cancelUpdater()
	err = s.startSubscribeBlockEvent(updaterCtx, &lastBlockHeight, chainId, blockC)
	if err != nil {
		errCode = commonErr.ERR_CODE_GET_SUBSCRIBER
		errMsg = s.getErrMsg(errCode, err)
		s.log.Warnf(errMsg + fmt.Sprintf("[reqTxId:%s, sender:%s]", txId, senderAddr))
		return status.Error(codes.Internal, errMsg)
	}

	if alreadySendHistoryBlockHeight == -1 {
		alreadySendHistoryBlockHeight = atomic.LoadInt64(&lastBlockHeight)
	}

	for {
		select {
		case <-blockC:
			// 首先判断是否结束发送数据。
			// 注意：当且仅当 endBlockHeight != -1 时，才有可能结束发送数据。
			// 当 endBlockHeight == -1 时，永不结束。
			if endBlock != -1 && alreadySendHistoryBlockHeight >= endBlock {
				s.log.Infof("send history block finish, alreadySendHistoryBlockHeight is %d, endBlock is %d, "+
					"reqTxId is %s, senderAddr:%s", alreadySendHistoryBlockHeight, endBlock, txId, senderAddr)
				return status.Error(codes.OK, "OK")
			}

			if alreadySendHistoryBlockHeight < atomic.LoadInt64(&lastBlockHeight) {
				alreadySendHistoryBlockHeight, err = s.sendHistoryTx(store, server, alreadySendHistoryBlockHeight+1,
					endBlock, contractName, txIds,
					preAlias, preTxId, preOrgId,
					txIdsMap, reqSender, reqSenderOrgId, txId, senderAddr)
				if err != nil {
					s.log.Warnf("send history block failed, err:%s,[txId:%s, sender:%s].",
						err, txId, senderAddr)
					return err
				}
			}
		case <-server.Context().Done():
			s.log.Infof("server context done[txId:%s, sender:%s].", txId, senderAddr)
			return nil
		case <-s.ctx.Done():
			s.log.Warnf("service context done[txId:%s, sender:%s].", txId, senderAddr)
			return nil
		}
	}
}

//func (s *ApiService) checkIsFinish(txIds []string, endBlock int64,
//	txIdsMap map[string]struct{}, blockInfo *commonPb.BlockInfo) bool {
//
//	if len(txIds) > 0 && len(txIdsMap) == 0 {
//		return true
//	}
//
//	if endBlock != -1 && int64(blockInfo.Block.Header.BlockHeight) >= endBlock {
//		return true
//	}
//
//	return false
//}

// sendHistoryTx - send history tx to subscriber
func (s *ApiService) sendHistoryTx(store protocol.BlockchainStore,
	server apiPb.RpcNode_SubscribeServer,
	startBlockHeight, endBlockHeight int64,
	contractName string, txIds []string,
	preAlias string, preTxId string, preOrgId string,
	txIdsMap map[string]struct{},
	reqSender protocol.Role, reqSenderOrgId, txId, senderAddr string) (int64, error) {

	var (
		err    error
		errMsg string
		block  *commonPb.Block
	)

	i := startBlockHeight
	for {
		select {
		case <-server.Context().Done():
			s.log.Infof("server context done[height:%d, txId:%s, sender:%s, contractName:%s].",
				i, txId, senderAddr, contractName)
			return -1, nil
		case <-s.ctx.Done():
			s.log.Warnf("service context done[height:%d, txId:%s, sender:%s, contractName:%s].",
				i, txId, senderAddr, contractName)
			return -1, status.Error(codes.Internal, "chainmaker is restarting, please retry later")
		default:
			getTokenStick := utils.CurrentTimeMillisSeconds()
			if err = s.getRateLimitToken(senderAddr); err != nil {
				s.log.Warnf("get rate limit token failed, %s. height:%d, txId:%s, sender:%s, contractName:%s",
					err, i, txId, senderAddr, contractName)
				return -1, status.Error(codes.Internal, err.Error())
			}
			getTokenCost := utils.CurrentTimeMillisSeconds() - getTokenStick

			if endBlockHeight != -1 && i > endBlockHeight {
				s.log.Infof("send history tx finish, alreadySendHistoryBlockHeight is %d, endBlock is %d, "+
					"reqTxId is %s, senderAddr:%s, contractName:%s", i, endBlockHeight, txId, senderAddr, contractName)
				return i - 1, nil
			}

			if len(txIds) > 0 && len(txIdsMap) == 0 {
				return i - 1, nil
			}

			block, err = store.GetBlock(uint64(i))

			if err != nil {
				errMsg = fmt.Sprintf("get block failed, at [height:%d], %s", i, err)
				s.log.Warnf(errMsg+" [txId:%s, sender:%s]", txId, senderAddr)
				return -1, status.Error(codes.Internal, errMsg)
			}

			if block == nil {
				s.log.Infof("get block[%d] nil.[txId:%s, sender:%s, contractName:%s]",
					i, txId, senderAddr, contractName)
				return i - 1, nil
			}

			s.log.Infof("get block[%d] finish.[txId:%s, sender:%s, contractName:%s]",
				i, txId, senderAddr, contractName)
			if err := s.sendSubscribeTx(server, block.Txs, contractName, txIds,
				preAlias, preTxId, preOrgId,
				txIdsMap,
				reqSender, reqSenderOrgId); err != nil {
				errMsg = fmt.Sprintf("send subscribe tx failed, %s", err)
				s.log.Warnf(errMsg+" [txId:%s, sender:%s]", txId, senderAddr)
				return -1, status.Error(codes.Internal, errMsg)
			}

			s.log.Infof("send subscribe tx finish, height:%d, txId:%s, sender:%s, contractName:%s, getTokenCost:%d",
				i, txId, senderAddr, contractName, getTokenCost)
			i++
		}
	}
}

func (s *ApiService) sendSubscribeTx(server apiPb.RpcNode_SubscribeServer,
	txs []*commonPb.Transaction, contractName string, txIds []string,
	preAlias string, preTxId string, preOrgId string,
	txIdsMap map[string]struct{}, reqSender protocol.Role, reqSenderOrgId string) error {

	var (
		err error
	)

	for _, tx := range txs {
		if contractName == "" && len(txIds) == 0 &&
			preAlias == "" && preTxId == "" && preOrgId == "" {
			if err = s.doSendSubscribeTx(server, tx, reqSender, reqSenderOrgId); err != nil {
				return err
			}
			continue
		}

		//preAlias
		if len(preAlias) > 0 {
			if err = s.handlePreAlias(server, tx, reqSender, reqSenderOrgId, preAlias); err != nil {
				return err
			}
			continue
		}
		//preTxId
		if len(preTxId) > 0 {
			if err = s.handlePreTxId(server, tx, reqSender, reqSenderOrgId, preTxId); err != nil {
				return err
			}
			continue
		}
		//preOrgId
		if len(preOrgId) > 0 {
			if err = s.handlePreOrgId(server, tx, reqSender, reqSenderOrgId, preOrgId); err != nil {
				return err
			}
			continue
		}
		if s.checkIsContinue(tx, contractName, txIds, txIdsMap) {
			continue
		}

		if err = s.doSendSubscribeTx(server, tx, reqSender, reqSenderOrgId); err != nil {
			return err
		}
	}

	return nil
}

func (s *ApiService) handlePreAlias(server apiPb.RpcNode_SubscribeServer, tx *commonPb.Transaction,
	reqSender protocol.Role, reqSenderOrgId string, preAlias string) error {
	//创世区块中的配置交易，sender为空
	if tx.Sender == nil || tx.Sender.Signer == nil || len(tx.Sender.Signer.OrgId) <= 0 {
		s.log.Debugf("Alias matching failed," +
			"The alias of the transaction sender was not found")
		return nil
	}
	if tx.Sender.Signer.MemberType != pbac.MemberType_ALIAS {
		s.log.Debugf("This transaction is not an alias transaction,MemberType=%s", tx.Sender.Signer.MemberType)
		return nil
	}
	//创世区块中的配置交易，sender为空
	if tx.Sender == nil || tx.Sender.Signer == nil || len(tx.Sender.Signer.MemberInfo) <= 0 {
		s.log.Debugf("Alias matching failed," +
			"The alias of the transaction sender was not found")
		return nil
	}
	if strings.HasPrefix(string(tx.Sender.Signer.MemberInfo), preAlias) {
		if err := s.doSendSubscribeTx(server, tx, reqSender, reqSenderOrgId); err != nil {
			return err
		}
	} else {
		s.log.Debugf("Alias matching failed，alias=[%v,%s], preAlias=[%s]",
			tx.Sender.Signer.MemberInfo,
			string(tx.Sender.Signer.MemberInfo),
			preAlias)
	}
	return nil
}

func (s *ApiService) handlePreTxId(server apiPb.RpcNode_SubscribeServer, tx *commonPb.Transaction,
	reqSender protocol.Role, reqSenderOrgId string, preTxId string) error {
	if strings.HasPrefix(tx.Payload.TxId, preTxId) {
		if err := s.doSendSubscribeTx(server, tx, reqSender, reqSenderOrgId); err != nil {
			return err
		}
	} else {
		s.log.Debugf("TxId matching failed，txId=[%s], preTxId=[%s]",
			tx.Payload.TxId,
			preTxId)
	}
	return nil
}

func (s *ApiService) handlePreOrgId(server apiPb.RpcNode_SubscribeServer, tx *commonPb.Transaction,
	reqSender protocol.Role, reqSenderOrgId string, preOrgId string) error {
	//创世区块中的配置交易，sender为空
	if tx.Sender == nil || tx.Sender.Signer == nil || len(tx.Sender.Signer.OrgId) <= 0 {
		s.log.Debugf("OrgId matching failed," +
			"The organization id of the transaction sender was not found")
		return nil
	}
	if strings.HasPrefix(tx.Sender.Signer.OrgId, preOrgId) {
		if err := s.doSendSubscribeTx(server, tx, reqSender, reqSenderOrgId); err != nil {
			return err
		}
	} else {
		s.log.Debugf("OrgId matching failed，orgId=[%s], preOrgId=[%s]",
			tx.Sender.Signer.OrgId,
			preOrgId)
	}
	return nil
}

func (s *ApiService) doSendSubscribeTx(server apiPb.RpcNode_SubscribeServer, tx *commonPb.Transaction,
	reqSender protocol.Role, reqSenderOrgId string) error {

	var (
		err    error
		errMsg string
		result *commonPb.SubscribeResult
	)

	txNew := utils.FilterBlacklistTxs([]*commonPb.Transaction{tx})[0]
	isReqSenderLightNode := reqSender == protocol.RoleLight
	isTxRelatedToSender := (tx.Sender != nil) && reqSenderOrgId == tx.Sender.Signer.OrgId

	if result, err = s.getTxSubscribeResult(txNew); err != nil {
		errMsg = fmt.Sprintf("get tx subscribe result failed, %s", err)
		s.log.Warnf(errMsg)
		return errors.New(errMsg)
	}

	if isReqSenderLightNode {
		if isTxRelatedToSender {
			if err := server.Send(result); err != nil {
				errMsg = fmt.Sprintf("send subscribe tx result failed, %s", err)
				s.log.Warnf(errMsg)
				return errors.New(errMsg)
			}
		}
	} else {
		if err := server.Send(result); err != nil {
			errMsg = fmt.Sprintf("send subscribe tx result failed, %s", err)
			s.log.Warnf(errMsg)
			return errors.New(errMsg)
		}
	}

	return nil
}

func (s *ApiService) getTxSubscribeResult(tx *commonPb.Transaction) (*commonPb.SubscribeResult, error) {
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		errMsg := fmt.Sprintf("marshal tx info failed, %s", err)
		s.log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	result := &commonPb.SubscribeResult{
		Data: txBytes,
	}

	return result, nil
}

func (s *ApiService) checkIsContinue(tx *commonPb.Transaction, contractName string, txIds []string,
	txIdsMap map[string]struct{}) bool {

	if contractName != "" && tx.Payload.ContractName != contractName {
		return true
	}

	if len(txIds) > 0 {
		_, ok := txIdsMap[tx.Payload.TxId]
		if !ok {
			return true
		}

		delete(txIdsMap, tx.Payload.TxId)
	}

	return false
}

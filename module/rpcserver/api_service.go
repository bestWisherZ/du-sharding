/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rpcserver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"

	"chainmaker.org/chainmaker-go/module/rpcserver/rateLimiter"

	"chainmaker.org/chainmaker/pb-go/v2/consensus"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"chainmaker.org/chainmaker-go/module/blockchain"
	"chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker-go/module/snapshot"
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	"chainmaker.org/chainmaker/common/v2/monitor"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
	txpoolPb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"chainmaker.org/chainmaker/protocol/v2"
	tbf "chainmaker.org/chainmaker/store/v2/types/blockfile"
	"chainmaker.org/chainmaker/utils/v2"
	native "chainmaker.org/chainmaker/vm-native/v2"
	"chainmaker.org/chainmaker/vm/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	//SYSTEM_CHAIN the system chain name
	SYSTEM_CHAIN = "system_chain"
)

var _ apiPb.RpcNodeServer = (*ApiService)(nil)

// ApiService struct define
type ApiService struct {
	chainMakerServer                  *blockchain.ChainMakerServer
	log                               *logger.CMLogger
	logBrief                          *logger.CMLogger
	subscriberRateLimiter             rateLimiter.SubscriberRateLimiter
	metricQueryCounter                *prometheus.CounterVec
	metricInvokeCounter               *prometheus.CounterVec
	metricInvokeTxSizeHistogram       *prometheus.HistogramVec
	metricQueryContractCounter        *prometheus.CounterVec
	metricTxInvokeIllegal             *prometheus.CounterVec
	metricSubscribeTotalCounter       *prometheus.CounterVec
	metricSubscribeInterruptedCounter *prometheus.CounterVec
	metricSubscribeActiveCounter      *prometheus.GaugeVec
	ctx                               context.Context
}

// NewApiService - new ApiService object
func NewApiService(ctx context.Context, chainMakerServer *blockchain.ChainMakerServer) *ApiService {
	log := logger.GetLogger(logger.MODULE_RPC)
	logBrief := logger.GetLogger(logger.MODULE_BRIEF)

	apiService := ApiService{
		chainMakerServer:      chainMakerServer,
		log:                   log,
		logBrief:              logBrief,
		subscriberRateLimiter: rateLimiter.NewSubscriberRateLimiter(log),
		ctx:                   ctx,
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		apiService.metricQueryCounter = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER, "metric_query_request_counter",
			"query request counts metric", "chainId", "state")
		apiService.metricQueryContractCounter = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER,
			"metric_query_contract_request_counter",
			"query contract request counts metric",
			"chainId",
			"contractName",
			"method",
			"date",
			"state")
		apiService.metricInvokeCounter = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER, "metric_invoke_request_counter",
			"invoke request counts metric", "chainId", "state")
		apiService.metricInvokeTxSizeHistogram = monitor.NewHistogramVec(
			monitor.SUBSYSTEM_RPCSERVER, "metric_invoke_tx_size_histogram",
			"invoke tx size histogram metric", prometheus.ExponentialBuckets(1024, 2, 12),
			"chainId", "state")
		apiService.metricTxInvokeIllegal = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER, "metric_tx_invoke_illegal",
			"Total number of tx invoke illegal",
			"chainId", "date", "signerMemberInfo")
		apiService.metricSubscribeTotalCounter = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER,
			"metric_subscribe_total_counter", "subscribe total counter metric",
			"chainId", "sender", "subscribeType", "contractName", "topic")
		apiService.metricSubscribeActiveCounter = monitor.NewGaugeVec(monitor.SUBSYSTEM_RPCSERVER,
			"metric_subscribe_active_counter", "subscribe active counter metric",
			"chainId", "sender", "subscribeType", "contractName", "topic")
		apiService.metricSubscribeInterruptedCounter = monitor.NewCounterVec(monitor.SUBSYSTEM_RPCSERVER,
			"metric_subscribe_interrupted_counter", "subscribe interrupted counter metric",
			"chainId", "sender", "subscribeType", "contractName", "topic")
	}

	return &apiService
}

// SendRequest - deal received TxRequest
func (s *ApiService) SendRequest(ctx context.Context, req *commonPb.TxRequest) (*commonPb.TxResponse, error) {
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("SendRequest[%s],payload:%#v,\n----signer:%v\n----endorsers:%+v",
			req.Payload.TxId, req.Payload, req.Sender, req.Endorsers)
	})

	startTime := time.Now()
	resp := s.invoke(ctx, &commonPb.Transaction{
		Payload:   req.Payload,
		Sender:    req.Sender,
		Endorsers: req.Endorsers,
		Result:    nil,
		Payer:     req.Payer,
	}, protocol.RPC, false)
	elapsed := time.Since(startTime)

	// audit log format: ip:port|orgId|chainId|TxType|TxId|Timestamp|ContractName|Method|retCode|retCodeMsg|retMsg
	// |invokeElapsed
	s.logBrief.Infof("|%s|%s|%s|%s|%s|%d|%s|%s|%d|%s|%s|%d", GetClientAddr(ctx), req.Sender.Signer.OrgId,
		req.Payload.ChainId, req.Payload.TxType, req.Payload.TxId, req.Payload.Timestamp, req.Payload.ContractName,
		req.Payload.Method, resp.Code, resp.Code, resp.Message, elapsed.Milliseconds())

	return resp, nil
}

// SendRequestSync - deal received TxRequest, sync tx result and send response
func (s *ApiService) SendRequestSync(ctx context.Context, req *commonPb.TxRequest) (*commonPb.TxResponse, error) {
	s.log.DebugDynamic(func() string {
		return fmt.Sprintf("SendRequestSync[%s],payload:%#v,\n----signer:%v\n----endorsers:%+v",
			req.Payload.TxId, req.Payload, req.Sender, req.Endorsers)
	})

	startTime := time.Now()
	resp := s.invoke(ctx, &commonPb.Transaction{
		Payload:   req.Payload,
		Sender:    req.Sender,
		Endorsers: req.Endorsers,
		Result:    nil,
		Payer:     req.Payer,
	}, protocol.RPC, true)
	elapsed := time.Since(startTime)

	// audit log format: ip:port|orgId|chainId|TxType|TxId|Timestamp|ContractName|Method|retCode|retCodeMsg|retMsg
	// |invokeElapsed
	s.logBrief.Infof("|%s|%s|%s|%s|%s|%d|%s|%s|%d|%s|%s|%d", GetClientAddr(ctx), req.Sender.Signer.OrgId,
		req.Payload.ChainId, req.Payload.TxType, req.Payload.TxId, req.Payload.Timestamp, req.Payload.ContractName,
		req.Payload.Method, resp.Code, resp.Code, resp.Message, elapsed.Milliseconds())

	return resp, nil
}

// validate tx
func (s *ApiService) validate(tx *commonPb.Transaction) (errCode commonErr.ErrCode, errMsg string) {
	var (
		err error
		bc  *blockchain.Blockchain
	)

	chainConfig, err := s.chainMakerServer.GetChainConf(tx.Payload.ChainId)
	if err != nil {
		errCode = commonErr.ERR_CODE_GET_CHAIN_CONF
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		return
	}

	if localconf.ChainMakerConfig.NodeConfig.CertKeyUsageCheck {
		err = checkTxSignCert(tx)
		if err != nil {
			errCode = commonErr.ERR_CODE_TX_VERIFY_FAILED
			errMsg = s.getErrMsg(errCode, err)
			s.log.Error(errMsg)
			return
		}
	}

	bc, err = s.chainMakerServer.GetBlockchain(tx.Payload.ChainId)
	if err != nil {
		errCode = commonErr.ERR_CODE_GET_BLOCKCHAIN
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		return
	}

	blockVersion := chainConfig.ChainConfig().GetBlockVersion()
	if err = utils.VerifyTxWithoutPayload(tx, tx.Payload.ChainId, bc.GetAccessControl(), blockVersion); err != nil {
		errCode = commonErr.ERR_CODE_TX_VERIFY_FAILED
		sender := ""
		if tx.Sender.Signer.MemberType == pbac.MemberType_CERT_HASH {
			sender = fmt.Sprintf("%x", tx.Sender.Signer.MemberInfo)
		} else {
			sender = string(tx.Sender.Signer.MemberInfo)
		}
		errMsg = fmt.Sprintf("%s, %s, txId:%s, sender:%s, endorsers-len:%d,\nendorsers:\n",
			errCode.String(), err.Error(), tx.Payload.TxId,
			sender, len(tx.Endorsers))
		for _, endorser := range tx.Endorsers {
			if endorser.Signer.MemberType == pbac.MemberType_CERT_HASH {
				errMsg += fmt.Sprintf("%x", endorser.Signer.MemberInfo)
			} else {
				errMsg += fmt.Sprintf("%s ", endorser.Signer.MemberInfo)
			}

		}
		s.log.Error(errMsg)
		if localconf.ChainMakerConfig.MonitorConfig.Enabled {
			if strings.Contains(err.Error(), "verify tx authentation failed") {
				sender := hex.EncodeToString(tx.Sender.Signer.MemberInfo)
				//交易发起者身份不合法 chainId,timeStamp,txId,signerMemberInfo
				s.log.Warnf("<METRIC> verify tx authentation failed, chainId:%s, date:%s, signerMemberInfo:%s",
					tx.Payload.ChainId, getCurrentDate(), sender)

				s.metricTxInvokeIllegal.WithLabelValues(tx.Payload.ChainId, getCurrentDate(),
					sender).Inc()
			}

		}
		return
	}

	return commonErr.ERR_CODE_OK, ""
}

func (s *ApiService) getErrMsg(errCode commonErr.ErrCode, err error) string {
	return fmt.Sprintf("%s, %s", errCode.String(), err.Error())
}

// invoke contract according to TxType
func (s *ApiService) invoke(ctx context.Context, tx *commonPb.Transaction, source protocol.TxSource,
	syncResult bool) *commonPb.TxResponse {
	var (
		errCode commonErr.ErrCode
		errMsg  string
		resp    = &commonPb.TxResponse{TxId: tx.Payload.TxId}
	)

	s.log.Debugf("ApiService invoke tx => id = %v, type = %v", tx.Payload.TxId, tx.Payload.TxType)
	if tx.Payload.ChainId != SYSTEM_CHAIN {
		errCode, errMsg = s.validate(tx)
		if errCode != commonErr.ERR_CODE_OK {
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = errMsg
			return resp
		}
	}

	switch tx.Payload.TxType {
	case commonPb.TxType_QUERY_CONTRACT:
		return s.dealQuery(tx, source)
	case commonPb.TxType_INVOKE_CONTRACT:
		return s.dealTransact(ctx, tx, source, syncResult)
	case commonPb.TxType_ARCHIVE:
		return s.doArchive(tx)
	case commonPb.TxType_NODE_CONFIG:
		return s.nodeConfig(tx)
	default:
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = commonErr.ERR_CODE_TXTYPE.String()
		return resp
	}
}

func getCurrentDate() string {
	// 获取当前时间
	now := time.Now()
	// 获取当前日期（年、月、日）
	year, month, day := now.Date()
	return fmt.Sprintf("%d-%d-%d", year, month, day)
}

// dealQuery - deal query tx
// nolint: revive, gocyclo
func (s *ApiService) dealQuery(tx *commonPb.Transaction, source protocol.TxSource) *commonPb.TxResponse {
	var (
		err         error
		errMsg      string
		errCode     commonErr.ErrCode
		store       protocol.BlockchainStore
		vmMgr       protocol.VmManager
		ledgerCache protocol.LedgerCache
		chainConf   protocol.ChainConf
		resp        = &commonPb.TxResponse{TxId: tx.Payload.TxId}
	)

	chainId := tx.Payload.ChainId
	if vmMgr, err = s.chainMakerServer.GetVmManager(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_VM_MGR
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = errMsg
		resp.TxId = tx.Payload.TxId
		return resp
	}
	// 系统链单独处理，不需要获取其他对象，由内部函数完成
	if chainId == SYSTEM_CHAIN {
		return s.dealSystemChainQuery(tx, vmMgr)
	}

	if store, err = s.chainMakerServer.GetStore(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_STORE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = errMsg
		resp.TxId = tx.Payload.TxId
		return resp
	}

	if ledgerCache, err = s.chainMakerServer.GetLedgerCache(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_LEDGERCACHE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = errMsg
		resp.TxId = tx.Payload.TxId
		return resp
	}
	blockVersion := protocol.DefaultBlockVersion
	var lastChainConfig *configPb.ChainConfig
	if chainConf, err = s.chainMakerServer.GetChainConf(tx.Payload.ChainId); err == nil {
		lastChainConfig = chainConf.ChainConfig()
		blockVersion = lastChainConfig.GetBlockVersion()
	} else {
		// 失败的情况下，从存储中重新获取一次
		lastChainConfig, err = store.GetLastChainConfig()
		if err != nil || lastChainConfig == nil {
			s.log.Error(fmt.Errorf("failed to get last chain config, %v", err))
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			resp.TxId = tx.Payload.TxId
			return resp
		}
	}
	if blockVersion == 0 {
		blockVersion = protocol.DefaultBlockVersion
	}

	var log = logger.GetLoggerByChain(logger.MODULE_SNAPSHOT, chainId)
	var snap protocol.Snapshot
	snap, err = snapshot.NewApiQuerySnapshot(ledgerCache.GetLastCommittedBlock(), lastChainConfig, store, log)
	if err != nil {
		s.log.Error(err)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = err.Error()
		resp.TxId = tx.Payload.TxId
		return resp
	}
	ctx := vm.NewTxSimContext(vmMgr, snap, tx, blockVersion, log)
	//contract, err := store.GetContractByName(tx.Payload.ContractName)
	contract, err := ctx.GetContractByName(tx.Payload.ContractName)
	if err != nil {
		s.log.Error(err)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = err.Error()
		resp.TxId = tx.Payload.TxId
		return resp
	}

	var bytecode []byte
	if contract.RuntimeType != commonPb.RuntimeType_NATIVE &&
		contract.RuntimeType != commonPb.RuntimeType_GO &&
		contract.RuntimeType != commonPb.RuntimeType_DOCKER_GO {
		bytecode, err = store.GetContractBytecode(contract.Name)
		if err != nil {
			s.log.Error(err)
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			resp.TxId = tx.Payload.TxId
			return resp
		}
	}

	gasUsed := uint64(0)
	gasRWSet := uint64(0)
	gasEvents := uint64(0)
	if blockVersion2312 <= blockVersion {
		gasUsed, err = calcTxGasUsed(ctx, s.log)
		s.log.Debugf("【gas calc】%v, before `RunContract` gasUsed = %v, err = %v",
			tx.Payload.TxId, gasUsed, err)
		if err != nil {
			s.log.Errorf("calculate tx gas failed, err = %v", err)
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			resp.ContractResult.Code = uint32(1)
			resp.ContractResult.Message = err.Error()
			return resp
		}
	}
	txResult, _, txStatusCode := vmMgr.RunContract(contract, tx.Payload.Method,
		bytecode, s.kvPair2Map(tx.Payload.Parameters), ctx, gasUsed, tx.Payload.TxType)
	s.log.DebugDynamic(func() string {
		contractJson, _ := json.Marshal(contract)
		return fmt.Sprintf("vmMgr.RunContract: txStatusCode:%d, resultCode:%d, contractName[%s](%s), "+
			"method[%s], txType[%s], message[%s],result len: %d",
			txStatusCode, txResult.Code, tx.Payload.ContractName, string(contractJson), tx.Payload.Method,
			tx.Payload.TxType, txResult.Message, len(txResult.Result))
	})
	if blockVersion2312 <= blockVersion {
		s.log.Debugf("【gas calc】%v, before `calcTxRWSetGasUsed` gasUsed = %v, err = %v",
			tx.Payload.TxId, txResult.GasUsed, err)
		gasRWSet, err = calcTxRWSetGasUsed(ctx, txStatusCode == commonPb.TxStatusCode_SUCCESS, s.log)
		if err != nil {
			s.log.Errorf("calculate tx rw_set gas failed, err = %v", err)
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			resp.ContractResult.Code = uint32(1)
			resp.ContractResult.Message = err.Error()
			return resp
		}
		txResult.GasUsed += gasRWSet
		s.log.Debugf("【gas calc】%v, before `calcTxEventGasUsed` gasUsed = %v, err = %v",
			tx.Payload.TxId, txResult.GasUsed, err)

		gasEvents, err = calcTxEventGasUsed(
			ctx, txResult.ContractEvent, s.log)
		if err != nil {
			s.log.Errorf("calculate tx events gas failed, err = %v", err)
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			resp.ContractResult.Code = uint32(1)
			resp.ContractResult.Message = err.Error()
			return resp
		}
		txResult.GasUsed += gasEvents
	}

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		if txStatusCode == commonPb.TxStatusCode_SUCCESS && txResult.Code != 1 {
			s.metricQueryCounter.WithLabelValues(chainId, "true").Inc()
			s.metricQueryContractCounter.WithLabelValues(chainId,
				tx.Payload.ContractName,
				tx.Payload.Method,
				getCurrentDate(),
				"true").Inc()
		} else {
			s.metricQueryCounter.WithLabelValues(chainId, "false").Inc()
			s.metricQueryContractCounter.WithLabelValues(chainId,
				tx.Payload.ContractName,
				tx.Payload.Method,
				getCurrentDate(),
				"true").Inc()
		}
	}
	if txStatusCode != commonPb.TxStatusCode_SUCCESS {
		errMsg = fmt.Sprintf("txStatusCode:%d, resultCode:%d, contractName[%s] method[%s] txType[%s], %s",
			txStatusCode, txResult.Code, tx.Payload.ContractName, tx.Payload.Method, tx.Payload.TxType, txResult.Message)
		s.log.Warn(errMsg)

		resp.Code = txStatusCode
		if txResult.Message == tbf.ErrArchivedBlock.Error() {
			resp.Code = commonPb.TxStatusCode_ARCHIVED_BLOCK
		} else if txResult.Message == tbf.ErrArchivedTx.Error() {
			resp.Code = commonPb.TxStatusCode_ARCHIVED_TX
		}

		resp.Message = errMsg
		resp.ContractResult = txResult
		resp.TxId = tx.Payload.TxId
		return resp
	}

	if txResult.Code == 1 {
		resp.Code = commonPb.TxStatusCode_CONTRACT_FAIL
		resp.Message = commonPb.TxStatusCode_CONTRACT_FAIL.String()
		resp.ContractResult = txResult
		resp.TxId = tx.Payload.TxId
		return resp
	}
	resp.Code = commonPb.TxStatusCode_SUCCESS
	resp.Message = commonPb.TxStatusCode_SUCCESS.String()
	resp.ContractResult = txResult
	resp.TxId = tx.Payload.TxId
	return resp
}

// dealSystemChainQuery - deal system chain query
func (s *ApiService) dealSystemChainQuery(tx *commonPb.Transaction, vmMgr protocol.VmManager) *commonPb.TxResponse {
	var (
		resp    = &commonPb.TxResponse{}
		store   protocol.BlockchainStore
		err     error
		errCode commonErr.ErrCode
		errMsg  string
	)

	chainId := tx.Payload.ChainId

	if store, err = s.chainMakerServer.GetStore(chainId); err != nil {
		errCode = commonErr.ERR_CODE_GET_STORE
		errMsg = s.getErrMsg(errCode, err)
		s.log.Error(errMsg)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = errMsg
		resp.TxId = tx.Payload.TxId
		return resp
	}

	var log = logger.GetLoggerByChain(logger.MODULE_SNAPSHOT, chainId)

	var snap protocol.Snapshot
	snap, err = snapshot.NewQuerySnapshot(store, log)
	if err != nil {
		s.log.Error(err)
		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = err.Error()
		resp.TxId = tx.Payload.TxId
		return resp
	}
	blockVersion := protocol.DefaultBlockVersion

	if cc, err1 := s.chainMakerServer.GetChainConf(tx.Payload.ChainId); err1 == nil {
		blockVersion = cc.ChainConfig().GetBlockVersion()
	}
	if blockVersion == 0 {
		blockVersion = protocol.DefaultBlockVersion
	}
	ctx := vm.NewTxSimContext(vmMgr, snap, tx, blockVersion, log)

	//defaultGas := uint64(0)
	//chainConfig, _ := s.chainMakerServer.GetChainConf(chainId)
	//if chainConfig.ChainConfig().AccountConfig != nil && chainConfig.ChainConfig().AccountConfig.EnableGas {
	//	defaultGas = chainConfig.ChainConfig().AccountConfig.DefaultGas
	//}
	//runtimeInstance := native.GetRuntimeInstance(chainId, defaultGas, s.log)
	runtimeInstance := native.GetRuntimeInstance(chainId)

	txResult := runtimeInstance.Invoke(&commonPb.Contract{
		Name: tx.Payload.ContractName,
	},
		tx.Payload.Method,
		nil,
		s.kvPair2Map(tx.Payload.Parameters),
		ctx,
	)

	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		if txResult.Code != 1 {
			s.metricQueryCounter.WithLabelValues(chainId, "true").Inc()
		} else {
			s.metricQueryCounter.WithLabelValues(chainId, "false").Inc()
		}
	}

	if txResult.Code == 1 {
		resp.Code = commonPb.TxStatusCode_CONTRACT_FAIL
		resp.Message = commonPb.TxStatusCode_CONTRACT_FAIL.String()
		resp.ContractResult = txResult
		resp.TxId = tx.Payload.TxId
		return resp
	}

	resp.Code = commonPb.TxStatusCode_SUCCESS
	resp.Message = commonPb.TxStatusCode_SUCCESS.String()
	resp.ContractResult = txResult
	resp.TxId = tx.Payload.TxId
	return resp
}

// kvPair2Map - change []*commonPb.KeyValuePair to map[string]string
func (s *ApiService) kvPair2Map(kvPair []*commonPb.KeyValuePair) map[string][]byte {
	kvMap := make(map[string][]byte)

	for _, kv := range kvPair {
		kvMap[kv.Key] = kv.Value
	}

	return kvMap
}

// dealTransact - deal transact tx, for TxType_INVOKE_CONTRACT only
func (s *ApiService) dealTransact(ctx context.Context, tx *commonPb.Transaction, source protocol.TxSource,
	syncResult bool) *commonPb.TxResponse {
	var (
		err       error
		resp      = &commonPb.TxResponse{TxId: tx.Payload.TxId}
		txResultC <-chan *TxResultExt
	)
	s.log.Infof("TX_TRACE stage=recv_tx tx=%s", tx.Payload.TxId)

	// if sync result, register tx first
	if syncResult {
		txResultC, err = dispatcher.Register(tx.Payload.ChainId, tx.Payload.TxId)
		if err != nil {
			s.log.Errorf("Register tx failed, %s, chainId:%s, txId:%s", err.Error(),
				tx.Payload.ChainId, tx.Payload.TxId)
			resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
			resp.Message = err.Error()
			return resp
		}
		defer dispatcher.Unregister(tx.Payload.ChainId, tx.Payload.TxId)
	}

	err = s.chainMakerServer.AddTx(tx.Payload.ChainId, tx, source)

	s.incInvokeCounter(tx.Payload.ChainId, err)
	s.updateTxSizeHistogram(tx, err)

	if err != nil {
		errMsg := fmt.Sprintf("Add tx failed, %s, chainId:%s, txId:%s",
			err.Error(), tx.Payload.ChainId, tx.Payload.TxId)
		s.log.Warn(errMsg)

		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = errMsg
		return resp
	}

	s.log.Debugf("Add tx success, chainId:%s, txId:%s", tx.Payload.ChainId, tx.Payload.TxId)

	// if sync result, wait tx result
	if syncResult {
		if shouldRejectSyncWait(s.chainMakerServer, tx.Payload.ChainId) {
			resp.Code = commonPb.TxStatusCode_TIMEOUT
			resp.Message = "当前未轮到本分片出块，交易已进入交易池"
			return resp
		}
		timeout := time.Duration(localconf.ChainMakerConfig.RpcConfig.SyncTxResultTimeout) * time.Second
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()
		select {
		case r := <-txResultC:
			resp.Code = r.Result.Code
			resp.Message = r.Result.Message
			resp.ContractResult = r.Result.ContractResult
			resp.TxTimestamp = r.TxTimestamp
			resp.TxBlockHeight = r.TxBlockHeight
			blockTag := ""
			if r.TxBlockHeight > 0 {
				blockTag = fmt.Sprintf(" block=[%d]-unknown", r.TxBlockHeight)
				if store, err := s.chainMakerServer.GetStore(tx.Payload.ChainId); err == nil && store != nil {
					if header, err := store.GetBlockHeaderByHeight(r.TxBlockHeight); err == nil && header != nil {
						blockTag = fmt.Sprintf(" block=[%d]-%x", r.TxBlockHeight, header.BlockHash)
					}
				}
			}
			s.log.Infof("TX_TRACE stage=contract_result tx=%s code=%s%s",
				tx.Payload.TxId, r.Result.Code.String(), blockTag)
		case <-ctx.Done():
			resp.Code = commonPb.TxStatusCode_TIMEOUT
			resp.Message = ctx.Err().Error()
		case <-ticker.C:
			resp.Code = commonPb.TxStatusCode_TIMEOUT
			resp.Message = fmt.Sprintf("request reached sync_tx_result_timeout, timeout=%s", timeout)
		}
	} else {
		resp.Code = commonPb.TxStatusCode_SUCCESS
		resp.Message = commonErr.ERR_CODE_OK.String()
	}

	return resp
}

func (s *ApiService) incInvokeCounter(chainId string, err error) {
	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		if err == nil {
			s.metricInvokeCounter.WithLabelValues(chainId, "true").Inc()
		} else {
			s.metricInvokeCounter.WithLabelValues(chainId, "false").Inc()
		}
	}
}

func shouldRejectSyncWait(server *blockchain.ChainMakerServer, chainId string) bool {
	if server == nil {
		return false
	}
	ledger, err := server.GetLedgerCache(chainId)
	if err != nil || ledger == nil {
		return false
	}
	last := ledger.GetLastCommittedBlock()
	if last == nil || last.Header == nil {
		return false
	}
	nextHeight := last.Header.BlockHeight + 1
	return !scheduler.ShouldProduce(nextHeight)
}

func (s *ApiService) updateTxSizeHistogram(tx *commonPb.Transaction, err error) {
	if localconf.ChainMakerConfig.MonitorConfig.Enabled {
		if err == nil {
			s.metricInvokeTxSizeHistogram.WithLabelValues(tx.Payload.ChainId, "true").Observe(float64(tx.Size()))
		} else {
			s.metricInvokeTxSizeHistogram.WithLabelValues(tx.Payload.ChainId, "false").Observe(float64(tx.Size()))
		}
	}
}

// RefreshLogLevelsConfig - refresh log level
func (s *ApiService) RefreshLogLevelsConfig(ctx context.Context, req *configPb.LogLevelsRequest) (
	*configPb.LogLevelsResponse, error) {

	if err := localconf.RefreshLogLevelsConfig(); err != nil {
		return &configPb.LogLevelsResponse{
			Code:    int32(1),
			Message: err.Error(),
		}, nil
	}
	return &configPb.LogLevelsResponse{
		Code: int32(0),
	}, nil
}

// UpdateDebugConfig - update debug config for test
func (s *ApiService) UpdateDebugConfig(ctx context.Context, req *configPb.DebugConfigRequest) (
	*configPb.DebugConfigResponse, error) {

	if err := localconf.UpdateDebugConfig(req.Pairs); err != nil {
		return &configPb.DebugConfigResponse{
			Code:    int32(1),
			Message: err.Error(),
		}, nil
	}
	return &configPb.DebugConfigResponse{
		Code: int32(0),
	}, nil
}

// CheckNewBlockChainConfig check new block chain config.
func (s *ApiService) CheckNewBlockChainConfig(context.Context, *configPb.CheckNewBlockChainConfigRequest) (
	*configPb.CheckNewBlockChainConfigResponse, error) {

	if err := localconf.CheckNewCmBlockChainConfig(); err != nil {
		return &configPb.CheckNewBlockChainConfigResponse{
			Code:    int32(1),
			Message: err.Error(),
		}, nil
	}

	return &configPb.CheckNewBlockChainConfigResponse{
		Code: int32(0),
	}, nil
}

// GetChainMakerVersion get chainmaker version by rpc request
func (s *ApiService) GetChainMakerVersion(ctx context.Context, req *configPb.ChainMakerVersionRequest) (
	*configPb.ChainMakerVersionResponse, error) {

	return &configPb.ChainMakerVersionResponse{
		Code:    int32(0),
		Version: s.chainMakerServer.Version(),
	}, nil
}

// GetPoolStatus Returns the max size of config transaction pool and common transaction pool,
// the num of config transaction in queue and pendingCache,
// and the the num of common transaction in queue and pendingCache.
func (s *ApiService) GetPoolStatus(ctx context.Context,
	request *txpoolPb.GetPoolStatusRequest) (*txpoolPb.TxPoolStatus, error) {
	return s.chainMakerServer.GetPoolStatus(request.ChainId)
}

// GetTxIdsByTypeAndStage Returns config or common txIds in different stage.
// TxType may be TxType_CONFIG_TX, TxType_COMMON_TX, (TxType_CONFIG_TX|TxType_COMMON_TX)
// TxStage may be TxStage_IN_QUEUE, TxStage_IN_PENDING, (TxStage_IN_QUEUE|TxStage_IN_PENDING)
func (s *ApiService) GetTxIdsByTypeAndStage(ctx context.Context,
	request *txpoolPb.GetTxIdsByTypeAndStageRequest) (*txpoolPb.GetTxIdsByTypeAndStageResponse, error) {
	txIds, err := s.chainMakerServer.GetTxIdsByTypeAndStage(request.ChainId,
		int32(request.TxType), int32(request.TxStage))
	if err != nil {
		return nil, err
	}
	return &txpoolPb.GetTxIdsByTypeAndStageResponse{TxIds: txIds}, nil
}

// GetTxsInPoolByTxIds Retrieve the transactions by the txIds from the txPool,
// return transactions in the txPool and txIds not in txPool.
// default query upper limit is 1w transaction, and error is returned if the limit is exceeded.
func (s *ApiService) GetTxsInPoolByTxIds(ctx context.Context,
	request *txpoolPb.GetTxsInPoolByTxIdsRequest) (*txpoolPb.GetTxsInPoolByTxIdsResponse, error) {
	txs, txIds, err := s.chainMakerServer.GetTxsInPoolByTxIds(request.ChainId, request.TxIds)
	if err != nil {
		return nil, err
	}
	return &txpoolPb.GetTxsInPoolByTxIdsResponse{
		Txs:   txs,
		TxIds: txIds,
	}, nil
}

// GetConsensusStateJSON Gets the status of the current consensus, including the height and view
// of the block participating in the consensus, timeout, and identity of the consensus node
func (s *ApiService) GetConsensusStateJSON(ctx context.Context,
	request *consensus.GetConsensusStatusRequest) (*wrapperspb.BytesValue, error) {
	bz, err := s.chainMakerServer.GetConsensusStateJSON(request.ChainId)
	if err != nil {
		return nil, err
	}
	return wrapperspb.Bytes(bz), nil
}

// GetConsensusValidators Gets the identity of all consensus nodes
func (s *ApiService) GetConsensusValidators(ctx context.Context,
	request *consensus.GetConsensusStatusRequest) (*consensus.Validators, error) {
	nodes, err := s.chainMakerServer.GetConsensusValidators(request.ChainId)
	if err != nil {
		return nil, err
	}
	return &consensus.Validators{Nodes: nodes}, nil
}

// GetConsensusHeight Gets the height of the block participating in the consensus
func (s *ApiService) GetConsensusHeight(ctx context.Context,
	request *consensus.GetConsensusStatusRequest) (*wrapperspb.UInt64Value, error) {
	height, err := s.chainMakerServer.GetConsensusHeight(request.ChainId)
	if err != nil {
		return nil, err
	}
	return wrapperspb.UInt64(height), nil
}

// GetSyncState get sync state by rpc request
func (s *ApiService) GetSyncState(ctx context.Context, request *syncPb.GetSyncStateRequest) (*syncPb.SyncState, error) {
	syncService, err := s.chainMakerServer.GetSync(request.ChainId)
	if err != nil {
		return nil, err
	}
	return syncService.GetState(request.WithPeers)
}

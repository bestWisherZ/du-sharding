/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package parallel 并发处理，主要用于压测的场景
package parallel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker/common/v2/ca"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	sdkutils "chainmaker.org/chainmaker/sdk-go/v2/utils"
	"chainmaker.org/chainmaker/utils/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ArriveTargetError = errors.New("arrive target value")

type Detail struct {
	TPS          float32                `json:"tps"`
	SuccessCount int                    `json:"successCount"`
	FailCount    int                    `json:"failCount"`
	Count        int                    `json:"count"`
	MinTime      int64                  `json:"minTime"`
	MaxTime      int64                  `json:"maxTime"`
	AvgTime      float32                `json:"avgTime"`
	StartTime    string                 `json:"startTime"`
	EndTime      string                 `json:"endTime"`
	Elapsed      float32                `json:"elapsed"`
	ThreadNum    int                    `json:"threadNum"`
	LoopNum      int                    `json:"loopNum"`
	Nodes        map[string]interface{} `json:"nodes"`
}

type reqStatOther struct {
	success bool
	elapsed int64
	nodeId  int
}

type StatisticianOther struct {
	reqStatC          chan *reqStatOther
	minSuccessElapsed int64
	maxSuccessElapsed int64
	sumSuccessElapsed int64
	totalCount        int32
	successCount      int

	lastIndex     int
	lastStartTime time.Time

	startTime time.Time
	endTime   time.Time

	// Classify by node id
	nodeMinSuccessElapsed []int64
	nodeMaxSuccessElapsed []int64
	nodeSumSuccessElapsed []int64
	nodeSuccessReqCount   []int
	nodeTotalReqCount     []int
}

func parallelOthers(parallelMethod string) error {
	if nodeNum > threadNum {
		//fmt.Println("threadNum:", threadNum, "less nodeNum:", nodeNum, "change threadNum=nodeNum")
		threadNum = nodeNum
	}
	timeoutChan := make(chan struct{}, threadNum)
	doneChan := make(chan struct{}, threadNum)
	doneCount := 0

	// Statistician updater
	statistician := &StatisticianOther{
		reqStatC: make(chan *reqStatOther, threadNum),

		nodeMinSuccessElapsed: make([]int64, nodeNum),
		nodeMaxSuccessElapsed: make([]int64, nodeNum),
		nodeSumSuccessElapsed: make([]int64, nodeNum),
		nodeSuccessReqCount:   make([]int, nodeNum),
		nodeTotalReqCount:     make([]int, nodeNum),
	}
	for i := 0; i < nodeNum; i++ {
		statistician.nodeMinSuccessElapsed[i] = math.MaxInt16
	}
	go statistician.Start()

	var threads []*ThreadOthers
	for i := 0; i < threadNum; i++ {
		t := &ThreadOthers{
			id:           i,
			loopNum:      loopNum,
			doneChan:     doneChan,
			timeoutChan:  timeoutChan,
			statistician: statistician,
		}
		switch parallelMethod {
		case invokerMethod:
			t.operationName = invokerMethod
			t.handler = &invokeHandler{threadId: i}
		case queryMethod:
			t.operationName = queryMethod
			t.handler = &queryHandler{threadId: i}
		case createContractStr:
			t.operationName = createContractStr
			t.handler = &createContractHandler{threadId: i}
		case upgradeContractStr:
			t.operationName = upgradeContractStr
			t.handler = &upgradeContractHandler{threadId: i}
		}
		threads = append(threads, t)
	}

	statistician.startTime = time.Now()
	statistician.lastStartTime = time.Now()

	for _, thread := range threads {
		if err := thread.Init(); err != nil {
			return err
		}
	}

	go othersParallelStart(threads)

	printTicker := time.NewTicker(time.Duration(printTime) * time.Second)
	timeoutTicker := time.NewTicker(time.Duration(timeout) * time.Second)
	timeoutOnce := sync.Once{}
	for {
		if doneCount >= threadNum {
			break
		}
		select {
		case <-doneChan:
			doneCount++
		case <-printTicker.C:
			go statistician.PrintDetails(false)
		case <-timeoutTicker.C:
			go func() {
				timeoutOnce.Do(func() {
					for i := 0; i < threadNum; i++ {
						timeoutChan <- struct{}{}
					}
				})
			}()
		}
	}

	statistician.endTime = time.Now()

	fmt.Println("Statistics for the entire test")
	statistician.PrintDetails(true)

	close(timeoutChan)
	close(doneChan)
	printTicker.Stop()
	timeoutTicker.Stop()
	for _, t := range threads {
		t.Stop()
	}
	return nil
}

func othersParallelStart(threads []*ThreadOthers) {
	count := threadNum / 10
	if count == 0 {
		count = 1
	}
	interval := time.Duration(climbTime/count) * time.Second
	for index := 0; index < threadNum; {
		for j := 0; j < 10; j++ {
			go threads[index].Start()
			index++
			if index >= threadNum {
				break
			}
		}
		time.Sleep(interval)
	}
}

func (s *StatisticianOther) Start() {
	for {
		select {
		case stat := <-s.reqStatC:
			if stat.success {
				if stat.elapsed < s.minSuccessElapsed {
					s.minSuccessElapsed = stat.elapsed
				}
				if stat.elapsed > s.maxSuccessElapsed {
					s.maxSuccessElapsed = stat.elapsed
				}

				if stat.elapsed < s.nodeMinSuccessElapsed[stat.nodeId] {
					s.nodeMinSuccessElapsed[stat.nodeId] = stat.elapsed
				}
				if stat.elapsed > s.nodeMaxSuccessElapsed[stat.nodeId] {
					s.nodeMaxSuccessElapsed[stat.nodeId] = stat.elapsed
				}

				s.successCount++
				s.sumSuccessElapsed += stat.elapsed

				s.nodeSuccessReqCount[stat.nodeId]++
				s.nodeSumSuccessElapsed[stat.nodeId] += stat.elapsed
			}

			s.nodeTotalReqCount[stat.nodeId]++
		}
	}
}

// PrintDetails print statistics results
// @param all
func (s *StatisticianOther) PrintDetails(all bool) {
	nowCount := atomic.LoadInt32(&s.totalCount)
	nowTime := time.Now()

	detail := s.statisticsResults(&numberResults{count: int(s.totalCount), successCount: s.successCount,
		max: s.maxSuccessElapsed, min: s.minSuccessElapsed, sum: s.sumSuccessElapsed,
		nodeSuccessCount: s.nodeSuccessReqCount,
		nodeCount:        s.nodeTotalReqCount, nodeMin: s.nodeMinSuccessElapsed,
		nodeMax: s.nodeMaxSuccessElapsed, nodeSum: s.nodeSumSuccessElapsed}, all, nowTime)
	s.lastIndex = int(nowCount)
	s.lastStartTime = time.Now()

	bytes, err := json.Marshal(detail)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(bytes))
	fmt.Println()
}

type numberResults struct {
	count, successCount         int
	min, max, sum               int64
	nodeSuccessCount, nodeCount []int
	nodeMin, nodeMax, nodeSum   []int64
}

func (s *StatisticianOther) statisticsResults(ret *numberResults, all bool, nowTime time.Time) (detail *Detail) {
	detail = &Detail{
		Nodes: make(map[string]interface{}),
	}
	if ret.count > 0 {
		detail = &Detail{
			SuccessCount: ret.successCount,
			FailCount:    ret.count - ret.successCount,
			Count:        ret.count,
			MinTime:      ret.min,
			MaxTime:      ret.max,
			AvgTime:      float32(ret.sum) / float32(ret.count),
			ThreadNum:    threadNum,
			LoopNum:      loopNum,
			Nodes:        make(map[string]interface{}),
		}
		for i := 0; i < nodeNum; i++ {
			detail.Nodes[fmt.Sprintf("node%d_successCount", i)] = ret.nodeSuccessCount[i]
			detail.Nodes[fmt.Sprintf("node%d_failCount", i)] = ret.nodeCount[i] - ret.nodeSuccessCount[i]
			detail.Nodes[fmt.Sprintf("node%d_count", i)] = ret.nodeCount[i]
			detail.Nodes[fmt.Sprintf("node%d_minTime", i)] = ret.nodeMin[i]
			detail.Nodes[fmt.Sprintf("node%d_maxTime", i)] = ret.nodeMax[i]
			detail.Nodes[fmt.Sprintf("node%d_avgTime", i)] = float32(ret.nodeSum[i]) / float32(ret.nodeCount[i])
		}
	}
	if all {
		detail.StartTime = s.startTime.Format("2006-01-02 15:04:05.000")
		detail.EndTime = s.endTime.Format("2006-01-02 15:04:05.000")
		detail.Elapsed = float32(s.endTime.Sub(s.startTime).Milliseconds()) / 1000
		detail.TPS = float32(ret.successCount) / float32(s.endTime.Sub(s.startTime).Seconds())
		for i := 0; i < nodeNum; i++ {
			detail.Nodes[fmt.Sprintf("node%d_tps", i)] = float32(ret.nodeSuccessCount[i]) / float32(s.endTime.Sub(s.startTime).Seconds())
		}
	} else {
		detail.StartTime = s.lastStartTime.Format("2006-01-02 15:04:05.000")
		detail.EndTime = nowTime.Format("2006-01-02 15:04:05.000")
		detail.Elapsed = float32(nowTime.Sub(s.lastStartTime).Milliseconds()) / 1000
		detail.TPS = float32(ret.successCount) / float32(nowTime.Sub(s.startTime).Seconds())
		for i := 0; i < nodeNum; i++ {
			detail.Nodes[fmt.Sprintf("node%d_tps", i)] = float32(ret.nodeSuccessCount[i]) / float32(nowTime.Sub(s.startTime).Seconds())
		}
	}
	return detail
}

// ThreadOthers for multi-thread object
type ThreadOthers struct {
	id            int
	loopNum       int
	doneChan      chan struct{}
	timeoutChan   chan struct{}
	handler       Handler
	statistician  *StatisticianOther
	operationName string

	conn   *grpc.ClientConn
	client apiPb.RpcNodeClient
	sk3    crypto.PrivateKey
	index  int
}

// Init init thread data
// @return error
func (t *ThreadOthers) Init() error {
	var err error
	t.index = t.id % len(hosts)
	t.conn, err = t.initGRPCConnect(useTLS, t.index)
	if err != nil {
		return err
	}
	t.client = apiPb.NewRpcNodeClient(t.conn)

	file, err := ioutil.ReadFile(signKeyPaths[t.index])
	if err != nil {
		return err
	}

	t.sk3, err = asym.PrivateKeyFromPEM(file, nil)
	if err != nil {
		return err
	}

	return nil
}

// Start thread start
func (t *ThreadOthers) Start() {
	for i := 0; i < t.loopNum; i++ {
		select {
		case <-t.timeoutChan:
			t.doneChan <- struct{}{}
			return
		default:
			start := time.Now()
			var err error
			if authType == sdk.Public {
				err = t.handler.handle(t.client, t.sk3, "", "", i)
			} else if authType == sdk.PermissionedWithKey {
				err = t.handler.handle(t.client, t.sk3, orgIDs[t.index], "", i)
			} else {
				err = t.handler.handle(t.client, t.sk3, orgIDs[t.index], signCrtPaths[t.index], i)
			}
			if errors.Is(err, ArriveTargetError) {
				t.doneChan <- struct{}{}
				return
			}
			elapsed := time.Since(start)

			atomic.AddInt32(&t.statistician.totalCount, 1)
			t.statistician.reqStatC <- &reqStatOther{
				success: err == nil,
				elapsed: elapsed.Milliseconds(),
				nodeId:  t.index,
			}

			if recordLog && err != nil {
				log.Errorf("threadId: %d, loopId: %d, nodeId: %d, err: %s", t.id, i, t.index, err)
			}

			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		}
	}
	t.doneChan <- struct{}{}
}

func getOtherPairInfos() ([]*KeyValuePair, error) {
	var ps []*KeyValuePair
	err := json.Unmarshal([]byte(pairsString), &ps)
	if err != nil {
		log.Errorf("unmarshal pair content failed, origin content: %s, err: %s", pairsString, err)
		return nil, err
	}

	for _, p := range ps {
		if p.Decrease || p.Increase {
			p.IntValue, err = strconv.ParseInt(p.Value, 10, 64)
			if err != nil {
				return nil, err
			}
		}
	}

	return ps, nil
}

// Stop thread stop
func (t *ThreadOthers) Stop() {
	t.conn.Close()
}

func (t *ThreadOthers) initGRPCConnect(useTLS bool, index int) (*grpc.ClientConn, error) {
	url := hosts[index]

	if useTLS {
		var serverName string
		if hostnamesString == "" {
			serverName = "chainmaker.org"
		} else {
			if len(hosts) != len(hostnames) {
				return nil, errors.New("required len(hosts) == len(hostnames)")
			}
			serverName = hostnames[index]
		}
		tlsClient := ca.CAClient{
			ServerName: serverName,
			CaPaths:    []string{caPaths[index]},
			CertFile:   userCrtPaths[index],
			KeyFile:    userKeyPaths[index],
		}

		c, err := tlsClient.GetCredentialsByCA()
		if err != nil {
			return nil, err
		}
		return grpc.Dial(url, grpc.WithTransportCredentials(*c))
	} else {
		return grpc.Dial(url, grpc.WithInsecure())
	}
}

// Handler do multi-thread operation action
type Handler interface {
	handle(client apiPb.RpcNodeClient, sk3 crypto.PrivateKey, orgId string, userCrtPath string, loopId int) error
}

// invokeHandler contract invoke handler
type invokeHandler struct {
	threadId int
}

var (
	respStr           = "proposalRequest error, resp: %+v"
	templateStrOthers = "%s_%d_%d_%d"
	resultStrOthers   = "exec result, orgid: %s, loop_id: %d, method: %s, txid: %s, resp: %+v"
)

var totalSentTxsOthers int64
var totalRandomSentTxsOthers int64
var resp *commonPb.TxResponse

type InvokerMsg struct {
	txType       commonPb.TxType
	chainId      string
	txId         string
	method       string
	contractName string
	oldSeq       uint64
	pairs        []*commonPb.KeyValuePair
}

func (h *invokeHandler) handle(client apiPb.RpcNodeClient, sk3 crypto.PrivateKey, orgId string,
	userCrtPath string, loopId int) error {
	txId := utils.GetTimestampTxId()

	// 构造Payload
	pairs, err := makeKvsOthers(h.threadId, loopId)
	if err != nil {
		return err
	}
	if showKey {
		j, err := json.Marshal(pairs)
		if err != nil {
			fmt.Println(err)
		}
		rate := totalRandomSentTxsOthers * 100 / totalSentTxsOthers
		fmt.Printf("totalSentTxs:%d\t totalRandomSentTxsOthers:%d\t randomRate:%d \t param:%s\t \n",
			totalSentTxsOthers, totalRandomSentTxsOthers, rate, string(j))
	}

	// 支持evm
	//var err error
	var abiData *[]byte
	if abiPath != "" {
		abiData = abiCache.Read(abiPath)
		runTime = 5 //evm
	}

	method1, pairs1, err := makePairs(method, abiPath, pairs, commonPb.RuntimeType(runTime), abiData)

	//fmt.Println("[exec_handle]orgId: ", orgId, ", userCrtPath: ", userCrtPath, ", loopId: ", loopId, ", method1: ", method1, ", pairs1: ", pairs1, ", method: ", method, ", pairs: ", pairs)
	payloadBytes, err := constructInvokePayload(chainId, contractName, method1, pairs1, gasLimit)
	if err != nil {
		return err
	}

	resp, err = sendRequest(sk3, client, &InvokerMsg{txType: commonPb.TxType_INVOKE_CONTRACT,
		txId: txId, chainId: chainId}, orgId, userCrtPath, payloadBytes, nil)
	if err != nil {
		return err
	}

	if outputResult {
		msg := fmt.Sprintf(resultStrOthers, orgId, loopId, method1, txId, resp)
		fmt.Println(msg)
	}

	if checkResult && resp.Code != commonPb.TxStatusCode_SUCCESS {
		return fmt.Errorf(respStr, resp)
	}

	return nil
}

// queryHandler contract query handler
type queryHandler struct {
	threadId int
}

func (h *queryHandler) handle(client apiPb.RpcNodeClient, sk3 crypto.PrivateKey, orgId string,
	userCrtPath string, loopId int) error {
	txId := utils.GetTimestampTxId()

	// 构造Payload
	pairs, err := makeKvsOthers(h.threadId, loopId)
	if err != nil {
		return err
	}
	if showKey {
		j, err := json.Marshal(pairs)
		if err != nil {
			fmt.Println(err)
		}
		rate := totalRandomSentTxsOthers * 100 / totalSentTxsOthers
		fmt.Printf("totalSentTxsOthers:%d\t totalRandomSentTxsOthers:%d\t randomRate:%d \t param:%s\t \n",
			totalSentTxsOthers, totalRandomSentTxsOthers, rate, string(j))
	}

	payloadBytes, err := constructQueryPayload(chainId, contractName, method, pairs, gasLimit)
	if err != nil {
		return err
	}

	resp, err = sendRequest(sk3, client, &InvokerMsg{txType: commonPb.TxType_QUERY_CONTRACT,
		txId: txId, chainId: chainId}, orgId, userCrtPath, payloadBytes, nil)
	if err != nil {
		return err
	}

	if checkResult && resp.Code != commonPb.TxStatusCode_SUCCESS {
		return fmt.Errorf(respStr, resp)
	}

	return nil
}

// createContractHandler create contract handler
type createContractHandler struct {
	threadId int
}

func (h *createContractHandler) handle(client apiPb.RpcNodeClient, sk3 crypto.PrivateKey, orgId string,
	userCrtPath string, loopId int) error {
	txId := utils.GetTimestampTxId()

	wasmBin, err := ioutil.ReadFile(wasmPath)
	if err != nil {
		return err
	}
	var pairs []*commonPb.KeyValuePair
	payload, _ := utils.GenerateInstallContractPayload(fmt.Sprintf(templateStrOthers, contractName, h.threadId, loopId, time.Now().Unix()),
		"1.0.0", commonPb.RuntimeType(runTime), wasmBin, pairs)
	// gas limit
	if gasLimit > 0 {
		var limit = &commonPb.Limit{GasLimit: gasLimit}
		payload.Limit = limit
	}

	//
	//method := syscontract.ContractManageFunction_INIT_CONTRACT.String()
	//
	//payload := &commonPb.Payload{
	//	ChainId: chainId,
	//	Contract: &commonPb.Contract{
	//		ContractName:    fmt.Sprintf(templateStr, contractName, h.threadId, loopId, time.Now().Unix()),
	//		ContractVersion: "1.0.0",
	//		RuntimeType:     commonPb.RuntimeType(runTime),
	//	},
	//	Method:      method,
	//	Parameters:  pairs,
	//	ByteCode:    wasmBin,
	//	Endorsement: nil,
	//}

	endorsement, err := acSign(payload)
	if err != nil {
		return err
	}

	resp, err = sendRequest(sk3, client, &InvokerMsg{txType: commonPb.TxType_INVOKE_CONTRACT,
		txId: txId, chainId: chainId}, orgId, userCrtPath, payload, endorsement)
	if err != nil {
		return err
	}

	if checkResult && resp.Code != commonPb.TxStatusCode_SUCCESS {
		return fmt.Errorf(respStr, resp)
	}

	return nil
}

// upgradeContractHandler upgrade contract handler
type upgradeContractHandler struct {
	threadId int
}

func (h *upgradeContractHandler) handle(client apiPb.RpcNodeClient, sk3 crypto.PrivateKey, orgId string,
	userCrtPath string, loopId int) error {
	txId := utils.GetTimestampTxId()

	wasmBin, err := ioutil.ReadFile(wasmPath)
	if err != nil {
		return err
	}

	var pairs []*commonPb.KeyValuePair
	payload, _ := GenerateUpgradeContractPayload(fmt.Sprintf(templateStrOthers, contractName, h.threadId, loopId, time.Now().Unix()),
		version, commonPb.RuntimeType(runTime), wasmBin, pairs)
	// gas limit
	if gasLimit > 0 {
		var limit = &commonPb.Limit{GasLimit: gasLimit}
		payload.Limit = limit
	}
	payload.TxId = txId
	payload.ChainId = chainId
	payload.Timestamp = time.Now().Unix()
	endorsement, err := acSign(payload)
	if err != nil {
		return err
	}

	resp, err = sendRequest(sk3, client, &InvokerMsg{txType: commonPb.TxType_INVOKE_CONTRACT,
		txId: txId, chainId: chainId}, orgId, userCrtPath, payload, endorsement)
	if err != nil {
		return err
	}

	if checkResult && resp.Code != commonPb.TxStatusCode_SUCCESS {
		return fmt.Errorf(respStr, resp)
	}

	return nil
}

func GenerateUpgradeContractPayload(contractName, version string, runtimeType commonPb.RuntimeType, bytecode []byte,
	initParameters []*commonPb.KeyValuePair) (*commonPb.Payload, error) {
	payload, err := utils.GenerateInstallContractPayload(contractName, version, runtimeType, bytecode, initParameters)
	if err != nil {
		return nil, err
	}
	payload.Method = syscontract.ContractManageFunction_UPGRADE_CONTRACT.String()
	return payload, nil
}

func sendRequest(sk3 crypto.PrivateKey, client apiPb.RpcNodeClient, msg *InvokerMsg, orgId, userCrtPath string,
	payload *commonPb.Payload, endorsers []*commonPb.EndorsementEntry) (*commonPb.TxResponse, error) {

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Duration(requestTimeout)*time.Second))
	defer cancel()

	// 构造Sender
	var sender *acPb.Member
	if authType == sdk.Public {
		pubKey := sk3.PublicKey()
		memberInfo, err := pubKey.String()
		if err != nil {
			return nil, err
		}
		sender = &acPb.Member{
			OrgId:      "",
			MemberInfo: []byte(memberInfo),
			MemberType: acPb.MemberType_PUBLIC_KEY,
		}
	} else if authType == sdk.PermissionedWithKey {
		pubKey := sk3.PublicKey()
		memberInfo, err := pubKey.String()
		if err != nil {
			return nil, err
		}
		sender = &acPb.Member{
			OrgId:      orgId,
			MemberInfo: []byte(memberInfo),
			MemberType: acPb.MemberType_PUBLIC_KEY,
		}
	} else {
		file := fileCache.Read(userCrtPath)
		if useShortCrt {
			certId, err := certCache.Read(userCrtPath, *file, hashAlgo)
			if err != nil {
				return nil, fmt.Errorf("fail to compute the identity for certificate [%v]", err)
			}
			sender = &acPb.Member{
				OrgId:      orgId,
				MemberInfo: *certId,
				MemberType: acPb.MemberType_CERT_HASH,
			}
		} else {
			sender = &acPb.Member{
				OrgId:      orgId,
				MemberInfo: *file,
				//IsFullCert: true,
			}
		}
	}

	// 构造Header

	req := &commonPb.TxRequest{
		Payload: payload,
		Sender: &commonPb.EndorsementEntry{
			Signer: sender,
		},
	}
	if len(endorsers) > 0 {
		req.Endorsers = endorsers
	}
	// 拼接后，计算Hash，对hash计算签名
	rawTxBytes, err := utils.CalcUnsignedTxRequestBytes(req)
	if err != nil {
		return nil, err
	}

	hashType, err := getHashType(hashAlgo)
	if err != nil {
		return nil, err
	}

	signBytes, err := sdkutils.SignPayloadBytesWithHashType(sk3, hashType, rawTxBytes)
	if err != nil {
		return nil, err
	}

	req.Sender.Signature = signBytes

	result, err := client.SendRequest(ctx, req)
	if err != nil {
		if statusErr, ok := status.FromError(err); ok && statusErr.Code() == codes.DeadlineExceeded {
			return nil, fmt.Errorf("client.call err: deadline\n")
		}
		return nil, fmt.Errorf("client.call err: %v\n", err)
	}
	return result, nil
}

func makeKvsOthers(threadId, loopId int) ([]*commonPb.KeyValuePair, error) {
	var outKvs []*commonPb.KeyValuePair
	atomic.AddInt64(&totalSentTxsOthers, 1)
	for _, p := range globalPairs {
		var val []byte
		switch {
		case p.Unique:
			val = []byte(fmt.Sprintf(templateStrOthers, p.Value, threadId, loopId, time.Now().UnixNano()))
		case 0 < p.RandomRate && p.RandomRate < 100:
			if isRandom(p.RandomRate) {
				val = []byte(fmt.Sprintf(templateStrOthers, p.Value, threadId, loopId, time.Now().UnixNano()))
				atomic.AddInt64(&totalRandomSentTxs, 1)
			} else {
				val = []byte(p.Value)
			}
		case p.Decrease:
			p.mu.Lock()
			val = []byte(fmt.Sprintf("%d", p.IntValue))
			p.IntValue--
			p.mu.Unlock()
			atomic.AddInt64(&totalRandomSentTxs, 1)
		case p.Increase:
			p.mu.Lock()
			val = []byte(fmt.Sprintf("%d", p.IntValue))
			p.IntValue++
			p.mu.Unlock()
			atomic.AddInt64(&totalRandomSentTxs, 1)
		case p.ValueFormat != "":
			var err error
			val, err = addFormatValue(p)
			if err != nil {
				return nil, err
			}
		default:
			val = []byte(p.Value)
		}

		outKvs = append(outKvs, &commonPb.KeyValuePair{
			Key:   p.Key,
			Value: val,
		})
	}
	return outKvs, nil
}

func addFormatValue(p *KeyValuePair) ([]byte, error) {
	valueParams := make([]int64, len(p.ValueParams))
	p.mu.Lock()
	for i := 0; i < len(p.ValueParams); i++ {
		v := p.ValueParams[i]
		valueParams[i] = p.Values[i]
		if v.Increase {
			if v.EndValue < p.Values[i] && v.LoopType == LoopTypeEnd {
				if !p.ArriveArr[i] {
					p.ArriveArr[i] = true
					p.ArriveCount++
				}
				if p.EndCount == len(p.ValueParams) && p.ArriveCount >= len(p.Values) {
					p.mu.Unlock()
					return nil, ArriveTargetError
				}
				continue
			} else if v.EndValue < p.Values[i] && v.LoopType == LoopTypeRestart {
				p.Values[i] = v.TempIntValue
			}
			p.Values[i]++
			if p.Values[i] > p.IntPows[i] {
				p.Values[i] = p.Values[i] % p.IntPows[i]
			}
		} else {
			if v.EndValue > p.Values[i] && v.LoopType == LoopTypeEnd {
				if !p.ArriveArr[i] {
					p.ArriveArr[i] = true
					p.ArriveCount++
				}
				if p.EndCount == len(p.ValueParams) && p.ArriveCount >= len(p.Values) {
					p.mu.Unlock()
					return nil, ArriveTargetError
				}
				continue
			} else if v.EndValue > p.Values[i] && v.LoopType == LoopTypeRestart {
				p.Values[i] = v.TempIntValue
			}
			p.Values[i]--
		}
	}
	args := make([]interface{}, len(valueParams))
	for i, v := range valueParams {
		args[i] = v
	}
	val := []byte(fmt.Sprintf(p.ValueFormat, args...))
	p.mu.Unlock()
	return val, nil
}

/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"chainmaker.org/chainmaker/common/v2/crypto"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
)

var (
	templateStr    = "%s_%d_%d"
	resultFmtStr   = "exec result, orgid: %s, host: %s, method: %s, txid: %s, resp: %+v \n"
	resultFmtStrPk = "exec result, host: %s, method: %s, txid: %s, resp: %+v \n"
)

const (
	invokerMethod      = "invoke"
	queryMethod        = "query"
	createContractStr  = "createContract"
	upgradeContractStr = "upgradeContract"
	analyse            = "analyse"
)

// 用来控制是否是最后一次打印结果信息
const (
	FinalPrint    = true
	NotFinalPrint = false
)

// 用来控制随机生成参数的数据（需要确认）
var totalSentTxs int64
var totalRandomSentTxs int64

// RequestParam 封装交易请求的结构体
type RequestParam struct {
	Param     *commonPb.TxRequest
	RequestId int64
}

// 交易请求时延，用来记录成功发起交易的时间 key:txId
var txLatency sync.Map

// 请求参数队列，存放请求参数，队列的数量为节点的数量
// 队列的index为队列的下标
var paramQueues []chan RequestParam

var privateKeys []crypto.PrivateKey

// 每一个请求参数的唯一id
var requestId int64

// 生产信号，当该chan接收到数据时，开始生产请求参数
var produceSignal chan int

// 首次生产完毕信号这个chan只会使用一次，收到值的时候记录开始时间
var firstComplete chan int

// 生产因子，用来控制生产消息的数量
var productFactor int

// 中断信号 不可逆中断状态，一旦检测到中断信号为true，停止所有的生产行为
var interruptSignal bool

// 默认存在节点数量的sdk客户端，用来生成请求参数，开启订阅等
var defaultSdkClients []*sdk.ChainClient

// 用来关闭订阅的chan
var closeSubChan chan struct{}

// ComputeFactor 计算因子，最短请求平均时延为2ms 参数构建时间为0.66ms 2/0.66 = 3所以这里使用3作为计算因子
// 所以为每3个线程分配1个chan使其达到生产消费协调
var computeFactor = 3

// paramChanCount 参数队列的数量,根据线程数量动态增加
var paramChanCount int

// endTime 用来记录请求结束的时间
var endTime time.Time

// initParallel 初始化压测的参数
func initParallel() error {
	if nodeNum > threadNum {
		threadNum = nodeNum
	}
	if err := initSubClient(); err != nil {
		return err
	}
	closeSubChan = make(chan struct{}, 1)
	// 以下的参数在预构建时才会用到，所以使用开关判断
	if prepareBuild {
		produceSignal = make(chan int, threadNum)
		firstComplete = make(chan int, 1)
		// 每次为每个线程生产5个待处理的请求参数, 这个参数作为生产因此，使生产>消费切占用最少内存
		// 确保有足够的生产时间，所以预留一个productFactor作为buffer用来给消费端消耗
		if loopNum > computeFactor*5 {
			productFactor = computeFactor * 5
		} else {
			productFactor = computeFactor
		}
		if threadNum < computeFactor && threadNum < 100 {
			computeFactor = 1
		}
		if threadNum%computeFactor > 0 {
			paramChanCount = threadNum/computeFactor + 1
		} else {
			paramChanCount = threadNum / computeFactor
		}
		paramQueues = make([]chan RequestParam, paramChanCount)
		for i := 0; i < paramChanCount; i++ {
			paramQueues[i] = make(chan RequestParam, productFactor*2)
		}
		// 初始化完成开始生产请求参数
		// 开始生产请求参数
		go producer(invokerMethod)
		produceSignal <- -1
	}
	// 初始化记录交易延时的map
	txLatency = sync.Map{}
	return nil
}

// parallel 压测入口方法，如果是执行交易使用新逻辑否则使用旧逻辑
func parallel(method string) error {
	switch method {
	case invokerMethod:
		return parallelInvoke(method)
	default:
		return parallelOthers(method)
	}
}

// 压力测试主方法
func parallelInvoke(method string) error {
	if err := initParallel(); err != nil {
		return err
	}
	timeoutChan := make(chan struct{}, threadNum)
	doneChan := make(chan struct{}, threadNum)
	statistician := getStatistician()
	// 创建线程对象
	threads, err := threadFactory(threadNum, doneChan, timeoutChan, statistician)
	if err != nil {
		return err
	}
	if !onlySend {
		// 开启节点订阅
		go subNodes(statistician, -1, -1)
	}
	// 开启结果收集
	go statistician.collect()
	// 首批请求参数生产完毕记录时间
	recordStartTime(statistician)
	// 启动线程，并发请求
	go parallelStart(threads)
	// 定时打印结果
	printTicker := time.NewTicker(time.Duration(printTime) * time.Second)
	go printResult(printTicker, statistician)
	// 等待超时或请求执行完毕
	listenAndExit(timeoutChan, doneChan)
	endTime = time.Now()
	finalPrint(statistician, printTicker)
	return nil
}

// recordStartTime 记录压测的开始时间
// 如果开启了请求参数预构建则等构建完毕后才开始记录，否则直接记录当前时间为开始时间并返回
func recordStartTime(statistician *Statistician) error {
	if !prepareBuild {
		statistician.startTime = time.Now()
		return nil
	}
	for {
		select {
		case _, ok := <-firstComplete:
			if !ok {
				fmt.Println("chan close exit;")
				return nil
			}
			// 订阅后记录当前时间
			statistician.startTime = time.Now()
			return nil
		}
	}
}

// finalPrint函数用于在区块链高度不再增长时输出统计信息。
//
// 参数:
// statistician (*Statistician): 一个指向Statistician结构体的指针，该结构体包含了需要打印的统计详情。
//
// 功能描述:
// 1. 首先打印提示信息"final print :"。
// 2. 初始化一个变量lastHeight为0，用于存储上一次查询到的区块高度。
// 3. 进入无限循环，不断尝试获取最新的区块高度。
// 4. 调用getBlockHeight函数获取当前区块链高度。如果获取过程中发生错误，则打印错误信息并退出函数。
// 5. 比较当前高度与上一次的高度，如果两者相同，说明区块高度未发生变化，此时调用statistician.PrintDetails()方法输出统计详情，并结束循环。
// 6. 如果区块高度有变化，则更新lastHeight为当前高度，并让程序暂停一秒后继续下一次循环，以避免频繁查询。
func finalPrint(statistician *Statistician, printTicker *time.Ticker) {
	fmt.Printf("\n[TRANSACTION] Sending complete \n")
	printTicker.Stop()
	if onlySend {
		fmt.Println("├─ Start time: ", statistician.startTime)
		fmt.Println("└─ End  time: ", endTime)
		fmt.Println()
		statistician.printDetails(FinalPrint)
		return
	}
	lastHeight := uint64(0)
	for {
		height, err := getBlockHeight()
		if err != nil {
			fmt.Printf("get block height err: %s\n", err.Error())
			return
		}
		if height == lastHeight {
			fmt.Println("├─ Latest block height: ", lastHeight)
			fmt.Println("├─ Start time: ", statistician.startTime)
			fmt.Println("└─ End  time: ", endTime)
			fmt.Println()
			statistician.printDetails(FinalPrint)
			closeSubChan <- struct{}{}
			return
		} else {
			lastHeight = height
			time.Sleep(time.Second * time.Duration(checkInterval))
		}
	}
}

// 函数负责定时输出统计信息
// 参数:
// printTicker (*time.Ticker): 一个时间ticker，每隔一定周期发送一个信号。
// statistician (*Statistician): 一个指向Statistician结构体的指针，封装了统计数据及其打印方法。
func printResult(printTicker *time.Ticker, statistician *Statistician) {
	for {
		select {
		case <-printTicker.C:
			go statistician.printDetails(NotFinalPrint)
		}
	}
}

// PrintDetails方法用于打印Statistician的统计详情。
// 成员变量修改:
// endTime (time.Time): 更新为当前时间，表示统计结束的时间点。
// elapsedSeconds (float32): 计算从开始到结束的持续时间（以秒为单位）。
func (s *Statistician) printDetails(isFinal bool) {
	m := make(map[string]interface{})
	if isFinal {
		s.endTime = endTime
		s.elapsedSeconds = float32(endTime.Sub(s.startTime).Seconds())
	} else {
		s.endTime = time.Now()
		s.elapsedSeconds = float32(time.Now().Sub(s.startTime).Seconds())
	}
	s.run(m, s.rpcPrint())
	if !onlySend {
		s.run(m, s.usualPrint(), s.chainPrint())
	}
	jsonChainByte, err := json.Marshal(m)
	if err != nil {
		fmt.Printf("printDetails json marshal err: %s\n", err.Error())
		fmt.Println("It may be due to the short pressure testing time or " +
			"insufficient quantity. Please adjust the pressure testing parameters")
		return
	}
	if !isFinal {
		fmt.Printf("\n[PROCESSING] Result set: %s \n", string(jsonChainByte))
	} else {
		fmt.Printf("[PERFORMANCE] Metrics result set: %s", string(jsonChainByte))
	}
}

// 定义打印选项类型为一个函数，该函数接收一个map[string]interface{}作为参数
type printOpt func(map[string]interface{})

// run方法遍历并执行所有提供的打印选项函数。
// 每个选项函数会修改传入的map，向其中添加或更新键值对，以收集各类统计信息。
func (s *Statistician) run(m map[string]interface{}, opts ...printOpt) {
	for _, opt := range opts {
		opt(m)
	}
}

// usualPrint 返回一个printOpt函数，用于填充常规统计信息到给定的映射中
// 包括线程数、循环次数以及统计的开始和结束时间
func (s *Statistician) usualPrint() printOpt {
	return func(m map[string]interface{}) {
		m["loopNum"] = loopNum
		m["startTime"] = s.startTime.Format("2006-01-02 15:04:05")
		m["endTime"] = s.endTime.Format("2006-01-02 15:04:05")
		m["elapsed"] = s.elapsedSeconds
	}
}

// chainPrint返回一个printOpt函数，它负责在映射中添加区块链相关的统计结果。
// 这包括调用outBlockInfo和outNodeBlockInfo方法获取数据，并存储在一个ChainResultSet结构中。
func (s *Statistician) chainPrint() printOpt {
	return func(m map[string]interface{}) {
		chainResult := &ChainResultSet{}
		s.outNodeBlockInfo(chainResult)
		m["chainResult"] = *chainResult
	}
}

// rpcPrint返回一个printOpt函数，用于向映射中添加RPC调用的统计结果。
// 具体操作是调用outRpcInfo方法填充RpcResultSet结构，并将其存入映射中。
func (s *Statistician) rpcPrint() printOpt {
	return func(m map[string]interface{}) {
		rpcResult := &RpcResultSet{Nodes: make(map[string]*RpcInfo)}
		s.outRpcInfo(rpcResult)
		m["rpcResult"] = *rpcResult
	}
}

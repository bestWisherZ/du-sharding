/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
)

// producer 是一个生成器函数，根据给定的方法名执行压力测试构建任务。
// 它持续工作直到满足预设的请求总数（由 threadNum 和 loopNum 共同决定）。
// 通过 select-case 机制监听来自 produceSignal 的通道信号，以控制任务的分配和执行。
// 当接收到特定信号时，会触发不同类型的生产行为：
//   - 若信号值为 -1，表示需要对所有节点执行生产操作。
//   - 否则，根据信号值创建新的 goroutine 针对特定节点执行生产操作。
//
// 参数:
//
//	method string: 压力测试所采用的方法名称，用于获取对应的构建器。
func producer(method string) {
	builder := stressBuilderFactory(method)
	for {
		select {
		case index := <-produceSignal:
			if index == -1 {
				for i := 0; i < paramChanCount; i++ {
					produce(builder, i)
				}
				firstComplete <- 1
			} else {
				go produce(builder, index)
			}
		}
	}
}

// stressBuilderFactory 是一个工厂函数，根据提供的方法名字符串，
// 创建并返回对应类型的 StressBuilder 实例。此函数支持以下方法：
//   - invokerMethod: 用于创建执行智能合约调用的构建器。
//   - queryMethod: 用于创建执行智能合约查询的构建器。
//   - createContractStr: 用于创建新智能合约的构建器。
//   - upgradeContractStr: 用于升级智能合约的构建器。
//
// 参数:
//
//	method string: 指定要创建的压力测试构建器类型的方法名。
//
// 返回:
//
//	builder StressBuilder: 与指定方法对应的压力测试构建器实例。
//	  可能是 Invoke、Query、Create 或 Upgrade 类型的实例。
func stressBuilderFactory(method string) (builder StressBuilder) {
	switch method {
	case invokerMethod:
		builder = Invoke{}
	}
	return
}

// produce 使用提供的压力测试构建器生成请求参数，并将这些参数放入指定索引的队列中。
// 函数内部通过循环生成多个请求参数，具体数量由 productFactor 决定。
// 在循环过程中，会检查全局的 interruptSignal，若信号为真，则中断循环并提前返回，
// 以此实现对生产过程的外部控制。每生成一个请求参数，都会将其原子递增的唯一 ID（requestId）
// 与参数一同封装进 RequestParam 结构体，并通过通道发送到对应的 paramQueues[index] 中。
// 如果在构建参数时发生错误，该函数会打印错误信息但不会停止其他参数的生成。
//
// 参数:
//
//	builder StressBuilder: 用于生成请求参数的压力测试构建器。
//	index int: 指定将生成的参数放入哪一个索引的队列中。
func produce(builder StressBuilder, index int) {
	for i := 0; i < productFactor; i++ {
		if interruptSignal {
			return
		}
		atomic.AddInt64(&requestId, 1)
		param, err := builder.Build(requestId, index)
		if err != nil {
			fmt.Printf("producer err: %s\n", err.Error())
			return
		}
		paramQueues[index] <- RequestParam{
			Param:     param,
			RequestId: requestId,
		}
	}
}

// parallelStart 分批次并发启动所有线程执行消耗操作。
// 首先，计算每批启动的线程数量，确保至少启动一批，即使线程总数少于10。
// 然后，基于总爬升时间分配每批启动之间的间隔，单位为秒。
// 函数通过两层循环来控制并发启动：
//   - 外层循环按每批count个线程启动。
//   - 内层循环在每批内实际启动线程，并计数直至达到当前批次应启动的数量。
//
// 启动完每批线程后，程序会暂停指定的interval时间，之后继续启动下一批，直到所有线程都被启动。
//
// 参数:
//
//	threads []*Thread: 一个线程切片，每个元素代表一个待启动的线程实体，包含消耗操作的方法consume。
func parallelStart(threads []*Thread) {
	// 计算每批启动的线程数量，若总数小于10，则至少启动1个
	count := threadNum / 10
	if count == 0 {
		count = 1
	}
	// 计算每批启动间隔，确保总爬升时间内均匀分布启动时间
	interval := time.Duration(climbTime/count) * time.Second
	// 主循环以控制启动批次
	for index := 0; index < threadNum; {
		// 内循环控制每批内线程的并发启动
		for j := 0; j < 10; j++ {
			if index >= threadNum {
				break
			}
			go threads[index].invoke()
			index++
		}
		time.Sleep(interval)
	}
}

// listenAndExit 监听完成信号和超时，以控制并发任务的有序退出。
// 该函数等待所有任务完成或到达预设的超时时间，然后执行必要的清理工作并终止相关信号通道。
//
// 参数:
//
//	timeoutChan chan struct{}: 超时信号通道，用于通知任务超时。
//	doneChan chan struct{}: 完成信号通道，每次有任务完成时接收信号。
//	printTicker *time.Ticker: 定时器，周期性执行某些打印操作，函数结束前需停止。
//
// 功能说明:
// 1. 初始化一个超时定时器，周期为 timeout 秒。
// 2. 使用 sync.Once 确保超时逻辑只执行一次，避免重复发送超时信号。
// 3. 循环监听 doneChan 和 timeoutTicker.C 的信号：
//   - 收到 doneChan 信号，表示有一个任务完成，累加完成计数。
//   - 若超时（从 timeoutTicker 收到信号），则通过 once 执行超时处理：
//     向 timeoutChan 发送 threadNum 次信号，通知所有未完成的任务超时。
//
// 4. 当所有任务完成（doneCount 达到 threadNum）或发生超时，执行以下操作：
//   - 设置全局中断信号为 true。
//   - 停止 printTicker 防止资源泄露。
//   - 关闭各个通道以结束相关协程的执行。
func listenAndExit(timeoutChan, doneChan chan struct{}) {
	doneCount := 0
	timeoutTicker := time.NewTicker(time.Duration(timeout) * time.Second)
	once := sync.Once{}
	for {
		if doneCount >= threadNum {
			break
		}
		select {
		case <-doneChan:
			doneCount++
		case <-timeoutTicker.C:
			once.Do(func() {
				for i := 0; i < threadNum; i++ {
					timeoutChan <- struct{}{}
				}
			})
		}
	}
	interruptSignal = true
	close(timeoutChan)
	close(doneChan)
	if prepareBuild {
		close(produceSignal)
	}
	timeoutTicker.Stop()
}

// Thread 结构体表示一个执行特定任务的线程或协程实例，
// 包含了执行循环次数、通信通道、统计信息以及与 gRPC 服务的连接信息。
type Thread struct {
	id           int                // 是线程的唯一标识符，用于区分不同的线程实例
	loopNum      int                // 指定线程需要执行循环操作的次数
	doneChan     chan struct{}      // 是一个通道，用于传递完成信号。当线程完成其指定任务后，会向此通道发送信号
	timeoutChan  chan struct{}      // 用于接收超时信号。当接收到信号时，表明当前线程可能需要根据超时情况做出响应或终止操作
	statistician *Statistician      // 是一个指向 Statistician 实例的指针，用于收集和统计线程执行过程中的性能数据或其他相关信息
	sdkClients   []*sdk.ChainClient // sdkClient对象用来发起请求,每个线程包含host数组数量的client以向不同节点发起请求
	index        int                // 用来线程去消费的队列索引
}

// threadFactory 是一个工厂方法，用于创建指定数量的线程实例。
// 每个线程实例都会被分配一个唯一的ID，并且配置了执行循环次数、通信通道、统计信息以及与gRPC服务的连接信息。
// 参数:
//   - number: 需要创建的线程数量。
//   - doneChan: 用于传递完成信号的通道。
//   - timeoutChan: 用于接收超时信号的通道。
//   - statistician: 用于收集和统计线程执行过程中的性能数据或其他相关信息的统计器实例。
//
// 返回值:
//   - []*Thread: 创建的线程实例列表。
//   - error: 如果在创建过程中发生错误，将返回非nil的error。
func threadFactory(number int, doneChan, timeoutChan chan struct{}, statistician *Statistician) ([]*Thread, error) {
	threads := make([]*Thread, number)
	for i := 0; i < number; i++ {
		t := &Thread{id: i, loopNum: loopNum, doneChan: doneChan, timeoutChan: timeoutChan, statistician: statistician}
		t.index = t.id / computeFactor
		for i := range hosts {
			sdkClient, err := getSdkClient(i)
			if err != nil {
				return nil, err
			}
			t.sdkClients = append(t.sdkClients, sdkClient)
		}
		threads[i] = t

	}
	return threads, nil
}

// consume 是 Thread 类型的方法，负责处理单个线程的工作流程，包括循环发送请求、处理响应、统计以及错误日志记录。
// 功能描述:
// 1. 循环处理: 方法会在给定的循环次数 (t.loopNum) 内持续工作。
// 2. 超时与中断控制:
//   - 监听 t.timeoutChan，一旦收到信号，线程将通过 t.doneChan 发送完成信号并提前返回。
//   - 内部的 select 语句检查 paramQueues[t.index] 是否关闭，若关闭则同样结束当前线程。
//
// 3. 请求发送与统计:
//   - 从 paramQueues[t.index] 队列接收请求参数，若队列非空且未中断，尝试发送交易至 gRPC 客户端。
//   - 成功发送后，更新统计信息，包括总请求数、请求耗时及节点 ID，并可能触发生产者线程继续生产数据。
//
// 4. 错误处理与日志:
//   - 发生错误时，根据配置 (recordLog) 记录错误日志，包含线程 ID、循环 ID 和节点 ID 等详细信息。
//
// 5. 延迟与节流:
//   - 每次循环后，线程会暂停 sleepTime 毫秒，以控制请求频率。
//
// 结束条件:
// - 执行完所有循环次数。
// - 收到超时信号。
// - 请求队列关闭。
// 最终，无论循环是否自然结束，都会通过 t.doneChan 发送完成信号。
func (t *Thread) invoke() {
	for i := 0; i < t.loopNum; i++ {
		select {
		case <-t.timeoutChan:
			// 超时，结束线程
			t.doneChan <- struct{}{}
			return
		default:
			reqParam := t.getParam(i)
			if reqParam == nil {
				return
			}
			start := time.Now()
			var err error
			nodeIndex := i % nodeNum
			var orgId = "public"
			if sdk.AuthType(authTypeUint32) != sdk.Public {
				if len(orgIDs) == 1 {
					orgId = orgIDs[0]
				} else if len(orgIDs) <= nodeIndex {
					panic("orgId count not equals host count")
				} else {
					orgId = orgIDs[nodeIndex]
				}
			}
			err = sendTx(t.sdkClients[nodeIndex], orgId, nodeIndex, reqParam)
			// 计算请求时延
			elapsed := time.Since(start).Milliseconds()
			go func(e error, elapsed int64) {
				// 结果进入结果集
				atomic.AddUint32(&t.statistician.totalCount, 1)
				t.statistician.reqStatC <- &reqStat{
					success: e == nil,
					elapsed: elapsed,
					nodeId:  nodeIndex,
				}
			}(err, elapsed)
			if recordLog && err != nil {
				log.Errorf("threadId: %d, loopId: %d, nodeId: %d, err: %s", t.id, i, t.index, err)
			}
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)

		}
	}
	// 自然循环结束，发送完成信号
	t.doneChan <- struct{}{}
}

func (t *Thread) getParam(loop int) *commonPb.TxRequest {
	// 如果开启了预构建则从预先设计好的channel中获取param, 否则
	if prepareBuild {
		// 如果chan 关闭，被分配到该chan的线程也一起关闭
		param, ok := <-paramQueues[t.index]
		if !ok {
			t.doneChan <- struct{}{}
			return nil
		}
		// 如果队列中数量小于生产因子，发送生产信号
		if len(paramQueues[t.index]) < productFactor && !interruptSignal {
			produceSignal <- t.index
		}
		return param.Param
	}
	// 使用线程id+当前的loop作为唯一id
	builder := stressBuilderFactory(invokerMethod)
	s := strconv.Itoa(t.id) + strconv.Itoa(loop)
	id, err := strconv.Atoi(s)
	if err != nil {
		t.doneChan <- struct{}{}
		return nil
	}
	param, err := builder.Build(int64(id), t.index)
	if err != nil {
		fmt.Println("param build err:", err.Error())
		t.doneChan <- struct{}{}
		return nil
	}
	return param
}

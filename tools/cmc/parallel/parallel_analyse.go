/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"github.com/spf13/cobra"
)

// 提供从某一个区块高度区间范围内的性能指标统计
func statCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   analyse,
		Short: "Analyse",
		RunE: func(_ *cobra.Command, _ []string) error {
			paramRead()
			if err := paramCheck(); err != nil {
				return err
			}
			return statMain()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		// 证书配置
		signCrtPathsStringFlag, signKeyPathsStringFlag, orgIDsStringFlag,
		userCrtPathsStringFlag, userKeyPathsStringFlag, caPathsStringFlag, useTLSFlag,
		userEncKeyPathsStringFlag, userEncCrtPathsStringFlag,
		// 压测请求配置
		checkIntervalFlag,
		// 链配置
		hostsStringFlag, hashAlgoFlag, chainIdFlag,
		authTypeUint32Flag, gasLimitFlag, hostnamesStringFlag,
	})
	flags := cmd.Flags()
	flags.Int64VarP(&startBlock, "start-block", "", 0, "subscribe start block height")
	flags.Int64VarP(&endBlock, "end-block", "", 1, "subscribe end block height")
	return cmd
}

// 定义退出信号
var (
	systemExitChan  chan os.Signal // 操作系统级别结束程序时使用该信号接收
	timeoutExitChan chan struct{}  // 等待超时使用该信号退出
	doneExitChan    chan struct{}  // 程序正常结束使用该退出信号退出
)

// 执行解析前需要做的一些必要的初始化操作，以及检查操作
func analysePrepare() error {
	if endBlock < 0 || startBlock < 0 {
		fmt.Println("start and end block number must be greater than -1")
		return nil
	}
	if endBlock-startBlock < 1 {
		fmt.Println("start height sub end block height must be greater than 1 or equals")
		return nil
	}
	// 初始化用来关闭订阅的chan
	closeSubChan = make(chan struct{}, 0)
	// 初始化用来计算交易延时的map
	txLatency = sync.Map{}
	// 初始化订阅节点客户端
	err := initSubClient()
	if err != nil {
		return err
	}
	// 检查最新区块高度
	maxBlockHeight, err := getBlockHeight()
	if err != nil {
		return err
	}
	if startBlock > int64(maxBlockHeight) {
		return fmt.Errorf("start block height %d is greater than latest block "+
			"height %d", startBlock, maxBlockHeight)
	}
	// 初始化退出信号的chan
	systemExitChan = make(chan os.Signal, 1)
	signal.Notify(systemExitChan, os.Interrupt)
	timeoutExitChan = make(chan struct{}, 1)
	doneExitChan = make(chan struct{}, 1)
	return nil
}

// statMain 对于发起订阅统计前做检查以及必要的初始化操作
func statMain() error {
	if err := analysePrepare(); err != nil {
		return err
	}
	// 获取结果统计对象
	statistician := getStatistician()
	// 订阅节点
	go subNodes(statistician, startBlock, endBlock)
	// 收集结果
	go statistician.collectStat(endBlock)
	// 接收信号并退出
	select {
	case <-systemExitChan:
		fmt.Printf("\n[BLOCK SUBSCRIPTION] Subscription system shutdown\n")
		closeChan()
		close(statistician.cReqStatC)
		printChainDetail(statistician)
		return nil
	case <-timeoutExitChan:
		fmt.Printf("\n[BLOCK SUBSCRIPTION] Subscription timeout\n")
		closeChan()
		close(statistician.cReqStatC)
		printChainDetail(statistician)
		return nil
	case <-doneExitChan:
		fmt.Printf("\n[BLOCK SUBSCRIPTION] Subscription done\n")
		closeChan()
		close(statistician.cReqStatC)
		printChainDetail(statistician)
		return nil
	}
	return nil
}

func closeChan() {
	closeSubChan <- struct{}{}
	close(systemExitChan)
	close(timeoutExitChan)
	close(doneExitChan)
}

// 为区块统计功能定制的参数收集功能，用来收集指定告区块高度区间范围内的需计算的参数指标
// 因为无法得知开始请求的时间，所以以第二个区块为开始区块，第一个区块作为时间依据，
// 以统计范围为 （start, end] 即左开右闭区间开始统计
func (s *Statistician) collectStat(endBlock int64) {
	isFirst := true
	startTime := int64(0)
	startTimeMilli := int64(0)
	// 启动定时任务
	go timeOutCtrl(s, startBlock)
	for {
		select {
		case stat, ok := <-s.cReqStatC:
			// 如果从外部被关闭则结束循环停止统计
			if !ok {
				return
			}
			if isFirst {
				startTime = stat.blockHeader.BlockTimestamp
				startTimeMilli = stat.blockHeader.BlockTimestamp * 1000
				isFirst = false
				continue
			}
			s.statisticianTxBlock(stat, startTimeMilli)
			// 统计节点区块信息（节点）
			s.statisticianNodeTxBlock(stat, startTimeMilli)
			// 开启另一协程，统计完毕回收内存
			go func() {
				for _, v := range stat.txs {
					txLatency.Delete(v)
				}
			}()
			// 计算交易处理速度
			computeSpeed(stat, s)
			s.elapsedSeconds = float32(stat.blockHeader.BlockTimestamp - startTime)
			// 订阅到endBlock的区块高度则发送退出信号
			if stat.blockHeader.BlockHeight == uint64(endBlock) {
				// 达到endBlock的时候发送退出信号
				doneExitChan <- struct{}{}
				return
			}
		}
	}
}

// timeOutCtrl 订阅超时控制
func timeOutCtrl(s *Statistician, latestHeight int64) {
	timeoutTicker := time.NewTicker(time.Duration(checkInterval) * time.Second)
	for {
		select {
		case <-timeoutTicker.C:
			if int64(s.lastBlockHeight) > latestHeight {
				latestHeight = int64(s.lastBlockHeight)
			} else {
				timeoutExitChan <- struct{}{}
				timeoutTicker.Stop()
				return
			}
		}
	}
}

// statCompute 指标计算
// 这里要以第一个区块的出块时间开始时间计算后面的区块的平均指标，所以没有复用压测的逻辑
// 统计链上交易的性能指标
func (s *Statistician) statCompute(stat *cReqStat, milliSec int64) {
	// 统计交易最多的区块高度，块交易数量
	if s.maxTxBlockCount < stat.blockHeader.TxCount {
		s.maxTxBlockHeight = stat.blockHeader.BlockHeight
		s.maxTxBlockCount = stat.blockHeader.TxCount
	}
	// 统计交易最少的区块高度，块交易数量
	if s.minTxBlockCount == 0 {
		s.minTxBlockHeight = stat.blockHeader.BlockHeight
		s.minTxBlockCount = stat.blockHeader.TxCount
	}
	if s.minTxBlockCount > stat.blockHeader.TxCount {
		s.minTxBlockHeight = stat.blockHeader.BlockHeight
		s.minTxBlockCount = stat.blockHeader.TxCount
	}
	// 统计第一次出块时间和最后一次出块时间
	if s.firstBlockTime == 0 {
		s.firstBlockTime = stat.blockHeader.BlockTimestamp
	}
	if s.firstBlockHeight == 0 {
		s.firstBlockHeight = stat.blockHeader.BlockHeight
	}
	// 记录交易总数，区块总数
	s.txTotal += stat.blockHeader.TxCount
	s.blockTotal++
	// 更新最后一次出块的时间，区块高度
	s.lastBlockTime = stat.blockHeader.BlockTimestamp
	s.lastBlockHeight = stat.blockHeader.BlockHeight
	// 当前区块的交易时延记录在一个数组
	for _, v := range stat.txs {
		start, ok := txLatency.Load(v.Payload.TxId)
		if !ok {
			continue
		}
		s.txLatencyMilli = append(s.txLatencyMilli, float64(milliSec-start.(int64)))
	}
	if s.preBlockTimeMilli == 0 {
		s.preBlockTimeMilli = milliSec - s.preBlockTimeMilli
	}
	s.blockMilli = append(s.blockMilli, float64(milliSec-s.preBlockTimeMilli))
	s.preBlockTimeMilli = milliSec
}

// printChainDetail 以json格式输出统计的结果集
func printChainDetail(s *Statistician) error {
	chainResult := &ChainResultSet{}
	s.outNodeBlockInfo(chainResult)
	// 如果没有产出区块则不记录统计直接返回
	jsonByte, err := json.Marshal(chainResult)
	if err != nil {
		fmt.Println("marshal chain result error:", err)
		return err
	}
	fmt.Println()
	fmt.Println("[PERFORMANCE] Metrics result set: ", string(jsonByte))
	return nil
}

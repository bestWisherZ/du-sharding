/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// subNodes 函数负责初始化并管理一组并发任务，这些任务针对每个区块链节点订阅新区块事件。
// 参数:
// statistician (*Statistician): 一个Statistician实例，用于在整个订阅过程中收集统计数据和管理线程。
func subNodes(statistician *Statistician, start, end int64) {
	// 并发启动订阅任务
	for i := 0; i < nodeNum; i++ {
		go func(index int) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			blockChan, err := defaultSdkClients[index].SubscribeBlock(ctx, start,
				end, false, false)
			if err != nil {
				fmt.Printf("[BLOCK SUBSCRIPTION]Node%d: Subscribe to real-time blocks [%d,%d] | Status: %s\n",
					index, start, end, err.Error())
				panic(err)
				return
			}
			// 调试日志，需要调试测试时接触下方代码注释
			//fmt.Printf("subscribe block success [%d,%d] \n", start, end)
			fmt.Printf("[BLOCK SUBSCRIPTION] Node%d: Subscribe to real-time blocks [%d,%d] | "+
				"Status: Active \n", index, start, end)
			// 接收区块并发送到统计对象
			for {
				select {
				case block, ok := <-blockChan:
					if !ok {
						if end == -1 && start == -1 {
							fmt.Printf("[BLOCK SUBSCRIPTION] Node%d: Subscribe to real-time blocks [%d,%d] | "+
								"Status: Inactive \n", index, start, end)
						}
						fmt.Println("[BLOCK SUBSCRIPTION] Subscription end")
						return
					}
					blockInfo, ok := block.(*commonPb.BlockInfo)
					if !ok {
						return
					}
					statistician.cReqStatC <- &cReqStat{
						blockInfo.Block.Header, index, blockInfo.Block.Txs,
					}
				case _, ok := <-closeSubChan:
					if !ok {
						return
					}
					close(closeSubChan)
					return
				}
			}

		}(i)
	}
}

// sendTx 向特定节点发送交易请求。如果请求超时，则返回错误信息
var reqIndex uint64

func sendTx(client *sdk.ChainClient, orgId string, nodeIndex int, req *commonPb.TxRequest) error {
	// 防止在收到响应之前上链的数据不一致情况，这里提前记录交易id
	txLatency.Store(req.Payload.TxId, time.Now().UnixNano()/1e6)
	result, err := client.SendTxRequest(req, requestTimeout, false)
	if err != nil {
		if statusErr, ok := status.FromError(err); ok && statusErr.Code() == codes.DeadlineExceeded {
			return fmt.Errorf("client.call err: deadline\n")
		}
		return fmt.Errorf("client.call err: %v\n", err)
	}
	if showKey {
		j, err := json.Marshal(req.Payload.Parameters)
		if err != nil {
			fmt.Println(err)
		}
		atomic.AddUint64(&reqIndex, 1)
		fmt.Printf("request Index:%d\t param:%s\t \n", reqIndex, string(j))
	}
	if outputResult {
		switch sdk.AuthType(authTypeUint32) {
		case sdk.Public:
			fmt.Printf(resultFmtStrPk, hosts[nodeIndex], method, result.TxId, result)
		default:
			fmt.Printf(resultFmtStr, orgId, hosts[nodeIndex], method, result.TxId, result)
		}
	}

	return nil
}

// getBlockHeight 查询当前区块链的高度。首先创建一个查询线程，然后构建查询高度的请求并发送，最后解析返回的区块信息以获取高度
func getBlockHeight() (uint64, error) {
	blockInfo, err := defaultSdkClients[0].GetLastBlock(false)
	if err != nil {
		return 0, err
	}
	return blockInfo.Block.Header.BlockHeight, nil
}

package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/pb/protogo"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sandbox"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
)

const (
	keyLastHeartbeatHeight = "__heartbeat_last_height"
	keyHeartbeatCount      = "__heartbeat_count"
)

// ShardingHeartbeatContract 分时调度心跳合约。
// 由业务分片在交易池空闲时由 Proposer 节点自动调用，
// 用于在不出空块的约束下推进分时出块计数，避免调度死锁。
type ShardingHeartbeatContract struct{}

func (c *ShardingHeartbeatContract) InitContract() protogo.Response {
	return sdk.Success([]byte("sharding_heartbeat init success"))
}

func (c *ShardingHeartbeatContract) UpgradeContract() protogo.Response {
	return sdk.Success([]byte("sharding_heartbeat upgrade success"))
}

func (c *ShardingHeartbeatContract) InvokeContract(method string) protogo.Response {
	switch method {
	case "ping":
		return c.Ping()
	case "last_height":
		return c.LastHeight()
	case "count":
		return c.Count()
	default:
		return sdk.Error(fmt.Sprintf("unknown method: %s", method))
	}
}

// Ping 心跳方法。
// 参数：shard_id（分片ID）、height（目标高度）
// 每次调用写入最新心跳高度和累计心跳次数，保证每次调用都有状态变更（不被去重）。
func (c *ShardingHeartbeatContract) Ping() protogo.Response {
	args := sdk.Instance.GetArgs()

	shardId := strings.TrimSpace(string(args["shard_id"]))
	heightStr := strings.TrimSpace(string(args["height"]))

	var height int64
	if heightStr != "" {
		var err error
		height, err = strconv.ParseInt(heightStr, 10, 64)
		if err != nil {
			return sdk.Error(fmt.Sprintf("invalid height: %s", heightStr))
		}
	}

	// 写入最新心跳高度
	if err := sdk.Instance.PutStateFromKey(keyLastHeartbeatHeight, heightStr); err != nil {
		return sdk.Error("save last heartbeat height failed")
	}

	// 累加心跳计数
	count, _ := c.loadCount()
	count++
	if err := sdk.Instance.PutStateFromKey(keyHeartbeatCount, strconv.FormatInt(count, 10)); err != nil {
		return sdk.Error("save heartbeat count failed")
	}

	result := fmt.Sprintf("heartbeat ok | shard=%s height=%d count=%d", shardId, height, count)
	return sdk.Success([]byte(result))
}

// LastHeight 查询最近一次心跳的目标高度
func (c *ShardingHeartbeatContract) LastHeight() protogo.Response {
	val, err := sdk.Instance.GetStateFromKey(keyLastHeartbeatHeight)
	if err != nil || strings.TrimSpace(val) == "" {
		return sdk.Success([]byte("0"))
	}
	return sdk.Success([]byte(strings.TrimSpace(val)))
}

// Count 查询心跳累计次数
func (c *ShardingHeartbeatContract) Count() protogo.Response {
	count, err := c.loadCount()
	if err != nil {
		return sdk.Error("load count failed")
	}
	return sdk.Success([]byte(strconv.FormatInt(count, 10)))
}

func (c *ShardingHeartbeatContract) loadCount() (int64, error) {
	s, err := sdk.Instance.GetStateFromKey(keyHeartbeatCount)
	if err != nil || strings.TrimSpace(s) == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func main() {
	if err := sandbox.Start(new(ShardingHeartbeatContract)); err != nil {
		log.Fatal(err)
	}
}

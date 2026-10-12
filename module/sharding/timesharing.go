package sharding

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"chainmaker.org/chainmaker-go/module/blockchain"
	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/globalstate"
	"chainmaker.org/chainmaker-go/module/sharding/heartbeat"
	"chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	netPb "chainmaker.org/chainmaker/pb-go/v2/net"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/spf13/viper"
)

var shardingLog = logger.GetLogger(logger.MODULE_CONSENSUS)

const (
	scheduleMsgPrefix = "SCHED_"
	scheduleAckPrefix = "SCHED_ACK:"
	ackRetryInterval  = 2 * time.Second
	ackMaxRetries     = 3
	relaySeenLimit    = 4096
)

type chainNetService struct {
	chainId    string
	netService protocol.NetService
}

type pendingAck struct {
	stop chan struct{}
}

type ackTracker struct {
	mu      sync.Mutex
	pending map[string]*pendingAck
}

func newAckTracker() *ackTracker {
	return &ackTracker{
		pending: make(map[string]*pendingAck),
	}
}

func (t *ackTracker) track(msg string, send func(string) error) {
	t.mu.Lock()
	if _, exists := t.pending[msg]; exists {
		t.mu.Unlock()
		return
	}
	item := &pendingAck{stop: make(chan struct{})}
	t.pending[msg] = item
	t.mu.Unlock()

	go func() {
		retries := 0
		for {
			if retries >= ackMaxRetries {
				shardingLog.Warnf("分时调度: 跨分片信号重试次数已达上限, msg=%s", msg)
				t.remove(msg)
				return
			}
			if err := send(msg); err != nil {
				shardingLog.Warnf("分时调度: 跨分片信号发送失败, msg=%s, err=%v", msg, err)
			} else {
				shardingLog.Infof("分时调度: 已发送跨分片信号, msg=%s, 第%d次", msg, retries+1)
			}
			retries++
			select {
			case <-item.stop:
				return
			case <-time.After(ackRetryInterval):
			}
		}
	}()
}

func (t *ackTracker) ack(msg string) {
	t.mu.Lock()
	item, ok := t.pending[msg]
	if ok {
		delete(t.pending, msg)
	}
	t.mu.Unlock()
	if ok {
		close(item.stop)
	}
}

func (t *ackTracker) remove(msg string) {
	t.mu.Lock()
	item, ok := t.pending[msg]
	if ok {
		delete(t.pending, msg)
	}
	t.mu.Unlock()
	if ok {
		close(item.stop)
	}
}

type scheduleSignalRelay struct {
	role           string
	shardId        string
	startHeight    uint64
	businessShards map[string]struct{}
	services       []chainNetService

	mu       sync.Mutex
	seen     map[string]struct{}
	seenKeys []string
}

func newScheduleSignalRelay(cfg *shardingsync.ShardingConfig, services []chainNetService) *scheduleSignalRelay {
	businessShards := make(map[string]struct{}, len(cfg.Schedule.BusinessShards))
	for _, shardId := range cfg.Schedule.BusinessShards {
		shardId = strings.TrimSpace(shardId)
		if shardId == "" {
			continue
		}
		businessShards[shardId] = struct{}{}
	}
	return &scheduleSignalRelay{
		role:           strings.ToUpper(cfg.ShardType),
		shardId:        cfg.ShardId,
		startHeight:    cfg.Schedule.StartHeight,
		businessShards: businessShards,
		services:       services,
		seen:           make(map[string]struct{}),
	}
}

func (r *scheduleSignalRelay) handleCrossShard(msg string) bool {
	return r.handleScheduleSignal(msg, "cross-shard", true)
}

func (r *scheduleSignalRelay) handleIntraShard(msg string, from string) {
	r.handleScheduleSignal(msg, "intra-shard:"+from, true)
}

func (r *scheduleSignalRelay) handleScheduleSignal(msg string, source string, relay bool) bool {
	kind, ok := r.validate(msg)
	if !ok {
		return false
	}
	firstSeen := r.markSeen(msg)
	if !firstSeen {
		shardingLog.Debugf("分时调度: 忽略重复阶段信号 | source=%s | msg=%s", source, msg)
		return true
	}

	shardingLog.Infof("分时调度: 接收阶段信号 | kind=%s | source=%s | msg=%s", kind, source, msg)
	scheduler.HandleMessage(msg)
	if relay {
		return r.broadcastInShard(msg, source)
	}
	return true
}

func (r *scheduleSignalRelay) validate(msg string) (string, bool) {
	if r == nil || msg == "" || strings.HasPrefix(msg, scheduleAckPrefix) {
		return "", false
	}
	switch {
	case strings.HasPrefix(msg, "SCHED_BUSINESS_DONE:"):
		if r.role != "BRIDGE" {
			return "", false
		}
		shardId, height, ok := parseScheduleBusinessDone(msg)
		if !ok {
			shardingLog.Warnf("分时调度: 忽略格式非法的业务完成信号 | msg=%s", msg)
			return "", false
		}
		if height <= r.startHeight {
			shardingLog.Warnf("分时调度: 忽略起始高度前的业务完成信号 | height=%d | start=%d | msg=%s", height, r.startHeight, msg)
			return "", false
		}
		if len(r.businessShards) > 0 {
			if shardId == "" {
				shardingLog.Warnf("分时调度: 忽略缺少业务分片ID的完成信号 | msg=%s", msg)
				return "", false
			}
			if _, exists := r.businessShards[shardId]; !exists {
				shardingLog.Warnf("分时调度: 忽略未知业务分片完成信号 | shard=%s | msg=%s", shardId, msg)
				return "", false
			}
		}
		return "business_done", true

	case strings.HasPrefix(msg, "SCHED_BRIDGE_DONE:"):
		if r.role != "BUSINESS" {
			return "", false
		}
		height, ok := parseScheduleBridgeDone(msg)
		if !ok {
			shardingLog.Warnf("分时调度: 忽略格式非法的桥接完成信号 | msg=%s", msg)
			return "", false
		}
		if height <= r.startHeight {
			shardingLog.Warnf("分时调度: 忽略起始高度前的桥接完成信号 | height=%d | start=%d | msg=%s", height, r.startHeight, msg)
			return "", false
		}
		return "bridge_done", true

	default:
		return "", false
	}
}

func (r *scheduleSignalRelay) markSeen(msg string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.seen[msg]; exists {
		return false
	}
	r.seen[msg] = struct{}{}
	r.seenKeys = append(r.seenKeys, msg)
	if len(r.seenKeys) > relaySeenLimit {
		oldest := r.seenKeys[0]
		delete(r.seen, oldest)
		r.seenKeys = r.seenKeys[1:]
	}
	return true
}

func (r *scheduleSignalRelay) broadcastInShard(msg string, source string) bool {
	if len(r.services) == 0 {
		shardingLog.Warnf("分时调度: 阶段信号无法分片内转发，未获取到链内网络服务 | source=%s | msg=%s", source, msg)
		return false
	}
	delivered := false
	for _, svc := range r.services {
		if svc.netService == nil {
			continue
		}
		if err := svc.netService.BroadcastMsg([]byte(msg), netPb.NetMsg_CONSENSUS_MSG); err != nil {
			shardingLog.Warnf("分时调度: 阶段信号分片内转发失败 | chainId=%s | source=%s | msg=%s | err=%v", svc.chainId, source, msg, err)
		} else {
			delivered = true
			shardingLog.Infof("分时调度: 阶段信号已分片内转发 | chainId=%s | source=%s | msg=%s", svc.chainId, source, msg)
		}
	}
	return delivered
}

func parseScheduleBusinessDone(msg string) (string, uint64, bool) {
	const prefix = "SCHED_BUSINESS_DONE:"
	payload := strings.TrimPrefix(msg, prefix)
	if payload == msg || payload == "" {
		return "", 0, false
	}
	parts := strings.Split(payload, ":")
	if len(parts) == 1 {
		height, ok := parseUintField(parts[0])
		return "", height, ok
	}
	height, ok := parseUintField(parts[1])
	if !ok {
		return "", 0, false
	}
	return strings.TrimSpace(parts[0]), height, true
}

func parseScheduleBridgeDone(msg string) (uint64, bool) {
	const prefix = "SCHED_BRIDGE_DONE:"
	payload := strings.TrimPrefix(msg, prefix)
	if payload == msg || payload == "" {
		return 0, false
	}
	parts := strings.Split(payload, ":")
	return parseUintField(parts[0])
}

func parseUintField(value string) (uint64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	var n uint64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + uint64(ch-'0')
	}
	return n, true
}

// StartTimeSharing initializes time-sharing scheduling and sharding sync.
// It keeps the main entry minimal and hosts the sharding-specific logic here.
func StartTimeSharing(server *blockchain.ChainMakerServer, chainConf *localconf.CMConfig) error {
	if server == nil || chainConf == nil {
		return fmt.Errorf("time-sharing requires a server and chain configuration")
	}

	cfg, err := shardingsync.LoadConfig(chainConf)
	var missing viper.ConfigFileNotFoundError
	if errors.As(err, &missing) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load time-sharing config: %w", err)
	}
	if err = cfg.Validate(); err != nil {
		return err
	}
	globalstate.InitWithConfig(cfg)
	globalstate.SetTxSubmitter(func(chainId string, tx interface{}) error {
		t, ok := tx.(*commonpb.Transaction)
		if !ok {
			return fmt.Errorf("invalid tx type for globalstate submitter")
		}
		return server.AddTx(chainId, t, protocol.RPC)
	})
	txPoolChecker := func(chainId, txId string) (bool, error) {
		txs, _, err := server.GetTxsInPoolByTxIds(chainId, []string{txId})
		if err != nil {
			return false, err
		}
		return len(txs) > 0, nil
	}
	globalstate.SetTxPoolChecker(txPoolChecker)
	globalstate.SetTxCommittedChecker(server.IsTxSuccessfullyCommitted)
	crossshard.InitWithConfig(cfg)
	crossshard.SetTxSubmitter(func(chainId string, tx interface{}) error {
		t, ok := tx.(*commonpb.Transaction)
		if !ok {
			return fmt.Errorf("invalid tx type for submitter")
		}
		return server.AddTx(chainId, t, protocol.RPC)
	})
	crossshard.SetTxPoolChecker(txPoolChecker)
	crossshard.SetTxCommittedChecker(server.IsTxSuccessfullyCommitted)

	role := strings.ToUpper(cfg.ShardType)

	// 为调度器注入 txpool pending 查询回调（业务分片和桥接分片均注入自己的交易池）
	chainId := ""
	if len(localconf.ChainMakerConfig.BlockChainConfig) > 0 {
		chainId = localconf.ChainMakerConfig.BlockChainConfig[0].ChainId
	}
	var getPendingFn func() int
	if chainId != "" {
		getPendingFn = func() int {
			status, err := server.GetPoolStatus(chainId)
			if err != nil || status == nil {
				return 0
			}
			return int(status.CommonTxNumInQueue + status.CommonTxNumInPending +
				status.ConfigTxNumInQueue + status.ConfigTxNumInPending)
		}
	}

	var broadcastFn func(string) error
	var tracker *ackTracker
	if cfg.SyncNetwork.Enable {
		tracker = newAckTracker()
		broadcastFn = func(msg string) error {
			tracker.track(msg, shardingsync.BroadcastMessage)
			return nil
		}
	}
	scheduler.Init(scheduler.Config{
		Enable:         true,
		Role:           role,
		ShardId:        cfg.ShardId,
		StartHeight:    cfg.Schedule.StartHeight,
		BusinessBatch:  cfg.Schedule.BusinessBatch,
		BridgeBatch:    cfg.Schedule.BridgeBatch,
		BusinessShards: cfg.Schedule.BusinessShards,
		TotalBatch:     cfg.Schedule.TotalBatch,
		GetPendingFn:   getPendingFn,
	}, nil, broadcastFn)

	shardingLog.Infof("分时调度初始化完成 | 角色=%s | 分片ID=%s | 起始高度=%d | 业务批次=%d | 桥接批次=%d | 动态调度T=%d | 业务分片列表=%v | 同步节点=%v",
		role, cfg.ShardId, cfg.Schedule.StartHeight, cfg.Schedule.BusinessBatch, cfg.Schedule.BridgeBatch, cfg.Schedule.TotalBatch, cfg.Schedule.BusinessShards, cfg.SyncNetwork.Enable)

	// 业务分片和桥接分片都需要心跳模块：
	// - 业务分片：交易池为空时无法出块，卡在业务阶段
	// - 桥接分片：业务阶段结束切换到桥接阶段后，若桥接交易池为空同样卡死
	// scheduler.IsEligible() 内部已通过 phase == role 区分角色，各自只在轮到自己时才注入
	idleThreshold := time.Duration(cfg.Heartbeat.IdleThresholdMs) * time.Millisecond
	if idleThreshold == 0 && cfg.Heartbeat.IdleThresholdSeconds > 0 {
		idleThreshold = time.Duration(cfg.Heartbeat.IdleThresholdSeconds) * time.Second
	}
	heartbeat.Init(heartbeat.Config{
		ShardId:       cfg.ShardId,
		IdleThreshold: idleThreshold,
		ContractName:  cfg.Heartbeat.ContractName,
		Method:        cfg.Heartbeat.Method,
	})
	heartbeat.SetSubmitter(func(chainId string, tx *commonpb.Transaction) error {
		return server.AddTx(chainId, tx, protocol.RPC)
	})
	shardingLog.Infof("心跳模块已启动 | shard=%s role=%s idleThreshold=%s", cfg.ShardId, role, idleThreshold)

	netServices := getNetServices(server, chainConf)
	if len(netServices) != len(chainConf.BlockChainConfig) || len(netServices) == 0 {
		return fmt.Errorf("missing chain network service for schedule signals")
	}
	relay := newScheduleSignalRelay(cfg, netServices)
	for _, svc := range netServices {
		svc := svc
		if err = server.RegisterNetServiceReadyHandler(svc.chainId, func() error {
			return subscribeScheduleSignals([]chainNetService{svc}, relay)
		}); err != nil {
			return err
		}
	}

	if cfg.SyncNetwork.Enable {
		shardingsync.RegisterMessageHandler(func(msg string) {
			if strings.HasPrefix(msg, scheduleAckPrefix) {
				original := strings.TrimPrefix(msg, scheduleAckPrefix)
				tracker.ack(original)
				shardingLog.Infof("分时调度: 收到跨分片确认, msg=%s", original)
				return
			}
			if !strings.HasPrefix(msg, scheduleMsgPrefix) {
				return
			}
			if relay.handleCrossShard(msg) {
				if err := shardingsync.BroadcastMessage(scheduleAckPrefix + msg); err != nil {
					shardingLog.Warnf("分时调度: 跨分片确认发送失败, msg=%s, err=%v", msg, err)
				}
			}
		})
	}

	go shardingsync.Start(chainConf)
	return nil
}

func getNetServices(server *blockchain.ChainMakerServer, chainConf *localconf.CMConfig) []chainNetService {
	if server == nil || chainConf == nil {
		return nil
	}
	services := make([]chainNetService, 0, len(chainConf.BlockChainConfig))
	for _, bc := range chainConf.BlockChainConfig {
		if bc.ChainId == "" {
			continue
		}
		netSvc, err := server.GetNetService(bc.ChainId)
		if err != nil {
			shardingLog.Warnf("分时调度: 获取链内网络服务失败 | chainId=%s | err=%v", bc.ChainId, err)
			continue
		}
		services = append(services, chainNetService{
			chainId:    bc.ChainId,
			netService: netSvc,
		})
	}
	return services
}

func subscribeScheduleSignals(services []chainNetService, relay *scheduleSignalRelay) error {
	if relay == nil {
		return fmt.Errorf("missing schedule relay")
	}
	for _, svc := range services {
		if svc.netService == nil {
			return fmt.Errorf("missing network service for chain %s", svc.chainId)
		}
		chainId := svc.chainId
		err := svc.netService.Subscribe(netPb.NetMsg_CONSENSUS_MSG, func(from string, msg []byte, _ netPb.NetMsg_MsgType) error {
			text := string(msg)
			if !strings.HasPrefix(text, scheduleMsgPrefix) || strings.HasPrefix(text, scheduleAckPrefix) {
				return nil
			}
			relay.handleIntraShard(text, from)
			return nil
		})
		if err != nil {
			return fmt.Errorf("subscribe schedule signals on %s: %w", chainId, err)
		}
		shardingLog.Infof("分时调度: 已订阅分片内阶段信号 | chainId=%s", chainId)
	}
	return nil
}

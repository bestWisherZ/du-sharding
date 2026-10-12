package scheduler

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/logger/v2"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
)

// schedulePlan 保存待生效的下一轮 N/M 计划。
type schedulePlan struct {
	businessBatch int
	bridgeBatch   int
}

type stateSyncProgress struct {
	height uint64
	total  int
	chunks map[int]struct{}
}

const (
	roleBusiness = "BUSINESS"
	roleBridge   = "BRIDGE"

	msgBusinessDone = "SCHED_BUSINESS_DONE"
	msgBridgeDone   = "SCHED_BRIDGE_DONE"

	schedMetricPrefix = "[SCHED_METRIC]"
)

// Scheduler 提供分时出块的许可判断与状态切换。
// 设计目标：在高度超过 startHeight 后，业务分片连出 businessBatch 个块，再切到桥接分片连出 bridgeBatch 个块，反复轮换。
type Scheduler struct {
	mu sync.Mutex

	cfg        Config
	canProduce bool   // 当前是否允许本分片出块
	active     bool   // 是否已经进入分时模式（高度超过 startHeight）
	counter    int    // 当前阶段已出块数量
	phase      string // 当前轮到谁：BUSINESS 或 BRIDGE

	registerFn  func(func(string))
	broadcastFn func(string) error

	expectedBusinessShards map[string]struct{}
	businessDone           map[string]bool
	lastBusinessDoneHeight map[string]uint64
	lastBridgeDoneHeight   uint64
	requiredSyncHeight     map[string]uint64
	requiredSyncHeights    map[string]map[uint64]struct{}
	appliedSyncHeight      map[string]uint64
	completedSyncHeights   map[string]map[uint64]struct{}
	stateSyncProgress      map[string]*stateSyncProgress
	businessSyncHeights    []uint64
	businessCycle          uint64
	lastCheckpointHeight   uint64
	lastCheckpointCycle    uint64
	lastCheckpointSlot     int
	lastCheckpointTotal    int

	// 动态调度相关字段
	pendingPlan   *schedulePlan // 下一轮待生效的 N/M 计划
	bizPendingSum int           // 本轮收到的所有业务分片 txpool pending 总和（桥接分片使用）
	bizPendingCnt int           // 已收到 pending 上报的业务分片数量
}

var (
	global            *Scheduler
	once              sync.Once
	log               *logger.CMLogger
	readyListenersMu  sync.RWMutex
	readyListeners    []func()
	specialCheckerMu  sync.RWMutex
	specialPendingFns []func() bool
)

// RegisterReadyListener registers a callback that is invoked when local
// scheduling conditions change and blocked consensus progress may be retried.
func RegisterReadyListener(listener func()) {
	if listener == nil {
		return
	}
	readyListenersMu.Lock()
	defer readyListenersMu.Unlock()
	readyListeners = append(readyListeners, listener)
}

func notifyReadyListeners() {
	readyListenersMu.RLock()
	listeners := append([]func(){}, readyListeners...)
	readyListenersMu.RUnlock()
	for _, listener := range listeners {
		listener()
	}
}

// NotifyConsensusGate wakes consensus after local conditions changed outside
// normal phase switching, for example when a special sync tx enters txpool.
func NotifyConsensusGate() {
	notifyReadyListeners()
}

// RegisterSpecialPendingChecker registers a local txpool checker used by the
// BlockInfo gate. It keeps non-current-round special sync blocks possible
// without letting ordinary blocks enter consensus while the shard is paused.
func RegisterSpecialPendingChecker(checker func() bool) {
	if checker == nil {
		return
	}
	specialCheckerMu.Lock()
	defer specialCheckerMu.Unlock()
	specialPendingFns = append(specialPendingFns, checker)
}

// HasSpecialPending reports whether the local txpool contains at least one
// configured state-sync transaction.
func HasSpecialPending() bool {
	specialCheckerMu.RLock()
	checkers := append([]func() bool{}, specialPendingFns...)
	specialCheckerMu.RUnlock()
	for _, checker := range checkers {
		if checker() {
			return true
		}
	}
	return false
}

// ShouldEnterConsensus reports whether TBFT may advance into the next height.
func ShouldEnterConsensus(height uint64) bool {
	if ShouldProduce(height) {
		return true
	}
	if !MayProduceSpecial(height) {
		return false
	}
	return HasSpecialPending()
}

// Init 初始化全局调度器。
// 需在节点启动时调用一次（如 CLI Start 时），并在配置中写明当前节点所属角色。
func Init(cfg Config, register func(func(string)), broadcast func(string) error) {
	once.Do(func() {
		// 初始化日志
		log = logger.GetLogger(logger.MODULE_CONSENSUS)

		cfg.fillDefaults()
		s := &Scheduler{
			cfg:                    cfg,
			canProduce:             cfg.Role == roleBusiness, // 初始由业务分片先出
			active:                 false,
			counter:                0,
			phase:                  roleBusiness,
			registerFn:             register,
			broadcastFn:            broadcast,
			expectedBusinessShards: make(map[string]struct{}),
			businessDone:           make(map[string]bool),
			lastBusinessDoneHeight: make(map[string]uint64),
			requiredSyncHeight:     make(map[string]uint64),
			requiredSyncHeights:    make(map[string]map[uint64]struct{}),
			appliedSyncHeight:      make(map[string]uint64),
			completedSyncHeights:   make(map[string]map[uint64]struct{}),
			stateSyncProgress:      make(map[string]*stateSyncProgress),
			businessCycle:          1,
		}
		for _, shardId := range cfg.BusinessShards {
			if shardId == "" {
				continue
			}
			s.expectedBusinessShards[shardId] = struct{}{}
			s.appliedSyncHeight[shardId] = cfg.StartHeight
			s.completedSyncHeights[shardId] = make(map[uint64]struct{})
		}
		global = s
		syncproposer.SetWakeFunc(NotifyConsensusGate)

		// 打印初始化信息
		log.Infof("[Scheduler] ========================================")
		log.Infof("[Scheduler] 分时共识调度器初始化完成")
		log.Infof("[Scheduler] - 启用状态: %v", cfg.Enable)
		log.Infof("[Scheduler] - 分片角色: %s", cfg.Role)
		log.Infof("[Scheduler] - 分片ID: %s", cfg.ShardId)
		log.Infof("[Scheduler] - 起始高度: %d", cfg.StartHeight)
		if cfg.Role == roleBridge {
			if len(cfg.BusinessShards) > 0 {
				log.Infof("[Scheduler] - 业务分片列表: %v", cfg.BusinessShards)
			} else {
				log.Warnf("[Scheduler] - 业务分片列表未配置，将沿用单分片信号切换")
			}
		}
		log.Infof("[Scheduler] ========================================")

		if cfg.Enable {
			// 注册消息回调：接收对端"完成"信号
			if s.registerFn != nil {
				s.registerFn(s.onMessage)
				log.Infof("[Scheduler] 已注册分片间消息回调处理器")
			}
		} else {
			log.Warnf("[Scheduler] 分时共识未启用，将允许所有高度自由出块")
		}
	})
}

// ShouldProduce 在出块前调用，判断当前是否允许出块。
// height 是即将提议的区块高度（last+1）。
func ShouldProduce(height uint64) bool {
	if global == nil || !global.cfg.Enable {
		return true
	}

	global.mu.Lock()
	defer global.mu.Unlock()

	// 未到分时起点，直接允许
	if height <= global.cfg.StartHeight {
		if log != nil && height == global.cfg.StartHeight {
			log.Infof("[Scheduler] 高度 %d 未达到分时起点 (%d)，允许自由出块", height, global.cfg.StartHeight)
		}
		return true
	}

	// 进入分时模式
	if !global.active {
		global.active = true
		log.Infof("[Scheduler] ✓ 进入分时共识模式 | 当前高度=%d, 当前阶段=%s, 当前角色=%s",
			height, global.phase, global.cfg.Role)
		global.logMetricLocked("schedule_start",
			"height", height,
		)
	}

	// 只有当前轮到自己且允许时才放行
	canProduce := global.canProduce && global.phase == global.cfg.Role

	if log != nil {
		if canProduce {
			log.Infof("[Scheduler] ✓ 允许出块 | 高度=%d, 角色=%s, 阶段=%s",
				height, global.cfg.Role, global.phase)
		} else {
			reason := ""
			if !global.canProduce {
				reason = "等待对端完成"
			} else if global.phase != global.cfg.Role {
				reason = fmt.Sprintf("当前阶段=%s, 非本分片轮次", global.phase)
			}
			log.Warnf("[Scheduler] ✗ 拒绝出块 | 高度=%d, 角色=%s, 原因: %s",
				height, global.cfg.Role, reason)
		}
	}

	return canProduce
}

// RecordProduceAllowed records the point where the local proposer is really
// allowed to start packaging a normal scheduled block.
func RecordProduceAllowed(height uint64, source string) {
	if global == nil || !global.cfg.Enable {
		return
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if height <= global.cfg.StartHeight {
		return
	}
	if !global.canProduce || global.phase != global.cfg.Role {
		return
	}
	global.logMetricLocked("allow_produce",
		"height", height,
		"source", source,
		"special", false,
	)
}

// RecordSpecialProduceAllowed records the point where a non-current-round
// state-sync block is allowed to start packaging. It uses the same metric event
// as normal scheduled blocks so performance scripts have one timing source.
func RecordSpecialProduceAllowed(height uint64, source string) {
	if global == nil || !global.cfg.Enable {
		return
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if height <= global.cfg.StartHeight {
		return
	}
	if !global.active {
		return
	}
	if global.canProduce && global.phase == global.cfg.Role {
		return
	}
	if global.cfg.Role != roleBridge && global.cfg.Role != roleBusiness {
		return
	}
	global.logMetricLocked("allow_produce",
		"height", height,
		"source", source,
		"special", true,
	)
}

// IsAfterStartHeight reports whether the given block height is in the
// time-sharing scheduling range. It does not mutate scheduler state.
func IsAfterStartHeight(height uint64) bool {
	if global == nil || !global.cfg.Enable {
		return false
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	return height > global.cfg.StartHeight
}

// BusinessCheckpointPosition returns the time-sharing position assigned to a
// committed normal business block. Global-state synchronization calls this
// immediately after OnBlockCommitted, so a delta can be grouped with the same
// ordinal block from every configured business shard even when physical block
// heights differ between shards.
func BusinessCheckpointPosition(height uint64) (cycle uint64, slot, total int, ok bool) {
	if global == nil || !global.cfg.Enable {
		return 0, 0, 0, false
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.cfg.Role != roleBusiness || global.lastCheckpointHeight != height {
		return 0, 0, 0, false
	}
	return global.lastCheckpointCycle, global.lastCheckpointSlot, global.lastCheckpointTotal, true
}

// getTargetBatch 获取当前角色的目标批次数量（辅助函数）
func (s *Scheduler) getTargetBatch() int {
	if s.cfg.Role == roleBridge {
		return s.cfg.BridgeBatch
	}
	return s.cfg.BusinessBatch
}

// OnBlockCommitted 在区块最终提交后调用，用于推进计数、状态同步屏障和阶段轮换。
func OnBlockCommitted(block *commonpb.Block) {
	if global == nil || !global.cfg.Enable {
		return
	}
	if block == nil || block.Header == nil {
		return
	}
	height := block.Header.BlockHeight
	committedTxCount := len(block.Txs)
	syncOnly := IsSyncOnlyBlock(block.Txs)

	global.mu.Lock()
	defer global.mu.Unlock()

	global.recordBridgeStateSyncChunksLocked(block)

	if height <= global.cfg.StartHeight {
		return
	}

	target := global.cfg.BusinessBatch
	if global.cfg.Role == roleBridge {
		target = global.cfg.BridgeBatch
	}

	// A system-sync-only block is a consistency side channel. It never consumes
	// the configured business/bridge normal-block quota, even if the local shard
	// has become the scheduled producer by the time the block commits.
	if syncOnly {
		global.logMetricLocked("block_commit",
			"height", height,
			"txs", committedTxCount,
			"counter", global.counter,
			"target", target,
			"counted", false,
			"special", true,
		)
		global.tryEnterBridgePhaseLocked()
		return
	}

	// 只在当前阶段且允许时计数。非本轮的状态同步块也记录 commit 埋点，
	// 但不推进分时调度计数。
	if !global.canProduce || global.phase != global.cfg.Role {
		if log != nil {
			log.Debugf("[Scheduler] 跳过计数 | 高度=%d (非本分片出块阶段)", height)
		}
		global.logMetricLocked("block_commit",
			"height", height,
			"txs", committedTxCount,
			"counter", global.counter,
			"target", target,
			"counted", false,
			"special", true,
		)
		return
	}

	global.counter++
	if global.cfg.Role == roleBusiness {
		global.businessSyncHeights = append(global.businessSyncHeights, height)
		global.lastCheckpointHeight = height
		global.lastCheckpointCycle = global.businessCycle
		global.lastCheckpointSlot = global.counter
		global.lastCheckpointTotal = target
	}

	if log != nil {
		log.Infof("[Scheduler] ▶ 区块已提交 | 高度=%d, 角色=%s",
			height, global.cfg.Role)
	}

	global.logMetricLocked("block_commit",
		"height", height,
		"txs", committedTxCount,
		"counter", global.counter,
		"target", target,
		"counted", true,
		"special", false,
	)

	if global.counter >= target {
		// 完成当前阶段
		global.counter = 0
		global.canProduce = false

		oppositeRole := roleBridge
		if global.cfg.Role == roleBridge {
			oppositeRole = roleBusiness
		}

		if log != nil {
			log.Infof("[Scheduler] ======================================")
			if global.cfg.ShardId != "" {
				log.Infof("[Scheduler] ★ %s 分片完成出块阶段 | shard=%s", global.cfg.Role, global.cfg.ShardId)
			} else {
				log.Infof("[Scheduler] ★ %s 分片完成出块阶段", global.cfg.Role)
			}
			log.Infof("[Scheduler] - 完成高度: %d", height)
			log.Infof("[Scheduler] - 下一阶段: %s 分片", oppositeRole)
			log.Infof("[Scheduler] ======================================")
		}

		// 向对端广播完成信号
		var msg string
		if global.cfg.Role == roleBusiness {
			// 业务分片：携带本地 txpool pending 数量，供桥接分片动态计算 N/M
			pending := global.pendingAfterCommitted(committedTxCount)
			if global.cfg.ShardId != "" {
				msg = fmt.Sprintf("%s:%s:%d:pending=%d:sync_heights=%s", msgBusinessDone,
					global.cfg.ShardId, height, pending, formatSyncHeights(global.businessSyncHeights))
			} else {
				msg = fmt.Sprintf("%s:%d:pending=%d", msgBusinessDone, height, pending)
			}
		} else {
			// 桥接分片：计算下一轮的 N/M，附在信号里下发给业务分片
			nNext, mNext := global.computeNextPlan(committedTxCount)
			if nNext > 0 && mNext > 0 {
				// 存入 pendingPlan，下一次收到 SCHED_BUSINESS_DONE 时桥接侧自己应用
				global.pendingPlan = &schedulePlan{businessBatch: nNext, bridgeBatch: mNext}
				msg = fmt.Sprintf("%s:%d:N=%d:M=%d", msgBridgeDone, height, nNext, mNext)
			} else {
				msg = fmt.Sprintf("%s:%d", msgBridgeDone, height)
			}
			// 重置本轮积累的业务分片 pending 数据
			global.bizPendingSum = 0
			global.bizPendingCnt = 0
		}
		global.logMetricLocked("phase_end",
			"height", height,
			"txs", committedTxCount,
			"completed_phase", global.cfg.Role,
			"next_phase", oppositeRole,
			"target", target,
			"message", msg,
		)

		global.broadcastDoneLocked(msg)
		if global.cfg.Role == roleBusiness {
			global.businessSyncHeights = global.businessSyncHeights[:0]
		}
	}
}

// onMessage 处理来自 shardingsync 的分时信号。
func (s *Scheduler) onMessage(msg string) {
	if !s.cfg.Enable {
		return
	}

	if log != nil {
		log.Debugf("[Scheduler] 收到分片间消息: %s", msg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case hasPrefix(msg, msgBusinessDone):
		if s.cfg.Role == roleBridge && s.phase == roleBridge && s.canProduce {
			if log != nil {
				log.Debugf("[Scheduler] 已处于桥接阶段，忽略业务完成信号 | msg=%s", msg)
			}
			return
		}

		shardId, height, pending, syncHeights, ok := parseBusinessDone(msg)
		if !ok {
			if log != nil {
				log.Warnf("[Scheduler] 无法解析业务完成信号高度: %s", msg)
			}
			return
		}

		if len(s.expectedBusinessShards) == 0 {
			if height <= s.lastBusinessDoneHeight["*"] {
				if log != nil {
					log.Debugf("[Scheduler] 忽略过期/重复业务完成信号 | height=%d", height)
				}
				return
			}
			s.lastBusinessDoneHeight["*"] = height
			s.recordBusinessPending(pending)
			if s.cfg.Role == roleBridge {
				if log != nil {
					if shardId != "" {
						log.Infof("[Scheduler] ◆ 收到业务分片完成信号 | shard=%s | height=%d | pending=%d", shardId, height, pending)
					} else {
						log.Infof("[Scheduler] ◆ 收到业务分片完成信号 | height=%d | pending=%d", height, pending)
					}
					log.Infof("[Scheduler] - 切换到桥接阶段")
				}
				s.phase = roleBridge
				s.canProduce = true
				s.counter = 0
				s.applyPendingPlan()
				s.logMetricLocked("phase_ready",
					"ready_phase", roleBridge,
					"source", msgBusinessDone,
					"source_shard", shardId,
					"source_height", height,
					"pending", pending,
				)
				notifyReadyListeners()
			}
			return
		}

		if shardId == "" {
			if log != nil {
				log.Warnf("[Scheduler] 业务完成信号缺少分片ID，已忽略: %s", msg)
			}
			return
		}
		if _, exists := s.expectedBusinessShards[shardId]; !exists {
			if log != nil {
				log.Warnf("[Scheduler] 收到未知业务分片完成信号 | shard=%s | msg=%s", shardId, msg)
			}
			return
		}
		previousDoneHeight := s.lastBusinessDoneHeight[shardId]
		if height < previousDoneHeight {
			if log != nil {
				log.Debugf("[Scheduler] 忽略过期/重复业务完成信号 | height=%d", height)
			}
			return
		}
		if len(syncHeights) > 0 && !s.validSyncHeightsLocked(height, syncHeights) {
			if log != nil {
				log.Warnf("[Scheduler] 业务完成信号检查点列表无效 | shard=%s height=%d sync_heights=%v",
					shardId, height, syncHeights)
			}
			return
		}
		if height == previousDoneHeight {
			if len(syncHeights) == 0 || sameHeightSet(s.requiredSyncHeights[shardId], syncHeights) {
				if log != nil {
					log.Debugf("[Scheduler] 忽略重复业务完成信号 | shard=%s height=%d", shardId, height)
				}
				return
			}
			if len(s.requiredSyncHeights[shardId]) > 0 {
				if log != nil {
					log.Warnf("[Scheduler] 相同业务高度携带不同检查点列表，已拒绝 | shard=%s height=%d",
						shardId, height)
				}
				return
			}
		}
		s.lastBusinessDoneHeight[shardId] = height
		s.requiredSyncHeight[shardId] = height
		if len(syncHeights) > 0 {
			s.requiredSyncHeights[shardId] = heightSet(syncHeights)
		} else {
			delete(s.requiredSyncHeights, shardId)
		}
		if height > previousDoneHeight {
			s.recordBusinessPending(pending)
		}

		if s.cfg.Role == roleBridge {
			s.businessDone[shardId] = true
			doneCount := s.countBusinessDone()
			expectedCount := len(s.expectedBusinessShards)
			if log != nil {
				log.Infof("[Scheduler] ◆ 收到业务分片完成信号 | shard=%s | height=%d | pending=%d | checkpoints=%d | 已收=%d/%d",
					shardId, height, pending, len(syncHeights), doneCount, expectedCount)
			}
			if doneCount == expectedCount && log != nil {
				log.Infof("[Scheduler] ◆ 已收到全部业务分片完成信号，等待状态同步交易在 bridge 提交")
			}
			if doneCount == expectedCount {
				// Keep draining even if the final state batch is below the
				// normal threshold or arrives after the completion signals.
				syncproposer.SetForceFlush(true)
			}
			s.tryEnterBridgePhaseLocked()
		} else {
			if log != nil {
				log.Debugf("[Scheduler] 忽略业务完成信号 (本分片也是业务分片)")
			}
		}

	case hasPrefix(msg, msgBridgeDone):
		height, nNext, mNext, ok := parseBridgeDone(msg)
		if !ok {
			if log != nil {
				log.Warnf("[Scheduler] 无法解析桥接完成信号高度: %s", msg)
			}
			return
		}
		if height <= s.lastBridgeDoneHeight {
			if log != nil {
				log.Debugf("[Scheduler] 忽略过期/重复桥接完成信号 | height=%d", height)
			}
			return
		}
		s.lastBridgeDoneHeight = height
		// 对端是桥接分片，意味着轮到业务
		if s.cfg.Role == roleBusiness {
			if s.phase == roleBusiness && s.canProduce {
				if log != nil {
					log.Debugf("[Scheduler] 已处于业务阶段，忽略桥接完成信号 | height=%d", height)
				}
				return
			}
			if log != nil {
				log.Infof("[Scheduler] ======================================")
				log.Infof("[Scheduler] ◆ 收到桥接分片完成信号")
				log.Infof("[Scheduler] - 切换到业务阶段")
				log.Infof("[Scheduler] ======================================")
			}
			s.phase = roleBusiness
			s.canProduce = true
			s.counter = 0
			s.businessCycle++
			s.businessSyncHeights = s.businessSyncHeights[:0]
			// 应用桥接分片下发的新一轮 N/M（动态调度）
			if nNext > 0 && mNext > 0 {
				s.cfg.BusinessBatch = nNext
				s.cfg.BridgeBatch = mNext
				if log != nil {
					log.Infof("[Scheduler] ★ 动态计划已应用 (业务侧) | N=%d M=%d", nNext, mNext)
				}
			}
			s.logMetricLocked("phase_ready",
				"ready_phase", roleBusiness,
				"source", msgBridgeDone,
				"source_height", height,
				"N", s.cfg.BusinessBatch,
				"M", s.cfg.BridgeBatch,
			)
			notifyReadyListeners()
		} else {
			if log != nil {
				log.Debugf("[Scheduler] 忽略桥接完成信号 (本分片也是桥接分片)")
			}
		}

	default:
		// 其他消息忽略
		if log != nil {
			log.Debugf("[Scheduler] 忽略未知消息: %s", msg)
		}
	}
}

// hasPrefix 是简化版的前缀判断，避免额外依赖。
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func (s *Scheduler) logMetricLocked(event string, keyValues ...interface{}) {
	if log == nil || s == nil {
		return
	}
	var b strings.Builder
	b.WriteString(schedMetricPrefix)
	b.WriteString(" event=")
	b.WriteString(metricValue(event))
	b.WriteString(" role=")
	b.WriteString(metricValue(s.cfg.Role))
	b.WriteString(" shard=")
	b.WriteString(metricValue(s.cfg.ShardId))
	b.WriteString(" phase=")
	b.WriteString(metricValue(s.phase))
	b.WriteString(" can_produce=")
	b.WriteString(strconv.FormatBool(s.canProduce))
	b.WriteString(" counter=")
	b.WriteString(strconv.Itoa(s.counter))
	b.WriteString(" business_batch=")
	b.WriteString(strconv.Itoa(s.cfg.BusinessBatch))
	b.WriteString(" bridge_batch=")
	b.WriteString(strconv.Itoa(s.cfg.BridgeBatch))
	for i := 0; i+1 < len(keyValues); i += 2 {
		key, ok := keyValues[i].(string)
		if !ok || key == "" {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(metricValue(key))
		b.WriteByte('=')
		b.WriteString(metricValue(fmt.Sprint(keyValues[i+1])))
	}
	log.Infof(b.String())
}

func metricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	replacer := strings.NewReplacer(" ", "_", "\t", "_", "\n", "_", "\r", "_")
	return replacer.Replace(value)
}

func parseHeight(msg string, prefix string) (uint64, bool) {
	if !hasPrefix(msg, prefix) {
		return 0, false
	}
	if len(msg) <= len(prefix) || msg[len(prefix)] != ':' {
		return 0, false
	}
	value := msg[len(prefix)+1:]
	if value == "" {
		return 0, false
	}
	height, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return height, true
}

// parseBusinessDone 解析 SCHED_BUSINESS_DONE 消息。
// 兼容旧格式：SCHED_BUSINESS_DONE:shardId:height
// 新格式：    SCHED_BUSINESS_DONE:shardId:height:pending=320:sync_heights=11,12,15
func parseBusinessDone(msg string) (shardId string, height uint64, pending int,
	syncHeights []uint64, ok bool) {
	if !hasPrefix(msg, msgBusinessDone) {
		return "", 0, 0, nil, false
	}
	if len(msg) <= len(msgBusinessDone) || msg[len(msgBusinessDone)] != ':' {
		return "", 0, 0, nil, false
	}
	payload := msg[len(msgBusinessDone)+1:]
	if payload == "" {
		return "", 0, 0, nil, false
	}
	parts := strings.Split(payload, ":")
	var heightStr string
	if len(parts) == 1 {
		heightStr = parts[0]
	} else if len(parts) >= 2 {
		if strings.HasPrefix(parts[1], "pending=") {
			heightStr = parts[0]
		} else {
			shardId = parts[0]
			heightStr = parts[1]
		}
		// 解析可选的 pending=X 字段
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "pending=") {
				val, err := strconv.Atoi(strings.TrimPrefix(p, "pending="))
				if err == nil {
					pending = val
				}
			} else if strings.HasPrefix(p, "sync_heights=") {
				var err error
				syncHeights, err = parseSyncHeights(strings.TrimPrefix(p, "sync_heights="))
				if err != nil {
					return "", 0, 0, nil, false
				}
			}
		}
	}
	h, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return "", 0, 0, nil, false
	}
	if len(syncHeights) > 0 && syncHeights[len(syncHeights)-1] != h {
		return "", 0, 0, nil, false
	}
	return shardId, h, pending, syncHeights, true
}

func formatSyncHeights(heights []uint64) string {
	var b strings.Builder
	for i, height := range heights {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatUint(height, 10))
	}
	return b.String()
}

func parseSyncHeights(value string) ([]uint64, error) {
	if value == "" {
		return nil, fmt.Errorf("sync heights are empty")
	}
	parts := strings.Split(value, ",")
	heights := make([]uint64, 0, len(parts))
	var previous uint64
	for _, part := range parts {
		height, err := strconv.ParseUint(part, 10, 64)
		if err != nil || height == 0 {
			return nil, fmt.Errorf("invalid sync height %q", part)
		}
		if len(heights) > 0 && height <= previous {
			return nil, fmt.Errorf("sync heights are not strictly increasing")
		}
		heights = append(heights, height)
		previous = height
	}
	return heights, nil
}

func heightSet(heights []uint64) map[uint64]struct{} {
	set := make(map[uint64]struct{}, len(heights))
	for _, height := range heights {
		set[height] = struct{}{}
	}
	return set
}

func sameHeightSet(existing map[uint64]struct{}, heights []uint64) bool {
	if len(existing) != len(heights) {
		return false
	}
	for _, height := range heights {
		if _, ok := existing[height]; !ok {
			return false
		}
	}
	return true
}

func (s *Scheduler) validSyncHeightsLocked(completionHeight uint64, heights []uint64) bool {
	if len(heights) == 0 || heights[len(heights)-1] != completionHeight {
		return false
	}
	expected := s.cfg.BusinessBatch
	if s.cfg.Role == roleBridge && s.pendingPlan != nil && s.pendingPlan.businessBatch > 0 {
		expected = s.pendingPlan.businessBatch
	}
	if expected > 0 && len(heights) != expected {
		return false
	}
	for _, height := range heights {
		if height <= s.cfg.StartHeight || height > completionHeight {
			return false
		}
	}
	return true
}

// parseBridgeDone 解析 SCHED_BRIDGE_DONE 消息。
// 兼容旧格式：SCHED_BRIDGE_DONE:height
// 新格式：    SCHED_BRIDGE_DONE:height:N=4:M=2
func parseBridgeDone(msg string) (height uint64, nNext int, mNext int, ok bool) {
	if !hasPrefix(msg, msgBridgeDone) {
		return 0, 0, 0, false
	}
	if len(msg) <= len(msgBridgeDone) || msg[len(msgBridgeDone)] != ':' {
		return 0, 0, 0, false
	}
	payload := msg[len(msgBridgeDone)+1:]
	parts := strings.Split(payload, ":")
	if len(parts) == 0 || parts[0] == "" {
		return 0, 0, 0, false
	}
	h, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	height = h
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "N=") {
			val, err := strconv.Atoi(strings.TrimPrefix(p, "N="))
			if err == nil {
				nNext = val
			}
		} else if strings.HasPrefix(p, "M=") {
			val, err := strconv.Atoi(strings.TrimPrefix(p, "M="))
			if err == nil {
				mNext = val
			}
		}
	}
	return height, nNext, mNext, true
}

func (s *Scheduler) pendingAfterCommitted(committedTxCount int) int {
	if s.cfg.GetPendingFn == nil {
		return 0
	}
	pending := s.cfg.GetPendingFn()
	if committedTxCount > 0 {
		pending -= committedTxCount
	}
	if pending < 0 {
		return 0
	}
	return pending
}

// computeNextPlan 由桥接分片在完成 M 块后调用，使用比例法计算下一轮 N/M。
// 公式：M_next = round(T × Q_bridge / (Q_bridge + avg(Q_biz)))；N_next = T - M_next
// 当 TotalBatch == 0 时，动态调度被禁用，返回 (0, 0)。
func (s *Scheduler) computeNextPlan(committedTxCount int) (nNext int, mNext int) {
	T := s.cfg.TotalBatch
	if T <= 1 {
		return 0, 0
	}

	qBridge := s.pendingAfterCommitted(committedTxCount)

	avgQBiz := 0
	if s.bizPendingCnt > 0 {
		avgQBiz = s.bizPendingSum / s.bizPendingCnt
	}

	if log != nil {
		log.Infof("[Scheduler] 动态计划计算 | T=%d qBridge=%d avgQBiz=%d (bizCnt=%d)",
			T, qBridge, avgQBiz, s.bizPendingCnt)
	}

	total := qBridge + avgQBiz
	if total == 0 {
		// 两侧队列均空，保持当前比例（或给业务侧更多块）
		mNext = 1
		nNext = T - mNext
		if log != nil {
			log.Infof("[Scheduler] 动态计划（两侧空队列，默认） | N=%d M=%d", nNext, mNext)
		}
		return nNext, mNext
	}

	mRaw := int(math.Round(float64(T) * float64(qBridge) / float64(total)))
	if mRaw < 1 {
		mRaw = 1
	}
	if mRaw > T-1 {
		mRaw = T - 1
	}
	mNext = mRaw
	nNext = T - mNext

	if log != nil {
		log.Infof("[Scheduler] 动态计划结果 | N=%d M=%d", nNext, mNext)
	}
	return nNext, mNext
}

func (s *Scheduler) recordBusinessPending(pending int) {
	if s.cfg.Role != roleBridge || s.cfg.TotalBatch <= 0 || pending < 0 {
		return
	}
	s.bizPendingSum += pending
	s.bizPendingCnt++
}

// applyPendingPlan 将 pendingPlan 应用到调度配置（桥接分片侧使用）。
func (s *Scheduler) applyPendingPlan() {
	if s.pendingPlan == nil {
		return
	}
	s.cfg.BusinessBatch = s.pendingPlan.businessBatch
	s.cfg.BridgeBatch = s.pendingPlan.bridgeBatch
	if log != nil {
		log.Infof("[Scheduler] ★ 动态计划已应用 (桥接侧) | N=%d M=%d",
			s.cfg.BusinessBatch, s.cfg.BridgeBatch)
	}
	s.pendingPlan = nil
}

func (s *Scheduler) resetBusinessDone() {
	s.businessDone = make(map[string]bool, len(s.expectedBusinessShards))
}

func (s *Scheduler) recordBridgeStateSyncChunksLocked(block *commonpb.Block) {
	if s.cfg.Role != roleBridge || block == nil {
		return
	}
	for _, tx := range block.Txs {
		if tx == nil || tx.Payload == nil || !crossshard.IsBridgeStateSyncTx(tx) {
			continue
		}
		if tx.Result != nil && tx.Result.Code != commonpb.TxStatusCode_SUCCESS {
			continue
		}
		info, err := crossshard.ParseBridgeStateSyncInfo(tx)
		if err != nil || !info.Checkpoint {
			continue
		}
		key := fmt.Sprintf("%s:%d", info.SourceShard, info.EndHeight)
		progress := s.stateSyncProgress[key]
		if progress == nil || progress.height != info.EndHeight || progress.total != info.ChunkTotal {
			progress = &stateSyncProgress{
				height: info.EndHeight,
				total:  info.ChunkTotal,
				chunks: make(map[int]struct{}, info.ChunkTotal),
			}
			s.stateSyncProgress[key] = progress
		}
		progress.chunks[info.ChunkIndex] = struct{}{}
		s.logMetricLocked("state_sync_chunk_commit",
			"source_shard", info.SourceShard,
			"source_start_height", info.StartHeight,
			"source_height", info.EndHeight,
			"sync_cycle", info.Cycle,
			"sync_slot", info.Slot,
			"sync_slot_total", info.SlotTotal,
			"chunk", info.ChunkIndex+1,
			"chunks", info.ChunkTotal,
			"committed_chunks", len(progress.chunks),
		)
		if len(progress.chunks) < progress.total {
			continue
		}
		s.markStateCheckpointCompleteLocked(info.SourceShard, info.EndHeight)
		delete(s.stateSyncProgress, key)
		s.logMetricLocked("state_sync_checkpoint_commit",
			"source_shard", info.SourceShard,
			"source_height", info.EndHeight,
			"sync_cycle", info.Cycle,
			"sync_slot", info.Slot,
			"sync_slot_total", info.SlotTotal,
			"chunks", info.ChunkTotal,
		)
	}
}

func (s *Scheduler) markStateCheckpointCompleteLocked(sourceShard string, height uint64) {
	if height <= s.appliedSyncHeight[sourceShard] {
		return
	}
	completed := s.completedSyncHeights[sourceShard]
	if completed == nil {
		completed = make(map[uint64]struct{})
		s.completedSyncHeights[sourceShard] = completed
	}
	completed[height] = struct{}{}
	for next := s.appliedSyncHeight[sourceShard] + 1; ; next++ {
		if _, ok := completed[next]; !ok {
			break
		}
		delete(completed, next)
		s.appliedSyncHeight[sourceShard] = next
	}
}

// completeRequiredSyncHeightsLocked advances the covered physical-height
// frontier after every explicitly required normal checkpoint has committed.
// Heights skipped by the list are bridge-originated writeback-only blocks and
// intentionally do not have a business-to-bridge checkpoint.
func (s *Scheduler) completeRequiredSyncHeightsLocked() {
	for shardId, exact := range s.requiredSyncHeights {
		if len(exact) == 0 {
			continue
		}
		frontier := s.requiredSyncHeight[shardId]
		if frontier > s.appliedSyncHeight[shardId] {
			s.appliedSyncHeight[shardId] = frontier
		}
		for height := range s.completedSyncHeights[shardId] {
			if height <= frontier {
				delete(s.completedSyncHeights[shardId], height)
			}
		}
		delete(s.requiredSyncHeights, shardId)
	}
}

func (s *Scheduler) broadcastDoneLocked(msg string) {
	if s.broadcastFn != nil {
		if log != nil {
			log.Infof("[Scheduler] 正在广播完成信号: %s", msg)
		}
		if err := s.broadcastFn(msg); err != nil {
			log.Errorf("[Scheduler] ✗ 广播完成信号失败: %v", err)
		} else if log != nil {
			log.Infof("[Scheduler] ✓ 完成信号已广播")
		}
	}
	if s.cfg.Role == roleBridge {
		s.resetBusinessDone()
	}
}

func (s *Scheduler) tryEnterBridgePhaseLocked() bool {
	if s.cfg.Role != roleBridge || len(s.expectedBusinessShards) == 0 {
		return false
	}
	if s.phase == roleBridge && s.canProduce {
		return true
	}
	if s.countBusinessDone() < len(s.expectedBusinessShards) {
		return false
	}
	for shardId := range s.expectedBusinessShards {
		required := s.requiredSyncHeight[shardId]
		applied := s.appliedSyncHeight[shardId]
		if exact := s.requiredSyncHeights[shardId]; len(exact) > 0 {
			committed := 0
			var missing uint64
			for height := range exact {
				if height <= applied {
					committed++
					continue
				}
				if _, ok := s.completedSyncHeights[shardId][height]; ok {
					committed++
					continue
				}
				if missing == 0 || height < missing {
					missing = height
				}
			}
			if committed < len(exact) {
				s.logMetricLocked("phase_sync_wait",
					"source_shard", shardId,
					"required_height", required,
					"applied_height", applied,
					"required_checkpoints", len(exact),
					"committed_checkpoints", committed,
					"missing_checkpoint", missing,
				)
				return false
			}
			continue
		}
		if required == 0 || applied < required {
			s.logMetricLocked("phase_sync_wait",
				"source_shard", shardId,
				"required_height", required,
				"applied_height", applied,
			)
			return false
		}
	}
	s.completeRequiredSyncHeightsLocked()

	s.phase = roleBridge
	s.canProduce = true
	s.counter = 0
	syncproposer.SetForceFlush(false)
	s.applyPendingPlan()
	s.logMetricLocked("phase_ready",
		"ready_phase", roleBridge,
		"source", "state_sync_barrier",
		"done_count", s.countBusinessDone(),
		"expected_count", len(s.expectedBusinessShards),
	)
	if log != nil {
		log.Infof("[Scheduler] ◆ 业务完成信号与链上状态同步均已齐备，切换到桥接阶段")
	}
	notifyReadyListeners()
	return true
}

func (s *Scheduler) countBusinessDone() int {
	count := 0
	for _, done := range s.businessDone {
		if done {
			count++
		}
	}
	return count
}

// GetStatus 获取当前调度器状态（用于调试和监控）
func GetStatus() string {
	if global == nil {
		return "[Scheduler] 未初始化"
	}

	global.mu.Lock()
	defer global.mu.Unlock()

	status := fmt.Sprintf(`[Scheduler] 当前状态:
  - 启用状态: %v
  - 分片角色: %s
  - 分片ID: %s
  - 分时模式: %v
  - 当前阶段: %s
  - 出块权限: %v
  - 业务分片列表: %v`,
		global.cfg.Enable,
		global.cfg.Role,
		global.cfg.ShardId,
		global.active,
		global.phase,
		global.canProduce,
		global.cfg.BusinessShards,
	)
	return status
}

// PrintStatus 打印当前调度器状态到日志
func PrintStatus() {
	if log != nil {
		log.Infof(GetStatus())
	}
}

// IsEligible 返回当前分片是否处于"可出块"状态。
// 与 ShouldProduce 的区别：ShouldProduce 在分时模式启动前（高度<=startHeight）也返回 true；
// IsEligible 仅在分时模式已激活且当前阶段轮到本分片时返回 true。
// 心跳模块通过此函数判断是否需要注入心跳交易。
func IsEligible() bool {
	if global == nil || !global.cfg.Enable {
		return false
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	return global.active && global.canProduce && global.phase == global.cfg.Role
}

// HandleMessage 用于处理分片内广播的调度消息。
func HandleMessage(msg string) {
	if global == nil {
		return
	}
	if log != nil {
		log.Infof("[Scheduler] 收到分片内调度消息: %s", msg)
	}
	global.onMessage(msg)
}

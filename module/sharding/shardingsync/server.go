package shardingsync

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/viper"
)

const (
	// SyncProtocolID 同步协议ID
	SyncProtocolID = protocol.ID("/chainmaker/sharding-sync/1.0.0")
	// TaskInterval 定时任务间隔
	TaskInterval = 60 * time.Second
)

var (
	log     *logger.CMLogger
	syncSvr *SyncServer
	once    sync.Once
	msgMu   sync.RWMutex
	// messageHandlers 用于外部模块（例如分时调度）注册回调
	messageHandlers []func(string)
	peerMetaMu      sync.RWMutex
	peerShard       = make(map[peer.ID]string)
	peerType        = make(map[peer.ID]string)
)

// SyncServer 分片同步服务
type SyncServer struct {
	config   ShardingConfig
	host     host.Host
	ctx      context.Context
	cancel   context.CancelFunc
	outboxMu sync.Mutex
	outbox   map[[32]byte]bool
}

// LoadConfig 读取 sharding.yml 配置，供外部（如调度模块或 CLI）复用。
func LoadConfig(chainConf *localconf.CMConfig) (*ShardingConfig, error) {
	v := viper.New()
	v.AddConfigPath("./") // 当前工作目录，通常是 bin/
	v.AddConfigPath("../config")

	// 动态添加 org_id 目录: ../config/{org_id}
	if chainConf != nil && chainConf.NodeConfig.OrgId != "" {
		v.AddConfigPath(fmt.Sprintf("../config/%s", chainConf.NodeConfig.OrgId))
	}

	v.SetConfigName("sharding")
	v.SetConfigType("yml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg ShardingConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Start 使用节点配置启动独立的分片同步网络。
func Start(chainConf *localconf.CMConfig) {
	once.Do(func() {
		// 1. 初始化日志
		log = logger.GetLogger(logger.MODULE_NET)

		// 2. 加载 sharding.yml 配置
		cfg, err := LoadConfig(chainConf)
		if err != nil {
			log.Warnf("[ShardingSync] 未找到 sharding.yml 或读取失败，跳过同步服务启动: %v", err)
			return
		}

		// 3. 检查开关
		if !cfg.SyncNetwork.Enable {
			log.Infof("[ShardingSync] 当前节点未启用同步服务 (Enable=false)，跳过")
			return
		}

		log.Infof("[ShardingSync] 正在启动同步服务... [ShardId: %s, Port: %d]", cfg.ShardId, cfg.SyncNetwork.Port)

		// 4. 创建服务实例
		ctx, cancel := context.WithCancel(context.Background())
		syncSvr = &SyncServer{
			config: *cfg,
			ctx:    ctx,
			cancel: cancel,
		}

		// 5. 启动 P2P Host
		if err := syncSvr.startP2P(chainConf); err != nil {
			log.Errorf("[ShardingSync] P2P Host 启动失败: %v", err)
			return
		}

		// 6. 启动连接守护协程 (负责连接种子节点和保持连接)
		go syncSvr.keepAliveSeeds()
	})
}

// RegisterMessageHandler 允许外部模块注册消息回调（如分时调度）。
func RegisterMessageHandler(fn func(string)) {
	msgMu.Lock()
	defer msgMu.Unlock()
	messageHandlers = append(messageHandlers, fn)
}

// BroadcastMessage 将消息发送给所有已连接的节点。
func BroadcastMessage(msg string) error {
	return broadcastMessageToPeers("", msg)
}

// BroadcastMessageToShard sends to peers whose shard id has been learned from
// the handshake. If no matching peer is known yet, it falls back to broadcast
// so delivery is preserved during startup.
func BroadcastMessageToShard(targetShard string, msg string) error {
	return broadcastMessageToPeers(targetShard, msg)
}

// BroadcastStateMessageToShard retains the exact signed message while retrying
// transport failures. It never re-signs a checkpoint during a retry.
func BroadcastStateMessageToShard(targetShard, msg string) error {
	if syncSvr == nil || syncSvr.host == nil {
		return fmt.Errorf("sharding sync server not started")
	}
	s := syncSvr
	id := sha256.Sum256([]byte(targetShard + "\x00" + msg))
	s.outboxMu.Lock()
	if s.outbox == nil {
		s.outbox = make(map[[32]byte]bool)
	}
	if s.outbox[id] {
		s.outboxMu.Unlock()
		return nil
	}
	s.outbox[id] = true
	s.outboxMu.Unlock()
	go func() {
		defer func() {
			s.outboxMu.Lock()
			delete(s.outbox, id)
			s.outboxMu.Unlock()
		}()
		for {
			if err := sendMessageToPeers(selectShardPeers(targetShard, false), []byte(msg), s.SendMessage); err == nil {
				return
			} else {
				log.Warnf("[ShardingSync] state message retained for retry | target=%s id=%x err=%v", targetShard, id, err)
			}
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return nil
}

func broadcastMessageToPeers(targetShard string, msg string) error {
	if syncSvr == nil || syncSvr.host == nil {
		return fmt.Errorf("sharding sync server not started")
	}

	return sendMessageToPeers(selectPeers(targetShard), []byte(msg), syncSvr.SendMessage)
}

func sendMessageToPeers(peers []peer.ID, data []byte, send func(peer.ID, []byte) error) error {
	if len(peers) == 0 {
		return fmt.Errorf("no connected destination peers")
	}
	var firstErr error
	for _, p := range peers {
		if err := send(p, data); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func selectPeers(targetShard string) []peer.ID {
	return selectShardPeers(targetShard, true)
}

func selectShardPeers(targetShard string, fallback bool) []peer.ID {
	peers := syncSvr.host.Network().Peers()
	targetShard = normalizeShardName(targetShard)
	if targetShard == "" {
		return peers
	}
	matches := make([]peer.ID, 0, len(peers))
	peerMetaMu.RLock()
	for _, p := range peers {
		if normalizeShardName(peerShard[p]) == targetShard {
			matches = append(matches, p)
		}
	}
	peerMetaMu.RUnlock()
	if len(matches) == 0 && fallback {
		return peers
	}
	return matches
}

// startP2P 启动 libp2p host
func (s *SyncServer) startP2P(chainConf *localconf.CMConfig) error {
	// 复用 ChainMaker 节点的 TLS 私钥
	keyPath := chainConf.NetConfig.TLSConfig.PrivKeyFile
	log.Infof("[ShardingSync] 正在加载节点私钥: %s", keyPath)

	privKey, err := loadPrivateKey(keyPath)
	if err != nil {
		// 这里必须报错返回，不能随机生成，否则无法保证 PeerID 的一致性
		return fmt.Errorf("加载节点私钥失败: %v", err)
	}

	// 监听地址
	listenAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", s.config.SyncNetwork.Port))

	// 创建 Host
	h, err := libp2p.New(
		s.ctx,
		libp2p.ListenAddrs(listenAddr),
		libp2p.Identity(privKey),
	)
	if err != nil {
		return err
	}

	s.host = h
	log.Infof("[ShardingSync] P2P Host Started. ID: %s, Addrs: %v", h.ID(), h.Addrs())

	// 设置流处理器
	h.SetStreamHandler(SyncProtocolID, s.handleStream)

	return nil
}

// keepAliveSeeds 持续尝试连接种子节点
func (s *SyncServer) keepAliveSeeds() {
	// 立即执行一次
	s.connectSeeds()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.connectSeeds()
		}
	}
}

// connectSeeds 连接种子节点
func (s *SyncServer) connectSeeds() {
	for _, seedAddr := range s.config.SyncNetwork.Seeds {
		if seedAddr == "" {
			continue
		}

		// 解析多地址
		maddr, err := ma.NewMultiaddr(seedAddr)
		if err != nil {
			log.Errorf("[ShardingSync] 无效的种子地址 %s: %v", seedAddr, err)
			continue
		}

		// 提取 PeerInfo
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			// 忽略不含有效 PeerID 的地址，libp2p 连接需要目标 PeerID。
			continue
		}

		// 检查是否已连接
		if s.host.Network().Connectedness(info.ID) == network.Connected {
			continue
		}

		log.Infof("[ShardingSync] 正在尝试连接种子节点: %s", seedAddr)

		// 异步连接，避免阻塞循环
		go func(pi peer.AddrInfo) {
			ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			defer cancel()

			if err := s.host.Connect(ctx, pi); err != nil {
				log.Warnf("[ShardingSync] 连接种子节点失败 [%s]: %v", pi.ID, err)
			} else {
				log.Infof("[ShardingSync] ✓ 成功连接到种子节点: %s", pi.ID)

				// 连接成功后，发送一条握手消息，证明连通性
				msg := fmt.Sprintf("HI. I am %s (%s), nice to meet you!", s.config.ShardId, s.config.ShardType)
				s.SendMessage(pi.ID, []byte(msg))
			}
		}(*info)
	}
}

// startScheduledTask 启动定时任务，定期向连接的节点发送消息
func (s *SyncServer) startScheduledTask() {
	ticker := time.NewTicker(TaskInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 向所有已连接的 Peer 发送状态检查消息
			for _, p := range s.host.Network().Peers() {
				msg := fmt.Sprintf("HI. I am %s. Just saying hello.", s.config.ShardId)
				// 异步发送，避免阻塞
				go s.SendMessage(p, []byte(msg))
			}
		}
	}
}

// SendMessage 发送消息给指定节点
func (s *SyncServer) SendMessage(peerId peer.ID, msg []byte) error {
	text := string(msg)
	logMessage("send", peerId.String(), text)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	stream, err := s.host.NewStream(ctx, peerId, SyncProtocolID)
	if err != nil {
		log.Warnf("[ShardingSync] Failed to open stream (to %s): %v", peerId, err)
		return err
	}
	defer stream.Close()
	if err := stream.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		_ = stream.Reset()
		return err
	}

	n, err := stream.Write(msg)
	if err == nil && n != len(msg) {
		err = io.ErrShortWrite
	}
	if err != nil {
		_ = stream.Reset()
		log.Warnf("[ShardingSync] Failed to send message: %v", err)
		return err
	}

	return nil
}

// handleStream 处理接收到的消息
func (s *SyncServer) handleStream(stream network.Stream) {
	defer stream.Close()

	// 读取完整数据（跨分片消息可能较大）
	payload, err := io.ReadAll(stream)
	if err != nil {
		return
	}

	msg := string(payload)
	remotePeer := stream.Conn().RemotePeer()
	recordPeerAnnouncement(remotePeer, msg)

	logMessage("received", remotePeer.String(), msg)

	if response := peerAnnouncementReply(s.config, msg); response != "" {
		go s.SendMessage(remotePeer, []byte(response))
	}

	// 分发给外部注册的回调
	msgMu.RLock()
	handlers := append([]func(string){}, messageHandlers...)
	msgMu.RUnlock()
	for _, h := range handlers {
		h(msg)
	}
}

func peerAnnouncementReply(cfg ShardingConfig, msg string) string {
	const acknowledgement = ", acknowledgement"
	if _, _, ok := parsePeerAnnouncement(msg); !ok || strings.HasSuffix(msg, acknowledgement) {
		return ""
	}
	return fmt.Sprintf("HI. I am %s (%s)%s", cfg.ShardId, cfg.ShardType, acknowledgement)
}

func recordPeerAnnouncement(remotePeer peer.ID, msg string) {
	shardId, shardType, ok := parsePeerAnnouncement(msg)
	if !ok {
		return
	}
	peerMetaMu.Lock()
	peerShard[remotePeer] = normalizeShardName(shardId)
	peerType[remotePeer] = strings.ToUpper(strings.TrimSpace(shardType))
	peerMetaMu.Unlock()
}

func parsePeerAnnouncement(msg string) (string, string, bool) {
	const prefix = "HI. I am "
	if !strings.HasPrefix(msg, prefix) {
		return "", "", false
	}
	body := strings.TrimPrefix(msg, prefix)
	shardId := body
	shardType := ""
	if idx := strings.Index(body, " ("); idx >= 0 {
		shardId = strings.TrimSpace(body[:idx])
		rest := body[idx+2:]
		if end := strings.Index(rest, ")"); end >= 0 {
			shardType = strings.TrimSpace(rest[:end])
		}
	} else if idx := strings.Index(body, "."); idx >= 0 {
		shardId = strings.TrimSpace(body[:idx])
	}
	return shardId, shardType, shardId != ""
}

func logMessage(direction, peerText, msg string) {
	kind, shard, ok := messageSummary(msg)
	if ok {
		if shard != "" {
			log.Infof("[ShardingSync] %s %s | peer=%s shard=%s len=%d", kind, direction, peerText, shard, len(msg))
		} else {
			log.Infof("[ShardingSync] %s %s | peer=%s len=%d", kind, direction, peerText, len(msg))
		}
		return
	}
	if strings.HasPrefix(msg, "HI.") {
		log.Debugf("[ShardingSync] HI %s | peer=%s len=%d", direction, peerText, len(msg))
		return
	}
	log.Debugf("[ShardingSync] message %s | peer=%s len=%d", direction, peerText, len(msg))
}

func messageSummary(msg string) (string, string, bool) {
	switch {
	case strings.HasPrefix(msg, "GLOBAL_STATE_DELTA_V3:"):
		return "GLOBAL_STATE_DELTA_V3", messageShard(msg, "GLOBAL_STATE_DELTA_V3:"), true
	case strings.HasPrefix(msg, "GLOBAL_STATE_DELTA_V2:"):
		return "GLOBAL_STATE_DELTA_V2", messageShard(msg, "GLOBAL_STATE_DELTA_V2:"), true
	case strings.HasPrefix(msg, "GLOBAL_STATE_DELTA:"):
		return "GLOBAL_STATE_DELTA", messageShard(msg, "GLOBAL_STATE_DELTA:"), true
	case strings.HasPrefix(msg, "CROSS_WRITE_BATCH:"):
		return "CROSS_WRITE_BATCH", messageShard(msg, "CROSS_WRITE_BATCH:"), true
	default:
		return "", "", false
	}
}

func messageShard(msg, prefix string) string {
	payload := strings.TrimPrefix(msg, prefix)
	if parts := strings.SplitN(payload, ":", 2); len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func normalizeShardName(shardId string) string {
	shardId = strings.TrimSpace(shardId)
	if shardId == "" {
		return ""
	}
	if shardId == "bridge" || shardId == "bridge-shard" {
		return "bridge-shard"
	}
	if strings.HasPrefix(shardId, "business-shard") {
		return shardId
	}
	if strings.HasPrefix(shardId, "business-") {
		suffix := strings.TrimPrefix(shardId, "business-")
		if suffix != "" {
			return "business-shard" + suffix
		}
	}
	return shardId
}

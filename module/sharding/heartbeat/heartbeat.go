/*
Copyright (C) BABEC. All rights reserved.
SPDX-License-Identifier: Apache-2.0

心跳交易模块：当业务分片在分时调度阶段交易池持续为空时，
由 Proposer 节点注入一笔心跳交易，以推进出块计数，避免分时调度死锁。
*/

package heartbeat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/scheduler"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	utilsv2 "chainmaker.org/chainmaker/utils/v2"
)

const (
	defaultIdleThreshold = 2 * time.Second
	defaultContractName  = "sharding_heartbeat"
	defaultMethod        = "ping"
)

var (
	once    sync.Once
	manager *Manager
	hbLog   = logger.GetLogger(logger.MODULE_CONSENSUS)
)

// Config 心跳模块配置
type Config struct {
	ShardId       string
	IdleThreshold time.Duration // 交易池持续为空多久后注入心跳，默认 2s
	ContractName  string        // 心跳合约名，默认 "sharding_heartbeat"
	Method        string        // 心跳方法名，默认 "ping"
}

// Manager 心跳管理器
type Manager struct {
	mu sync.Mutex

	shardId       string
	contractName  string
	method        string
	idleThreshold time.Duration

	lastCommittedHeight uint64
	lastCommittedTime   time.Time
	sentForHeight       uint64 // 已为该高度注入过心跳，防止同一高度重复注入
	workloadStarted     bool   // 进入分时后见到真实交易/区块前，不注入心跳

	submitTx func(chainId string, tx *commonpb.Transaction) error
}

// Init 初始化心跳模块（仅执行一次，仅业务分片调用）
func Init(cfg Config) {
	once.Do(func() {
		if cfg.IdleThreshold == 0 {
			cfg.IdleThreshold = defaultIdleThreshold
		}
		if cfg.ContractName == "" {
			cfg.ContractName = defaultContractName
		}
		if cfg.Method == "" {
			cfg.Method = defaultMethod
		}
		manager = &Manager{
			shardId:           cfg.ShardId,
			idleThreshold:     cfg.IdleThreshold,
			contractName:      cfg.ContractName,
			method:            cfg.Method,
			lastCommittedTime: time.Now(),
		}
		hbLog.Infof("[Heartbeat] 心跳模块已初始化 | shard=%s idleThreshold=%s contract=%s method=%s",
			cfg.ShardId, cfg.IdleThreshold, cfg.ContractName, cfg.Method)
	})
}

// SetSubmitter 注入交易提交函数（避免循环依赖）
func SetSubmitter(fn func(chainId string, tx *commonpb.Transaction) error) {
	if manager != nil {
		manager.mu.Lock()
		manager.submitTx = fn
		manager.mu.Unlock()
	}
}

// OnBlockCommitted 区块提交时调用，重置空闲计时器
func OnBlockCommitted(height uint64) {
	if manager == nil {
		return
	}
	afterStartHeight := scheduler.IsAfterStartHeight(height)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if afterStartHeight {
		manager.markWorkloadStartedLocked("block_commit", height)
	}
	manager.lastCommittedHeight = height
	manager.lastCommittedTime = time.Now()
	hbLog.Debugf("[Heartbeat] 更新最后提交高度 | height=%d", height)
}

// MarkWorkloadStarted allows the proposer to enable idle heartbeat injection
// after real workload reaches txpool in the time-sharing range.
func MarkWorkloadStarted(source string, height uint64) {
	if manager == nil || !scheduler.IsAfterStartHeight(height) {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.markWorkloadStartedLocked(source, height)
}

func (m *Manager) markWorkloadStartedLocked(source string, height uint64) {
	if m.workloadStarted {
		return
	}
	m.workloadStarted = true
	hbLog.Infof("[Heartbeat] 压测流量已开始，允许后续空阶段注入心跳 | shard=%s source=%s height=%d",
		m.shardId, source, height)
}

// TryInject 尝试注入心跳交易。
// 仅当：1) 当前轮到本分片出块 2) 空闲时间超过阈值 3) 未为该高度注入过，才注入。
// 返回 true 表示心跳交易已成功提交到 txpool。
// 调用方（block_proposer_impl.go）在本函数返回后应继续 return false，
// 等待 txpool 收到交易后触发下一次提案。
func TryInject(chainId string, height uint64) bool {
	if manager == nil {
		return false
	}
	// 仅在分时调度激活且轮到本分片时注入
	if !scheduler.IsEligible() {
		return false
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	if !manager.workloadStarted {
		hbLog.Debugf("[Heartbeat] 压测流量尚未开始，跳过心跳注入 | height=%d", height)
		return false
	}

	// 已为该高度注入过，不重复注入
	if manager.sentForHeight >= height {
		hbLog.Debugf("[Heartbeat] 该高度已注入过心跳，跳过 | height=%d", height)
		return false
	}

	// 未达到空闲阈值，等待真实交易到来
	idleDuration := time.Since(manager.lastCommittedTime)
	if idleDuration < manager.idleThreshold {
		hbLog.Debugf("[Heartbeat] 空闲时间未达阈值，跳过注入 | idle=%s threshold=%s height=%d",
			idleDuration, manager.idleThreshold, height)
		return false
	}

	if manager.submitTx == nil {
		hbLog.Warnf("[Heartbeat] submitTx 未设置，无法注入心跳")
		return false
	}

	tx, err := manager.buildHeartbeatTx(chainId, height)
	if err != nil {
		hbLog.Warnf("[Heartbeat] 构造心跳交易失败 | height=%d err=%v", height, err)
		return false
	}

	if err := manager.submitTx(chainId, tx); err != nil {
		hbLog.Warnf("[Heartbeat] 提交心跳交易失败 | height=%d txId=%s err=%v",
			height, tx.Payload.TxId, err)
		return false
	}

	manager.sentForHeight = height
	hbLog.Infof("[Heartbeat] 心跳交易已注入 txpool | shard=%s height=%d txId=%s idle=%s",
		manager.shardId, height, tx.Payload.TxId, idleDuration)
	return true
}

// buildHeartbeatTx 构造心跳交易，使用节点自身证书签名。
// txId 为确定性值：SHA256("heartbeat:{shardId}:{height}")，
// 保证同一分片不同节点为同一高度生成相同的 txId，mempool 自动去重。
func (m *Manager) buildHeartbeatTx(chainId string, height uint64) (*commonpb.Transaction, error) {
	if localconf.ChainMakerConfig == nil {
		return nil, fmt.Errorf("chainmaker config not loaded")
	}
	nodeCfg := localconf.ChainMakerConfig.NodeConfig
	if nodeCfg.CertFile == "" || nodeCfg.PrivKeyFile == "" || nodeCfg.OrgId == "" {
		return nil, fmt.Errorf("node cert/key/org_id not configured")
	}

	certPem, err := os.ReadFile(nodeCfg.CertFile)
	if err != nil {
		return nil, fmt.Errorf("read cert file failed: %v", err)
	}
	keyPem, err := os.ReadFile(nodeCfg.PrivKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read key file failed: %v", err)
	}

	txId := buildHeartbeatTxId(m.shardId, height)
	now := time.Now()

	payload := &commonpb.Payload{
		ChainId:        chainId,
		TxType:         commonpb.TxType_INVOKE_CONTRACT,
		TxId:           txId,
		Timestamp:      now.Unix(),
		ExpirationTime: now.Add(5 * time.Minute).Unix(),
		ContractName:   m.contractName,
		Method:         m.method,
		Parameters: []*commonpb.KeyValuePair{
			{Key: "shard_id", Value: []byte(m.shardId)},
			{Key: "height", Value: []byte(fmt.Sprintf("%d", height))},
		},
	}

	tx := &commonpb.Transaction{Payload: payload}
	signer, err := signTx(tx, nodeCfg.OrgId, certPem, keyPem)
	if err != nil {
		return nil, fmt.Errorf("sign heartbeat tx failed: %v", err)
	}
	tx.Sender = signer
	return tx, nil
}

// buildHeartbeatTxId 生成确定性心跳交易 ID。
// 同一分片、同一目标高度，所有节点生成相同的 txId，供 mempool 去重。
func buildHeartbeatTxId(shardId string, height uint64) string {
	raw := fmt.Sprintf("heartbeat:%s:%d", shardId, height)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ---- 签名辅助函数（与 crossshard/receiver.go 保持一致）----

func signTx(tx *commonpb.Transaction, orgId string, certPem, keyPem []byte) (*commonpb.EndorsementEntry, error) {
	sk, err := asym.PrivateKeyFromPEM(keyPem, nil)
	if err != nil {
		return nil, fmt.Errorf("load private key failed: %v", err)
	}
	cert, err := parseCert(certPem)
	if err != nil {
		return nil, err
	}
	txBytes, err := utilsv2.CalcUnsignedTxBytes(tx)
	if err != nil {
		return nil, fmt.Errorf("calc tx bytes failed: %v", err)
	}
	sig, err := signBytes(sk, cert, txBytes)
	if err != nil {
		return nil, err
	}
	return &commonpb.EndorsementEntry{
		Signer: &pbac.Member{
			OrgId:      orgId,
			MemberType: pbac.MemberType_CERT,
			MemberInfo: certPem,
		},
		Signature: sig,
	}, nil
}

func parseCert(certPEM []byte) (*bcx509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("decode pem failed, invalid certificate")
	}
	cert, err := bcx509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509 parse cert failed: %v", err)
	}
	return cert, nil
}

func signBytes(privateKey crypto.PrivateKey, cert *bcx509.Certificate, msg []byte) ([]byte, error) {
	hashAlgo, err := bcx509.GetHashFromSignatureAlgorithm(cert.SignatureAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("invalid signature algorithm: %v", err)
	}
	opts := &crypto.SignOpts{
		Hash: hashAlgo,
		UID:  crypto.CRYPTO_DEFAULT_UID,
	}
	return privateKey.SignWithOpts(msg, opts)
}

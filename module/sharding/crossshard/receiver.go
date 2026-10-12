package crossshard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker-go/module/sharding/syncproposer"
	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	commonerrors "chainmaker.org/chainmaker/common/v2/errors"
	"chainmaker.org/chainmaker/localconf/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
)

const crossApplyTxExpiration = 24 * time.Hour
const defaultApplyBatchMaxWrites = 4000

type applyWriteChunk struct {
	batch        *CrossWriteBatch
	contractName string
	writes       []BridgeStateWrite
	writesRaw    []byte
	chunkIndex   int
	chunkTotal   int
	idKey        string
	txId         string
	logicalId    string
}

type applySigner struct {
	orgId   string
	certPem []byte
	sk      crypto.PrivateKey
	cert    *bcx509.Certificate
}

var (
	applySignerMu sync.Mutex
	applySignerID *applySigner
)

func handleMessage(msg string) {
	if crossCfg == nil || strings.ToUpper(crossCfg.ShardType) != "BUSINESS" {
		return
	}
	targetShard, batch, ok := ParseBatchMessage(msg)
	if !ok || batch == nil {
		return
	}
	if normalizeShardId(targetShard) != normalizeShardId(crossCfg.ShardId) {
		return
	}
	if !crossCfg.SyncNetwork.Enable {
		return
	}
	queued := enqueueApplyBatch(batch)
	crossLog.Infof("[CrossShard] 通用回写批次已缓存 | batch=%s writes=%d queued=%d proposer=%v",
		batch.BatchId, len(batch.Writes), queued, syncproposer.IsLocalProposer())
}

func submitApplyWriteChunk(chunk *applyWriteChunk) error {
	if !syncproposer.IsLocalProposer() {
		return fmt.Errorf("local node is no longer the writeback proposer")
	}
	if chunk == nil || chunk.batch == nil {
		return fmt.Errorf("invalid writeback chunk")
	}
	chainId, err := pickChainId()
	if err != nil {
		return err
	}
	if submitTx == nil {
		return fmt.Errorf("tx submitter not set")
	}
	if txCommitted != nil {
		exists, checkErr := txCommitted(chainId, chunk.txId)
		if checkErr != nil {
			return fmt.Errorf("check committed writeback tx failed: %v", checkErr)
		}
		if exists {
			return nil
		}
	}
	if txInPool != nil {
		exists, checkErr := txInPool(chainId, chunk.txId)
		if checkErr != nil {
			return fmt.Errorf("check writeback tx in pool failed: %v", checkErr)
		}
		if exists {
			syncproposer.NotifyTxSubmitted(chunk.txId)
			crossLog.Infof("[CrossShard] 批量回写交易已由 proposer 传播 | batch=%s tx_id=%s",
				chunk.batch.BatchId, chunk.txId)
			return nil
		}
	}
	if !syncproposer.IsLocalProposer() {
		return fmt.Errorf("writeback proposer changed during ledger lookup")
	}
	tx, err := buildApplyWriteChunkTx(chainId, chunk)
	if err != nil {
		return err
	}
	if !syncproposer.IsLocalProposer() {
		return fmt.Errorf("writeback proposer changed during signing")
	}
	if submitErr := submitTx(chainId, tx); submitErr != nil {
		if isDuplicateSubmitError(submitErr) {
			syncproposer.NotifyTxSubmitted(chunk.txId)
			crossLog.Infof("[CrossShard] 批量回写交易已存在 | batch=%s tx_id=%s err=%v",
				chunk.batch.BatchId, chunk.txId, submitErr)
			return nil
		}
		return submitErr
	}
	syncproposer.NotifyTxSubmitted(chunk.txId)
	crossLog.Infof("[CrossShard] proposer 已提交批量回写交易 | batch=%s tx_id=%s contract=%s chunk=%d/%d writes=%d",
		chunk.batch.BatchId, chunk.txId, chunk.contractName, chunk.chunkIndex+1,
		chunk.chunkTotal, len(chunk.writes))
	return nil
}

func buildApplyWriteChunks(batch *CrossWriteBatch) ([]*applyWriteChunk, error) {
	if batch == nil || strings.TrimSpace(batch.BatchId) == "" ||
		normalizeShardId(batch.TargetShard) == "" || batch.BridgeBlockHeight == 0 {
		return nil, fmt.Errorf("invalid cross write batch")
	}
	byContract := make(map[string][]BridgeStateWrite)
	for _, write := range batch.Writes {
		contract := strings.TrimSpace(write.ContractName)
		if contract == "" || len(write.Key) == 0 {
			return nil, fmt.Errorf("invalid cross write")
		}
		byContract[contract] = append(byContract[contract], BridgeStateWrite{
			ContractName: contract,
			Key:          cloneStateBytes(write.Key),
			Value:        cloneStateBytes(write.Value),
			SourceTxId:   write.BridgeTxId,
		})
	}
	if len(byContract) == 0 {
		return nil, fmt.Errorf("cross write batch is empty")
	}

	contracts := make([]string, 0, len(byContract))
	for contract := range byContract {
		contracts = append(contracts, contract)
	}
	sort.Strings(contracts)
	maxWrites := defaultApplyBatchMaxWrites
	if crossCfg != nil && crossCfg.Schedule.StateSyncMaxWritesPerTx > 0 {
		maxWrites = crossCfg.Schedule.StateSyncMaxWritesPerTx
	}

	chunks := make([]*applyWriteChunk, 0)
	for _, contract := range contracts {
		writes := byContract[contract]
		sort.Slice(writes, func(i, j int) bool {
			if cmp := bytes.Compare(writes[i].Key, writes[j].Key); cmp != 0 {
				return cmp < 0
			}
			return bytes.Compare(writes[i].Value, writes[j].Value) < 0
		})
		for i := 1; i < len(writes); i++ {
			if bytes.Equal(writes[i-1].Key, writes[i].Key) {
				return nil, fmt.Errorf("duplicate state key in batch: contract=%s key=%x",
					contract, writes[i].Key)
			}
		}
		chunkTotal := (len(writes) + maxWrites - 1) / maxWrites
		for chunkIndex, start := 0, 0; start < len(writes); chunkIndex, start = chunkIndex+1, start+maxWrites {
			end := start + maxWrites
			if end > len(writes) {
				end = len(writes)
			}
			chunkWrites := append([]BridgeStateWrite(nil), writes[start:end]...)
			chunk, err := newApplyWriteChunk(batch, contract, chunkWrites, chunkIndex, chunkTotal)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, chunk)
		}
	}
	return chunks, nil
}

func newApplyWriteChunk(batch *CrossWriteBatch, contract string, writes []BridgeStateWrite,
	chunkIndex, chunkTotal int) (*applyWriteChunk, error) {
	writesRaw, err := json.Marshal(writes)
	if err != nil {
		return nil, err
	}
	sum := sha256.New()
	fmt.Fprintf(sum, "%s|%s|%d|%s|%s|%d|%d|", batch.BatchId,
		normalizeShardId(batch.TargetShard), batch.BridgeBlockHeight, batch.BridgeBlockHash,
		contract, chunkIndex, chunkTotal)
	_, _ = hex.NewEncoder(sum).Write(writesRaw)
	idKey := "batch:" + hex.EncodeToString(sum.Sum(nil))
	copyBatch := *batch
	copyBatch.Writes = nil
	if batch.Authorizations != nil {
		copyBatch.Authorizations = make(map[string][]byte, len(batch.Authorizations))
		for id, proof := range batch.Authorizations {
			copyBatch.Authorizations[id] = cloneStateBytes(proof)
		}
	}
	logicalId := deterministicTxId(idKey)
	chunk := &applyWriteChunk{
		batch:        &copyBatch,
		contractName: contract,
		writes:       writes,
		writesRaw:    writesRaw,
		chunkIndex:   chunkIndex,
		chunkTotal:   chunkTotal,
		idKey:        idKey,
		txId:         logicalId,
		logicalId:    logicalId,
	}
	var chainId string
	if crossCfg != nil {
		chainId = crossCfg.SyncAuth.ChainIDs[syncauth.NormalizeShard(batch.TargetShard)]
	}
	chunk.txId = applyWriteChunkPayload(chainId, chunk).TxId
	return chunk, nil
}

func buildApplyWriteChunkTx(chainId string, chunk *applyWriteChunk) (*commonpb.Transaction, error) {
	if chunk == nil || chunk.batch == nil || chunk.contractName == "" || len(chunk.writes) == 0 || len(chunk.writesRaw) == 0 {
		return nil, fmt.Errorf("invalid writeback chunk")
	}
	tx := &commonpb.Transaction{Payload: applyWriteChunkPayload(chainId, chunk)}
	signer, err := getApplySigner()
	if err != nil {
		return nil, err
	}
	tx.Sender, err = signer.sign(tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func applyWriteChunkPayload(chainId string, chunk *applyWriteChunk) *commonpb.Payload {
	// Keep the task's encoded buffer private so retries cannot inherit changes
	// made to a previously submitted transaction.
	writesRaw := append([]byte(nil), chunk.writesRaw...)
	txTimestamp, expirationTime := deterministicPayloadTimes(chunk.batch.Timestamp, crossApplyTxExpiration)
	params := []*commonpb.KeyValuePair{
		{Key: syncauth.LogicalIDParameter, Value: []byte(chunk.logicalId)},
		{Key: TargetContractParamKey, Value: []byte(chunk.contractName)},
		{Key: TargetShardParamKey, Value: []byte(chunk.batch.TargetShard)},
		{Key: BatchIdParamKey, Value: []byte(chunk.batch.BatchId)},
		{Key: StateWritesParamKey, Value: writesRaw},
		{Key: SourceShardParamKey, Value: []byte("bridge")},
		{Key: SourceHeightParamKey, Value: []byte(strconv.FormatUint(chunk.batch.BridgeBlockHeight, 10))},
		{Key: SourceBlockHashParamKey, Value: []byte(chunk.batch.BridgeBlockHash)},
		{Key: SyncChunkIndexParamKey, Value: []byte(strconv.Itoa(chunk.chunkIndex))},
		{Key: SyncChunkTotalParamKey, Value: []byte(strconv.Itoa(chunk.chunkTotal))},
	}
	if authorization := chunk.batch.Authorizations[chunk.logicalId]; len(authorization) > 0 {
		params = append(params, &commonpb.KeyValuePair{Key: syncauth.Parameter, Value: append([]byte(nil), authorization...)})
	}
	payload := &commonpb.Payload{
		ChainId:        chainId,
		TxType:         commonpb.TxType_INVOKE_CONTRACT,
		TxId:           chunk.txId,
		Timestamp:      txTimestamp,
		ExpirationTime: expirationTime,
		ContractName:   chunk.contractName,
		Method:         SystemStateSyncApplyBatchMethod,
		Parameters:     params,
	}
	syncauth.BindPayloadID(payload)
	return payload
}

func isDuplicateSubmitError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, commonerrors.ErrTxIdExist) || errors.Is(err, commonerrors.ErrTxIdExistDB)
}

func deterministicPayloadTimes(blockTimestamp int64, ttl time.Duration) (int64, int64) {
	if blockTimestamp <= 0 {
		return 0, 0
	}
	return blockTimestamp, blockTimestamp + int64(ttl/time.Second)
}

func pickChainId() (string, error) {
	if localconf.ChainMakerConfig == nil || len(localconf.ChainMakerConfig.BlockChainConfig) == 0 {
		return "", fmt.Errorf("chain config not loaded")
	}
	chainId := localconf.ChainMakerConfig.BlockChainConfig[0].ChainId
	if chainId == "" {
		return "", fmt.Errorf("chain_id empty in chainmaker config")
	}
	return chainId, nil
}

func deterministicTxId(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func getApplySigner() (*applySigner, error) {
	applySignerMu.Lock()
	defer applySignerMu.Unlock()
	if applySignerID != nil {
		return applySignerID, nil
	}
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
	sk, err := asym.PrivateKeyFromPEM(keyPem, nil)
	if err != nil {
		return nil, fmt.Errorf("load private key failed: %v", err)
	}
	cert, err := parseCert(certPem)
	if err != nil {
		return nil, err
	}
	applySignerID = &applySigner{
		orgId:   nodeCfg.OrgId,
		certPem: certPem,
		sk:      sk,
		cert:    cert,
	}
	return applySignerID, nil
}

func (s *applySigner) sign(tx *commonpb.Transaction) (*commonpb.EndorsementEntry, error) {
	if tx == nil {
		return nil, fmt.Errorf("tx is nil")
	}
	txBytes, err := utils.CalcUnsignedTxBytes(tx)
	if err != nil {
		return nil, fmt.Errorf("calc tx bytes failed: %v", err)
	}
	sig, err := signTxBytes(s.sk, s.cert, txBytes)
	if err != nil {
		return nil, err
	}
	member := &pbac.Member{
		OrgId:      s.orgId,
		MemberType: pbac.MemberType_CERT,
		MemberInfo: s.certPem,
	}
	return &commonpb.EndorsementEntry{
		Signer:    member,
		Signature: sig,
	}, nil
}

func parseCert(crtPEM []byte) (*bcx509.Certificate, error) {
	block, _ := pem.Decode(crtPEM)
	if block == nil {
		return nil, fmt.Errorf("decode pem failed, invalid certificate")
	}
	cert, err := bcx509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509 parse cert failed: %v", err)
	}
	return cert, nil
}

func signTxBytes(privateKey crypto.PrivateKey, cert *bcx509.Certificate, msg []byte) ([]byte, error) {
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

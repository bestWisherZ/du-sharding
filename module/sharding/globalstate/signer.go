package globalstate

import (
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"

	"chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	"chainmaker.org/chainmaker/localconf/v2"
	pbac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
)

const systemSyncTxExpiration = 24 * time.Hour

type stateSyncSigner struct {
	orgId   string
	certPem []byte
	sk      crypto.PrivateKey
	cert    *bcx509.Certificate
}

var (
	stateSyncSignerMu sync.Mutex
	stateSyncSignerID *stateSyncSigner
)

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

func getStateSyncSigner() (*stateSyncSigner, error) {
	stateSyncSignerMu.Lock()
	defer stateSyncSignerMu.Unlock()
	if stateSyncSignerID != nil {
		return stateSyncSignerID, nil
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
	stateSyncSignerID = &stateSyncSigner{
		orgId:   nodeCfg.OrgId,
		certPem: certPem,
		sk:      sk,
		cert:    cert,
	}
	return stateSyncSignerID, nil
}

func (s *stateSyncSigner) sign(tx *commonpb.Transaction) (*commonpb.EndorsementEntry, error) {
	if tx == nil {
		return nil, fmt.Errorf("tx is nil")
	}
	txBytes, err := utils.CalcUnsignedTxBytes(tx)
	if err != nil {
		return nil, fmt.Errorf("calc tx bytes failed: %v", err)
	}
	return s.signBytes(txBytes)
}

func (s *stateSyncSigner) signBytes(txBytes []byte) (*commonpb.EndorsementEntry, error) {
	sig, err := signTxBytes(s.sk, s.cert, txBytes)
	if err != nil {
		return nil, err
	}
	return &commonpb.EndorsementEntry{
		Signer: &pbac.Member{
			OrgId:      s.orgId,
			MemberType: pbac.MemberType_CERT,
			MemberInfo: s.certPem,
		},
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
	return privateKey.SignWithOpts(msg, &crypto.SignOpts{
		Hash: hashAlgo,
		UID:  crypto.CRYPTO_DEFAULT_UID,
	})
}

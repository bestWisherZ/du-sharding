// Package syncauth authenticates committed-state attestations from explicitly
// trusted source nodes. An attestation is not a source-chain consensus proof.
package syncauth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"chainmaker.org/chainmaker/common/v2/crypto"
	bcx509 "chainmaker.org/chainmaker/common/v2/crypto/x509"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
)

const Parameter = "__sync_source_authorization"
const LogicalIDParameter = "__sync_logical_tx_id"

// BindPayloadID covers the complete authorized payload, including its timestamp.
// Logical IDs remain independent of the source replica that attested the state.
func BindPayloadID(p *commonpb.Payload) {
	if p == nil {
		return
	}
	logical, authorized := logicalPayloadID(p)
	if logical == "" || !authorized {
		return
	}
	copyPayload := *p
	copyPayload.TxId = logical
	raw, err := proto.Marshal(&copyPayload)
	if err != nil {
		return
	}
	h := sha256.Sum256(raw)
	p.TxId = hex.EncodeToString(h[:])
}

func LogicalID(p *commonpb.Payload) string {
	logical, _ := logicalPayloadID(p)
	if logical != "" {
		return logical
	}
	if p != nil {
		return p.TxId
	}
	return ""
}

func logicalPayloadID(p *commonpb.Payload) (string, bool) {
	var logical string
	var authorized bool
	if p != nil {
		for _, kv := range p.Parameters {
			if kv == nil {
				continue
			}
			if kv.Key == LogicalIDParameter {
				logical = string(kv.Value)
			}
			if kv.Key == Parameter && len(kv.Value) > 0 {
				authorized = true
			}
		}
	}
	return logical, authorized
}

type Config struct {
	NetworkID string              `mapstructure:"network_id" yaml:"network_id"`
	ChainIDs  map[string]string   `mapstructure:"chain_ids" yaml:"chain_ids"`
	Signers   map[string][]string `mapstructure:"signers" yaml:"signers"`
}

type Authorization struct {
	Certificate []byte `json:"certificate"`
	Signature   []byte `json:"signature"`
}

func NormalizeShard(shard string) string {
	shard = strings.TrimSpace(shard)
	if shard == "bridge-shard" {
		return "bridge"
	}
	if strings.HasPrefix(shard, "business-") && !strings.HasPrefix(shard, "business-shard") {
		return "business-shard" + strings.TrimPrefix(shard, "business-")
	}
	return shard
}

func CertificateID(cert *bcx509.Certificate) string {
	h := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(h[:])
}

func (c Config) Validate(shards []string) error {
	if c.NetworkID == "" {
		return fmt.Errorf("sync_auth.network_id is required")
	}
	chains := make(map[string]bool)
	for _, shard := range shards {
		shard = NormalizeShard(shard)
		id := c.ChainIDs[shard]
		if id == "" || chains[id] || len(c.Signers[shard]) == 0 {
			return fmt.Errorf("sync_auth requires a unique chain ID and trusted signers for %s", shard)
		}
		chains[id] = true
		for _, pin := range c.Signers[shard] {
			decoded, err := hex.DecodeString(pin)
			if err != nil || len(decoded) != sha256.Size {
				return fmt.Errorf("invalid SHA256 certificate pin for %s", shard)
			}
		}
	}
	return nil
}

func signingBytes(cfg Config, source, target string, p *commonpb.Payload) ([]byte, error) {
	if p == nil || cfg.NetworkID == "" || cfg.ChainIDs[NormalizeShard(source)] == "" ||
		cfg.ChainIDs[NormalizeShard(target)] != p.ChainId || p.ChainId == "" {
		return nil, fmt.Errorf("sync authorization network/source/target chain mismatch")
	}
	copyPayload := *p
	copyPayload.Parameters = make([]*commonpb.KeyValuePair, 0, len(p.Parameters))
	if logical, _ := logicalPayloadID(p); logical != "" {
		copyPayload.TxId = logical
	}
	seen := make(map[string]bool)
	for _, kv := range p.Parameters {
		if kv == nil || seen[kv.Key] {
			return nil, fmt.Errorf("nil or duplicate sync parameter")
		}
		seen[kv.Key] = true
		if kv.Key != Parameter {
			copyPayload.Parameters = append(copyPayload.Parameters, kv)
		}
	}
	raw, err := proto.Marshal(&copyPayload)
	if err != nil {
		return nil, err
	}
	// Length-delimited domain and shard names prevent cross-network replay.
	envelope := struct {
		Version string
		Network string
		Source  string
		Target  string
		Payload []byte
	}{"state-sync-attestation-v1", cfg.NetworkID, NormalizeShard(source), NormalizeShard(target), raw}
	return json.Marshal(envelope)
}

func Sign(cfg Config, source, target string, p *commonpb.Payload, certificate []byte, key crypto.PrivateKey) ([]byte, error) {
	cert, err := utils.ParseCert(certificate)
	if err != nil {
		return nil, err
	}
	if !trusted(cfg, source, cert) {
		return nil, fmt.Errorf("local signing certificate is not authorized for %s", source)
	}
	raw, err := signingBytes(cfg, source, target, p)
	if err != nil {
		return nil, err
	}
	hash, err := bcx509.GetHashFromSignatureAlgorithm(cert.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}
	sig, err := key.SignWithOpts(raw, &crypto.SignOpts{Hash: hash, UID: crypto.CRYPTO_DEFAULT_UID})
	if err != nil {
		return nil, err
	}
	return json.Marshal(Authorization{Certificate: certificate, Signature: sig})
}

func Verify(cfg Config, source, target string, p *commonpb.Payload) error {
	if logical, authorized := logicalPayloadID(p); logical != "" && authorized {
		bound := *p
		BindPayloadID(&bound)
		if bound.TxId != p.TxId {
			return fmt.Errorf("authorized sync payload does not match transaction ID")
		}
	}
	var encoded []byte
	if p != nil {
		for _, kv := range p.Parameters {
			if kv != nil && kv.Key == Parameter {
				encoded = kv.Value
			}
		}
	}
	if len(encoded) == 0 || len(encoded) > 64*1024 {
		return fmt.Errorf("missing or oversized source authorization")
	}
	var auth Authorization
	if err := json.Unmarshal(encoded, &auth); err != nil {
		return fmt.Errorf("invalid source authorization: %w", err)
	}
	cert, err := utils.ParseCert(auth.Certificate)
	if err != nil || !trusted(cfg, source, cert) {
		return fmt.Errorf("untrusted source signer for %s", source)
	}
	raw, err := signingBytes(cfg, source, target, p)
	if err != nil {
		return err
	}
	hash, err := bcx509.GetHashFromSignatureAlgorithm(cert.SignatureAlgorithm)
	if err != nil {
		return err
	}
	ok, err := cert.PublicKey.VerifyWithOpts(raw, auth.Signature, &crypto.SignOpts{Hash: hash, UID: crypto.CRYPTO_DEFAULT_UID})
	if err != nil || !ok {
		return fmt.Errorf("source authorization signature verification failed")
	}
	return nil
}

func trusted(cfg Config, source string, cert *bcx509.Certificate) bool {
	if cert == nil {
		return false
	}
	id := CertificateID(cert)
	for _, pin := range cfg.Signers[NormalizeShard(source)] {
		if strings.EqualFold(id, pin) {
			return true
		}
	}
	return false
}

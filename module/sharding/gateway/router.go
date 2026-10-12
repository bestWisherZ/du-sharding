package gateway

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"chainmaker.org/chainmaker/common/v2/crypto"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	syscontract "chainmaker.org/chainmaker/pb-go/v2/syscontract"
	"chainmaker.org/chainmaker/utils/v2"
)

// ResolveRoute may be called before signing, with the caller's member attached.
// The caller must sign the final request using the returned chain ID.
func ResolveRoute(cfg *GatewayConfig, req *commonpb.TxRequest) (string, string, error) {
	backend, err := selectBackendByTx(cfg, req)
	if err != nil {
		return "", "", err
	}
	b := cfg.Backends[backend]
	if b == nil || b.ChainID == "" {
		return "", "", fmt.Errorf("missing backend/chain_id: %s", backend)
	}
	return backend, b.ChainID, nil
}

func selectBackendByTx(cfg *GatewayConfig, req *commonpb.TxRequest) (string, error) {
	if cfg == nil || req == nil || req.Payload == nil {
		return "", fmt.Errorf("missing gateway config or payload")
	}
	p := req.Payload
	if _, system := syscontract.SystemContract_value[p.ContractName]; system ||
		(p.TxType != commonpb.TxType_INVOKE_CONTRACT && p.TxType != commonpb.TxType_QUERY_CONTRACT) {
		for name, backend := range cfg.Backends {
			if backend != nil && backend.ChainID == p.ChainId {
				return name, nil
			}
		}
		return "", fmt.Errorf("no backend for chain_id=%s", p.ChainId)
	}
	var rule *RoutingRule
	for i := range cfg.RoutingRules {
		r := &cfg.RoutingRules[i]
		if r.ContractName == p.ContractName && r.Method == p.Method {
			rule = r
			break
		}
		if r.ContractName == p.ContractName && r.Method == "*" {
			rule = r
		}
	}
	if rule == nil {
		return "", fmt.Errorf("no routing rule for %s/%s", p.ContractName, p.Method)
	}
	switch rule.effectiveStrategy() {
	case routingStrategyFixed:
		return rule.TargetBackend, nil
	case routingStrategyHash:
	default:
		return "", fmt.Errorf("unsupported routing strategy %q for %s/%s", rule.Strategy, p.ContractName, p.Method)
	}
	params := make(map[string][]byte, len(p.Parameters))
	for _, kv := range p.Parameters {
		if kv == nil {
			return "", fmt.Errorf("nil parameter")
		}
		if _, duplicate := params[kv.Key]; duplicate {
			return "", fmt.Errorf("duplicate parameter: %s", kv.Key)
		}
		params[kv.Key] = kv.Value
	}
	accounts := make([]string, 0, len(rule.AccountParameters)+1)
	for _, key := range rule.AccountParameters {
		if len(params[key]) == 0 {
			return "", fmt.Errorf("routing parameter %q is required", key)
		}
		accounts = append(accounts, string(params[key]))
	}
	if rule.IncludeSender {
		if req.Sender == nil || req.Sender.Signer == nil {
			return "", fmt.Errorf("sender member is required for routing")
		}
		addrType, ok := configpb.AddrType_value[strings.ToUpper(cfg.AddressType)]
		if !ok {
			return "", fmt.Errorf("address_type must match the chains' address configuration")
		}
		hashType, ok := crypto.HashAlgoMap[strings.ToUpper(cfg.HashType)]
		if !ok {
			return "", fmt.Errorf("hash_type must match the chains' hash configuration")
		}
		address, err := utils.GetStrAddrFromPbMember(req.Sender.Signer, configpb.AddrType(addrType), hashType)
		if err != nil {
			return "", fmt.Errorf("derive sender address (use full certificate/public key): %v", err)
		}
		accounts = append(accounts, address)
	}
	if len(accounts) == 0 || len(cfg.BusinessShards) == 0 {
		return "", fmt.Errorf("partitioned route has no accounts or shards")
	}
	target := shardForAccount(cfg.BusinessShards, accounts[0])
	for _, account := range accounts[1:] {
		if shardForAccount(cfg.BusinessShards, account) != target {
			return cfg.BridgeBackend, nil
		}
	}
	return target, nil
}

func shardForAccount(shards []string, account string) string {
	if len(shards) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(account))
	return shards[binary.BigEndian.Uint64(sum[:8])%uint64(len(shards))]
}

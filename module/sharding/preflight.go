package sharding

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/crossshard"
	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/spf13/viper"
)

// Preflight runs before VM initialization or ledger replay. A missing sharding
// file means an ordinary chain; an invalid file must never disable enforcement.
func Preflight(chainConf *localconf.CMConfig) error {
	cfg, err := shardingsync.LoadConfig(chainConf)
	var missing viper.ConfigFileNotFoundError
	if errors.As(err, &missing) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load sharding config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid sharding config: %w", err)
	}
	if chainConf == nil || len(chainConf.BlockChainConfig) != 1 {
		return fmt.Errorf("sharding currently requires exactly one chain per node process")
	}
	local := syncauth.NormalizeShard(cfg.ShardId)
	if chainConf.BlockChainConfig[0].ChainId != cfg.SyncAuth.ChainIDs[local] {
		return fmt.Errorf("local chain_id does not match sync_auth.chain_ids[%s]", local)
	}
	certBytes, err := os.ReadFile(chainConf.NodeConfig.CertFile)
	if err != nil {
		return fmt.Errorf("read local signing certificate: %w", err)
	}
	cert, err := utils.ParseCert(certBytes)
	if err != nil {
		return fmt.Errorf("parse local signing certificate: %w", err)
	}
	trusted := false
	for _, pin := range cfg.SyncAuth.Signers[local] {
		if strings.EqualFold(pin, syncauth.CertificateID(cert)) {
			trusted = true
		}
	}
	if !trusted {
		return fmt.Errorf("local signing certificate is not pinned in sync_auth.signers[%s]", local)
	}
	crossshard.InitWithConfig(cfg)
	return nil
}

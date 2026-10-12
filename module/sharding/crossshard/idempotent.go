package crossshard

import (
	"fmt"
	"path/filepath"
	"sync"

	"chainmaker.org/chainmaker/localconf/v2"
	storeconf "chainmaker.org/chainmaker/store/v2/conf"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	idemMu  sync.Mutex
	idemDBs = make(map[string]*leveldb.DB)
)

func markProcessed(shardId, crossTxId string) error {
	db, err := openIdemDB(shardId)
	if err != nil {
		return err
	}
	key := []byte("X:" + crossTxId)
	return db.Put(key, []byte("1"), nil)
}

func isProcessed(shardId, crossTxId string) (bool, error) {
	db, err := openIdemDB(shardId)
	if err != nil {
		return false, err
	}
	key := []byte("X:" + crossTxId)
	ok, err := db.Has(key, nil)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func openIdemDB(shardId string) (*leveldb.DB, error) {
	idemMu.Lock()
	defer idemMu.Unlock()
	rootDir, err := resolveIdemDir(shardId)
	if err != nil {
		return nil, err
	}
	if db, ok := idemDBs[rootDir]; ok {
		return db, nil
	}
	db, err := leveldb.OpenFile(rootDir, nil)
	if err != nil {
		return nil, err
	}
	idemDBs[rootDir] = db
	return db, nil
}

func resolveIdemDir(shardId string) (string, error) {
	dataDir := resolveDataDir()
	if dataDir == "" {
		return "", fmt.Errorf("data directory not found")
	}
	return filepath.Join(dataDir, "sharding", shardId, "crossshard", "idempotent"), nil
}

func resolveDataDir() string {
	if localconf.ChainMakerConfig == nil {
		return ""
	}
	cfg, err := storeconf.NewStorageConfig(localconf.ChainMakerConfig.StorageConfig)
	if err != nil || cfg == nil || cfg.BlockDbConfig == nil {
		return ""
	}
	if cfg.BlockDbConfig.LevelDbConfig == nil {
		return ""
	}
	storePath, ok := cfg.BlockDbConfig.LevelDbConfig["store_path"].(string)
	if !ok || storePath == "" {
		return ""
	}
	return filepath.Dir(storePath)
}

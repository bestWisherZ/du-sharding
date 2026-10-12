package crossshard

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"chainmaker.org/chainmaker/localconf/v2"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
)

var (
	clientOnce sync.Once
	clientInst *sdk.ChainClient
	clientErr  error
)

func getClient() (*sdk.ChainClient, error) {
	clientOnce.Do(func() {
		cfgPath := resolveSDKConfigPath()
		if cfgPath == "" {
			clientErr = fmt.Errorf("sdk config path not found")
			return
		}
		if _, err := os.Stat(cfgPath); err != nil {
			clientErr = fmt.Errorf("sdk config not found: %s", cfgPath)
			return
		}
		clientInst, clientErr = sdk.NewChainClient(sdk.WithConfPath(cfgPath))
	})
	return clientInst, clientErr
}

func resolveSDKConfigPath() string {
	if localconf.ChainMakerConfig == nil {
		return ""
	}
	orgId := localconf.ChainMakerConfig.NodeConfig.OrgId
	if orgId == "" {
		return ""
	}
	logPath := localconf.ChainMakerConfig.LogConfig.ConfigFile
	if logPath == "" {
		logPath = localconf.ChainMakerConfig.LogConfig.SystemLog.FilePath
	}
	if logPath == "" {
		return ""
	}
	absPath, err := filepath.Abs(logPath)
	if err != nil {
		absPath = logPath
	}
	baseDir := filepath.Dir(filepath.Dir(absPath))
	return filepath.Join(baseDir, "config", orgId, "sdk_config.yml")
}

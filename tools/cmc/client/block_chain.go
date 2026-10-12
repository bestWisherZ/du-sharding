/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"fmt"

	"github.com/spf13/cobra"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
)

// blockChainsCMD blockchains sub command
// @return *cobra.Command
func blockChainsCMD() *cobra.Command {
	chainConfigCmd := &cobra.Command{
		Use:   "blockchains",
		Short: "blockchains command",
		Long:  "blockchains command",
	}
	chainConfigCmd.AddCommand(checkNewBlockchainsCMD())
	chainConfigCmd.AddCommand(updateChainsCMD())
	return chainConfigCmd
}

// checkNewBlockchainsCMD check ChainMaker server local config has changed
// @return *cobra.Command
func checkNewBlockchainsCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checknew",
		Short: "check new blockchains",
		Long:  "check new blockchains",
		RunE: func(_ *cobra.Command, _ []string) error {
			return checkNewBlockchains()
		},
	}

	attachFlags(cmd, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath,
		flagSdkConfPath, flagOrgId, flagChainId, flagUserTlsCrtFilePath, flagUserTlsKeyFilePath,
	})

	return cmd
}

func updateChainsCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update_node_config",
		Short: "update node config",
		Long:  "update node config",
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateNodeConfig()
		},
	}

	attachFlags(cmd, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath,
		flagSdkConfPath, flagOrgId, flagChainId, flagUserTlsCrtFilePath, flagUserTlsKeyFilePath,
		flagEnableCertHash,
	})

	return cmd
}

func checkNewBlockchains() error {
	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
	if err != nil {
		return fmt.Errorf("create user client failed, %s", err.Error())
	}
	defer client.Stop()
	err = client.CheckNewBlockChainConfig()
	if err != nil {
		return fmt.Errorf("check new blockchains failed, %s", err.Error())
	}
	fmt.Printf("check new blockchains ok \n")
	return nil
}

func updateNodeConfig() error {
	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
	if err != nil {
		return fmt.Errorf("create user client failed, %s", err.Error())
	}
	defer client.Stop()
	err = client.UpdateNodeConfig()
	if err != nil {
		return fmt.Errorf("update chains failed, %s", err.Error())
	}
	fmt.Printf("update chains ok \n")
	return nil
}

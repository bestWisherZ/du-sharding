/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"fmt"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	"github.com/hokaccha/go-prettyjson"
	"github.com/spf13/cobra"
)

// chainConfigCMD chain config command
// @return *cobra.Command
func chainConfigCMD() *cobra.Command {
	chainConfigCmd := &cobra.Command{
		Use:   "chainconfig",
		Short: "chain config command",
		Long:  "chain config command",
	}
	chainConfigCmd.AddCommand(queryChainConfigCMD())
	chainConfigCmd.AddCommand(updateBlockConfigCMD())
	chainConfigCmd.AddCommand(configTrustRootCMD())
	chainConfigCmd.AddCommand(configConsensueNodeIdCMD())
	chainConfigCmd.AddCommand(configConsensueNodeOrgCMD())
	chainConfigCmd.AddCommand(configConsensueExtraCMD())
	chainConfigCmd.AddCommand(configTrustMemberCMD())
	chainConfigCmd.AddCommand(alterAddrTypeCMD())
	chainConfigCmd.AddCommand(permissionResourceCMD())
	chainConfigCmd.AddCommand(enableMultiSignManualRunCMD())
	chainConfigCmd.AddCommand(vmSupportCMD())
	return chainConfigCmd
}

// queryChainConfigCMD query current chain config
// @return *cobra.Command
func queryChainConfigCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "query chain config",
		Long:  "query chain config",
		RunE: func(_ *cobra.Command, _ []string) error {
			return queryChainConfig()
		},
	}

	attachFlags(cmd, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath,
		flagSdkConfPath, flagOrgId, flagEnableCertHash,
		flagUserTlsCrtFilePath, flagUserTlsKeyFilePath,
	})

	return cmd
}

func queryChainConfig() error {
	client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
		userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
	if err != nil {
		return fmt.Errorf("create user client failed, %s", err.Error())
	}
	defer client.Stop()
	chainConfig, err := client.GetChainConfig()
	if err != nil {
		return fmt.Errorf("get chain config failed, %s", err.Error())
	}

	output, err := prettyjson.Marshal(chainConfig)
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

// vmCmd vmSupportCMD command
// @return *cobra.Command
func vmSupportCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm-support",
		Short: "vm support command",
		Long:  "vm support command",
	}
	cmd.AddCommand(addVmCommand())
	return cmd
}

func addVmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add vm support command",
		Long:  "add vm support command",
		RunE: func(_ *cobra.Command, _ []string) error {
			return addVmSupport()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath, flagUserTlsKeyFilePath, flagUserTlsCrtFilePath,
		flagOrgId, flagChainId, flagSendTimes, flagEnableCertHash, flagSdkConfPath, flagPayerKeyFilePath,
		flagTimeout, flagSyncResult, flagAdminKeyFilePaths, flagAdminCrtFilePaths, flagAdminOrgIds,
		flagVmType,
	})
	return cmd
}

func addVmSupport() error {
	if err := checkVmType(); err != nil {
		return err
	}
	cc, err := sdk.NewChainClient(
		sdk.WithConfPath(sdkConfPath),
		sdk.WithChainClientChainId(chainId),
		sdk.WithChainClientOrgId(orgId),
		sdk.WithUserCrtFilePath(userTlsCrtFilePath),
		sdk.WithUserKeyFilePath(userTlsKeyFilePath),
		sdk.WithUserSignCrtFilePath(userSignCrtFilePath),
		sdk.WithUserSignKeyFilePath(userSignKeyFilePath),
	)
	if err != nil {
		return err
	}
	payload, err := cc.CreateChainConfigVMSupportListAddPayload(vmType)
	if err != nil {
		return err
	}
	adminKeys, adminCrts, adminOrgs, err := util.MakeAdminInfo(cc, adminKeyFilePaths, adminCrtFilePaths, adminOrgIds)
	if err != nil {
		return err
	}
	endorsementEntrys, err := util.MakeEndorsement(adminKeys, adminCrts, adminOrgs, cc, payload)
	if err != nil {
		return err
	}
	resp, err := cc.SendChainConfigUpdateRequest(payload, endorsementEntrys, timeout, syncResult)
	if err != nil {
		return err
	}
	util.PrintPrettyJson(resp)
	return nil
}

func checkVmType() error {
	if vmType == "" {
		return fmt.Errorf("vm type is empty")
	}
	supportList := []string{"wasmer", "gasm", "evm", "dockergo", "wxvm", "dockerjava"}
	supportM := make(map[string]struct{})
	for _, value := range supportList {
		supportM[value] = struct{}{}
	}
	if _, ok := supportM[vmType]; !ok {
		return fmt.Errorf("vm type %s is not support", vmType)
	}
	return nil
}

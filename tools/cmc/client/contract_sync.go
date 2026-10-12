/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package client

import (
	"fmt"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"github.com/spf13/cobra"
)

var (
	ruleCommonFlags = []string{
		flagSdkConfPath,
		flagOrgId, flagChainId,
		flagUserTlsCrtFilePath, flagUserTlsKeyFilePath,
		flagUserSignCrtFilePath, flagUserSignKeyFilePath,
	}
	ruleEndorsementFlags = []string{flagAdminKeyFilePaths, flagAdminCrtFilePaths, flagAdminOrgIds}
)

// syncAddRule 同步模块添加规则
// @return *cobra.Command
func syncAddRule() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-syncrule",
		Short: "add rule to the node for syncing blocks",
		RunE: func(_ *cobra.Command, _ []string) error {
			var (
				err error
			)
			client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
				userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
			if err != nil {
				return err
			}
			defer client.Stop()

			adminKeys, adminCrts, adminOrgs, err := util.MakeAdminInfo(client,
				adminKeyFilePaths, adminCrtFilePaths, adminOrgIds)
			if err != nil {
				return err
			}

			payload, err := client.CreateAddSyncRulePayload(nodeId, syncRule, int(beginHeight), int(endHeight))
			if err != nil {
				return fmt.Errorf("get add rule payload failed, %s", err.Error())
			}

			endorsementEntries, err := util.MakeEndorsement(adminKeys, adminCrts, adminOrgs, client, payload)
			if err != nil {
				return err
			}

			resp, err := client.SendSyncRuleRequest(payload, endorsementEntries, DEFAULT_TIMEOUT, syncResult)
			if err != nil {
				return fmt.Errorf("send request failed, %s", err.Error())
			}
			util.PrintPrettyJson(resp)

			return nil
		},
	}

	attachFlags(cmd, ruleCommonFlags)
	attachFlags(cmd, ruleEndorsementFlags)
	attachFlags(cmd, []string{
		flagNodeId, flagRule, flagBeginHeight, flagEndHeight,
		flagSyncResult,
	})

	cmd.MarkFlagRequired(flagNodeId)
	cmd.MarkFlagRequired(flagRule)

	return cmd
}

// syncGetRule 同步模块获取规则
// @return *cobra.Command
func syncGetRule() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-syncrule",
		Short: "get rule of the node for syncing blocks",
		RunE: func(_ *cobra.Command, _ []string) error {
			var (
				err error
			)
			client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
				userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
			if err != nil {
				return err
			}
			defer client.Stop()
			resp, err := client.GetNodeRule(nodeId)
			if err != nil {
				return fmt.Errorf("get rule failed, %s", err.Error())
			}

			util.PrintPrettyJson(resp)

			return nil
		},
	}

	attachFlags(cmd, ruleCommonFlags)
	attachFlags(cmd, []string{
		flagNodeId,
		flagSyncResult,
	})

	cmd.MarkFlagRequired(flagNodeId)

	return cmd
}

// syncClearRule 同步模块清除节点规则
// @return *cobra.Command
func syncClearRule() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-syncrule",
		Short: "clear all rules of the node for syncing blocks",
		RunE: func(_ *cobra.Command, _ []string) error {
			var (
				err error
			)
			client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
				userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
			if err != nil {
				return err
			}
			defer client.Stop()

			adminKeys, adminCrts, adminOrgs, err := util.MakeAdminInfo(client,
				adminKeyFilePaths, adminCrtFilePaths, adminOrgIds)
			if err != nil {
				return err
			}

			payload := client.CreateClearNodeRulePayload(nodeId)

			endorsementEntries, err := util.MakeEndorsement(adminKeys, adminCrts, adminOrgs, client, payload)
			if err != nil {
				return err
			}

			resp, err := client.SendSyncRuleRequest(payload, endorsementEntries, DEFAULT_TIMEOUT, syncResult)
			if err != nil {
				return fmt.Errorf("send request failed, %s", err.Error())
			}
			util.PrintPrettyJson(resp)

			return nil
		},
	}

	attachFlags(cmd, ruleCommonFlags)
	attachFlags(cmd, ruleEndorsementFlags)
	attachFlags(cmd, []string{
		flagNodeId,
		flagSyncResult,
	})

	cmd.MarkFlagRequired(flagNodeId)

	return cmd
}

func syncCompareRule() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare-syncrule",
		Short: "compare whether rule1 contains rule2",
		Long: "compare whether rule1 contains rule2 \n" +
			"The \"contain\" here refers to whether the transaction " +
			"scope defined by rule1 can include the transaction scope defined by rule2.\n" +
			"The node can only synchronize blocks from the nodes whose's rule contains it's rule.",
		RunE: func(_ *cobra.Command, _ []string) error {
			var (
				err error
			)
			client, err := util.CreateChainClient(sdkConfPath, chainId, orgId, userTlsCrtFilePath, userTlsKeyFilePath,
				userSignCrtFilePath, userSignKeyFilePath, enableCertHash)
			if err != nil {
				return err
			}
			defer client.Stop()
			contained, err := client.CompareRuleContain(syncRule1ToCmp, syncRule2ToCmp)
			if err != nil {
				return fmt.Errorf("compare rule request failed, %s", err.Error())
			}
			if contained {
				fmt.Printf("the rule1 [%s] contains rule2 [%s]\n", syncRule1ToCmp, syncRule2ToCmp)
			} else {
				fmt.Printf("the rule1 [%s] does not contains rule2[%s]\n", syncRule1ToCmp, syncRule2ToCmp)
			}
			return nil
		},
	}

	attachFlags(cmd, ruleCommonFlags)
	attachFlags(cmd, []string{
		flagSyncRule1, flagSyncRule2,
	})

	cmd.MarkFlagRequired(flagSyncRule1)
	cmd.MarkFlagRequired(flagSyncRule2)
	return cmd
}

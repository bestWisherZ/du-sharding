/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package parallel

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"github.com/spf13/cobra"
)

// invokeCMD invoke contract
// @return *cobra.Command
func invokeCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   invokerMethod,
		Short: "Invoke",
		RunE: func(_ *cobra.Command, _ []string) error {
			paramRead()
			if err := paramCheck(); err != nil {
				return err
			}
			return parallel(invokerMethod)
		},
	}
	util.AttachFlags(cmd, flags, []string{
		// 压力测试配置
		threadNumFlag, loopNumFlag, timeoutFlag, printTimeFlag, sleepTimeFlag, climbTimeFlag, prepareBuildFlag,
		// 证书配置
		signCrtPathsStringFlag, signKeyPathsStringFlag, orgIDsStringFlag, orgIdsFlag,
		userCrtPathsStringFlag, userKeyPathsStringFlag, caPathsStringFlag, useTLSFlag,
		userEncKeyPathsStringFlag, userEncCrtPathsStringFlag,
		adminSignKeysFlag, adminSignCrtsFlag,
		// 压测请求配置
		checkResultFlag, recordLogFlag, outputResultFlag, showKeyFlag, requestTimeoutFlag,
		checkIntervalFlag, onlySendFlag,
		// 链配置
		hostsStringFlag, hashAlgoFlag, chainIdFlag, contractNameFlag, useShortCrtFlag,
		authTypeUint32Flag, gasLimitFlag, hostnamesStringFlag,
		methodFlag, abiPathFlag,
		pairsStringFlag, pairsFileFlag,
	})
	return cmd
}

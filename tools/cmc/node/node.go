/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package node

import (
	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	sdkConfPath string
	flags       *pflag.FlagSet
)

const (
	flagSdkConfPath = "sdk-conf-path"
)

func init() {
	flags = &pflag.FlagSet{}

	flags.StringVar(&sdkConfPath, flagSdkConfPath, "", "specify sdk config path")
}

// NewNodeCMD new command for node
func NewNodeCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "node command",
		Long: "node command\n" +
			"In principle, this command is only effective for the accessed node.",
	}

	cmd.AddCommand(newSyncStateCMD())
	return cmd
}

func newSyncStateCMD() *cobra.Command {
	var (
		withOthersState bool
		flagWithOthers  = "with-others"
	)
	cmd := &cobra.Command{
		Use:   "syncstate",
		Short: "sync state of node",
		Long:  "get sync state of node accessed",
		RunE: func(_ *cobra.Command, _ []string) error {
			cc, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer cc.Stop()
			s, err := cc.GetSyncState(withOthersState)
			if err != nil {
				return err
			}
			util.PrintPrettyJson(s)
			return nil
		},
	}

	flags.BoolVar(&withOthersState, flagWithOthers, false,
		"specify whether to attach the height state of other nodes")
	util.AttachAndRequiredFlags(cmd, flags, []string{
		flagSdkConfPath,
	})
	util.AttachFlags(cmd, flags, []string{flagWithOthers})
	return cmd
}

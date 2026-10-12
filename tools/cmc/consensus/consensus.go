/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 */

package consensus

import (
	"fmt"

	"chainmaker.org/chainmaker-go/tools/cmc/util"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	sdkConfPath string
)

const (
	flagSdkConfPath = "sdk-conf-path"
)

// NewConsensusCMD new consensus command
func NewConsensusCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consensus",
		Short: "consensus command",
		Long:  "consensus command",
	}

	cmd.AddCommand(newGetConsensusHeightCMD())
	cmd.AddCommand(newGetConsensusStateJSONCMD())
	cmd.AddCommand(newGetConsensusValidatorsCMD())
	return cmd
}

var flags *pflag.FlagSet

func init() {
	flags = &pflag.FlagSet{}

	flags.StringVar(&sdkConfPath, flagSdkConfPath, "", "specify sdk config path")
}

// newGetConsensusHeightCMD get height of the consensus
// @return *cobra.Command
func newGetConsensusHeightCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "height",
		Short: "get the consensus height",
		Long:  "get the height of the block participating in the consensus",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer cc.Stop()

			height, err := cc.GetConsensusHeight()
			if err != nil {
				return err
			}

			util.PrintPrettyJson(
				struct {
					Height uint64
				}{
					Height: height,
				})
			return nil
		},
	}

	util.AttachAndRequiredFlags(cmd, flags, []string{
		flagSdkConfPath,
	})
	return cmd
}

// newGetConsensusValidatorsCMD get validators of the consensus
// @return *cobra.Command
func newGetConsensusValidatorsCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validators",
		Short: "get consensus nodes",
		Long:  "get the identity of all consensus nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer cc.Stop()

			validators, err := cc.GetConsensusValidators()
			if err != nil {
				return err
			}

			util.PrintPrettyJson(validators)
			return nil
		},
	}

	util.AttachAndRequiredFlags(cmd, flags, []string{
		flagSdkConfPath,
	})
	return cmd
}

// newGetConsensusStateJSONCMD get consensus status
// @return *cobra.Command
func newGetConsensusStateJSONCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "get consensus status",
		Long:  "Get the status of the consensus node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer cc.Stop()

			jsonState, err := cc.GetConsensusStateJSON()
			if err != nil {
				return err
			}

			fmt.Println(jsonState)
			return nil
		},
	}

	util.AttachAndRequiredFlags(cmd, flags, []string{
		flagSdkConfPath,
	})
	return cmd
}

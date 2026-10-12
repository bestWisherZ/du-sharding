/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cmd

import (
	"fmt"

	"chainmaker.org/chainmaker/protocol/v2"

	"chainmaker.org/chainmaker-go/module/blockchain"
	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
)

func VersionCMD() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show ChainMaker version",
		Long:  "Show ChainMaker version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			PrintVersion()
			return nil
		},
	}
}

func logo() string {
	fig := figure.NewFigure("ChainMaker", "slant", true)
	s := fig.String()
	fragment := "================================================================================="
	//versionInfo := "::ChainMaker::  version(" + protocol.DefaultBlockVersion + ")"
	// blockchain.CurrentVersion 为对外发布的版本，仅支持三位，如：v2.3.3
	versionInfo := fmt.Sprintf("ChainMaker Version: %s\n", blockchain.CurrentVersion)

	// protocol.DefaultBlockVersion 为当前执行逻辑时的判断版本号，支持四位，如02030300（最前面0可忽略）
	versionInfo += fmt.Sprintf("Block Version:%6s%d\n", " ", protocol.DefaultBlockVersion)

	if blockchain.BuildDateTime != "" {
		versionInfo += fmt.Sprintf("Build Time:%9s%s\n", " ", blockchain.BuildDateTime)
	}

	if blockchain.GitBranch != "" {
		versionInfo += fmt.Sprintf("Git Commit:%9s%s", " ", blockchain.GitBranch)
		if blockchain.GitCommit != "" {
			versionInfo += fmt.Sprintf("(%s)", blockchain.GitCommit)
		}
	}
	return fmt.Sprintf("\n%s\n%s%s\n%s\n", fragment, s, fragment, versionInfo)
}

func PrintVersion() {
	//fmt.Printf("ChainMaker version: %s\n", CurrentVersion)
	fmt.Println(logo())
	fmt.Println()
}

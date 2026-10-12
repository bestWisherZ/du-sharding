/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	"github.com/spf13/cobra"
)

// subscribeCMD subscribe command
// @return *cobra.Command
func subscribeCMD() *cobra.Command {
	contractCmd := &cobra.Command{
		Use:   "subscribe",
		Short: "subscribe command",
		Long:  "subscribe command",
	}
	contractCmd.AddCommand(subscribeBlockCMD())
	contractCmd.AddCommand(subscribeTxCMD())
	contractCmd.AddCommand(subscribeContractEventCMD())

	return contractCmd
}

func subscribeBlockCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block",
		Short: "subscribe command",
		Long:  "subscribe command",
		RunE: func(_ *cobra.Command, _ []string) error {
			return subscribeBlock()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath, flagUserTlsKeyFilePath, flagUserTlsCrtFilePath,
		flagOrgId, flagChainId, flagSendTimes, flagEnableCertHash, flagSdkConfPath, flagPayerKeyFilePath,
		flagAdminCrtFilePaths, flagAdminKeyFilePaths, flagAdminOrgIds,
		flagPayerCrtFilePath, flagPayerOrgId, flagWithRWSet,
		flagStartBlockHeight, flagEndBlockHeight, flagOnlyHeader,
	})
	return cmd
}

func subscribeBlock() error {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = cc.Stop()
	}()
	blockChan, err := cc.SubscribeBlock(ctx, startBlock, endBlock,
		withRWSet, onlyHeader)
	if err != nil {
		fmt.Println("error sendSubscribe :", err)
		return err
	}
	receivedBlockHeight := uint64(0)
	// 接收区块并发送到统计对象
	for {
		select {
		case block, ok := <-blockChan:
			if !ok {
				if endBlock != -1 && receivedBlockHeight >= uint64(endBlock) {
					fmt.Println("received enough block, will exit")
					return nil
				}
				fmt.Println("subscribe interrupt check log please")
				return nil
			}
			if onlyHeader {
				header, ok := block.(*common.BlockHeader)
				if !ok {
					return errors.New("not a blockHeader type")
				}
				receivedBlockHeight = header.BlockHeight
				util.PrintJson(util.FormatHeader(header))
			} else {
				blockInfo, ok := block.(*common.BlockInfo)
				if !ok {
					return errors.New("not a blockInfo type")
				}
				receivedBlockHeight = blockInfo.Block.Header.BlockHeight
				util.PrintJson(util.FormatBlockInfo(blockInfo))
			}
		case <-ctx.Done():
			return nil
		}
	}

}

func subscribeTxCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx",
		Short: "subscribe tx command",
		Long:  "subscribe tx command",
		RunE: func(_ *cobra.Command, _ []string) error {
			return subscribeTx()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath, flagUserTlsKeyFilePath, flagUserTlsCrtFilePath,
		flagOrgId, flagChainId, flagSendTimes, flagEnableCertHash, flagAdminCrtFilePaths, flagAdminKeyFilePaths,
		flagAdminOrgIds, flagSdkConfPath, flagPayerKeyFilePath, flagPayerCrtFilePath,
		flagStartBlockHeight, flagEndBlockHeight, flagTxIds, flagContractName,
	})
	return cmd
}

func subscribeTx() error {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = cc.Stop()
	}()
	txChan, err := cc.SubscribeTx(ctx, startBlock, endBlock,
		contractName, strings.Split(txIds, ","))
	if err != nil {
		fmt.Println("error sendSubscribe :", err)
		return err
	}
	// 接收区块并发送到统计对象
	for {
		select {
		case tx, ok := <-txChan:
			if !ok {
				fmt.Println("subscribe interrupt check log please")
				return nil
			}
			t, ok := tx.(*common.Transaction)
			if !ok {
				return errors.New("not a transaction type")
			}
			util.PrintJson(util.FormatTxs([]*common.Transaction{t}))
		case <-ctx.Done():
			return nil
		}
	}
}

func subscribeContractEventCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "subscribe event command",
		Long:  "subscribe event command",
		RunE: func(_ *cobra.Command, _ []string) error {
			return subscribeEvent()
		},
	}
	util.AttachFlags(cmd, flags, []string{
		flagUserSignKeyFilePath, flagUserSignCrtFilePath, flagUserTlsKeyFilePath, flagUserTlsCrtFilePath,
		flagOrgId, flagChainId, flagSendTimes, flagEnableCertHash, flagSdkConfPath, flagPayerKeyFilePath,
		flagAdminCrtFilePaths, flagAdminKeyFilePaths, flagAdminOrgIds, flagPayerCrtFilePath,
		flagStartBlockHeight, flagEndBlockHeight, flagContractName, flagTopic,
	})
	return cmd
}

func subscribeEvent() error {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = cc.Stop()
	}()
	txChan, err := cc.SubscribeContractEvent(ctx, startBlock, endBlock,
		contractName, topic)
	if err != nil {
		fmt.Println("error sendSubscribe :", err)
		return err
	}
	// 接收区块并发送到统计对象
	for {
		select {
		case tx, ok := <-txChan:
			if !ok {
				fmt.Println("subscribe interrupt check log please")
				return nil
			}
			t, ok := tx.(*common.ContractEventInfo)
			if !ok {
				return errors.New("not a contract event type")
			}
			fmt.Println(t.String())
		case <-ctx.Done():
			return nil
		}
	}
}

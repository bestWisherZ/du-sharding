/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package parallel

import (
	"io/ioutil"

	sdk "chainmaker.org/chainmaker/sdk-go/v2"
)

// initSubClient 初始化订阅节点使用的sdkClient
func initSubClient() error {
	// 读取enc证书
	for i := range encCrtPaths {
		keyBytes, err := ioutil.ReadFile(encKeyPaths[i])
		if err != nil {
			panic(err)
		}
		encKeyBytes = append(encKeyBytes, keyBytes)
		crtBytes, err := ioutil.ReadFile(encCrtPaths[i])
		if err != nil {
			panic(err)
		}
		encCrtBytes = append(encCrtBytes, crtBytes)
	}
	for i := range hosts {
		sdkClient, err := getSdkClient(i)
		if err != nil {
			return err
		}
		defaultSdkClients = append(defaultSdkClients, sdkClient)
	}
	return nil
}

// getSdkClient 用来获取一个SdkClient对象， i为对应node的数组下标
// 初始化sdk配置，与sdk_config.yml对应
func getSdkClient(i int) (*sdk.ChainClient, error) {
	switch sdk.AuthType(authTypeUint32) {
	case sdk.Public:
		return pkClient(i)
	case sdk.PermissionedWithCert:
		return certClient(i)
	default:
		return pwkClient(i)
	}
}

func pkClient(i int) (*sdk.ChainClient, error) {
	nodeConf := newNodeConfig(i)
	opts := make([]sdk.ChainClientOption, 0)
	opts = append(opts, sdk.WithAuthType(sdk.AuthTypeToStringMap[sdk.AuthType(authTypeUint32)]))
	opts = append(opts, sdk.WithChainClientChainId(chainId))
	opts = append(opts, sdk.WithCryptoConfig(sdk.NewCryptoConfig(sdk.WithHashAlgo(hashAlgo))))
	if len(signKeyPaths) == 1 {
		opts = append(opts, sdk.WithUserSignKeyFilePath(signKeyPaths[0]))
	} else {
		opts = append(opts, sdk.WithUserSignKeyFilePath(signKeyPaths[i]))
	}
	// 如果开启tls校验userKey和userCrt
	if useTLS {
		if len(userKeyPaths) == 1 && len(userCrtPaths) == 1 {
			opts = append(opts, sdk.WithUserKeyFilePath(userKeyPaths[0]))
			opts = append(opts, sdk.WithUserCrtFilePath(userCrtPaths[0]))
		} else {
			opts = append(opts, sdk.WithUserKeyFilePath(userKeyPaths[i]))
			opts = append(opts, sdk.WithUserCrtFilePath(userCrtPaths[i]))
		}
	}
	// 如果传入这两个参数则默认开启了双证书模式
	if len(encCrtPaths) > 0 && len(encKeyPaths) > 0 {
		if len(encCrtPaths) == 1 && len(encKeyPaths) == 1 {
			opts = append(opts, sdk.WithUserEncKeyBytes(encKeyBytes[0]))
			opts = append(opts, sdk.WithUserEncCrtBytes(encCrtBytes[0]))
		} else {
			opts = append(opts, sdk.WithUserEncKeyBytes(encKeyBytes[i]))
			opts = append(opts, sdk.WithUserEncCrtBytes(encCrtBytes[i]))
		}

	}
	opts = append(opts, sdk.AddChainClientNodeConfig(nodeConf))
	return sdk.NewChainClient(opts...)
}

func certClient(i int) (*sdk.ChainClient, error) {
	nodeConf := newNodeConfig(i)
	opts := make([]sdk.ChainClientOption, 0)
	opts = append(opts, sdk.WithAuthType(sdk.AuthTypeToStringMap[sdk.AuthType(authTypeUint32)]))
	opts = append(opts, sdk.WithChainClientChainId(chainId))
	opts = append(opts, sdk.WithCryptoConfig(sdk.NewCryptoConfig(sdk.WithHashAlgo(hashAlgo))))
	if len(orgIDs) == 1 {
		opts = append(opts, sdk.WithChainClientOrgId(orgIDs[0]))
	} else {
		opts = append(opts, sdk.WithChainClientOrgId(orgIDs[i]))
	}
	if len(signCrtPaths) == 1 && len(signKeyPaths) == 1 {
		opts = append(opts, sdk.WithUserSignCrtFilePath(signCrtPaths[0]))
		opts = append(opts, sdk.WithUserSignKeyFilePath(signKeyPaths[0]))
	} else {
		opts = append(opts, sdk.WithUserSignCrtFilePath(signCrtPaths[i]))
		opts = append(opts, sdk.WithUserSignKeyFilePath(signKeyPaths[i]))
	}
	// 如果开启tls校验userKey和userCrt
	if useTLS {
		if len(userKeyPaths) == 1 && len(userCrtPaths) == 1 {
			opts = append(opts, sdk.WithUserKeyFilePath(userKeyPaths[0]))
			opts = append(opts, sdk.WithUserCrtFilePath(userCrtPaths[0]))
		} else {
			opts = append(opts, sdk.WithUserKeyFilePath(userKeyPaths[i]))
			opts = append(opts, sdk.WithUserCrtFilePath(userCrtPaths[i]))
		}
	}
	// 如果传入这两个参数则默认开启了双证书模式
	if len(encCrtPaths) > 0 && len(encKeyPaths) > 0 {
		if len(encCrtPaths) == 1 && len(encKeyPaths) == 1 {
			opts = append(opts, sdk.WithUserEncKeyBytes(encKeyBytes[0]))
			opts = append(opts, sdk.WithUserEncCrtBytes(encCrtBytes[0]))
		} else {
			opts = append(opts, sdk.WithUserEncKeyBytes(encKeyBytes[i]))
			opts = append(opts, sdk.WithUserEncCrtBytes(encCrtBytes[i]))
		}

	}
	opts = append(opts, sdk.AddChainClientNodeConfig(nodeConf))
	return sdk.NewChainClient(opts...)
}

func pwkClient(i int) (*sdk.ChainClient, error) {
	nodeConf := newNodeConfig(i)
	opts := make([]sdk.ChainClientOption, 0)
	opts = append(opts, sdk.WithAuthType(sdk.AuthTypeToStringMap[sdk.AuthType(authTypeUint32)]))
	opts = append(opts, sdk.WithChainClientChainId(chainId))
	opts = append(opts, sdk.WithUserSignKeyFilePath(signKeyPaths[i]))
	opts = append(opts, sdk.WithCryptoConfig(sdk.NewCryptoConfig(sdk.WithHashAlgo(hashAlgo))))
	opts = append(opts, sdk.WithChainClientOrgId(orgIDs[i]))
	opts = append(opts, sdk.AddChainClientNodeConfig(nodeConf))
	return sdk.NewChainClient(opts...)
}

func newNodeConfig(i int) *sdk.NodeConfig {
	var nodeConf *sdk.NodeConfig
	if useTLS {
		var tlsHost string
		if len(hostnames) == 1 {
			tlsHost = hostnames[0]
		} else {
			tlsHost = hostnames[i]
		}
		nodeConf = sdk.NewNodeConfig(
			// 节点地址，格式：127.0.0.1:12301
			sdk.WithNodeAddr(hosts[i]),
			// 节点连接数
			sdk.WithNodeConnCnt(10),
			// 节点是否启用TLS认证
			sdk.WithNodeUseTLS(useTLS),
			// 根证书路径，支持多个
			sdk.WithNodeCAPaths(caPaths),
			// TLS Hostname
			sdk.WithNodeTLSHostName(tlsHost),
		)
	} else {
		nodeConf = sdk.NewNodeConfig(
			// 节点地址，格式：127.0.0.1:12301
			sdk.WithNodeAddr(hosts[i]),
			// 节点连接数
			sdk.WithNodeConnCnt(10),
			// 节点是否启用TLS认证
			sdk.WithNodeUseTLS(useTLS),
		)
	}
	return nodeConf
}

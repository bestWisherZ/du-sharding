package gas

import (
	"errors"
	"fmt"
	"io/ioutil"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/common/v2/random/uuid"
	acPb "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
	sdk "chainmaker.org/chainmaker/sdk-go/v2"
	sdkutils "chainmaker.org/chainmaker/sdk-go/v2/utils"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
)

func setContractMethodPayerCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-payer contract_name [method] payer_address",
		Short: "set gas payer for contract method",
		RunE: func(_ *cobra.Command, args []string) error {
			var (
				err error
			)

			// 建立链接
			client, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer client.Stop()

			payerKeyPem, err := ioutil.ReadFile(payerKeyFilePath)
			if err != nil {
				return err
			}
			payerCertPem, err := ioutil.ReadFile(payerCrtFilePath)
			if err != nil {
				return err
			}
			privateKey, err := asym.PrivateKeyFromPEM(payerKeyPem, nil)
			if err != nil {
				return err
			}

			// 构建参数
			params := syscontract.SetContractMethodPayerParams{}
			if len(args) == 3 {
				params.ContractName = args[0]
				params.Method = args[1]
				params.PayerAddress = args[2]
			} else if len(args) == 2 {
				params.ContractName = args[0]
				params.PayerAddress = args[1]
			} else {
				return errors.New("command syntax error")
			}
			params.RequestId += uuid.GetUUID()
			message, err := proto.Marshal(&params)
			if err != nil {
				return fmt.Errorf("marshal params failed, err = %v", err)
			}

			signature, err := sdkutils.SignPayloadBytesWithHashType(
				privateKey,
				client.GetHashType(),
				[]byte(message))
			if err != nil {
				return err
			}

			memberInfo := payerCertPem
			var memberType acPb.MemberType
			if client.GetAuthType() == sdk.PermissionedWithCert {
				memberType = acPb.MemberType_CERT
			} else if client.GetAuthType() == sdk.PermissionedWithKey {
				memberType = acPb.MemberType_PUBLIC_KEY
			} else if client.GetAuthType() == sdk.Public {
				memberType = acPb.MemberType_PUBLIC_KEY
			}
			endorsement := sdkutils.NewEndorserWithMemberType(payerOrgId, memberInfo, memberType, signature)
			endorsementBytes, err := proto.Marshal(endorsement)
			if err != nil {
				return err
			}

			var parameters []*common.KeyValuePair
			parameters = append(parameters, &common.KeyValuePair{
				Key:   syscontract.SetContractMethodPayer_PARAMS.String(),
				Value: []byte(message),
			})
			parameters = append(parameters, &common.KeyValuePair{
				Key:   syscontract.SetContractMethodPayer_ENDORSEMENT.String(),
				Value: endorsementBytes,
			})

			// 构建 payload
			var payload *common.Payload
			if multiSign {
				parameters = append(parameters, &common.KeyValuePair{
					Key:   "SYS_CONTRACT_NAME",
					Value: []byte(syscontract.SystemContract_ACCOUNT_MANAGER.String()),
				})
				parameters = append(parameters, &common.KeyValuePair{
					Key:   "SYS_METHOD",
					Value: []byte(syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String()),
				})
				payload = client.CreatePayload(
					"", common.TxType_INVOKE_CONTRACT,
					syscontract.SystemContract_MULTI_SIGN.String(),
					syscontract.MultiSignFunction_REQ.String(),
					parameters,
					0, &common.Limit{GasLimit: gasLimit})
			} else {
				payload = client.CreatePayload(
					"", common.TxType_INVOKE_CONTRACT,
					syscontract.SystemContract_ACCOUNT_MANAGER.String(),
					syscontract.GasAccountFunction_SET_CONTRACT_METHOD_PAYER.String(),
					parameters,
					0, &common.Limit{GasLimit: gasLimit})
			}

			// 产生 Request
			request, err := client.GenerateTxRequest(payload, nil)
			if err != nil {
				return err
			}
			fmt.Printf("request = %v \n", request.String())

			// 发送 Request, 读取 Response
			resp, err := client.SendTxRequest(request, -1, true)
			if err != nil {
				return err
			}

			fmt.Printf("resp: %+v\n", resp)
			return nil
		},
	}

	util.AttachFlags(cmd, flags, []string{
		flagPayerKeyFilePath, flagPayerCrtFilePath, flagPayerOrgId,
		flagSdkConfPath, flagGasLimit, flagMultiSign,
	})

	cmd.MarkFlagRequired(flagPayerKeyFilePath)
	cmd.MarkFlagRequired(flagPayerCrtFilePath)
	cmd.MarkFlagRequired(flagPayerOrgId)

	return cmd
}

func unsetContractMethodPayerCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset-payer contract_name [method]",
		Short: "clear the gas payer setting for contract's method",
		RunE: func(_ *cobra.Command, args []string) error {
			var (
				err error
			)

			// 建立链接
			client, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer client.Stop()

			// 构建参数
			var params []*common.KeyValuePair
			if len(args) == 2 {
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.UnsetContractMethodPayer_CONTRACT_NAME.String(),
					Value: []byte(args[0]),
				})
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.UnsetContractMethodPayer_METHOD.String(),
					Value: []byte(args[1]),
				})
			} else if len(args) == 1 {
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.UnsetContractMethodPayer_CONTRACT_NAME.String(),
					Value: []byte(args[0]),
				})
			} else {
				return errors.New("command syntax error")
			}

			// 构建 payload
			var payload *common.Payload
			if multiSign {
				payload = client.CreatePayload(
					"", common.TxType_INVOKE_CONTRACT,
					syscontract.SystemContract_MULTI_SIGN.String(),
					syscontract.MultiSignFunction_REQ.String(),
					params,
					0, nil)
			} else {
				payload = client.CreatePayload(
					"", common.TxType_INVOKE_CONTRACT,
					syscontract.SystemContract_ACCOUNT_MANAGER.String(),
					syscontract.GasAccountFunction_UNSET_CONTRACT_METHOD_PAYER.String(),
					params,
					0, &common.Limit{GasLimit: gasLimit})
			}

			// 产生 Request
			request, err := client.GenerateTxRequest(payload, nil)
			if err != nil {
				return err
			}

			// 发送 Request, 读取 Response
			resp, err := client.SendTxRequest(request, -1, true)
			if err != nil {
				return err
			}

			fmt.Printf("resp: %+v\n", resp)
			return nil
		},
	}

	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
	})

	return cmd
}

func queryContractMethodPayerCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query-method-payer contract_name [method]",
		Short: "query the payer setting for the contract's method",
		RunE: func(_ *cobra.Command, args []string) error {
			var (
				err error
			)

			// 获取 client
			client, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer client.Stop()

			// 构建参数
			var params []*common.KeyValuePair
			if len(args) == 1 {
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.GetContractMethodPayer_CONTRACT_NAME.String(),
					Value: []byte(args[0]),
				})
			} else if len(args) == 2 {
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.GetContractMethodPayer_CONTRACT_NAME.String(),
					Value: []byte(args[0]),
				})
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.GetContractMethodPayer_METHOD.String(),
					Value: []byte(args[1]),
				})
			} else {
				return errors.New("command syntax error")
			}

			// 构建 payload
			payload := client.CreatePayload(
				"", common.TxType_INVOKE_CONTRACT,
				syscontract.SystemContract_ACCOUNT_MANAGER.String(),
				syscontract.GasAccountFunction_GET_CONTRACT_METHOD_PAYER.String(),
				params,
				0, &common.Limit{GasLimit: gasLimit})

			// 产生 Request
			request, err := client.GenerateTxRequest(payload, nil)
			if err != nil {
				return err
			}

			// 发送 Request, 读取 Response
			resp, err := client.SendTxRequest(request, -1, true)
			if err != nil {
				return err
			}

			fmt.Printf("resp: %+v\n", resp)

			return nil
		},
	}

	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath,
	})

	return cmd
}

func queryTxPayerCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query-tx-payer txId",
		Short: "query the payer address of the tx",
		RunE: func(_ *cobra.Command, args []string) error {
			var (
				err error
			)

			client, err := util.CreateChainClientWithConfPath(sdkConfPath, false)
			if err != nil {
				return err
			}
			defer client.Stop()

			var params []*common.KeyValuePair
			if len(args) == 1 {
				params = append(params, &common.KeyValuePair{
					Key:   syscontract.GetTxPayer_TX_ID.String(),
					Value: []byte(args[0]),
				})
			} else {
				errors.New("command syntax error")
			}

			// 构建 payload
			payload := client.CreatePayload(
				"", common.TxType_INVOKE_CONTRACT,
				syscontract.SystemContract_ACCOUNT_MANAGER.String(),
				syscontract.GasAccountFunction_GET_TX_PAYER.String(),
				params,
				0, &common.Limit{GasLimit: gasLimit})

			// 产生 Request
			request, err := client.GenerateTxRequest(payload, nil)
			if err != nil {
				return err
			}

			// 发送 Request, 读取 Response
			resp, err := client.SendTxRequest(request, -1, true)
			if err != nil {
				return err
			}

			fmt.Printf("resp: %+v\n", resp)

			return nil
		},
	}

	util.AttachFlags(cmd, flags, []string{
		flagSdkConfPath, flagSyncResult,
	})

	return cmd
}

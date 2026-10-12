package rpcserver

import (
	commonErr "chainmaker.org/chainmaker/common/v2/errors"
	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/syscontract"
)

func (s *ApiService) nodeConfig(tx *commonPb.Transaction) *commonPb.TxResponse {
	if tx.Payload.TxType != commonPb.TxType_NODE_CONFIG {
		return &commonPb.TxResponse{
			Code:    commonPb.TxStatusCode_INTERNAL_ERROR,
			Message: commonErr.ERR_CODE_TXTYPE.String(),
			TxId:    tx.Payload.TxId,
		}
	}

	switch tx.Payload.Method {

	case syscontract.ChainsConfigFunction_UPDATE.String():
		return s.updateChainsConfig(tx)

	default:
		return &commonPb.TxResponse{
			Code:    commonPb.TxStatusCode_INTERNAL_ERROR,
			Message: commonErr.ERR_CODE_TXTYPE.String(),
		}
	}
}

func (s *ApiService) updateChainsConfig(tx *commonPb.Transaction) *commonPb.TxResponse {
	var (
		resp = &commonPb.TxResponse{TxId: tx.Payload.TxId}
	)

	if err := localconf.CheckNewCmBlockChainConfig(); err != nil {

		resp.Code = commonPb.TxStatusCode_INTERNAL_ERROR
		resp.Message = err.Error()
		return resp
	}
	resp.Code = commonPb.TxStatusCode_SUCCESS
	resp.Message = commonPb.TxStatusCode_SUCCESS.String()
	return resp
}

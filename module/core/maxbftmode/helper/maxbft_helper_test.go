package helper

import (
	"testing"

	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/pb-go/v2/config"
	consensusPb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

func TestDiscardBlocks(t *testing.T) {

	ctrl := gomock.NewController(t)

	mockTxPool := mock.NewMockTxPool(ctrl)

	mockChainConf := mock.NewMockChainConf(ctrl)
	mockChainConf.EXPECT().ChainConfig().Return(&config.ChainConfig{
		ChainId: "chain1",
		Consensus: &config.ConsensusConfig{
			Type: consensusPb.ConsensusType_MAXBFT,
		},
	})
	mockProposalCache := mock.NewMockProposalCache(ctrl)
	mockLogger := mock.NewMockLogger(ctrl)

	height := uint64(10)
	delBlocks := make([]*commonPb.Block, 2)
	tx1 := &commonPb.Transaction{Payload: &commonPb.Payload{TxId: "tx1"}}
	tx2 := &commonPb.Transaction{Payload: &commonPb.Payload{TxId: "tx2"}}
	tx3 := &commonPb.Transaction{Payload: &commonPb.Payload{TxId: "tx3"}}
	//tx4 := &commonPb.Transaction{Payload:   &commonPb.Payload{TxId: "tx4"}}

	delBlocks[0] = &commonPb.Block{
		Txs:            []*commonPb.Transaction{tx1, tx2},
		AdditionalData: nil, // normal tx pool 不需要
	}

	delBlocks[1] = &commonPb.Block{
		Txs:            []*commonPb.Transaction{tx3, tx2},
		AdditionalData: nil, // normal tx pool 不需要
	}

	mockProposalCache.EXPECT().DiscardBlocks(height).Return(delBlocks)

	mockTxPool.EXPECT().RetryAndRemoveTxs(gomock.Any(), gomock.Any())

	helperInstance := NewMaxbftHelper(mockTxPool, mockChainConf, mockProposalCache, mockLogger)

	helperInstance.DiscardBlocks(height)
}

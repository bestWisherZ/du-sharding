package blockchain

import (
	common "chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"errors"
	"github.com/golang/mock/gomock"
	"testing"
)

func TestSyncResultRequiresSuccessfulExecution(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tx            *common.Transaction
		err           error
		want, wantErr bool
	}{
		{name: "absent"},
		{name: "store error", err: errors.New("unavailable"), wantErr: true},
		{name: "missing result", tx: &common.Transaction{}, wantErr: true},
		{name: "failed transaction", tx: &common.Transaction{Result: &common.Result{Code: common.TxStatusCode_CONTRACT_FAIL}}, wantErr: true},
		{name: "failed contract", tx: &common.Transaction{Result: &common.Result{ContractResult: &common.ContractResult{Code: 1}}}, wantErr: true},
		{name: "success", tx: &common.Transaction{Result: &common.Result{ContractResult: &common.ContractResult{}}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mock.NewMockBlockchainStore(gomock.NewController(t))
			store.EXPECT().GetTx("sync-tx").Return(tc.tx, tc.err)
			server := &ChainMakerServer{}
			server.blockchains.Store("chain1", &Blockchain{store: store})
			got, err := server.IsTxSuccessfullyCommitted("chain1", "sync-tx")
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}

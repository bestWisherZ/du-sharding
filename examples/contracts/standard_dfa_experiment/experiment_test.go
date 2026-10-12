package main

import (
	"testing"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
	"chainmaker.org/chainmaker/contract-utils/safemath"
	"github.com/golang/mock/gomock"
)

func TestTransferExperimentUsesDisjointLazyBalances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := map[string]string{
		experimentInitialBalanceKey + "#": "1000000000",
	}
	mock := sdk.NewMockSDKInterface(ctrl)
	mock.EXPECT().GetState(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(key, field string) (string, error) { return state[key+"#"+field], nil })
	mock.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(key, field, value string) error {
			state[key+"#"+field] = value
			return nil
		})
	mock.EXPECT().EmitEvent("transfer", gomock.Any()).Times(1)
	sdk.Instance = mock

	from := "1111111111111111111111111111111111111111"
	to := "2222222222222222222222222222222222222222"
	contract := NewCmdfaContract()
	if err := contract.TransferExperiment(from, to, safemath.NewSafeUint256(1)); err != nil {
		t.Fatalf("TransferExperiment() error = %v", err)
	}
	if got := state[balanceKey+"#"+from]; got != "999999999" {
		t.Fatalf("source balance = %s, want 999999999", got)
	}
	if got := state[balanceKey+"#"+to]; got != "1" {
		t.Fatalf("destination balance = %s, want 1", got)
	}
	if _, ok := state[totalSupplyKey+"#"]; ok {
		t.Fatal("TransferExperiment must not write the shared totalSupply key")
	}
}

func TestStandardTransferStillRequiresFundedSender(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := sdk.NewMockSDKInterface(ctrl)
	mock.EXPECT().Sender().Return("1111111111111111111111111111111111111111", nil)
	mock.EXPECT().GetState(gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)
	sdk.Instance = mock

	contract := NewCmdfaContract()
	err := contract.Transfer("2222222222222222222222222222222222222222", safemath.NewSafeUint256(1))
	if err == nil {
		t.Fatal("standard Transfer unexpectedly accepted an unfunded sender")
	}
}

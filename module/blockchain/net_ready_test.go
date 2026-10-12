package blockchain

import (
	"errors"
	"testing"

	"chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

func TestNetReadyHandlerRunsBeforeConsensus(t *testing.T) {
	for _, tc := range []struct {
		name       string
		networkErr error
		handlerErr error
	}{
		{name: "ready"},
		{name: "network failure", networkErr: errors.New("network failed")},
		{name: "subscription failure", handlerErr: errors.New("subscribe failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			network := mock.NewMockNetService(ctrl)
			consensus := mock.NewMockConsensusEngine(ctrl)
			bc := &Blockchain{log: log, chainId: "chain1", netService: network, consensus: consensus,
				initModules: map[string]struct{}{moduleNameNetService: {}, moduleNameConsensus: {}}, startModules: map[string]struct{}{}}
			server := &ChainMakerServer{}
			server.blockchains.Store(bc.chainId, bc)
			stage := 0
			network.EXPECT().Start().DoAndReturn(func() error { stage = 1; return tc.networkErr })
			if err := server.RegisterNetServiceReadyHandler(bc.chainId, func() error {
				if stage != 1 {
					t.Fatal("subscription ran before network startup")
				}
				stage = 2
				return tc.handlerErr
			}); err != nil {
				t.Fatal(err)
			}
			if tc.networkErr == nil && tc.handlerErr == nil {
				consensus.EXPECT().Start().DoAndReturn(func() error {
					if stage != 2 {
						t.Fatal("consensus started before subscriptions")
					}
					stage = 3
					return nil
				})
			}
			err := bc.Start()
			wantErr := tc.networkErr != nil || tc.handlerErr != nil
			if (err != nil) != wantErr {
				t.Fatalf("Start: %v, want error %v", err, wantErr)
			}
			wantStage := 3
			if tc.networkErr != nil {
				wantStage = 1
			} else if tc.handlerErr != nil {
				wantStage = 2
			}
			if stage != wantStage {
				t.Fatalf("stage=%d, want %d", stage, wantStage)
			}
			if err := server.RegisterNetServiceReadyHandler(bc.chainId, func() error { return nil }); err == nil {
				t.Fatal("duplicate startup handler accepted")
			}
		})
	}
}

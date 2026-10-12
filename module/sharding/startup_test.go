package sharding

import (
	"errors"
	"testing"

	netpb "chainmaker.org/chainmaker/pb-go/v2/net"
	"chainmaker.org/chainmaker/protocol/v2/mock"
	"github.com/golang/mock/gomock"
)

func TestScheduleSubscriptionErrorsReachStartup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		network := mock.NewMockNetService(gomock.NewController(t))
		var err error
		if fail {
			err = errors.New("pub-sub not ready")
		}
		network.EXPECT().Subscribe(netpb.NetMsg_CONSENSUS_MSG, gomock.Any()).Return(err)
		if got := subscribeScheduleSignals([]chainNetService{{chainId: "chain1", netService: network}}, &scheduleSignalRelay{}); (got != nil) != fail {
			t.Fatalf("subscribe: %v, want error %v", got, fail)
		}
	}
}

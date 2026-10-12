package syncauth_test

import (
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	commonpb "chainmaker.org/chainmaker/pb-go/v2/common"
	"github.com/gogo/protobuf/proto"
)

func TestAttestationBindsDomainTargetAndEveryPayloadField(t *testing.T) {
	id := testidentity.New(t)
	cfg := id.Config()
	p := &commonpb.Payload{ChainId: "chain1", TxType: commonpb.TxType_INVOKE_CONTRACT, TxId: "test-id", Method: "sync", ContractName: "token",
		Parameters: []*commonpb.KeyValuePair{{Key: "writes", Value: []byte(" original value ")}}}
	auth, err := syncauth.Sign(cfg, "bridge", "business-shard1", p, id.Certificate, id.Key)
	if err != nil {
		t.Fatal(err)
	}
	p.Parameters = append(p.Parameters, &commonpb.KeyValuePair{Key: syncauth.Parameter, Value: auth})
	if err := syncauth.Verify(cfg, "bridge", "business-shard1", p); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*commonpb.Payload){
		func(p *commonpb.Payload) { p.Parameters[0].Value = []byte("other") },
		func(p *commonpb.Payload) { p.Method = "other" },
		func(p *commonpb.Payload) { p.ContractName = "CHAIN_CONFIG" },
		func(p *commonpb.Payload) { p.ChainId = "chain2" },
		func(p *commonpb.Payload) {
			p.Parameters = append(p.Parameters, &commonpb.KeyValuePair{Key: "writes", Value: []byte("duplicate")})
		},
	} {
		copy := proto.Clone(p).(*commonpb.Payload)
		mutate(copy)
		if err := syncauth.Verify(cfg, "bridge", "business-shard1", copy); err == nil {
			t.Fatal("tampered payload accepted")
		}
	}
	cfg.NetworkID = "other-network"
	if err := syncauth.Verify(cfg, "bridge", "business-shard1", p); err == nil {
		t.Fatal("cross-network replay accepted")
	}
	cfg = id.Config()
	cfg.Signers["bridge"] = []string{testidentity.New(t).Pin}
	if err := syncauth.Verify(cfg, "bridge", "business-shard1", p); err == nil {
		t.Fatal("untrusted signer accepted")
	}
}

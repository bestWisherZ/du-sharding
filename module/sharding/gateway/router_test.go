package gateway

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"chainmaker.org/chainmaker-go/module/sharding/internal/testidentity"
	"chainmaker.org/chainmaker/common/v2/crypto"
	ac "chainmaker.org/chainmaker/pb-go/v2/accesscontrol"
	api "chainmaker.org/chainmaker/pb-go/v2/api"
	common "chainmaker.org/chainmaker/pb-go/v2/common"
	configpb "chainmaker.org/chainmaker/pb-go/v2/config"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
)

func routeTestConfig() *GatewayConfig {
	return &GatewayConfig{ListenAddress: "127.0.0.1:14301", DefaultBackend: "business-shard1", BridgeBackend: "bridge",
		BusinessShards: []string{"business-shard1", "business-shard2"}, AddressType: "CHAINMAKER", HashType: "SHA256",
		Backends: map[string]*BackendConfig{"bridge": {Address: "127.0.0.1:1", ChainID: "chain0"},
			"business-shard1": {Address: "127.0.0.1:2", ChainID: "chain1"}, "business-shard2": {Address: "127.0.0.1:3", ChainID: "chain2"}},
		RoutingRules: []RoutingRule{{ContractName: "dfa", Method: "Transfer", IncludeSender: true, AccountParameters: []string{"to"}},
			{ContractName: "custom", Method: "move", AccountParameters: []string{"src", "dst"}}}}
}

func TestStandardTransferRoutesUsingOriginalSender(t *testing.T) {
	cfg := routeTestConfig()
	id := testidentity.New(t)
	member := &ac.Member{MemberType: ac.MemberType_CERT, MemberInfo: id.Certificate}
	address, err := utils.GetStrAddrFromPbMember(member, configpb.AddrType_CHAINMAKER, crypto.HASH_TYPE_SHA256)
	if err != nil {
		t.Fatal(err)
	}
	req := &common.TxRequest{Payload: &common.Payload{TxType: common.TxType_INVOKE_CONTRACT, ContractName: "dfa", Method: "Transfer",
		Parameters: []*common.KeyValuePair{{Key: "to", Value: []byte(address)}, {Key: "amount", Value: []byte("1")}}}, Sender: &common.EndorsementEntry{Signer: member}}
	want := shardForAccount(cfg.BusinessShards, address)
	if got, _, err := ResolveRoute(cfg, req); err != nil || got != want {
		t.Fatalf("route=%s want=%s err=%v", got, want, err)
	}
	for n := 0; ; n++ {
		other := fmt.Sprintf("other-%d", n)
		if shardForAccount(cfg.BusinessShards, other) == want {
			continue
		}
		req.Payload.Parameters[0].Value = []byte(other)
		break
	}
	if got, _, err := ResolveRoute(cfg, req); err != nil || got != "bridge" {
		t.Fatalf("cross route=%s err=%v", got, err)
	}
	req.Sender = nil
	if _, _, err := ResolveRoute(cfg, req); err == nil {
		t.Fatal("missing Sender was ignored")
	}
}

type captureClient struct {
	api.RpcNodeClient
	called int
	data   []byte
}

func (c *captureClient) SendRequest(_ context.Context, r *common.TxRequest, _ ...grpc.CallOption) (*common.TxResponse, error) {
	c.called++
	c.data, _ = proto.Marshal(r)
	return &common.TxResponse{}, nil
}
func (c *captureClient) SendRequestSync(ctx context.Context, r *common.TxRequest, opts ...grpc.CallOption) (*common.TxResponse, error) {
	return c.SendRequest(ctx, r, opts...)
}

func TestForwardingPreservesSignedRequestBytes(t *testing.T) {
	cfg := routeTestConfig()
	req := &common.TxRequest{Payload: &common.Payload{TxType: common.TxType_INVOKE_CONTRACT, ContractName: "custom", Method: "move", TxId: "tx-original",
		Parameters: []*common.KeyValuePair{{Key: "src", Value: []byte(" alice ")}, {Key: "dst", Value: []byte(" alice ")},
			{Key: "method", Value: []byte("  business-value  ")}, {Key: "binary", Value: []byte{0, 255}}}},
		Sender:    &common.EndorsementEntry{Signer: &ac.Member{OrgId: "caller", MemberInfo: []byte("original certificate")}, Signature: []byte("signature")},
		Endorsers: []*common.EndorsementEntry{{Signature: []byte("endorsement")}}, Payer: &common.EndorsementEntry{Signature: []byte("payer")}}
	backend, chain, err := ResolveRoute(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Payload.ChainId = chain
	want, _ := proto.Marshal(req)
	client := &captureClient{}
	s := &Service{cfg: cfg, backends: &backendManager{clients: map[string]api.RpcNodeClient{backend: client}}}
	for _, send := range []func(context.Context, *common.TxRequest) (*common.TxResponse, error){s.SendRequest, s.SendRequestSync} {
		if _, err := send(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		after, _ := proto.Marshal(req)
		if !bytes.Equal(want, client.data) || !bytes.Equal(want, after) {
			t.Fatal("gateway changed signed request")
		}
	}
	req.Payload.ChainId = "incorrect-chain"
	if _, err := s.SendRequest(context.Background(), req); err == nil || client.called != 2 {
		t.Fatal("wrong chain was forwarded or re-signed")
	}
}

func TestRoutingFailsClosed(t *testing.T) {
	cfg := routeTestConfig()
	p := &common.Payload{TxType: common.TxType_INVOKE_CONTRACT, ContractName: "unknown", Method: "move"}
	r := &common.TxRequest{Payload: p}
	if _, _, err := ResolveRoute(cfg, r); err == nil {
		t.Fatal("unconfigured contract accepted")
	}
	p.ContractName = "custom"
	if _, _, err := ResolveRoute(cfg, r); err == nil {
		t.Fatal("missing routing accounts accepted")
	}
	p.Parameters = []*common.KeyValuePair{{Key: "src", Value: []byte("x")}, {Key: "src", Value: []byte("x")}, {Key: "dst", Value: []byte("x")}}
	if _, _, err := ResolveRoute(cfg, r); err == nil {
		t.Fatal("ambiguous parameters accepted")
	}
	p.ContractName = "CONTRACT_MANAGE"
	p.ChainId = "chain2"
	if got, _, err := ResolveRoute(cfg, r); err != nil || got != "business-shard2" {
		t.Fatalf("system request ignored signed chain: %s %v", got, err)
	}
}

func TestGatewayConfigRejectsOldProxyCredentials(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gateway.yml")
	if err := os.WriteFile(p, []byte("user_config_path: /old/private/credentials.yml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("legacy signing proxy was silently accepted")
	}
	tls := TLSConfig{CertFile: "cert.pem", KeyFile: "/absolute/key.pem"}
	resolveTLSPaths(filepath.Dir(p), &tls)
	if tls.CertFile != filepath.Join(filepath.Dir(p), "cert.pem") || tls.KeyFile != "/absolute/key.pem" {
		t.Fatal("TLS path resolution changed absolute paths")
	}
}

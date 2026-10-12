package gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
	configPb "chainmaker.org/chainmaker/pb-go/v2/config"
	consensusPb "chainmaker.org/chainmaker/pb-go/v2/consensus"
	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
	txpoolPb "chainmaker.org/chainmaker/pb-go/v2/txpool"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Service struct {
	apiPb.UnimplementedRpcNodeServer
	cfg      *GatewayConfig
	backends *backendManager
}

func NewService(cfg *GatewayConfig) (*Service, error) {
	backends, err := newBackendManager(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:      cfg,
		backends: backends,
	}, nil
}

func (s *Service) Close() {
	if s.backends != nil {
		s.backends.close()
	}
}

func (s *Service) SendRequest(ctx context.Context, req *commonPb.TxRequest) (*commonPb.TxResponse, error) {
	backend, err := selectBackendByTx(s.cfg, req)
	if err != nil {
		return nil, err
	}
	if err := s.validateBackendChainID(backend, req); err != nil {
		return nil, err
	}
	client, err := s.client(backend)
	if err != nil {
		return nil, err
	}
	log.Printf("[gateway] SendRequest route backend=%s tx=%s contract=%s method=%s type=%s",
		backend, getTxID(req), getContract(req), getMethod(req), getTxTypeName(req))
	return client.SendRequest(ctx, req)
}

func (s *Service) SendRequestSync(ctx context.Context, req *commonPb.TxRequest) (*commonPb.TxResponse, error) {
	backend, err := selectBackendByTx(s.cfg, req)
	if err != nil {
		return nil, err
	}
	if err := s.validateBackendChainID(backend, req); err != nil {
		return nil, err
	}
	client, err := s.client(backend)
	if err != nil {
		return nil, err
	}
	log.Printf("[gateway] SendRequestSync route backend=%s tx=%s contract=%s method=%s type=%s",
		backend, getTxID(req), getContract(req), getMethod(req), getTxTypeName(req))
	return client.SendRequestSync(ctx, req)
}

func (s *Service) Subscribe(req *commonPb.TxRequest, srv apiPb.RpcNode_SubscribeServer) error {
	backend, err := selectBackendByTx(s.cfg, req)
	if err != nil {
		return err
	}
	if err := s.validateBackendChainID(backend, req); err != nil {
		return err
	}
	client, err := s.client(backend)
	if err != nil {
		return err
	}
	upstream, err := client.Subscribe(srv.Context(), req)
	if err != nil {
		return err
	}
	for {
		msg, err := upstream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := srv.Send(msg); err != nil {
			return err
		}
	}
}

func (s *Service) SubscribeWS(req *commonPb.RawTxRequest, srv apiPb.RpcNode_SubscribeWSServer) error {
	backend := strings.TrimSpace(s.cfg.DefaultBackend)
	client, err := s.client(backend)
	if err != nil {
		return err
	}
	upstream, err := client.SubscribeWS(srv.Context(), req)
	if err != nil {
		return err
	}
	for {
		msg, err := upstream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := srv.Send(msg); err != nil {
			return err
		}
	}
}

func (s *Service) UpdateDebugConfig(ctx context.Context, req *configPb.DebugConfigRequest) (*configPb.DebugConfigResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.UpdateDebugConfig(ctx, req)
}

func (s *Service) RefreshLogLevelsConfig(ctx context.Context, req *configPb.LogLevelsRequest) (*configPb.LogLevelsResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.RefreshLogLevelsConfig(ctx, req)
}

func (s *Service) GetChainMakerVersion(ctx context.Context, req *configPb.ChainMakerVersionRequest) (*configPb.ChainMakerVersionResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetChainMakerVersion(ctx, req)
}

func (s *Service) CheckNewBlockChainConfig(ctx context.Context, req *configPb.CheckNewBlockChainConfigRequest) (*configPb.CheckNewBlockChainConfigResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.CheckNewBlockChainConfig(ctx, req)
}

func (s *Service) GetPoolStatus(ctx context.Context, req *txpoolPb.GetPoolStatusRequest) (*txpoolPb.TxPoolStatus, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetPoolStatus(ctx, req)
}

func (s *Service) GetTxIdsByTypeAndStage(ctx context.Context, req *txpoolPb.GetTxIdsByTypeAndStageRequest) (*txpoolPb.GetTxIdsByTypeAndStageResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetTxIdsByTypeAndStage(ctx, req)
}

func (s *Service) GetTxsInPoolByTxIds(ctx context.Context, req *txpoolPb.GetTxsInPoolByTxIdsRequest) (*txpoolPb.GetTxsInPoolByTxIdsResponse, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetTxsInPoolByTxIds(ctx, req)
}

func (s *Service) GetConsensusStateJSON(ctx context.Context, req *consensusPb.GetConsensusStatusRequest) (*wrapperspb.BytesValue, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetConsensusStateJSON(ctx, req)
}

func (s *Service) GetConsensusValidators(ctx context.Context, req *consensusPb.GetConsensusStatusRequest) (*consensusPb.Validators, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetConsensusValidators(ctx, req)
}

func (s *Service) GetConsensusHeight(ctx context.Context, req *consensusPb.GetConsensusStatusRequest) (*wrapperspb.UInt64Value, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetConsensusHeight(ctx, req)
}

func (s *Service) GetSyncState(ctx context.Context, req *syncPb.GetSyncStateRequest) (*syncPb.SyncState, error) {
	client, err := s.defaultClient()
	if err != nil {
		return nil, err
	}
	return client.GetSyncState(ctx, req)
}

func (s *Service) validateBackendChainID(backend string, req *commonPb.TxRequest) error {
	if s == nil || s.cfg == nil || req == nil || req.Payload == nil {
		return fmt.Errorf("gateway or transaction is not initialized")
	}
	bc := s.cfg.Backends[strings.TrimSpace(backend)]
	if bc == nil {
		return fmt.Errorf("backend not found: %s", backend)
	}
	expected := strings.TrimSpace(bc.ChainID)
	if expected == "" {
		return fmt.Errorf("backend chain id is empty: %s", backend)
	}
	actual := strings.TrimSpace(req.Payload.ChainId)
	if actual == "" {
		return fmt.Errorf("request chain id is empty, backend=%s expected=%s", backend, expected)
	}
	if actual != expected {
		return fmt.Errorf("route requires backend=%s chain_id=%s; request chain_id=%s: sign the request for the target chain; gateway never re-signs", backend, expected, actual)
	}
	return nil
}

func (s *Service) client(name string) (apiPb.RpcNodeClient, error) {
	if s.backends == nil {
		return nil, fmt.Errorf("backend manager is nil")
	}
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		if c, ok := s.backends.getClient(trimmed); ok {
			return c, nil
		}
	}
	return nil, fmt.Errorf("backend not found: %s", name)
}

func (s *Service) defaultClient() (apiPb.RpcNodeClient, error) {
	if s.backends == nil || s.cfg == nil {
		return nil, fmt.Errorf("gateway not initialized")
	}
	if c, ok := s.backends.getClient(strings.TrimSpace(s.cfg.DefaultBackend)); ok {
		return c, nil
	}
	return nil, fmt.Errorf("default backend not found: %s", s.cfg.DefaultBackend)
}

func getTxID(req *commonPb.TxRequest) string {
	if req == nil || req.Payload == nil {
		return ""
	}
	return req.Payload.TxId
}

func getContract(req *commonPb.TxRequest) string {
	if req == nil || req.Payload == nil {
		return ""
	}
	return req.Payload.ContractName
}

func getMethod(req *commonPb.TxRequest) string {
	if req == nil || req.Payload == nil {
		return ""
	}
	return req.Payload.Method
}

func getTxTypeName(req *commonPb.TxRequest) string {
	if req == nil || req.Payload == nil {
		return "unknown"
	}
	name, ok := commonPb.TxType_name[int32(req.Payload.TxType)]
	if !ok {
		return "unknown"
	}
	return name
}

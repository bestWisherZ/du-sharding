package gateway

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"

	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Server struct {
	cfg     *GatewayConfig
	service *Service
	grpcSrv *grpc.Server
	lis     net.Listener
}

func NewServer(cfg *GatewayConfig) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gateway config is nil")
	}
	service, err := NewService(cfg)
	if err != nil {
		return nil, err
	}
	opts := make([]grpc.ServerOption, 0, 1)
	if cfg.ListenTLS.Enable {
		creds, err := buildServerTLSCreds(cfg.ListenTLS)
		if err != nil {
			service.Close()
			return nil, err
		}
		opts = append(opts, grpc.Creds(creds))
	}
	grpcSrv := grpc.NewServer(opts...)
	apiPb.RegisterRpcNodeServer(grpcSrv, service)
	return &Server{
		cfg:     cfg,
		service: service,
		grpcSrv: grpcSrv,
	}, nil
}

func (s *Server) Start() error {
	if s == nil || s.grpcSrv == nil || s.cfg == nil {
		return fmt.Errorf("gateway server not initialized")
	}
	lis, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return err
	}
	s.lis = lis
	log.Printf("[gateway] listening at %s", s.cfg.ListenAddress)
	return s.grpcSrv.Serve(lis)
}

func (s *Server) Stop() {
	if s == nil {
		return
	}
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
	if s.service != nil {
		s.service.Close()
	}
	if s.lis != nil {
		_ = s.lis.Close()
	}
}

func buildServerTLSCreds(cfg TLSConfig) (credentials.TransportCredentials, error) {
	if strings.TrimSpace(cfg.CertFile) == "" || strings.TrimSpace(cfg.KeyFile) == "" {
		return nil, fmt.Errorf("listen tls cert_file/key_file is empty")
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if strings.TrimSpace(cfg.CaFile) != "" {
		caPem, err := ioutil.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPem) {
			return nil, fmt.Errorf("append server ca cert failed: %s", cfg.CaFile)
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(tlsCfg), nil
}

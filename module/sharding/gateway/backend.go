package gateway

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	apiPb "chainmaker.org/chainmaker/pb-go/v2/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type backendManager struct {
	mu      sync.RWMutex
	clients map[string]apiPb.RpcNodeClient
	conns   map[string]*grpc.ClientConn
}

func newBackendManager(cfg *GatewayConfig) (*backendManager, error) {
	mgr := &backendManager{
		clients: make(map[string]apiPb.RpcNodeClient),
		conns:   make(map[string]*grpc.ClientConn),
	}
	for name, bc := range cfg.Backends {
		if bc == nil {
			continue
		}
		conn, err := dialBackend(bc)
		if err != nil {
			mgr.close()
			return nil, fmt.Errorf("dial backend[%s] failed: %v", name, err)
		}
		mgr.clients[name] = apiPb.NewRpcNodeClient(conn)
		mgr.conns[name] = conn
	}
	return mgr, nil
}

func (m *backendManager) getClient(name string) (apiPb.RpcNodeClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[name]
	return c, ok
}

func (m *backendManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, conn := range m.conns {
		_ = conn.Close()
		delete(m.conns, name)
		delete(m.clients, name)
	}
}

func dialBackend(cfg *BackendConfig) (*grpc.ClientConn, error) {
	if cfg == nil {
		return nil, fmt.Errorf("backend config is nil")
	}
	addr := strings.TrimSpace(cfg.Address)
	if addr == "" {
		return nil, fmt.Errorf("backend address is empty")
	}
	var (
		conn *grpc.ClientConn
		err  error
	)
	if cfg.TLS.Enable {
		creds, err := buildClientTLSCreds(cfg.TLS)
		if err != nil {
			return nil, err
		}
		conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	} else {
		conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func buildClientTLSCreds(cfg TLSConfig) (credentials.TransportCredentials, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if strings.TrimSpace(cfg.ServerName) != "" {
		tlsCfg.ServerName = strings.TrimSpace(cfg.ServerName)
	}
	if strings.TrimSpace(cfg.CaFile) != "" {
		caPem, err := ioutil.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPem) {
			return nil, fmt.Errorf("append ca cert failed: %s", cfg.CaFile)
		}
		tlsCfg.RootCAs = pool
	}
	if strings.TrimSpace(cfg.CertFile) != "" && strings.TrimSpace(cfg.KeyFile) != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsCfg), nil
}

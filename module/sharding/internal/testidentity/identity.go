package testidentity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	cmcrypto "chainmaker.org/chainmaker/common/v2/crypto"
	"chainmaker.org/chainmaker/common/v2/crypto/asym"
	"chainmaker.org/chainmaker/utils/v2"
)

type Identity struct {
	Certificate []byte
	Key         cmcrypto.PrivateKey
	Pin         string
}

func New(t testing.TB) Identity {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sync-test"},
		NotBefore: time.Unix(0, 0), NotAfter: time.Unix(4102444800, 0), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	sk, err := asym.PrivateKeyFromPEM(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}), nil)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := utils.ParseCert(crt)
	if err != nil {
		t.Fatal(err)
	}
	return Identity{crt, sk, syncauth.CertificateID(parsed)}
}

func (i Identity) Config() syncauth.Config {
	return syncauth.Config{NetworkID: "test-network", ChainIDs: map[string]string{"bridge": "chain0", "business-shard1": "chain1", "business-shard2": "chain2"},
		Signers: map[string][]string{"bridge": {i.Pin}, "business-shard1": {i.Pin}, "business-shard2": {i.Pin}}}
}

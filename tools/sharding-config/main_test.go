package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
)

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func fixtureTopology(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Unix(0, 0), NotAfter: time.Unix(4102444800, 0)}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	for i, shard := range []string{"bridge-shard", "business-shard1", "business-shard2"} {
		role := "BUSINESS"
		if i == 0 {
			role = "BRIDGE"
		}
		node := filepath.Join(root, shard, "config", "node1")
		doc := fmt.Sprintf("shard_id: %s\nshard_type: %s\nschedule:\n  business_shards: [business-shard1, business-shard2]\n  business_batch: 6\n  bridge_batch: 4\n  cross_write_routing:\n    strategy: none\n    rules: []\nsync_network:\n  enable: true\n  port: 23401\n", shard, role)
		writeFixture(t, filepath.Join(node, "sharding.yml"), []byte(doc))
		writeFixture(t, filepath.Join(node, "chainconfig", "bc1.yml"), []byte(fmt.Sprintf("chain_id: chain%d\n", i)))
		writeFixture(t, filepath.Join(root, shard, "crypto-config", "org1", "node", "consensus1", "consensus1.sign.crt"), crt)
	}
	return root
}

func TestGeneratePublicSyncTrust(t *testing.T) {
	root := fixtureTopology(t)
	path := filepath.Join(root, "bridge-shard", "config", "node1", "sharding.yml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(root, "policy.yml")
	writeFixture(t, policy, []byte("cross_write_routing:\n  strategy: none\n  rules:\n    - contract_name: token\n      strategy: key_hash\n"))
	if err = run(options{build: root, network: "fixture", policy: policy}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("dry run modified files")
	}
	if err = run(options{build: root, network: "fixture", policy: policy, write: true}); err != nil {
		t.Fatal(err)
	}
	v, err := readYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg shardingsync.ShardingConfig
	if err = v.Unmarshal(&cfg); err != nil {
		t.Fatal(err)
	}
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.SyncAuth.Signers) != 3 || len(cfg.Schedule.CrossWriteRouting.Rules) != 1 {
		t.Fatalf("incomplete config: %+v", cfg)
	}
	after, _ = os.ReadFile(path)
	if err = run(options{build: root, write: true}); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if !bytes.Equal(after, again) {
		t.Fatal("repeated generation changed deployment identity")
	}
	if err = run(options{build: root, network: "another-network", write: true}); err == nil {
		t.Fatal("existing deployment ID replaced")
	}
}

func TestMissingStatePolicyDoesNotWriteConfigurations(t *testing.T) {
	root := fixtureTopology(t)
	path := filepath.Join(root, "bridge-shard", "config", "node1", "sharding.yml")
	before, _ := os.ReadFile(path)
	err := run(options{build: root, write: true})
	if err == nil || !strings.Contains(err.Error(), "no state ownership rules") {
		t.Fatalf("empty state policy accepted: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid state policy modified configurations")
	}
}

func TestCheckStatePolicyWithoutGeneratedNodes(t *testing.T) {
	for _, tc := range []struct {
		name, data, wantError string
	}{
		{"valid", "cross_write_routing:\n  rules:\n    - contract_name: new_business\n      strategy: key_hash\n", ""},
		{"empty", "cross_write_routing:\n  strategy: none\n  rules: []\n", "no state ownership rules"},
		{"wrong field", "cross_write_routing:\n  rules:\n    - contract_name: new_business\n      stratgey: key_hash\n", "stratgey"},
		{"unknown shard", "cross_write_routing:\n  rules:\n    - contract_name: new_business\n      target_shard: business-shard3\n", "known target_shard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "policy.yml")
			writeFixture(t, path, []byte(tc.data))
			build := filepath.Join(root, "build-not-created")
			err := run(options{build: build, policy: path, checkPolicy: true, businessShards: "business-shard1,business-shard2"})
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("want %q, got %v", tc.wantError, err)
			}
			if _, err := os.Stat(build); !os.IsNotExist(err) {
				t.Fatal("policy check touched the build directory")
			}
		})
	}
}

func TestDefaultStatePolicy(t *testing.T) {
	path := filepath.Join("..", "..", "config", "sharding", "state_routing.yml")
	if err := run(options{policy: path, checkPolicy: true, businessShards: "business-shard1,business-shard2"}); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareScriptChecksPolicyBeforeCleaning(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "prepare_sharding.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	scriptPath := filepath.Join(root, "scripts", "prepare_sharding.sh")
	writeFixture(t, scriptPath, script)
	sentinel := filepath.Join(root, "build", "keep-existing-ledger")
	writeFixture(t, sentinel, []byte("must remain"))
	writeFixture(t, filepath.Join(root, "config", "sharding", "state_routing.yml"), []byte("invalid policy"))
	writeFixture(t, filepath.Join(root, "config", "config_tpl", "chainconfig", "bc_4_7.tpl"),
		[]byte("# trust roots list start\n# trust roots list end\n"))
	crypto := filepath.Join(root, "tools", "chainmaker-cryptogen")
	writeFixture(t, filepath.Join(crypto, "config", "crypto_config_template.yml"), []byte("{}"))
	cryptogen := filepath.Join(crypto, "bin", "chainmaker-cryptogen")
	writeFixture(t, cryptogen, []byte("#!/bin/sh\nexit 99\n"))
	if err = os.Chmod(cryptogen, 0700); err != nil {
		t.Fatal(err)
	}
	goPath := filepath.Join(root, "bin", "go")
	writeFixture(t, goPath, []byte("#!/bin/sh\ncase \"$*\" in\n  *--check-policy*) echo policy-check-rejected; exit 17;;\n  *) exit 99;;\nesac\n"))
	if err = os.Chmod(goPath, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + filepath.Join(root, "bin") + ":/usr/bin:/bin", "HOME=" + root}
	cmd.Stdin = strings.NewReader(strings.Repeat("\n", 30))
	output, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 17 || !bytes.Contains(output, []byte("policy-check-rejected")) {
		t.Fatalf("prepare did not stop at policy validation: %v\n%s", err, output)
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "must remain" {
		t.Fatalf("invalid policy removed existing build data: %v", err)
	}
}

func TestInvalidTopologyDoesNotWritePartialConfigurations(t *testing.T) {
	root := fixtureTopology(t)
	path := filepath.Join(root, "bridge-shard", "config", "node1", "sharding.yml")
	before, _ := os.ReadFile(path)
	writeFixture(t, filepath.Join(root, "business-shard2", "config", "node1", "chainconfig", "bc1.yml"), []byte("chain_id: chain1\n"))
	if err := run(options{build: root, write: true}); err == nil {
		t.Fatal("duplicate chain IDs accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("partially updated invalid topology")
	}
}

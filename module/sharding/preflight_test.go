package sharding

import (
	"os"
	"path/filepath"
	"testing"

	"chainmaker.org/chainmaker/localconf/v2"
)

func TestPreflightMissingVersusInvalidConfig(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	if err = Preflight(&localconf.CMConfig{}); err != nil {
		t.Fatal("ordinary single chain should not require sharding config", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "sharding.yml"), []byte("shard_id: business-shard1\nshard_type: BUSINESS\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Preflight(&localconf.CMConfig{}); err == nil {
		t.Fatal("incomplete sharding config silently disabled enforcement")
	}
}

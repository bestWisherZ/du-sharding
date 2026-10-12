// sharding-config installs public source-certificate pins into generated node
// configurations. It never reads private keys, changes ledgers or starts nodes.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chainmaker.org/chainmaker-go/module/sharding/shardingsync"
	"chainmaker.org/chainmaker-go/module/sharding/syncauth"
	"chainmaker.org/chainmaker/utils/v2"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type options struct {
	build, network, policy string
	write                  bool
	checkPolicy            bool
	businessShards         string
}

func main() {
	var o options
	flag.StringVar(&o.build, "build-dir", "build", "generated sharding build directory (not release)")
	flag.StringVar(&o.network, "network-id", "", "new deployment ID; existing IDs cannot be overwritten")
	flag.StringVar(&o.policy, "policy", "", "YAML state policy; required if generated configs have no ownership rules")
	flag.BoolVar(&o.write, "write", false, "write validated configurations; default is dry run")
	flag.BoolVar(&o.checkPolicy, "check-policy", false, "validate only the state policy, without reading or writing build files")
	flag.StringVar(&o.businessShards, "business-shards", "", "comma-separated shard IDs for --check-policy")
	flag.Parse()
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type document struct {
	path     string
	settings map[string]interface{}
	cfg      shardingsync.ShardingConfig
}

func readYAML(path string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

func run(o options) error {
	if o.checkPolicy {
		if o.policy == "" || o.businessShards == "" || o.write {
			return fmt.Errorf("--check-policy requires --policy and --business-shards, and cannot be combined with --write")
		}
		_, routing, err := readStatePolicy(o.policy)
		if err != nil {
			return err
		}
		if err = routing.Validate(strings.Split(o.businessShards, ",")); err != nil {
			return fmt.Errorf("%s: %w", o.policy, err)
		}
		fmt.Printf("state routing policy validated: %s rules=%d\n", o.policy, len(routing.Rules))
		return nil
	}
	paths, err := filepath.Glob(filepath.Join(o.build, "*", "config", "node*", "sharding.yml"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no generated node sharding.yml under %s", o.build)
	}
	auth := syncauth.Config{NetworkID: o.network, ChainIDs: map[string]string{}, Signers: map[string][]string{}}
	docs := make([]document, 0, len(paths))
	var businessOrder []byte
	for _, path := range paths {
		v, err := readYAML(path)
		if err != nil {
			return err
		}
		d := document{path: path, settings: v.AllSettings()}
		if err = v.Unmarshal(&d.cfg); err != nil {
			return err
		}
		if prior := d.cfg.SyncAuth.NetworkID; prior != "" {
			if auth.NetworkID != "" && auth.NetworkID != prior {
				return fmt.Errorf("refusing to change deployment ID in %s", path)
			}
			auth.NetworkID = prior
		}
		order, _ := yaml.Marshal(d.cfg.Schedule.BusinessShards)
		if businessOrder != nil && !bytes.Equal(order, businessOrder) {
			return fmt.Errorf("business shard ordering differs in %s", path)
		}
		businessOrder = order
		genesis, err := filepath.Glob(filepath.Join(filepath.Dir(path), "chainconfig", "*.yml"))
		if err != nil {
			return err
		}
		if len(genesis) != 1 {
			return fmt.Errorf("expected one genesis per node in %s", path)
		}
		bc, err := readYAML(genesis[0])
		if err != nil {
			return err
		}
		shard := syncauth.NormalizeShard(d.cfg.ShardId)
		chain := bc.GetString("chain_id")
		if chain == "" {
			return fmt.Errorf("empty chain ID in %s", genesis[0])
		}
		if prior := auth.ChainIDs[shard]; prior != "" && prior != chain {
			return fmt.Errorf("inconsistent chain ID for %s", shard)
		}
		auth.ChainIDs[shard] = chain
		if len(auth.Signers[shard]) == 0 {
			root := filepath.Dir(filepath.Dir(filepath.Dir(path)))
			certs, err := filepath.Glob(filepath.Join(root, "crypto-config", "*", "node", "*", "*.sign.crt"))
			if err != nil {
				return err
			}
			seen := map[string]bool{}
			for _, file := range certs {
				raw, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				cert, err := utils.ParseCert(raw)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				pin := syncauth.CertificateID(cert)
				if !seen[pin] {
					auth.Signers[shard] = append(auth.Signers[shard], pin)
					seen[pin] = true
				}
			}
			sort.Strings(auth.Signers[shard])
		}
		docs = append(docs, d)
	}
	if auth.NetworkID == "" {
		raw := make([]byte, 16)
		if _, err = rand.Read(raw); err != nil {
			return err
		}
		auth.NetworkID = hex.EncodeToString(raw)
	}
	var policy interface{}
	if o.policy != "" {
		settings, _, err := readStatePolicy(o.policy)
		if err != nil {
			return err
		}
		policy = settings
	}
	outputs := make(map[string][]byte, len(docs))
	var commonSchedule []byte
	for _, d := range docs {
		d.settings["sync_auth"] = auth
		if policy != nil {
			schedule, ok := d.settings["schedule"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid schedule in %s", d.path)
			}
			schedule["cross_write_routing"] = policy
		}
		raw, err := yaml.Marshal(d.settings)
		if err != nil {
			return err
		}
		v := viper.New()
		v.SetConfigType("yaml")
		if err = v.ReadConfig(bytes.NewReader(raw)); err != nil {
			return err
		}
		var cfg shardingsync.ShardingConfig
		if err = v.Unmarshal(&cfg); err != nil {
			return err
		}
		if err = cfg.Validate(); err != nil {
			return fmt.Errorf("%s: %w", d.path, err)
		}
		schedule, _ := yaml.Marshal(d.settings["schedule"])
		if commonSchedule != nil && !bytes.Equal(schedule, commonSchedule) {
			return fmt.Errorf("schedule or state policy differs in %s", d.path)
		}
		commonSchedule = schedule
		outputs[d.path] = raw
	}
	if o.write {
		// Validate the whole topology before replacing any configuration.
		for _, path := range paths {
			if err := os.WriteFile(path+".sync-auth.tmp", outputs[path], 0644); err != nil {
				return err
			}
			if err := os.Rename(path+".sync-auth.tmp", path); err != nil {
				return err
			}
		}
	}
	fmt.Printf("sharding configuration: nodes=%d shards=%d network_id=%s write=%t\n", len(paths), len(auth.ChainIDs), auth.NetworkID, o.write)
	return nil
}

func readStatePolicy(path string) (map[string]interface{}, shardingsync.CrossWriteRoutingConfig, error) {
	var doc struct {
		Routing shardingsync.CrossWriteRoutingConfig `mapstructure:"cross_write_routing"`
	}
	v, err := readYAML(path)
	if err != nil {
		return nil, doc.Routing, err
	}
	if !v.IsSet("cross_write_routing") {
		return nil, doc.Routing, fmt.Errorf("%s must contain cross_write_routing", path)
	}
	if err = v.UnmarshalExact(&doc); err != nil {
		return nil, doc.Routing, fmt.Errorf("%s: %w", path, err)
	}
	return v.GetStringMap("cross_write_routing"), doc.Routing, nil
}

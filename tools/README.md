# Development Tools

The sharding preparation scripts require ChainMaker Cryptogen v2.4.0. Install
the tool separately; machine-local symlinks and generated identities are not
part of this repository.

Run from the repository root:

```bash
git clone --branch v2.4.0 --depth 1 https://git.chainmaker.org.cn/chainmaker/chainmaker-cryptogen.git tools/chainmaker-cryptogen
make -C tools/chainmaker-cryptogen
```

The preparation scripts use these paths:

- `tools/chainmaker-cryptogen/bin/chainmaker-cryptogen`
- `tools/chainmaker-cryptogen/config/crypto_config_template.yml`

`cmc/` contains the chain management CLI. `sharding-config/` validates and
installs the common state routing policy and synchronization trust settings
into generated node configurations.

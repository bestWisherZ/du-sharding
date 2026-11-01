# Du-Sharding：分时共识分片系统

本项目基于长安链 `v2.4.0_alpha`实现分时共识分片系统。系统由多个业务分片与一个桥接分片组成，各分片独立运行 TBFT 共识。

## 仓库内容

| 路径 | 内容 |
| --- | --- |
| `main/`、`module/` | 完整节点源码及分片扩展 |
| `module/sharding/gateway/` | 可配置交易路由与网关 |
| `module/sharding/scheduler/` | 阶段调度 |
| `module/sharding/globalstate/` | 业务区块写集同步与桥接状态镜像 |
| `module/sharding/crossshard/` | 桥接写集回写与系统同步交易执行 |
| `module/sharding/syncproposer/` | 当前 proposer 构造本地同步交易 |
| `config/config_tpl/` | 节点与创世配置模板 |
| `scripts/` | 配置生成、构建打包、启动、停止和清理脚本 |
| `tools/cmc/` | 长安链命令行客户端 |
| `test/testdata/sharding/` | 推进出块、心跳合约源码 |
| `examples/contracts/standard_dfa_experiment/` | 实验 DFA 转账合约源码 |


## 环境要求

- Go `1.19.x`，本项目验证使用 `1.19.9`。
- Git、Make、GCC、GNU Bash、GNU sed、tar、7-Zip。
- Docker，以及当前用户操作 Docker 的权限。
- 证书生成工具 `chainmaker-cryptogen v2.4.0`。

Docker Go VM 使用 `chainmakerofficial/chainmaker-vm-engine:v2.4.0`。自带辅助合约和实验 DFA 合约依赖 `contract-sdk-go v2.3.9`。

源码构建需要 Go。已经构建完成的节点可以在兼容的 Linux 环境中直接运行，无需再安装 Go，但仍需要节点包内的运行库、Docker 和 `7z`。

## 获取源码与依赖

```bash
git clone https://github.com/bestWisherZ/du-sharding.git
cd du-sharding
go mod download
git clone --branch v2.4.0 --depth 1 \
  https://git.chainmaker.org.cn/chainmaker/chainmaker-cryptogen.git \
  tools/chainmaker-cryptogen
make -C tools/chainmaker-cryptogen
docker pull chainmakerofficial/chainmaker-vm-engine:v2.4.0
```

准备脚本会检查以下文件；不能仅复制一个空的 `tools/chainmaker-cryptogen` 目录：

```text
tools/chainmaker-cryptogen/bin/chainmaker-cryptogen
tools/chainmaker-cryptogen/config/crypto_config_template.yml
tools/chainmaker-cryptogen/config/hsm_keys.yml
```

## 单机启动一个分片网络

以下示例为 **2 个业务分片 + 1 个桥接分片，各 4 个共识节点**，共 12 个节点。多机部署需先阅读下一节。

### 1. 配置交易路由与状态归属

网关示例对应上述两业务分片的本地部署；更多分片需要修改后端列表和端口：

```bash
cp module/sharding/gateway/gateway.example.yml \
  module/sharding/gateway/gateway.yml
```

两个配置入口分别控制不同的事情：

- `module/sharding/gateway/gateway.yml` 的 `routing_rules`：决定某合约方法按哪些业务参数选择目标分片。
- `config/sharding/state_routing.yml` 的 `cross_write_routing`：决定合约真实状态 key 的归属。

所有节点必须使用相同的状态规则与业务分片顺序。新增合约需要同时检查这两层配置；规则变更后须重启相关程序。网关不加载用户签名私钥，不代签或修改已经签名的交易。

可以先只读检查状态策略：

```bash
go run -mod=readonly ./tools/sharding-config --check-policy \
  --policy ./config/sharding/state_routing.yml \
  --business-shards business-shard1,business-shard2
```

### 2. 生成配置和证书

```bash
cd scripts
bash prepare_sharding.sh
```

按提示填写：

| 参数 | 本示例填写值 |
| --- | --- |
| 业务分片数量 | `2` |
| 每个业务分片节点数 | `4` |
| 桥接分片节点数 | `4` |
| 分时调度起始高度 | `10` |
| 业务阶段连续出块数量 | `6` |
| 桥接阶段连续出块数量 | `4` |
| 动态调度总批次数 | `0`，即固定轮数 |
| 共识类型 | `1`，即 TBFT |
| 日志级别 | `INFO` |
| 启用 Go VM | `YES` |
| Go VM 日志级别 | `INFO` |
| 启用 Java VM | `NO` |

脚本自身的默认业务/桥接轮数为 `3/2`，需要按表格输入 `6/4`，不要直接全部回车。轮数表示出块配额，不代表两阶段实际耗时比例。

输出位于 `build/business-shard1/`、`build/business-shard2/`、`build/bridge-shard/`。脚本同时安装状态策略和同步来源公开证书信任表。

**注意：`prepare_sharding.sh` 会清理旧的 `build/`。只在新实验目录中运行，不能用于保留已有账本的升级。**

### 3. 构建与打包

保持当前目录为 `scripts/`：

```bash
bash build_release_sharding.sh
```

脚本构建 `bin/chainmaker`、`bin/chainmaker-gateway`，并将节点包输出到 `build/release/<分片名>/`。每个节点包包含节点程序、运行库、启动脚本和该节点配置。

### 4. 启动

仍在 `scripts/` 下执行：

```bash
bash cluster_quick_start_sharding.sh normal
```

`normal` 为后台启动模式，会解压节点包、启动节点和相应 VM，并启动网关。不要省略该参数：无参数模式会使用 tmux。

本示例节点 RPC 端口为业务分片 1 的 `12301-12304`、业务分片 2 的 `12305-12308`、桥接分片的 `12309-12312`。网关示例监听 `127.0.0.1:14301`，只面向本机；对外提供服务前应配置访问控制和 TLS。

节点系统日志位于：

```text
build/release/<分片名>/chainmaker-*/log/system.log
build/release/<分片名>/chainmaker-*/log/<组织名>/go/contract-engine.log
```

`build/gateway.log` 为后台网关日志。

## 多服务器部署

1. 只在配置生成机器执行一次 `prepare_sharding.sh`。所有机器必须使用同一批证书、分片顺序、状态策略和部署 ID，不能各自重新生成一套身份。
2. 在打包前修改 `build/<分片名>/config/node*/chainmaker.yml` 的 P2P 地址、seeds、RPC/VM 端口，以及 `sharding.yml` 的同步网络地址和 seeds。跨机地址不能保留 `127.0.0.1`。
3. 在每台机器核对端口无冲突并安装 Docker、`7z`。网关后端地址必须与节点实际 RPC 地址对应，CA 路径需指向同批证书。
4. 将生成的节点包分发到目标机器的 `build/release/<分片名>/` 下，每台机器只放置分配给它的节点包，再运行该机器的启动脚本。
5. 若手动启动某个节点，进入其 `bin/` 目录执行 `bash start.sh full`；停止时在同一目录执行 `bash stop.sh full`。

同一业务分片的 4 个共识节点宜分散在不同机器上。桥接分片的 4 个节点和负载发送程序可独立部署。需要统一代码时，各机器从本仓库 `main` 分支获取同一提交；运行二进制必须由该提交构建。

## 合约准备与业务调用

**启动节点不会自动部署合约，也不会自动把所有链推进到高度 10。**

1. 在业务分片和桥接分片部署对应业务合约。
2. 若采用系统的自动心跳推进方式，在各分片部署 `sharding_heartbeat`；`poke_block` 可用于准备阶段主动推进区块。
3. 使用 SDK 或 CMC 将各分片准备至配置的起始高度。准备交易不属于正式压测负载。
4. 在目标链准备好身份、权限及合约后发送业务交易。网关调用方先解析目标 `chain_id`，再签名并发送；网关不会改写不匹配的 `chain_id`。

自带辅助合约需要从源码静态编译并打包，例如在仓库根目录：

```bash
cd test/testdata/sharding/poke_block
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o poke_block .
7z a poke_block.7z poke_block
```

`sharding_heartbeat` 和 `examples/contracts/standard_dfa_experiment` 的构建方式相同，输出文件名分别为 `sharding_heartbeat` 和 `standard_dfa_experiment`。在节点中部署时，运行时类型选择 `DOCKER_GO`。

CMC 可以从仓库根目录构建：

```bash
go build -mod=readonly -o bin/cmc ./tools/cmc
bin/cmc client contract user create --help
bin/cmc client contract user invoke --help
```

CMC/SDK 的用户私钥、证书与信任根由部署时生成，须按实际链 ID、组织和节点地址建立本地 SDK 配置。本仓库不携带某台实验机器的用户配置，也不内置 SSH 密码。

实验 DFA 的入口为 `TransferExperiment(from,to,amount)`：未写入状态的转出账户使用部署参数 `experimentInitialBalance`，未写入状态的转入账户余额为 0。各分片须使用相同的初始余额配置，运行期间不得改变该值。该入口用于构造独立账户负载，不等同于未经修改的标准 DFA 授权/发行语义；是否无冲突仍取决于发送端是否为每笔交易分配互不重复的账户。

## 停止与清理

从仓库根目录进入 `scripts/`：

```bash
cd scripts
bash cluster_quick_stop_sharding.sh
```

停止不会默认清空账本。若需要全新实验，先确认节点和 VM 均已停止，再检查清理范围：

```bash
DRY_RUN=true bash clean_sharding_runtime.sh
bash clean_sharding_runtime.sh
```

清理脚本删除 `build/release/` 中节点的 `data/` 和 `log/`，保留程序、配置与证书。Docker 挂载目录可能需要 sudo 权限。不要在仍有相关节点运行时清理，也不要对共享服务器执行无筛选的 `pkill` 或 `docker prune`。

## 构建与测试

在仓库根目录：

```bash
go build -mod=readonly -o bin/chainmaker ./main
go build -mod=readonly -o bin/chainmaker-gateway ./module/sharding/gateway/cmd
go build -mod=readonly -o bin/sharding-config ./tools/sharding-config
go test -mod=readonly ./module/sharding/... ./tools/sharding-config
go test -mod=readonly ./module/core/syncmode/proposer ./module/snapshot
go test -mod=readonly ./module/core/common/scheduler -run '^TestBridge'
```

这里只列出分片扩展及相关模块的测试。原生长安链部分集成测试依赖另外的测试证书、账本或已启动网络；未随本仓库发布私钥的测试场景需单独生成测试身份，不能把上述模块通过等同于所有集成测试通过。

## 更新代码

```bash
git pull --ff-only origin main
```

更新后重新构建节点程序，再按同一提交同步各服务器。修改同步执行逻辑或协议版本后，应使用新实验账本，不混跑新旧节点。

## 许可证

本项目基于长安链源代码开发，保留其版权声明、[NOTICE](NOTICE) 和 [Apache License 2.0](LICENSE)。

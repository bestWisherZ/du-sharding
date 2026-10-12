# 分时共识与分片模块

系统通过业务分片和桥接分片交替共识处理交易，使用系统状态同步交易传递已提交写集。
业务合约不需要实现专用的状态回写接口。具体执行约束见
[合约兼容性与配置](CONTRACT_COMPATIBILITY.md)。

## 运行模块

| 目录或文件 | 职责 |
|------------|------|
| `preflight.go` | 启动前校验拓扑、身份、状态策略，并初始化执行策略 |
| `timesharing.go` | 接入链服务、调度信号与分片间通信 |
| `gateway/` | 按配置选择交易目标，转发调用者签名的原始请求 |
| `scheduler/` | 分时调度、出块许可与 TBFT BlockInfo 门控 |
| `shardingsync/` | 分片同步网络及节点配置 |
| `globalstate/` | 业务区块写集同步、桥接分片状态镜像和检查点 |
| `crossshard/` | 桥接分片写集回写、系统同步交易执行与状态归属检查 |
| `syncauth/` | 同步来源签名和可信证书校验 |
| `syncproposer/` | 仅由当前 proposer 构造本地同步交易 |
| `heartbeat/` | 通过心跳交易推进允许出块的空闲分片 |

## 配置入口

1. **交易分发**：`module/sharding/gateway/gateway.yml` 的 `routing_rules`。
   此文件保留当前部署的后端地址和证书路径，换机器时需按实际部署调整。
   `gateway.example.yml` 提供独立的两分片格式示例。
2. **状态归属**：`config/sharding/state_routing.yml`。
   配置合约状态 key 的哈希或固定归属，所有节点使用相同规则和分片顺序。
3. **节点运行**：生成的各节点 `sharding.yml`。
   包含分片角色、轮数、同步网络、公开信任表及状态策略。

`scripts/prepare_sharding.sh` 默认加载上述状态策略，在清理旧 `build` 前校验，
生成节点后将策略安装到所有 `sharding.yml`。可用 `SHARDING_STATE_POLICY` 指定另一个文件。
缺少文件、空策略、规则错误或未知目标分片会报错，不再生成没有业务状态归属的配置。
准备脚本仍会清理旧 `build`，不要用于保留账本的运行目录。

当前状态策略包含 `token_business` 的账户 key、标准 DFA 的 `b#账户`，
以及实验 DFA 的余额和只读初始余额配置。标准 DFA 的其他状态固定归桥接分片。
这不是任意合约自动分片：新增合约时，交易参数路由和真实状态布局必须对应。
实验初始余额需在各分片部署为相同值，运行期间不得通过合约升级修改。

## 合约与测试

推进出块和心跳合约位于 `test/testdata/sharding/`，使用 contract-sdk-go v2.3.9。
业务压测继续使用 `experiment/32-shards` 的准备、数据集和发送脚本及独立合约包。

在仓库根目录执行：

```bash
go build -mod=readonly -o bin/chainmaker ./main
go build -mod=readonly -o bin/chainmaker-gateway ./module/sharding/gateway/cmd
go test -mod=readonly ./module/sharding/... ./tools/sharding-config
```

网关读取 `--conf` 指定的文件；配置修改后需要重启网关。
调用方先依据路由取得目标 `chain_id`，再签名发送；网关不代签或修改业务参数。

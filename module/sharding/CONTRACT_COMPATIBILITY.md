# 合约兼容性与配置

## 合约调用与执行

业务合约不需要增加 `sharding_apply` 接口，状态同步不改变存储引擎。
系统仍通过交易模拟上下文形成读写集，再走长安链的验证、共识和提交路径。
带来源证明的同步协议不兼容未携带来源证明的旧协议，节点须使用同一协议版本。

### 原始调用身份

- 网关不加载用户签名私钥，不使用固定 SDK client 代签。
- 转发原始 `TxRequest`，不修改 Sender、Endorsers、Payer、参数、方法或 chain_id。
- 参数字节和业务 `method` 参数保持原样。
- 通过 `routing_rules` 指定合约、方法、账户参数和是否包含真实 Sender。
- `ResolveRoute` 可在签名前返回目标 backend 和 chain_id。调用方随后签名并发送；
  已签名的 chain_id 不匹配时明确拒绝，不偷偷重签。
- 依赖 Sender 的哈希路由需要完整证书或公钥；CERT_HASH/ALIAS 不能在网关离线还原身份。
  目标链还必须认可原始调用方的身份与权限。TLS 客户端证书不等于交易 Sender。

这保留标准 DFA 的 `Transfer` / `Approve` / `TransferFrom` 调用身份，
但不代表它们的所有状态布局都已自动适配分片。

### 交易路由配置

网关负责按配置分发交易，不执行合约业务逻辑。运行时读取 `--conf` 指定的 YAML 文件，
通常为 `module/sharding/gateway/gateway.yml`；
`gateway.example.yml` 是配置样例，不会自动加载。业务规则集中在该文件的 `routing_rules` 中。

每条规则以 `contract_name` 和 `method` 匹配合约调用，名称区分大小写。
精确方法优先于同一合约的 `method: "*"`，不依赖两条规则的排列顺序。
没有匹配规则时拒绝业务请求，不隐式发送给 `default_backend`。
长安链系统合约调用仍按照请求中已签名的 `chain_id` 转发。

支持的路由策略：

| 策略 | 配置 | 行为 |
|------|------|------|
| `hash` | `account_parameters`、可选 `include_sender` | 对参与路由的标识分别计算所属分片，全部同片则发往该业务分片，否则发往 `bridge_backend` |
| `fixed` | `target_backend` | 将该合约方法固定发送到指定后端，可以是业务分片或桥接分片 |

`account_parameters` 指交易参数名，可以是账户、订单号或其他业务标识，
不会限定为 `from/to`；哈希输入使用参数原始字节，不做大小写转换或去空格。
`include_sender: true` 将真实调用方地址也纳入归属判断，不是改写交易 Sender。
哈希计算为 `SHA256(标识)` 前 8 字节按大端解释，再对业务分片数量取模，
以 `business_shards` 的配置顺序选择分片。顶层 `hash_type` 用于推导 Sender 地址，
不改变这个分片哈希算法。

例如，新合约 `asset_ledger` 的 `move` 方法使用参数 `source` 和 `destination`：

```yaml
routing_rules:
  - contract_name: asset_ledger
    method: move
    strategy: hash
    account_parameters: [source, destination]
  - contract_name: order_book
    method: "*"
    strategy: fixed
    target_backend: business-shard1
```

将上述条目追加到现有 `routing_rules`，不要重复创建第二个同名 YAML 节点。
在已有策略范围内增加合约、方法或参数规则，只需修改配置并重启网关，不需要修改合约
或重新编译路由模块。配置在启动时校验，不热加载；未知字段、未知策略、重复规则、
不存在的固定后端或冲突的策略参数会报错。不写 `strategy` 时，有 `target_backend`
按 `fixed`，否则按 `hash` 处理。增加全新的路由算法需要实现该算法，不能只填一个新策略名称。

调用端在签名前通过 `ResolveRoute` 获取目标 `chain_id`，然后签名和发送原始请求。
规则变更后，调用端也须使用相同规则；网关不会修改已签名请求的 `chain_id`。
交易路由和下文的状态归属是两层配置：前者决定交易发往哪里，后者决定状态属于哪里。
接入新的分片业务还要配置与实际合约读写布局一致的状态归属，不能仅添加网关条目。
变更分片数量、顺序或已有状态的归属会改变映射，不能在已有账本上直接当作普通路由更新。

### 状态归属

- `from_shard/to_shard` 参数不覆盖状态归属，状态所有者由节点配置确定。
- `cross_write_routing` 显式配置 contract、key 前缀或正则、固定分片或哈希策略。
- 正则哈希规则使用唯一捕获组，例如 `^b#(.+)$` 提取 DFA 余额账户。
- 所有节点必须使用相同的分片顺序、规则与身份地址算法。
- 业务分片执行后校验实际读写集，不允许读取或修改另一分片所属状态。
- 未知状态拒绝执行，不再隐式猜测归属。共享可变状态应只有一个执行所有者。
- `immutable` 只适用于部署时即一致、此后永不修改的复制常量。
  不能用它标注会被 Mint/Burn 修改的 totalSupply 或可变管理员。
- 同步执行再次检查来源和目标所有权，禁止改写长安链系统合约及分片内部元数据。

默认策略：`config/sharding/state_routing.yml`，另有 `state_routing.example.yml` 格式示例。
默认策略不自动支持 DFA 的所有授权、DID、全局统计或跨合约调用场景。
实验合约的 `experimentInitialBalance` 配置为只读复制值，各分片部署参数须相同，
运行期间不能通过合约升级改变该值。

### 状态同步

1. 源节点在区块提交后生成聚合写集，对目标同步 payload 签名。
2. 目标节点先验证源证书指纹、部署 ID、来源、目标 chain_id 与 payload 签名，再缓存。
3. 仍只有目标分片 proposer 构造、签名本地系统交易；来源证明作为交易参数携带。
4. 验证节点执行系统交易时再次校验来源证明及写入范围。
5. 同步写集通过 `TxSimContext.Put` 直接写入业务合约状态。
   业务到桥接的每笔同步交易另写一条检查点记录，供调度器确认同步进度。
6. 只有交易状态和合约状态均成功，才推进待同步队列、检查点与完成标记。
   已落块但执行失败的同步交易报错并保留待处理状态，不自动视为成功。
7. 空字节值与 nil 删除值保持区分。单 key 同步使用显式 `__state_delete=true` 表示删除。

来源证明是**被配置为可信的单个源节点签名**，不是源分片 2f+1 共识证明。

## 配置和启动

以下命令在仓库根目录执行；只用于新生成的配置目录。
`scripts/prepare_sharding.sh` 仍会清理其原有 build 目录，不要在生产数据目录执行。

```bash
# 不修改 build，先检查状态策略（分片列表按实际拓扑填写）
go run -mod=readonly ./tools/sharding-config --check-policy \
  --policy ./config/sharding/state_routing.yml \
  --business-shards business-shard1,business-shard2

# 对已生成的 build 配置做只读检查
go run -mod=readonly ./tools/sharding-config \
  --build-dir ./build --policy ./config/sharding/state_routing.yml

# 在生成配置后、分发节点包前安装同一份公开信任表和业务状态策略
go run -mod=readonly ./tools/sharding-config \
  --build-dir ./build \
  --policy ./config/sharding/state_routing.yml \
  --write
```

工具只读取源节点公开签名证书，写入每个节点 sharding.yml 的 `sync_auth`。
首次生成部署 ID，重复执行保留该 ID；拒绝覆盖不同部署 ID。
配置生成脚本默认使用 `config/sharding/state_routing.yml`，`SHARDING_STATE_POLICY` 可覆盖路径。
脚本在清理旧 build 之前检查策略，生成配置后自动安装到所有节点。
模板里的空规则只是生成占位；配置工具及节点启动检查都会拒绝没有状态归属的配置。

运行网关配置为 `module/sharding/gateway/gateway.yml`，已使用显式路由规则，
保留当前部署的地址和证书路径；换环境时仍需按实际部署调整。
独立示例为 `gateway.example.yml`。其他旧配置含 `user_config_path/shard_credentials`
或 SDK 后端字段时会明确报错，不自动代签交易。
准备脚本只生成公开信任表，不生成供网关代签的用户私钥配置。

启动前校验：分片角色、拓扑、状态规则、信任表、本地 chain_id、签名证书。
当前分片模块为进程级配置，强制一个进程只承载一条链。
没有 sharding.yml 时仍按普通单链启动；存在但非法时启动失败。
执行策略在账本重放前初始化，同步与调度回调在出块前注册。
链内阶段信号订阅通过 `RegisterNetServiceReadyHandler` 在网络 pub-sub 初始化后、
共识模块启动前完成；订阅失败中止该链启动，不再只打印警告后继续运行。

## 运行约束

1. **账本状态**：部署同名合约不会迁移原有余额，也不会自动复制旧链的历史状态。
   单链已有数据的切换须使用独立的状态迁移流程。
2. **阶段信号**：业务到桥接通过检查点等待同步提交。桥接完成信号只表示桥接阶段结束，
   不证明所有业务目标都已提交完整回写集合，不能作为跨片全部写回完成的凭证。
3. **信任模型**：来源证明使用配置中受信任节点的签名，不是源分片最终性证明。
   阶段控制消息不使用上述来源签名验证机制，分片网络访问必须受控。
4. **状态布局**：任意范围查询、SQL、多所有者共享计数器不能依靠简单 key 哈希透明支持。
   路由策略须匹配合约的实际读写范围。依赖链 ID、块高或跨合约调用上下文的业务，
   其语义还取决于目标链的执行上下文。
5. **重试与恢复**：已经落块但执行失败的同步交易不能用同一 txId 重发。
   待传输任务保存在内存中，队列为空不构成同步完整性证明。
6. **版本一致性**：来源证明参与交易校验，同步执行代码决定读写集。
   同一条链的节点必须使用一致的同步执行代码；执行语义变更后使用新实验账本。
7. **配置生成依赖**：`scripts/prepare_sharding.sh` 使用 `tools/chainmaker-cryptogen`
   下的证书生成程序及配置模板，并在生成配置时检查这些文件。

## 执行成本

来源签名验证、业务写集构建和区块提交构成同步交易的执行成本。
纯桥接同步块通过读写依赖检查后，可以使用互不相交写集的 DAG 快速路径。
TPS 对比需要使用相同的协议、配置和交易负载。

# 分片配置文件
# 用于标识当前节点所属的分片信息

# 分片唯一标识 (如: business-shard1, bridge-shard)
shard_id: {shard_id}

# 分片类型 (BUSINESS: 业务分片, BRIDGE: 桥接分片)
shard_type: {shard_type}

# 分时共识调度配置
schedule:
  # 分时模式起始高度（高度 > start_height 才启用分时）
  start_height: {start_height}

  # 业务分片连续出块数量（片内共识轮数）
  business_batch: {business_batch}

  # 桥接分片连续出块数量（片间共识轮数）
  bridge_batch: {bridge_batch}

  # 动态调度总批次数 (N + M)，0 表示禁用动态调度，每轮固定使用 business_batch 和 bridge_batch
  # 启用时，桥接分片在每轮结束后按比例法自动计算下一轮的 N 和 M：
  #   M_next = round(T × Q_bridge / (Q_bridge + avg(Q_biz)))，N_next = T - M_next
  total_batch: {total_batch}

  # 单笔系统状态同步交易最多携带的状态写入数量
  state_sync_max_writes_per_tx: 4000

  # 未配置 business_shards 时使用的兼容刷新阈值；配置分片列表后，bridge
  # 按 cycle/slot 等待每个业务分片各一个检查点，并将完整波次一起出块
  state_sync_flush_writes: 64000
  state_sync_flush_bytes: 8388608

  # 业务分片列表（桥接分片需等待全部业务分片完成信号）
  business_shards:
    {business_shards}

  # 桥接分片把跨片交易 RWSet 回写到业务分片时的目标分片路由策略
  # strategy:
  #   key_hash           - 按状态 key 哈希路由，适合 token_business 这类 key=account 的合约
  #   contract_key_hash  - 按 contract+key 哈希路由
  #   first_business     - 全部写到第一个业务分片
  #   none               - 不自动路由
  # 业务路由必须显式配置；未配置的业务状态拒绝执行，不推测其归属。
  # prepare_sharding.sh 从 config/sharding/state_routing.yml 安装公共策略。
  # SHARDING_STATE_POLICY 可覆盖路径；下面的空占位不能直接用于启动节点。
  cross_write_routing:
    strategy: "none"
    rules: []

# 心跳交易配置
# 当交易池持续为空时，由 Proposer 注入心跳交易推进分时出块计数，避免调度死锁
heartbeat:
  # 交易池空闲多少毫秒后触发心跳注入；压测默认 100ms，避免空分片长时间阻塞轮次推进
  idle_threshold_ms: 100
  # 心跳合约名（需在链上部署，若合约不存在交易执行失败但块仍会提交）
  contract_name: "sharding_heartbeat"
  # 心跳方法名
  method: "ping"

# 分片同步网络配置 (用于跨分片状态同步)
sync_network:
  # 是否启用同步服务
  enable: {sync_enable}
  
  # 同步服务监听端口 (libp2p)
  port: {sync_port}
  
  # 种子节点列表 (格式: /ip4/<ip>/tcp/<port>/p2p/<peer_id>)
  # 业务分片节点应填写桥接分片节点的地址
  seeds:
    {sync_seeds}

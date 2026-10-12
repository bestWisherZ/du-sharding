#
# Copyright (C) BABEC. All rights reserved.
# Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# [*] the represented items could not be modified after startup

# "auth_type" should be consistent among the whole chain configuration files(e.g., bc1.yml and chainmaker.yml)
# The auth type can be permissionedWithCert, permissionedWithKey, public.
# By default it is permissionedWithCert.
# permissionedWithCert: permissioned blockchain, using x.509 certificate to identify members.
# permissionedWithKey: permissioned blockchain, using public key to identify members.
# public: public blockchain, using public key to identify members.
auth_type: "permissionedWithCert" # [*]

# Logger settings
log:
  # Logger configuration file path.
  config_file: ../config/{org_path}/log.yml

# Crypto engine config, support gmssl, tencentsm and tjfoc
crypto_engine: tjfoc # [*]

# Chains the node currently joined in
blockchain:
  # chain id and its genesis block file path.
#  - chainId: chain1
#    genesis: ../config/{org_path1}/chainconfig/bc1.yml
#  - chainId: chain2
#    genesis: ../config/{org_path2}/chainconfig/bc2.yml
#  - chainId: chain3
#    genesis: ../config/{org_path3}/chainconfig/bc3.yml
#  - chainId: chain4
#    genesis: ../config/{org_path4}/chainconfig/bc4.yml

# Blockchain node settings
node:
  # Organization id is the node belongs to.
  # When the auth type is public, org id is ignored.
  org_id:            {org_id}  # [*]

  # Private key file path
  priv_key_file:     ../config/{org_path}/certs/{node_cert_path}.key  # [*]

  # Certificate file path
  cert_file:         ../config/{org_path}/certs/{node_cert_path}.crt

  # Certificate cache size, used to speed up member identity verification.
  # By default the cache size is 1000.
  cert_cache_size:   1000

  # CertKeyUsageCheck, used to check if tx sender use the proper certificate to sign transactions
  cert_key_usage_check: true

  # fast sync settings
  fast_sync:
    # Enable it or not, true means do not execute smart contract
    enabled: true

  # PKCS#11 crypto settings
  pkcs11:
    # Enable it or not
    enabled: false  # [*]

    # Type only support pkcs11 and sdf
    type: pkcs11

    # Path for the pkcs11 interface file(.so)
    library: /usr/local/lib64/pkcs11/libupkcs11.so

    # Label for the slot to be used
    label: HSM

    # HSM Password
    password: 11111111

    # Size for HSM session cache, default value is 10.
    session_cache_size: 10

    # Hash algorithm is used to calculate SKI.
    # It can be SHA256 or SM3.
    hash: "SHA256"  # [*]

  # KMS settings
  kms:
    # Enable it or not
    enabled: false

    # Public cloud or private cloud
    is_public: true

    # KMS SecretId
    secret_id: ""

    # KMS SecretKey
    secret_key: ""

    # KMS server address, ip or dns
    address: "kms.tencentcloudapi.com"

    # KMS server region
    region: "ap-guangzhou"

    # KMS sdk scheme, http or https
    sdk_scheme: "https"

    # Optional settings，style "{k1:v1, k2:v2}".
    ext_params: ""

# Network Settings
net:
  # Network provider, can be libp2p or liquid.
  # libp2p: using libp2p components to build the p2p module.
  # liquid: a new p2p network module. We build it from 0 to 1.
  # This item must be consistent across the blockchain network.
  provider: Libp2p

  # The address and port the node listens on.
  # By default, it uses 0.0.0.0 to listen on all network interfaces.
  listen_addr: /ip4/0.0.0.0/tcp/{net_port}

  # Max stream of a connection.
  # peer_stream_pool_size: 100

  # Max number of peers the node can connect.
  # max_peer_count_allow: 20

  # The strategy for eliminating node when the amount of connected peers reaches the max value
  # It could be: 1 Random, 2 FIFO, 3 LIFO. The default strategy is LIFO.
  # peer_elimination_strategy: 3

  # The seeds list used to setup network among all the peer seed when system starting.
  # The connection supervisor will try to dial seed peer whenever the connection is broken.
  # Example ip format: "/ip4/127.0.0.1/tcp/11301/p2p/"+nodeid
  # Example dns format："/dns/cm-node1.org/tcp/11301/p2p/"+nodeid
  seeds:
   # Example node1："node1id-//127.0.0.1"
   # Example  "QmeyNRs2DwWjcHTpcVHoUSaDAAif4VQZ2wQDQAUNDP33gH/http://cm-node1.org"
  seed_target_address_list:
      # Example - "QmeyNRs2DwWjcHTpcVHoUSaDAAif4VQZ2wQDQAUNDP33gH/127.0.0.1"
      # Example - "QmeyNRs2DwWjcHTpcVHoUSaDAAif4VQZ2wQDQAUNDP33gH/http://cm-node1.org"
      # Example - "QmeyNRs2DwWjcHTpcVHoUSaDAAif4VQZ2wQDQAUNDP33gH/cm-node1.org"
  # Network tls settings.
  tls:
    # Enable tls or not. Currently it can only be true...
    enabled: true

    # TLS private key file path.
    priv_key_file: ../config/{org_path}/certs/{net_cert_path}.key

    # TLS Certificate file path.
    cert_file: ../config/{org_path}/certs/{net_cert_path}.crt

    # TLS enc private key file path. (only for gmtls1.1)
    priv_enc_key_file: ../config/{org_path}/certs/{net_cert_path}.enc.key

    # TLS enc Certificate file path.
    cert_enc_file: ../config/{org_path}/certs/{net_cert_path}.enc.crt

  # The blacklist is automatically block the listed seed to connect.
  # blacklist:
      # The addresses in blacklist.
      # The address format can be ip or ip+port.
      # addresses:
      #   - "127.0.0.1:11301"
      #   - "192.168.1.8"

      # The node ids in blacklist.
      # node_ids:
      #   - "QmeyNRs2DwWjcHTpcVHoUSaDAAif4VQZ2wQDQAUNDP33gH"

# Sync Settings
sync:
  # ----The following two configurations are shared by new sync after version 240 and old sync before version 240
  # Maximum number of blocks cached waiting to be verified and committed.
  block_pool_size: 128
  # Synchronize blocks from the configured nodes.
  from_nodes:
  #  - {nodeId}
  #  - {nodeId}

  # ---The following configuration items are used by new sync after version 240.

  # The base time of connection timeout(ms). The timeout for various type of messages is an multiple of this value.
  conn_timeout_scale: 1000

  # the difference between the local height has been synchronized and the latest height is within the specified range.
  ideal_height_distance: 1

  # the time interval for checking whether there are new blocks to be synced after syncing to the latest block.
  wait_period_seconds_after_latest: 5

  # the maximum number of nodes selected from the node pool for syncing.
  # This means that at the same time it will only find {max_nodes_selected_count} nodes to synchronize data.
  max_nodes_selected_count: 10

  # ---The following configuration items are used by old sync before version 240.

  # Timeout for waiting for block request response, unit second.
  # wait_time_requested: 30

  # The number of blocks requested at a time.
  # batch_size_from_one_node: 1

  # The time interval for verifying cached block, unit millisecond.
  # process_block_tick: 20

  # The time interval for broadcasting a request to obtain the status of other nodes, unit second.
  # node_status_tick: 2

  # The time interval for checking whether there is a block request timeout, unit second.
  # liveness_tick: 1

  # The time interval between sending a block request, unit millisecond.
  # scheduler_tick: 20

  # The time interval between sending a block request when the difference between own block height and the highest block height is 1
  # unit second.
  # req_time_threshold: 1

  # The time interval for processing block requests of the same height from the same node.
  # Requests within this time will be ignored. Unit second.
  # block_request_time: 5

  # broadcast node status once when {broadcast_status_per_blocks_committed} blocks are committed.
  # broadcast_status_per_blocks_committed: 3


# Transaction pool settings
# Other tx_pool settings can be found in tx_Pool_config.go
txpool:
  # tx_pool type, can be single, normal, batch.
  # By default the tx_pool type is normal.
  # Note: please delete dump_tx_wal folder in storage.store_path when change tx_pool type
  pool_type: "normal"

  # Max common transaction count in tx_pool.
  # If tx_pool is full, the following transactions will be discarded.
  max_txpool_size: 50000

  # Max config transaction count in tx_pool.
  max_config_txpool_size: 10

  # Whether dump unpacked config and common transactions in queue when stop node,
  # and replay these transactions when restart node.
  is_dump_txs_in_queue: true

  # Common transaction queue num, only for normal tx_pool.
  # Note: the num should be an exponent of 2 and less than 256, such as, 1, 2, 4, 8, 16, ..., 256
  common_queue_num: 8

  # The number of transactions contained in a batch, for normal and batch tx_pool.
  # Note: make sure that block.block_tx_capacity in bc.yml is an integer multiple of batch_max_size
  batch_max_size: 100

  # Interval of creating a transaction batch, for normal and batch tx_pool, in millisecond(ms).
  batch_create_timeout: 50

  # common node tx pool secondary broadcast to prevent stranded txs in common tx pool
  rebroadcast:
    # enable tx pool re sync or not
    tx_pool_sync_enable: false
    # tx pool sync tick duration(second)
    tx_pool_status_tick: 10
    # tx pool sync proportion (tx num in queue / tx pool size)
    # when it is less than this ratio, re broadcast will be triggered
    tx_pool_sync_proportion: 0.1

# RPC service setting
rpc:
  # RPC type, can only be grpc now
  provider: grpc  # [*]

  # RPC host eg. 127.0.0.1,0.0.0.0,localhost
  host: 0.0.0.0
  # RPC port
  port: {rpc_port}

  # Interval of checking trust root changes, in seconds.
  # If changed, the rpc server's root certificate pool will also change.
  # Only valid if tls is enabled.
  # The minium value is 10.
  check_chain_conf_trust_roots_change_interval: 60

  # API SendRequestSync timeout in seconds.
  # SendRequestSync return response with tx execution result.
  # The default value is 60, recommend set it to 60
  sync_tx_result_timeout: 60

  # restful api gateway
  gateway:
    # enable restful api
    enabled: false
    # max resp body buffer size, unit: M
    max_resp_body_size: 16

  # Rate limit related settings
  # Here we use token bucket to limit rate.
  ratelimit:
    # Ratelimit switch. Default is false.
    enabled: false

    # Rate limit type
    # 0: limit globally, 1: limit by ip
    type: 0

    # Token number added to bucket per second.
    # -1: unlimited, by default is 10000.
    token_per_second: -1

    # Token bucket size.
    # -1: unlimited, by default is 10000.
    token_bucket_size: -1

  # Rate limit settings for subscriber
  subscriber:
    ratelimit:
      # Ratelimit switch. Default is false.
      enabled: false

      # Rate limit type
      # 0: limit globally, 1: limit by sender. Default is globally.
      type: 0

      # Token number added to bucket per second. Default is 100.
      token_per_second: 100

      # Token bucket size. Default is 100.
      token_bucket_size: 100

  # RPC TLS Setting
  tls:
    # TLS mode, can be disable, twoway.
    mode: twoway

    # RPC TLS private key file path
    priv_key_file: ../config/{org_path}/certs/{rpc_cert_path}.key

    # RPC TLS public key file path
    cert_file: ../config/{org_path}/certs/{rpc_cert_path}.crt

    # RPC enc TLS private key file path (only for gmtls1.1)
    #priv_enc_key_file: ../config/{org_path}/certs/{rpc_cert_path}.enc.key

    # RPC enc TLS public key file path
    #cert_enc_file: ../config/{org_path}/certs/{rpc_cert_path}.enc.crt

  # RPC blacklisted ip addresses
  blacklist:
    addresses:
      # - "127.0.0.1"

  # RPC server max send/receive message size in MB
  max_send_msg_size: 100
  max_recv_msg_size: 100

# Tx Filter Setting
tx_filter:
  # default(store) 0; bird's nest 1; map 2; 3 sharding bird's nest
  # 3 is recommended.
  type: 0

  # sharding bird's nest config
  # total keys = sharding.length * sharding.birds_nest.length * sharding.birds_nest.cuckoo.max_num_keys
  sharding:
    # sharding number
    length: 5

    # sharding task timeout in seconds
    timeout: 60

    # memory storage to disk
    snapshot:
      # serialize type
      # 0 serialization by height interval
      # 1 serialization by time interval
      type: 0

      # effective when type is 1
      timed:
        # Time interval in seconds
        interval: 10

      # effective when type is 0
      block_height:
        # Block height interval
        interval: 10

      # effective when type is 1, serialization interval in seconds, compatible with version 221
      serialize_interval: 10

      # storage to disk file path
      path: ../data/{org_id}/tx_filter

    # bird's nest config
    birds_nest:
      # bird's nest size
      length: 10

      # Transaction filter rules
      rules:
        # Absolute expiration time /second
        # Based on the number of transactions per day, for example, the current total capacity of blockchain transaction
        # filters is 100 million, and there are 10 million transaction requests per day.
        #
        # total keys = sharding.length * sharding.birds_nest.length * sharding.birds_nest.cuckoo.max_num_keys
        #
        # absolute expire time = total keys / number of requests per day
        absolute_expire_time: 172800

      cuckoo:
        # 0 NormalKey; 1 TimestampKey
        key_type: 1

        # num of tags for each bucket, which is b in paper. tag is fingerprint, which is f in paper.
        # If you are using a semi-sorted bucket, the default is 4
        # 2 is recommended.
        tags_per_bucket: 2

        # num of bits for each item, which is length of tag(fingerprint)
        # 11 is recommended.
        bits_per_item: 11

        # keys number
        max_num_keys: 2000000

        # 0 TableTypeSingle normal single table
        # 1 TableTypePacked packed table, use semi-sort to save 1 bit per item
        # 0 is recommended
        table_type: 0

  # bird's nest config
  # total keys = birds_nest.length * birds_nest.cuckoo.max_num_keys
  birds_nest:
    # bird's nest size
    length: 10

    # memory storage to disk
    snapshot:
      # serialize type
      # 0 serialization by height interval
      # 1 serialization by time interval
      type: 0

      # effective when type is 1
      timed:
        # Time interval in seconds
        interval: 10

      # effective when type is 0
      block_height:
        # Block height interval
        interval: 10

      # effective when type is 1, serialization interval in seconds, compatible with version 221
      serialize_interval: 10

      # storage to disk file path
      path: ../data/{org_id}/tx_filter

    # Transaction filter rules
    rules:
      # Absolute expiration time /second
      # Based on the number of transactions per day, for example, the current total capacity of blockchain transaction
      # filters is 100 million, and there are 10 million transaction requests per day.
      #
      # total keys = sharding.length * sharding.birds_nest.length * sharding.birds_nest.cuckoo.max_num_keys
      #
      # absolute expire time = total keys / number of requests per day
      absolute_expire_time: 172800

    cuckoo:
      # 0 NormalKey; 1 TimestampKey
      key_type: 1

      # num of tags for each bucket, which is b in paper. tag is fingerprint, which is f in paper.
      # If you are using a semi-sorted bucket, the default is 4
      # 2 is recommended.
      tags_per_bucket: 2

      # num of bits for each item, which is length of tag(fingerprint)
      # 11 is recommended.
      bits_per_item: 11

      # keys number
      max_num_keys: 2000000

      # 0 TableTypeSingle normal single table
      # 1 TableTypePacked packed table, use semi-sort to save 1 bit per item
      # 0 is recommended
      table_type: 0

# Monitor related settings
monitor:
  # Monitor service switch, default is false.
  enabled: true

  # Monitor service port
  port: {monitor_port}

# PProf Settings
pprof:
  # If pprof is enabled or not
  enabled: false

  # PProf port
  port: {pprof_port}

# Consensus related settings
consensus:
  raft:
    # Take a snapshot based on the set the number of blocks.
    # If raft nodes change, a snapshot is taken immediately.
    snap_count: 10

    # Saving wal asynchronously switch. Default is true.
    async_wal_save: true

    # Min time unit in rate election and heartbeat.
    ticker: 1

# Scheduler related settings
scheduler:
  # whether log the txRWSet map in debug mode
  rwset_log: false

# Storage config settings
# Contains blockDb, stateDb, historyDb, resultDb, contractEventDb
#
# blockDb: block transaction data,                          support leveldb, mysql, badgerdb, tikvdb
# stateDb: world state data,                                support leveldb, mysql, badgerdb, tikvdb
# historyDb: world state change history of transactions,    support leveldb, mysql, badgerdb, tikvdb
# resultDb: transaction execution results data,             support leveldb, mysql, badgerdb, tikvdb
# contractEventDb: contract emit event data,                support mysql
#
# provider, sqldb_type cannot be changed after startup.
# store_path, dsn the content cannot be changed after startup.
storage:
  # Default store path
  store_path: ../data/{org_id}/ledgerData1 # [*]

  # Prefix for mysql db name
  # db_prefix: org1_  # [*]

  # Minimum block height not allowed to be archived
  unarchive_block_height: 300000

  # Archive dir scan interval time(s), default: 10(s)
  archive_check_interval: 10

  # Restore data merge on chain data wait time,
  # restore action start after "restoreBlock" action finished "restore_interval" time(s), default: 60(s)
  restore_interval: 60

  # Symmetric encryption algorithm for writing data to disk. can be sm4 or aes
  # encryptor: sm4    # [*]

  # Symmetric encryption key:16 bytes key
  # If pkcs11 is enabled, it is the keyID
  # encrypt_key: "1234567890123456"

  # wal encrypt mode: "sync: sync encrypt, "async": async encrypt; default no encrypt
  # wal_encrypt_mode: "sync"

  # Disable block file db, default: true
  disable_block_file_db: false  # [*]

  # async write block in file block db to disk (by blockfiledb or wal), default: false, so default is sync write disk
  logdb_segment_async: false

  # file size of stored block file
  # if disable_block_file_db: false, we use block filedb, this means .fdb file size(MB), default: 64
  # if disable_block_file_db: true, we use wal, this means .wal file size(MB), default: 20
  logdb_segment_size: 64

  # read bfdb block file time out(ms), default: 1000
  read_bfdb_timeout: 1000

  # bigfilter config, default false
  enable_bigfilter: false

  # effective when enable_bigfilter is true
  bigfilter_config:
    # redis host:port
    redis_hosts_port: "127.0.0.1:6300,127.0.0.1:6301"

    # redis password
    redis_password: abcpass

    # support max transaction capacity
    tx_capacity: 1000000000

    # false postive rate
    fp_rate: 0.000000001

  # RWC config, default false
  enable_rwc: true

  # effective when enable_rwc is true, default 1000000
  # suggest greater than max_txpool_size*1.1
  rolling_window_cache_capacity: 55000

  # 0 common write，1 quick write
  write_block_type: 0
  # must be greater than 0 otherwise the default value 100 is used.
  quick_write_channel_size: 100

  # record DB slow log (INFO level) when query spend time more than this value (millisecond), 0 means no record
  slow_log: 0

  # state db cache
  disable_state_cache: false # default enable state cache

  # effective when disable_state_cache is false
  state_cache_config:
    # provider include slru, bigcache. default slru. 
    # if slru is enabled, make sure spec item is configured.
    provider: slru
    
    # key/value ttl time, ns, just for bigcache
    # life_window: 3000000000000

    # interval between removing expired keys and values(clean up). just for bigcache
    # clean_window: 1000000000

    # max size of entry in bytes. just for bigcache
    # max_entry_size: 500

    # max cache size MB, just for bigcache
    # hard_max_cache_size: 1024

    # the cache specification, currently only used by slru
    spec: 
      # at least one of memory and capacity is greater than 0.
      memory: 1024  # maximum memory usage in MB
      capacity: 10000  # maximum number of cache entries

  # Block db config
  blockdb_config:
    # Databases type support leveldb, sql, badgerdb, tikvdb
    provider: leveldb # [*]

    # If provider is leveldb, leveldb_config should not be null.
    leveldb_config:
      # LevelDb store path
      store_path: ../data/{org_id}/block
      write_buffer_size: 64

    # Example for sql provider
    # Databases type support leveldb, sql, badgerdb, tikvdb
    # provider: sql # [*]
    # If provider is sql, sqldb_config should not be null.
    # sqldb_config:
      # Sql db type, can be mysql, sqlite. sqlite only for test
      # sqldb_type: mysql # # [*]
      # Mysql connection info, the database name is not required. such as:  root:admin@tcp(127.0.0.1:3306)/
      # dsn: root:password@tcp(127.0.0.1:3306)/

    # Example for badgerdb provider
    # Databases type support leveldb, sql, badgerdb, tikvdb
    # provider: badgerdb
    # If provider is badgerdb, badgerdb_config should not be null.
    # badgerdb_config:
      # BadgerDb store path
      # store_path: ../data/wx-org1.chainmaker.org/history
      # Whether compression is enabled for stored data, default is 0: disabled
      # compression: 0
      # Key and value are stored separately when value is greater than this byte, default is 1024 * 10
      # value_threshold: 256
      # Number of key value pairs written in batch. default is 128
      # write_batch_size: 128

    # Example for tikv provider
    # provider: tikvdb
    # If provider is tikvdb, tikvdb_config should not be null.
    # tikvdb_config:
      # db_prefix: "node1_" #default is ""
      # endpoints: "127.0.0.1:2379" # tikv pd server url，support multi url, example :"192.168.1.2:2379,192.168.1.3:2379"
      # max_batch_count: 128  # max tikv commit batch size, default: 128
      # grpc_connection_count: 16 # chainmaker and tikv connect count, default: 4
      # grpc_keep_alive_time: 10 # keep connnet alive count, default: 10
      # grpc_keep_alive_timeout: 3  # keep connnect alive time, default: 3
      # write_batch_size: 128 # commit tikv bacth size each time, default: 128

  # State db config
  statedb_config:
    provider: leveldb
    leveldb_config:
      store_path: ../data/{org_id}/state
      write_buffer_size: 64

    # bloom config for state db, it only takes effect when using kvdb
    bloom_config:
      enable: true # enable bloom filter or not
      dump_path: "../data/{org_id}/state/bloom" # bloom filter data dump to file in the path
      # bloom filter calculates the number of enabled hash functions and the number of bits used
      # based on the key capacity and false positive rate.
      keys_capacity: 1000000000
      false_positive_rate: 0.01
      dump_per_blocks_committed: 1000 # do dump every n blocks added to bloom filter

  # History db config, default enable history db
  disable_historydb: false
  historydb_config:
    provider: leveldb
    disable_key_history: false
    disable_contract_history: true
    disable_account_history: true
    leveldb_config:
      store_path: ../data/{org_id}/history
      write_buffer_size: 64

  # Result db config, default enable result db
  disable_resultdb: false
  resultdb_config:
    provider: leveldb
    leveldb_config:
      store_path: ../data/{org_id}/result
      write_buffer_size: 64

  # Disable contract event database or not. If it is false, contract_eventdb_config must be mysql
  disable_contract_eventdb: true
  contract_eventdb_config:
    # Event db only support sql
    provider: sql
    # Sql db config
    sqldb_config:
      # Event db only support mysql
      sqldb_type: mysql
      # Mysql connection info, such as:  root:admin@tcp(127.0.0.1:3306)/
      dsn: root:password@tcp(127.0.0.1:3306)/

# Contract Virtual Machine(VM) configs
vm:
  # Common configs of docker vm
  common:
    # Grpc configs during communication between chainmaker and contract VM
    # Grpc max send message size, Default size is 100, unit: MB
    max_send_msg_size: 100
    # Grpc max receive message size, Default size is 100, unit: MB
    max_recv_msg_size: 100
    # Grpc dialing timeout, default size is 10, uint: s
    dial_timeout: 10

    #  Configs of docker runtime server (handle messages with contract sandbox)
    runtime_server:
      # runtime server host, default 127.0.0.1
      # host: 127.0.0.1
      # Runtime server port, default 32351
      port: {docker_vm_runtime_port}

    contract_engine:
      # Preload contract processes or not when starting
      preload_disable: false

      # The number of preload processes calculated by use frequency
      preload_num_by_use_frequency: 10

      # The number of preload processes calculated by recent invocation time
      preload_num_by_last_time: 10

      # cgroup is used to limit the resource usage of contract processes on the host machine
      cgroup:
        # disable the cgroup function means that there will be no restrictions
        # on the resource usage of the contract process on the host machine
        disable: true
        # max memory size per sandbox(MiB), -1 means no limit
        max_mem_size_per_process: -1
        # max cpu percent per sandbox, -1 means no limit
        max_cpu_percent_per_process: -1

        # allow devices list
        # detailed information can be found in the document:
        #     https://access.redhat.com/documentation/zh-cn/red_hat_enterprise_linux/7/html/resource_management_guide/sec-devices//
        devices_allow: ""

        # deny devices list
        # detailed information can be found in the document:
        #     https://access.redhat.com/documentation/zh-cn/red_hat_enterprise_linux/7/html/resource_management_guide/sec-devices//
        devices_deny: ""

  # Golang runtime in docker container
  go:
    # Enable docker go virtual machine, default: false
    enable: {enable_docker_go}
    # Mount data path in chainmaker, include contracts, uds socks
    data_mount_path: ../data/{org_id}/go

    # Mount log path in chainmaker
    log_mount_path: ../log/{org_id}/go

    # Communication protocol, used for chainmaker and docker manager communication
    # 1. tcp: docker vm uses TCP to communicate with chain
    # 2. uds: docker vm uses unix domain socket to communicate with chain
    protocol: tcp

    # If use a customized VM configuration file, supplement it; else, do not configure
    # Priority: chainmaker.yml > vm.yml > default settings
    # dockervm_config_path: /config_path/vm.yml
    # Whether to print log on terminal
    log_in_console: false

    # Log level of docker vm go
    log_level: {docker_go_log_level}

    # max process num for execute original txs
    max_concurrency: 20

    # Configs of contract engine server (handle messages with contract engine)
    contract_engine:
      # Docker vm contract engine server host, default 127.0.0.1
      host: 127.0.0.1

      # Docker vm contract engine server port, default 22351
      port: {docker_go_engine_port}
      # Max number of connection created to connect docker vm service
      max_connection: 5

  # Java runtime in docker container
  java:
    # Enable docker java virtual machine, default: false
    enable: {enable_docker_java}

    # Mount data path in chainmaker, include contracts, uds socks
    data_mount_path: ../data/{org_id}/java

    # Mount log path in chainmaker
    log_mount_path: ../log/{org_id}/java

    # If use a customized VM configuration file, supplement it; else, do not configure
    # Priority: chainmaker.yml > vm.yml > default settings
    # dockervm_config_path: /config_path/vm.yml
    # Whether to print log on terminal
    log_in_console: false

    # Log level of docker vm java
    log_level: {docker_java_log_level}

    # max process num for execute original txs
    max_concurrency: 20

    # Configs of contract engine server (handle messages with contract engine)
    contract_engine:
      # Docker vm contract engine server host, default 127.0.0.1
      host: 127.0.0.1

      # Docker vm contract engine server port, default 23351
      port: {docker_java_engine_port}

      # Max number of connection created to connect docker vm engine service
      max_connection: 5

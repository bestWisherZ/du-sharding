#!/usr/bin/env bash
#
# Copyright (C) BABEC. All rights reserved.
# Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# check mac gun-getopt
function checkEnv() {
  if [ "$(uname)" == "Darwin" ];then
    getopt --test
    if [ "$?" != "4" ];then
      brew -v > /dev/null
      if [ "$?" != "0" ];then
        echo 'Please install brew for Mac: ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"'
      fi
      echo 'Please install gnu-getopt for Mac: brew install gnu-getopt and set to PATH (brew link --force gnu-getopt)'
      exit
    fi
  fi
}
checkEnv

set -e

VERSION='"2040000"'
GENESIS_TIME='"2020-11-30T01:01:01+08:00"'

if [ "$(uname)" = "Darwin" ]; then
    time_part=$(date "+%Y-%m-%dT%H:%M:%S")
    tz_part=$(date "+%z")
    gt_tmp="${time_part}${tz_part:0:3}:${tz_part:3:2}"
    GENESIS_TIME="\"${gt_tmp}\""
elif [ "$(uname)" = "Linux" ]; then
    gt_tmp="$(date "+%Y-%m-%dT%H:%M:%S%:z")"
    GENESIS_TIME="\"${gt_tmp}\""
else
    echo "未知系统"
fi

# ==============================================================================
# 分片环境生成参数
# ==============================================================================
# 共识与 VM 参数由 collect_common_config 统一读取。
NODE_CNT=$1
CHAIN_CNT=$2
P2P_PORT=$3
RPC_PORT=$4
DOCKER_VM_RUNTIME_PORT=$5
DOCKER_GO_ENGINE_PORT=$6
DOCKER_JAVA_ENGINE_PORT=$7

CURRENT_PATH=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
BUILD_PATH=${PROJECT_PATH}/build
CONFIG_TPL_PATH=${PROJECT_PATH}/config/config_tpl
BUILD_CRYPTO_CONFIG_PATH=${BUILD_PATH}/crypto-config
BUILD_CONFIG_PATH=${BUILD_PATH}/config
CRYPTOGEN_TOOL_PATH=${PROJECT_PATH}/tools/chainmaker-cryptogen
CRYPTOGEN_TOOL_BIN=${CRYPTOGEN_TOOL_PATH}/bin/chainmaker-cryptogen
CRYPTOGEN_TOOL_CONF=${CRYPTOGEN_TOOL_PATH}/config/crypto_config_template.yml
CRYPTOGEN_TOOL_PKCS11_KEYS=${CRYPTOGEN_TOOL_PATH}/config/hsm_keys.yml
STATE_POLICY=${SHARDING_STATE_POLICY:-${PROJECT_PATH}/config/sharding/state_routing.yml}
if [[ "$STATE_POLICY" != /* ]]; then
    STATE_POLICY="$(pwd)/${STATE_POLICY}"
fi

if [ ! -r "$STATE_POLICY" ]; then
    echo "error: state policy not found: ${STATE_POLICY}; set SHARDING_STATE_POLICY to a readable policy file" >&2
    exit 1
fi

if [ ! -x "${CRYPTOGEN_TOOL_BIN}" ] || [ ! -r "${CRYPTOGEN_TOOL_CONF}" ]; then
    echo "error: install/build chainmaker-cryptogen under ${CRYPTOGEN_TOOL_PATH} before preparing shards" >&2
    exit 1
fi
if ! command -v go >/dev/null 2>&1; then
    echo "error: Go is required to validate and generate sharding trust configuration" >&2
    exit 1
fi

BC_YML_TRUST_ROOT_LINE=$(awk '/trust roots list start/{print NR}' ${CONFIG_TPL_PATH}/chainconfig/bc_4_7.tpl)
BC_YML_TRUST_ROOT_LINE_END=$(awk '/trust roots list end/{print NR}' ${CONFIG_TPL_PATH}/chainconfig/bc_4_7.tpl)

function show_help() {
    echo "Usage:  ./prepare_sharding.sh"
    echo "Follow the interactive prompts to generate sharding configuration."
}

# ==============================================================================
# 证书与节点配置生成函数
# ==============================================================================

function xsed() {
    system=$(uname)

    if [ "${system}" = "Linux" ]; then
        sed -i "$@"
    else
        sed -i '' "$@"
    fi
}


function check_params() {
    echo "begin check params..."
    if  [[ ! -n $NODE_CNT ]] ;then
        echo "node cnt is empty"
        # show_help # 分片模式下不显示help
        exit 1
    fi

    if  [ ! $NODE_CNT -eq 1 ] && [ ! $NODE_CNT -eq 4 ] && [ ! $NODE_CNT -eq 7 ]&& [ ! $NODE_CNT -eq 10 ]&& [ ! $NODE_CNT -eq 13 ]&& [ ! $NODE_CNT -eq 16 ];then
        echo "node cnt should be 1 or 4 or 7 or 10 or 13"
        # show_help
        exit 1
    fi

    if  [[ ! -n $CHAIN_CNT ]] ;then
        echo "chain cnt is empty"
        # show_help
        exit 1
    fi

    if  [ ${CHAIN_CNT} -lt 1 ] || [ ${CHAIN_CNT} -gt 4 ] ;then
        echo "chain cnt should be 1 - 4"
        # show_help
        exit 1
    fi

    # 判断是否是数字
    if [ "$P2P_PORT" -gt 0 ] 2>/dev/null ;then
      # 判断数字范围
      if  [ ${P2P_PORT} -ge 60000 ] || [ ${P2P_PORT} -le 10000 ];then
        P2P_PORT=11301
      fi
    else
        P2P_PORT=11301
    fi
    echo "param P2P_PORT $P2P_PORT"

    if [ "$RPC_PORT" -gt 0 ] 2>/dev/null ;then
      if  [ ${RPC_PORT} -ge 60000 ] || [ ${RPC_PORT} -le 10000 ];then
        RPC_PORT=12301
      fi
    else
        RPC_PORT=12301
    fi
    echo "param RPC_PORT $RPC_PORT"

    if [ "$DOCKER_VM_RUNTIME_PORT" -gt 0 ] 2>/dev/null ;then
      if  [ ${DOCKER_VM_RUNTIME_PORT} -ge 60000 ] || [ ${DOCKER_VM_RUNTIME_PORT} -le 10000 ];then
        DOCKER_VM_RUNTIME_PORT=32351
      fi
    else
        DOCKER_VM_RUNTIME_PORT=32351
    fi
    echo "param DOCKER_VM_RUNTIME_PORT $DOCKER_VM_RUNTIME_PORT"

    if [ "$DOCKER_GO_ENGINE_PORT" -gt 0 ] 2>/dev/null ;then
      if  [ ${DOCKER_GO_ENGINE_PORT} -ge 60000 ] || [ ${DOCKER_GO_ENGINE_PORT} -le 10000 ];then
        DOCKER_GO_ENGINE_PORT=22351
      fi
    else
        DOCKER_GO_ENGINE_PORT=22351
    fi
    echo "param DOCKER_GO_ENGINE_PORT $DOCKER_GO_ENGINE_PORT"

    if [ "$DOCKER_JAVA_ENGINE_PORT" -gt 0 ] 2>/dev/null ;then
      if  [ ${DOCKER_JAVA_ENGINE_PORT} -ge 60000 ] || [ ${DOCKER_JAVA_ENGINE_PORT} -le 10000 ];then
        DOCKER_JAVA_ENGINE_PORT=23351
      fi
    else
        DOCKER_JAVA_ENGINE_PORT=23351
    fi
    echo "param DOCKER_JAVA_ENGINE_PORT $DOCKER_JAVA_ENGINE_PORT"
}

function generate_certs() {
    #echo "begin generate certs, cnt: ${NODE_CNT}"
    mkdir -p ${BUILD_PATH}
    cd "${BUILD_PATH}"
    
    # 备份已有证书目录。
    if [ -d crypto-config ]; then
        mkdir -p backup/backup_certs
        mv crypto-config  backup/backup_certs/crypto-config_$(date "+%Y%m%d%H%M%S")
    fi

    cp $CRYPTOGEN_TOOL_CONF crypto_config.yml
    cp $CRYPTOGEN_TOOL_PKCS11_KEYS hsm_keys.yml

    xsed "s%count: 4%count: ${NODE_CNT}%g" crypto_config.yml

    ${CRYPTOGEN_TOOL_BIN} generate -c ./crypto_config.yml  -p ./hsm_keys.yml
}

function generate_config() {
    # 使用 collect_common_config 设置的全局配置。
    # 确保默认值（如果外部未设置）
    MONITOR_PORT=${MONITOR_PORT:-14321}
    PPROF_PORT=${PPROF_PORT:-24321}
    TRUSTED_PORT=${TRUSTED_PORT:-13301}
    VM_GO_CONTAINER_NAME_PREFIX=${VM_GO_CONTAINER_NAME_PREFIX:-"chainmaker-vm-go-container"}
    CONSENSUS_TYPE=${CONSENSUS_TYPE:--1}


    # set CONSENSUS_TYPE
    if [ $CONSENSUS_TYPE == -1 ] ;then
      if  [ $NODE_CNT -gt 1 ] ;then
        read -p "input consensus type (1-TBFT(default),3-MAXBFT,4-RAFT): " tmp
        if  [ ! -z "$tmp" ] ;then
          if [ $tmp -eq 1 ] || [ $tmp -eq 3 ] || [ $tmp -eq 4 ] ;then
            CONSENSUS_TYPE=$tmp
          else
            echo "invalid consensus type [" $tmp "], so use default"
          fi
        fi
      else
        read -p "input consensus type (0-SOLO,1-TBFT(default),3-MAXBFT,4-RAFT): " tmp
        if  [ ! -z "$tmp" ] ;then
          if  [ $tmp -eq 0 ] || [ $tmp -eq 1 ] || [ $tmp -eq 3 ] || [ $tmp -eq 4 ] ;then
            CONSENSUS_TYPE=$tmp
          else
            echo "unknown consensus type [" $tmp "], so use default"
          fi
        fi
      fi
    fi
    if [ $CONSENSUS_TYPE == -1 ] ;then
          CONSENSUS_TYPE=1
    fi
    if [ $CONSENSUS_TYPE == 3 ] && [ $NODE_CNT -lt 4 ] ;then
      echo  "the current version of maxbft does not support the deployment of less than four nodes"
      exit
    fi
    echo "param CONSENSUS_TYPE $CONSENSUS_TYPE"

    # set LOG_LEVEL
    if [ "$LOG_LEVEL" == "" ] ;then
      read -p "input log level (DEBUG|INFO(default)|WARN|ERROR): " tmp
      if  [ ! -z "$tmp" ] ;then
        if  [ $tmp == "DEBUG" ] || [ $tmp == "INFO" ] || [ $tmp == "WARN" ] || [ $tmp == "ERROR" ];then
            LOG_LEVEL=$tmp
        else
          echo "unknown log level [" $tmp "], so use default"
        fi
      fi
    fi
    if [ "$LOG_LEVEL" == "" ] ;then
        LOG_LEVEL="INFO"
    fi
    echo "param LOG_LEVEL $LOG_LEVEL"

    # set ENABLE_VM_GO
    if [ "$ENABLE_VM_GO" == "" ] ;then
      read -p "enable vm go (YES|NO(default))" enable_vm_go
      if  [ ! -z "$enable_vm_go" ]; then
        if  [ $enable_vm_go == "yes" ] || [ $enable_vm_go == "YES" ]; then
            ENABLE_VM_GO="true"

            if [ "$DOCKER_GO_LOG_LEVEL" == "" ] ;then
                read -p "input vm go log level (DEBUG|INFO(default)|WARN|ERROR): " docker_go_log_level
                if  [ ! -z "$docker_go_log_level" ] ;then
                if  [ $docker_go_log_level == "DEBUG" ] || [ $docker_go_log_level == "INFO" ] || [ $docker_go_log_level == "WARN" ] || [ $docker_go_log_level == "ERROR" ];then
                    DOCKER_GO_LOG_LEVEL=$docker_go_log_level
                else
                    echo "unknown vm go log level [" $docker_go_log_level "], so use default"
                fi
              fi
            fi
            if [ "$DOCKER_GO_LOG_LEVEL" == "" ] ;then
              DOCKER_GO_LOG_LEVEL="INFO"
            fi

        fi
      fi
    fi
    if [ "$ENABLE_VM_GO" == "" ] ;then
      ENABLE_VM_GO="false"
    elif [ $ENABLE_VM_GO == "true" ] ;then
      echo "param DOCKER_GO_LOG_LEVEL $DOCKER_GO_LOG_LEVEL"
    fi
    echo "param ENABLE_VM_GO $ENABLE_VM_GO"

    # set ENABLE_VM_JAVA
    if [ "$ENABLE_VM_JAVA" == "" ] ;then
      read -p "enable vm java (YES|NO(default))" enable_vm_java
      if  [ ! -z "$enable_vm_java" ]; then
        if  [ $enable_vm_java == "yes" ] || [ $enable_vm_java == "YES" ]; then
            ENABLE_VM_JAVA="true"

            if [ "$DOCKER_JAVA_LOG_LEVEL" == "" ] ;then
                read -p "input vm java log level (DEBUG|INFO(default)|WARN|ERROR): " docker_java_log_level
                if  [ ! -z "$docker_java_log_level" ] ;then
                if  [ $docker_java_log_level == "DEBUG" ] || [ $docker_java_log_level == "INFO" ] || [ $docker_java_log_level == "WARN" ] || [ $docker_java_log_level == "ERROR" ];then
                    DOCKER_JAVA_LOG_LEVEL=$docker_java_log_level
                else
                    echo "unknown vm java log level [" $docker_java_log_level "], so use default"
                fi
              fi
            fi
            if [ "$DOCKER_JAVA_LOG_LEVEL" == "" ] ;then
              DOCKER_JAVA_LOG_LEVEL="INFO"
            fi

        fi
      fi
    fi
    if [ "$ENABLE_VM_JAVA" == "" ] ;then
      ENABLE_VM_JAVA="false"
    elif [ $ENABLE_VM_JAVA == "true" ] ;then
      echo "param DOCKER_JAVA_LOG_LEVEL $DOCKER_JAVA_LOG_LEVEL"
    fi
    echo "param ENABLE_VM_JAVA $ENABLE_VM_JAVA"

    cd "${BUILD_PATH}"
    if [ -d config ]; then
        mkdir -p backup/backup_config
        mv config  backup/backup_config/config_$(date "+%Y%m%d%H%M%S")
    fi

    mkdir -p ${BUILD_PATH}/config
    cd ${BUILD_PATH}/config

    node_count=$(ls -l $BUILD_CRYPTO_CONFIG_PATH|grep "^d"| wc -l)
    echo "config node total $node_count"
    for ((i = 1; i < $node_count + 1; i = i + 1)); do
#    for ((i = 1; i < $NODE_CNT + 1; i = i + 1)); do
        echo "begin generate node$i config..."
        mkdir -p ${BUILD_PATH}/config/node$i
        mkdir -p ${BUILD_PATH}/config/node$i/chainconfig
        cp $CONFIG_TPL_PATH/log.tpl node$i/log.yml
        xsed "s%{log_level}%$LOG_LEVEL%g" node$i/log.yml
        cp $CONFIG_TPL_PATH/chainmaker.tpl node$i/chainmaker.yml

        xsed "s%{net_port}%$(($P2P_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{rpc_port}%$(($RPC_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{monitor_port}%$(($MONITOR_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{pprof_port}%$(($PPROF_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{trusted_port}%$(($TRUSTED_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{enable_docker_go}%$ENABLE_VM_GO%g" node$i/chainmaker.yml
        xsed "s%{dockervm_container_name}%"${VM_GO_CONTAINER_NAME_PREFIX}$i"%g" node$i/chainmaker.yml
        xsed "s%{docker_vm_runtime_port}%$(($DOCKER_VM_RUNTIME_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{docker_go_engine_port}%$(($DOCKER_GO_ENGINE_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{docker_go_log_level}%$DOCKER_GO_LOG_LEVEL%g" node$i/chainmaker.yml
        xsed "s%{enable_docker_java}%$ENABLE_VM_JAVA%g" node$i/chainmaker.yml
        xsed "s%{docker_java_engine_port}%$(($DOCKER_JAVA_ENGINE_PORT+$i-1))%g" node$i/chainmaker.yml
        xsed "s%{docker_java_log_level}%$DOCKER_JAVA_LOG_LEVEL%g" node$i/chainmaker.yml

        system=$(uname)

        if [ "${system}" = "Linux" ]; then
            for ((k = $NODE_CNT; k > 0; k = k - 1)); do
                xsed "/  seeds:/a\    - \"/ip4/127.0.0.1/tcp/$(($P2P_PORT+$k-1))/p2p/{org${k}_peerid}\"" node$i/chainmaker.yml
            done
        else
            ver=$(sw_vers | grep ProductVersion | cut -d':' -f2 | sed 's/\t//g')
            version=${ver:0:2}
            if [ $version -ge 11 ]; then
                for ((k = $NODE_CNT; k > 0; k = k - 1)); do
                xsed  "/  seeds:/a\\
    - \"/ip4/127.0.0.1/tcp/$(($P2P_PORT+$k-1))/p2p/{org${k}_peerid}\"\\
" node$i/chainmaker.yml
                done
            else
                for ((k = $NODE_CNT; k > 0; k = k - 1)); do
                  xsed  "/  seeds:/a\\
                  \ \ \ \ - \"/ip4/127.0.0.1/tcp/$(($P2P_PORT+$k-1))/p2p/{org${k}_peerid}\"\\
                  " node$i/chainmaker.yml
                done
            fi
        fi

        for ((j = 1; j < $CHAIN_CNT + 1; j = j + 1)); do
            # 确定当前链的 ID 名称
            # 如果定义了全局 TARGET_CHAIN_ID 且只有一条链，则优先使用它
            if [ -n "$TARGET_CHAIN_ID" ] && [ $CHAIN_CNT -eq 1 ]; then
                CURRENT_CHAIN_ID=$TARGET_CHAIN_ID
            else
                CURRENT_CHAIN_ID="chain${j}"
            fi

            xsed "s%#\(.*\)- chainId: chain${j}%\1- chainId: ${CURRENT_CHAIN_ID}%g" node$i/chainmaker.yml
            xsed "s%#\(.*\)genesis: ../config/{org_path$j}/chainconfig/bc${j}.yml%\1genesis: ../config/{org_path$j}/chainconfig/bc${j}.yml%g" node$i/chainmaker.yml

            if  [ $NODE_CNT -eq 1 ]; then
                if [ $CONSENSUS_TYPE -eq 0 ]; then
                    cp $CONFIG_TPL_PATH/chainconfig/bc_solo.tpl node$i/chainconfig/bc$j.yml
                    xsed "s%{consensus_type}%0%g" node$i/chainconfig/bc$j.yml
                else
                    cp $CONFIG_TPL_PATH/chainconfig/bc_solo.tpl node$i/chainconfig/bc$j.yml
                    xsed "s%{consensus_type}%$CONSENSUS_TYPE%g" node$i/chainconfig/bc$j.yml
                fi
            elif [ $NODE_CNT -eq 4 ] || [ $NODE_CNT -eq 7 ]; then
                cp $CONFIG_TPL_PATH/chainconfig/bc_4_7.tpl node$i/chainconfig/bc$j.yml
                xsed "s%{consensus_type}%$CONSENSUS_TYPE%g" node$i/chainconfig/bc$j.yml
            elif [ $NODE_CNT -eq 16 ]; then
                cp $CONFIG_TPL_PATH/chainconfig/bc_16.tpl node$i/chainconfig/bc$j.yml
                xsed "s%{consensus_type}%$CONSENSUS_TYPE%g" node$i/chainconfig/bc$j.yml
            else
                cp $CONFIG_TPL_PATH/chainconfig/bc_10_13.tpl node$i/chainconfig/bc$j.yml
                xsed "s%{consensus_type}%$CONSENSUS_TYPE%g" node$i/chainconfig/bc$j.yml
            fi

            xsed "s%{chain_id}%${CURRENT_CHAIN_ID}%g" node$i/chainconfig/bc$j.yml
            xsed "s%{version}%$VERSION%g" node$i/chainconfig/bc$j.yml
            xsed "s%{genesis_time}%$GENESIS_TIME%g" node$i/chainconfig/bc$j.yml
            xsed "s%{org_top_path}%$file%g" node$i/chainconfig/bc$j.yml

            if  [ $NODE_CNT -eq 7 ] || [ $NODE_CNT -eq 13 ] || [ $NODE_CNT -eq 16 ]; then
                xsed "s%#\(.*\)- org_id:%\1- org_id:%g" node$i/chainconfig/bc$j.yml
                xsed "s%#\(.*\)node_id:%\1node_id:%g" node$i/chainconfig/bc$j.yml
                xsed "s%#\(.*\)address:%\1address:%g" node$i/chainconfig/bc$j.yml
                xsed "s%#\(.*\)root:%\1root:%g" node$i/chainconfig/bc$j.yml
                xsed "s%#\(.*\)- \"%\1- \"%g" node$i/chainconfig/bc$j.yml

                # dpos cancel kv annotation
                if  [ $CONSENSUS_TYPE -eq 5 ]; then
                    xsed "s%#\(.*\)- key:%\1- key:%g" node$i/chainconfig/bc$j.yml
                    xsed "s%#\(.*\)value:%\1value:%g" node$i/chainconfig/bc$j.yml
                fi
            fi

            # dpos update erc20.total and epochValidatorNum
            if [ $CONSENSUS_TYPE -eq 5 ]; then
               TOTAL=$(($NODE_CNT*2500000))
               xsed "s%{erc20_total}%$TOTAL%g" node$i/chainconfig/bc$j.yml
               xsed "s%{epochValidatorNum}%$NODE_CNT%g" node$i/chainconfig/bc$j.yml
               xsed "s%{epochBlockNum}%$(($NODE_CNT*3))%g" node$i/chainconfig/bc$j.yml
            fi

            if [ $NODE_CNT -eq 4 ] || [ $NODE_CNT -eq 7 ]; then
              xsed "${BC_YML_TRUST_ROOT_LINE},${BC_YML_TRUST_ROOT_LINE_END}d" node$i/chainconfig/bc$j.yml
            fi
            echo "begin node$i chain$j cert config..."

            c=0
            for file in `ls -tr $BUILD_CRYPTO_CONFIG_PATH`
            do
                c=$(($c+1))
                xsed "s%{org${c}_id}%$file%g" node$i/chainconfig/bc$j.yml

                peerId=`cat $BUILD_CRYPTO_CONFIG_PATH/$file/node/consensus1/consensus1.nodeid`
                xsed "s%{org${c}_peerid}%$peerId%g" node$i/chainconfig/bc$j.yml

                # dpos modify node address
                if  [ $CONSENSUS_TYPE -eq 5 ]; then
                    peerAddr=`cat $BUILD_CRYPTO_CONFIG_PATH/$file/user/client1/client1.addr`
                    xsed "s%{org${c}_peeraddr}%$peerAddr%g" node$i/chainconfig/bc$j.yml
                fi

                if  [ $j -eq 1 ]; then
                    xsed "s%{org${c}_peerid}%$peerId%g" node$i/chainmaker.yml
                fi
                # cp ca
                mkdir -p $BUILD_CONFIG_PATH/node$i/certs/ca/$file
                cp $BUILD_CRYPTO_CONFIG_PATH/$file/ca/ca.crt $BUILD_CONFIG_PATH/node$i/certs/ca/$file

                if  [ $c -eq $i ]; then
                    if [ $c -gt $NODE_CNT ]; then
                      xsed "s%{node_cert_path}%node\/common1\/common1.sign%g" node$i/chainmaker.yml
                      xsed "s%{net_cert_path}%node\/common1\/common1.tls%g" node$i/chainmaker.yml
                      xsed "s%{rpc_cert_path}%node\/common1\/common1.tls%g" node$i/chainmaker.yml
                    else
                      xsed "s%{node_cert_path}%node\/consensus1\/consensus1.sign%g" node$i/chainmaker.yml
                      xsed "s%{net_cert_path}%node\/consensus1\/consensus1.tls%g" node$i/chainmaker.yml
                      xsed "s%{rpc_cert_path}%node\/consensus1\/consensus1.tls%g" node$i/chainmaker.yml
                    fi
                    xsed "s%{org_path}%$file%g" node$i/chainconfig/bc$j.yml
                    xsed "s%{node_cert_path}%node\/consensus1\/consensus1.sign%g" node$i/chainmaker.yml
                    xsed "s%{net_cert_path}%node\/consensus1\/consensus1.tls%g" node$i/chainmaker.yml
                    xsed "s%{rpc_cert_path}%node\/consensus1\/consensus1.tls%g" node$i/chainmaker.yml
                    xsed "s%{org_id}%$file%g" node$i/chainmaker.yml
                    xsed "s%{org_path}%$file%g" node$i/chainmaker.yml
                    xsed "s%{org_path$j}%$file%g" node$i/chainmaker.yml

                    cp -r $BUILD_CRYPTO_CONFIG_PATH/$file/node $BUILD_CONFIG_PATH/node$i/certs
                    cp -r $BUILD_CRYPTO_CONFIG_PATH/$file/user $BUILD_CONFIG_PATH/node$i/certs
                fi

            done
        done

        echo "begin node$i trust config..."
        if  [ $NODE_CNT -eq 4 ] || [ $NODE_CNT -eq 7 ]; then
          trust_path=""
          c=0
          for file in `ls -tr $BUILD_CRYPTO_CONFIG_PATH`
          do
            c=$(($c+1))
            if  [ $c -eq $i ]; then
              trust_path=$file
              break
            fi
          done
          for ((k = 1; k < $CHAIN_CNT + 1; k = k + 1)); do
            for file in `ls -tr $BUILD_CRYPTO_CONFIG_PATH`
            do
                org_id_tmp="\ - org_id: \"${file}\""
                org_root="\ \ \ root:"
                org_root_tmp="\ \ \ \ \ - \"../config/${trust_path}/certs/ca/${file}/ca.crt\""
                if [ "${system}" = "Linux" ]; then
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\ ${org_root_tmp}" node$i/chainconfig/bc$k.yml
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\ ${org_root}" node$i/chainconfig/bc$k.yml
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\ ${org_id_tmp}"   node$i/chainconfig/bc$k.yml
                else
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\\
\ ${org_root_tmp}\\
" node$i/chainconfig/bc$k.yml
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\\
\ ${org_root}\\
"  node$i/chainconfig/bc$k.yml
                  xsed "${BC_YML_TRUST_ROOT_LINE}i\\
\ ${org_id_tmp}\\
"    node$i/chainconfig/bc$k.yml
                fi
            done
          done
        fi

    done
}


# ==============================================================================
# 分片环境生成逻辑
# ==============================================================================

# 一次性收集所有分片共用的共识、日志和 VM 配置。
function collect_common_config() {
    # 共用配置写入全局变量。
    CONSENSUS_TYPE=1
    LOG_LEVEL="INFO"
    ENABLE_VM_GO="false"
    DOCKER_GO_LOG_LEVEL="INFO"
    ENABLE_VM_JAVA="false"
    DOCKER_JAVA_LOG_LEVEL="INFO"

    # 1. 询问共识类型
    read -p "input consensus type (1-TBFT(default),3-MAXBFT,4-RAFT): " tmp
    if  [ ! -z "$tmp" ] ;then
      if [ $tmp -eq 1 ] || [ $tmp -eq 3 ] || [ $tmp -eq 4 ] ;then
        CONSENSUS_TYPE=$tmp
      else
        echo "invalid consensus type [" $tmp "], so use default"
      fi
    fi
    echo "Common Config: CONSENSUS_TYPE $CONSENSUS_TYPE"

    # 2. 询问日志级别
    read -p "input log level (DEBUG|INFO(default)|WARN|ERROR): " tmp
    if  [ ! -z "$tmp" ] ;then
      if  [ $tmp == "DEBUG" ] || [ $tmp == "INFO" ] || [ $tmp == "WARN" ] || [ $tmp == "ERROR" ];then
          LOG_LEVEL=$tmp
      else
        echo "unknown log level [" $tmp "], so use default"
      fi
    fi
    echo "Common Config: LOG_LEVEL $LOG_LEVEL"

    # 3. 询问 VM Go
    read -p "enable vm go (YES|NO(default))" enable_vm_go
    if  [ ! -z "$enable_vm_go" ]; then
      if  [ $enable_vm_go == "yes" ] || [ $enable_vm_go == "YES" ]; then
          ENABLE_VM_GO="true"
          read -p "input vm go log level (DEBUG|INFO(default)|WARN|ERROR): " docker_go_log_level
          if  [ ! -z "$docker_go_log_level" ] ;then
            if  [ $docker_go_log_level == "DEBUG" ] || [ $docker_go_log_level == "INFO" ] || [ $docker_go_log_level == "WARN" ] || [ $docker_go_log_level == "ERROR" ];then
                DOCKER_GO_LOG_LEVEL=$docker_go_log_level
            fi
          fi
      fi
    fi
    echo "Common Config: ENABLE_VM_GO $ENABLE_VM_GO"

    # 4. 询问 VM Java
    read -p "enable vm java (YES|NO(default))" enable_vm_java
    if  [ ! -z "$enable_vm_java" ]; then
      if  [ $enable_vm_java == "yes" ] || [ $enable_vm_java == "YES" ]; then
          ENABLE_VM_JAVA="true"
          read -p "input vm java log level (DEBUG|INFO(default)|WARN|ERROR): " docker_java_log_level
          if  [ ! -z "$docker_java_log_level" ] ;then
            if  [ $docker_java_log_level == "DEBUG" ] || [ $docker_java_log_level == "INFO" ] || [ $docker_java_log_level == "WARN" ] || [ $docker_java_log_level == "ERROR" ];then
                DOCKER_JAVA_LOG_LEVEL=$docker_java_log_level
            fi
          fi
      fi
    fi
    echo "Common Config: ENABLE_VM_JAVA $ENABLE_VM_JAVA"

    # 导出变量，确保在 generate_config 中可见
    export CONSENSUS_TYPE LOG_LEVEL ENABLE_VM_GO DOCKER_GO_LOG_LEVEL ENABLE_VM_JAVA DOCKER_JAVA_LOG_LEVEL
}

function generate_one_shard() {
    local DIR_NAME=$1
    local NODE_COUNT=$2
    local SHARD_TYPE=$3
    local TARGET_CHAIN_ID=$4  # 新增：目标 ChainID

    # 设置当前分片的节点数与链数。
    NODE_CNT=$NODE_COUNT
    CHAIN_CNT=1 

    # 动态修改 BUILD_PATH
    BUILD_PATH="${PROJECT_PATH}/build/${DIR_NAME}"
    BUILD_CRYPTO_CONFIG_PATH="${BUILD_PATH}/crypto-config"
    BUILD_CONFIG_PATH="${BUILD_PATH}/config"
    
	# 导出 TARGET_CHAIN_ID 供 generate_config 使用
	export TARGET_CHAIN_ID

	echo "=================================================================="
	echo "正在生成分片: ${DIR_NAME} (类型: ${SHARD_TYPE}, 链ID: ${TARGET_CHAIN_ID})"
	echo "节点数量: ${NODE_CNT}"
	echo "起始端口: P2P=${P2P_PORT}, RPC=${RPC_PORT}"
	echo "输出目录: ${BUILD_PATH}"
	echo "=================================================================="

	# 清理旧目录
	rm -rf "${BUILD_PATH}"
	mkdir -p "${BUILD_PATH}"

	# 校验参数并生成证书。
	check_params
	generate_certs
	# 使用当前分片的全局参数生成节点配置。
	generate_config

	# ==========================================================================
	# 生成 sharding.yml
	# ==========================================================================
	echo "正在为 ${DIR_NAME} 生成 sharding.yml..."
	SHARDING_TPL="${CONFIG_TPL_PATH}/sharding.tpl"

	if [ ! -f "$SHARDING_TPL" ]; then
		echo "Warning: ${SHARDING_TPL} not found! Skipping sharding.yml generation."
	else
		# 遍历生成的节点目录
		for ((idx = 1; idx <= NODE_CNT; idx++)); do
			NODE_CONFIG_DIR="${BUILD_CONFIG_PATH}/node${idx}"
			TARGET_YAML="${NODE_CONFIG_DIR}/sharding.yml"

			# 复制模板
			cp "${SHARDING_TPL}" "${TARGET_YAML}"

			# 替换变量
			xsed "s%{shard_id}%${DIR_NAME}%g" "${TARGET_YAML}"
			xsed "s%{shard_type}%${SHARD_TYPE}%g" "${TARGET_YAML}"
			xsed "s%{start_height}%${SCHED_START_HEIGHT}%g" "${TARGET_YAML}"
			xsed "s%{business_batch}%${SCHED_BUSINESS_BATCH}%g" "${TARGET_YAML}"
			xsed "s%{bridge_batch}%${SCHED_BRIDGE_BATCH}%g" "${TARGET_YAML}"
			xsed "s%{total_batch}%${SCHED_TOTAL_BATCH}%g" "${TARGET_YAML}"
			BUSINESS_SHARDS_YAML=""
			for ((biz_idx=1; biz_idx<=SHARD_CNT; biz_idx++)); do
				if [ -z "$BUSINESS_SHARDS_YAML" ]; then
					BUSINESS_SHARDS_YAML="- business-shard${biz_idx}"
				else
					BUSINESS_SHARDS_YAML="${BUSINESS_SHARDS_YAML}\\n    - business-shard${biz_idx}"
				fi
			done
			if [ -z "$BUSINESS_SHARDS_YAML" ]; then
				xsed "s%{business_shards}%%g" "${TARGET_YAML}"
			else
				xsed "s%{business_shards}%${BUSINESS_SHARDS_YAML}%g" "${TARGET_YAML}"
			fi

			# 同步网络配置：所有节点均启用，同编号节点（同机构）互联
			# 业务分片节点 idx 连接桥接分片节点 idx（同机构）
			# 桥接分片节点不配置种子，等待各业务分片同编号节点主动连入（libp2p 连接双向可用）
			SYNC_PORT=$(($P2P_PORT + $idx - 1 + 2000))
			SYNC_ENABLE="true"
			SYNC_SEEDS=""

			if [ "$SHARD_TYPE" == "BUSINESS" ]; then
				# 桥接分片先于业务分片生成，此时其 PeerID 文件已存在
				# 桥接分片节点 idx 的 sync 端口 = 桥接基础 P2P 端口 + (idx-1) + 2000
				BRIDGE_BASE_P2P_PORT=$((11301 + SHARD_CNT * BIZ_NODE_CNT))
				BRIDGE_SYNC_PORT_K=$(($BRIDGE_BASE_P2P_PORT + $idx - 1 + 2000))
				BRIDGE_NODEID_FILE="${PROJECT_PATH}/build/bridge-shard/crypto-config/wx-org${idx}.chainmaker.org/node/consensus1/consensus1.nodeid"

				if [ -f "$BRIDGE_NODEID_FILE" ]; then
					BRIDGE_PEER_ID=$(cat "$BRIDGE_NODEID_FILE")
					SYNC_SEEDS="- \"/ip4/127.0.0.1/tcp/${BRIDGE_SYNC_PORT_K}/p2p/${BRIDGE_PEER_ID}\""
				else
					echo "Warning: Bridge Node${idx} PeerID file not found at $BRIDGE_NODEID_FILE"
					SYNC_SEEDS="- \"/ip4/127.0.0.1/tcp/${BRIDGE_SYNC_PORT_K}\""
				fi
			fi
			# BRIDGE 类型：不填 seeds，libp2p 监听端口，等待业务分片节点主动连入

			xsed "s%{sync_enable}%${SYNC_ENABLE}%g" "${TARGET_YAML}"
			xsed "s%{sync_port}%${SYNC_PORT}%g" "${TARGET_YAML}"

			# 处理 Seeds 换行符问题
			if [ -z "$SYNC_SEEDS" ]; then
				xsed "s%{sync_seeds}%%g" "${TARGET_YAML}"
			else
				xsed "s%{sync_seeds}%${SYNC_SEEDS}%g" "${TARGET_YAML}"
			fi
		done
	fi

	# 更新端口累加器 (连续递增)
    P2P_PORT=$((P2P_PORT + NODE_CNT))
    RPC_PORT=$((RPC_PORT + NODE_CNT))
    MONITOR_PORT=$((MONITOR_PORT + NODE_CNT))
    PPROF_PORT=$((PPROF_PORT + NODE_CNT))
    TRUSTED_PORT=$((TRUSTED_PORT + NODE_CNT))
    DOCKER_VM_RUNTIME_PORT=$((DOCKER_VM_RUNTIME_PORT + NODE_CNT))
    DOCKER_GO_ENGINE_PORT=$((DOCKER_GO_ENGINE_PORT + NODE_CNT))
    DOCKER_JAVA_ENGINE_PORT=$((DOCKER_JAVA_ENGINE_PORT + NODE_CNT))
}

# ==============================================================================
# 主交互逻辑
# ==============================================================================

echo "===> 欢迎使用长安链分片环境构建工具 (Prepare Sharding)"

# 1. 获取分片结构信息
read -p "请输入业务分片(Business Shard)的数量 (默认: 1): " INPUT_SHARD_CNT
SHARD_CNT=${INPUT_SHARD_CNT:-1}

read -p "请输入每个业务分片的节点数量 (支持 1/4/7/10/13/16, 默认: 4): " INPUT_BIZ_NODE_CNT
BIZ_NODE_CNT=${INPUT_BIZ_NODE_CNT:-4}

read -p "请输入桥接分片(Bridge Shard)的节点数量 (支持 1/4/7/10/13/16, 默认: 4): " INPUT_BRIDGE_NODE_CNT
BRIDGE_NODE_CNT=${INPUT_BRIDGE_NODE_CNT:-4}

# 分时调度配置
read -p "请输入分时调度起始高度 (默认: 10): " INPUT_START_HEIGHT
SCHED_START_HEIGHT=${INPUT_START_HEIGHT:-10}

read -p "请输入业务分片连续出块数量 (默认: 3): " INPUT_BUSINESS_BATCH
SCHED_BUSINESS_BATCH=${INPUT_BUSINESS_BATCH:-3}

read -p "请输入桥接分片连续出块数量 (默认: 2): " INPUT_BRIDGE_BATCH
SCHED_BRIDGE_BATCH=${INPUT_BRIDGE_BATCH:-2}

read -p "请输入动态调度总批次数 N+M，0 表示禁用动态调度 (默认: 6): " INPUT_TOTAL_BATCH
SCHED_TOTAL_BATCH=${INPUT_TOTAL_BATCH:-6}

# 2. 收集通用配置 (一次性)
collect_common_config

# Validate ownership before deleting any existing build directory.
if [[ ! "$SHARD_CNT" =~ ^[1-9][0-9]*$ ]]; then
    echo "error: business shard count must be a positive integer" >&2
    exit 1
fi
business_shard_ids=()
for ((shard_idx=1; shard_idx<=SHARD_CNT; shard_idx++)); do
    business_shard_ids+=("business-shard${shard_idx}")
done
printf -v policy_shards '%s,' "${business_shard_ids[@]}"
(
    cd "${PROJECT_PATH}"
    go run -mod=readonly ./tools/sharding-config --check-policy \
        --policy "$STATE_POLICY" --business-shards "${policy_shards%,}"
)

# 3. 初始化端口计数器 (从默认起始值开始)
P2P_PORT=11301
RPC_PORT=12301
DOCKER_VM_RUNTIME_PORT=32351
DOCKER_GO_ENGINE_PORT=22351
DOCKER_JAVA_ENGINE_PORT=23351

echo "-----------------------------------------------------------------"
echo "配置概览:"
echo "  业务分片数量: ${SHARD_CNT}"
echo "  业务分片节点: ${BIZ_NODE_CNT}"
echo "  桥接分片节点: ${BRIDGE_NODE_CNT}"
echo "  分时调度起始高度: ${SCHED_START_HEIGHT}"
echo "  业务分片批次数: ${SCHED_BUSINESS_BATCH}"
echo "  桥接分片批次数: ${SCHED_BRIDGE_BATCH}"
echo "  动态调度总批次 T: ${SCHED_TOTAL_BATCH}"
echo "  状态归属策略: ${STATE_POLICY}"
echo "-----------------------------------------------------------------"

# 清理整个 build 目录
echo "正在清理旧的 build 目录..."
rm -rf "${PROJECT_PATH}/build"

# 4. 循环生成
# 先生成桥接分片，供业务分片的种子配置引用其 PeerID。
# 端口按业务分片在前、桥接分片在后的顺序分配。

# 保存初始端口
BASE_P2P_PORT=11301
BASE_RPC_PORT=12301
BASE_VM_PORT=32351
BASE_GO_PORT=22351
BASE_JAVA_PORT=23351

# --- 先生成 Bridge Shard ---
# 计算 Bridge 的端口偏移量: 所有 Business 分片的节点总数
TOTAL_BIZ_NODES=$((SHARD_CNT * BIZ_NODE_CNT))

P2P_PORT=$((BASE_P2P_PORT + TOTAL_BIZ_NODES))
RPC_PORT=$((BASE_RPC_PORT + TOTAL_BIZ_NODES))
DOCKER_VM_RUNTIME_PORT=$((BASE_VM_PORT + TOTAL_BIZ_NODES))
DOCKER_GO_ENGINE_PORT=$((BASE_GO_PORT + TOTAL_BIZ_NODES))
DOCKER_JAVA_ENGINE_PORT=$((BASE_JAVA_PORT + TOTAL_BIZ_NODES))

generate_one_shard "bridge-shard" ${BRIDGE_NODE_CNT} "BRIDGE" "chain0"

# --- 再生成 Business Shard ---
# 重置为初始端口，然后循环生成
P2P_PORT=${BASE_P2P_PORT}
RPC_PORT=${BASE_RPC_PORT}
DOCKER_VM_RUNTIME_PORT=${BASE_VM_PORT}
DOCKER_GO_ENGINE_PORT=${BASE_GO_PORT}
DOCKER_JAVA_ENGINE_PORT=${BASE_JAVA_PORT}

for ((shard_idx=1; shard_idx<=SHARD_CNT; shard_idx++)); do
    generate_one_shard "business-shard${shard_idx}" ${BIZ_NODE_CNT} "BUSINESS" "chain${shard_idx}"
done


echo "===> Validate sharding policies and generate public source-certificate pins"
config_args=(--build-dir "${PROJECT_PATH}/build" --policy "$STATE_POLICY" --write)
(
    cd "${PROJECT_PATH}"
    go run -mod=readonly ./tools/sharding-config "${config_args[@]}"
)
echo "===> All shard configurations generated under ${PROJECT_PATH}/build/"
echo "Gateway routing must be configured explicitly; gateway never stores user signing keys."

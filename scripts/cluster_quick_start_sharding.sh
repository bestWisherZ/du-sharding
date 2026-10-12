#!/usr/bin/env bash
#
# Copyright (C) BABEC. All rights reserved.
# Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

set -e

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
RELEASE_PATH=${PROJECT_PATH}/build/release
START_TYPE=$1

GATEWAY_BIN=${PROJECT_PATH}/bin/chainmaker-gateway
GATEWAY_CONF=${PROJECT_PATH}/module/sharding/gateway/gateway.yml
GATEWAY_LOG=${PROJECT_PATH}/build/gateway.log
GATEWAY_PID=${PROJECT_PATH}/build/gateway.pid

function start_gateway() {
    if [ ! -f "$GATEWAY_BIN" ]; then
        echo "Warning: gateway binary not found: $GATEWAY_BIN, skipping"
        return
    fi
    if [ ! -f "$GATEWAY_CONF" ]; then
        echo "Warning: gateway config not found: $GATEWAY_CONF, skipping"
        return
    fi
    # 如果已有旧进程，先杀掉
    if [ -f "$GATEWAY_PID" ]; then
        old_pid=$(cat "$GATEWAY_PID")
        if kill -0 "$old_pid" 2>/dev/null; then
            echo "===> Stopping old gateway (pid=$old_pid)"
            kill "$old_pid" 2>/dev/null || true
            sleep 1
        fi
        rm -f "$GATEWAY_PID"
    fi
    echo "===> Starting Gateway (listen: 0.0.0.0:14301)"
    nohup "$GATEWAY_BIN" --conf "$GATEWAY_CONF" > "$GATEWAY_LOG" 2>&1 &
    echo $! > "$GATEWAY_PID"
    echo "    Gateway PID=$(cat $GATEWAY_PID)  log=$GATEWAY_LOG"
}

# 基础分屏函数 (4宫格)
function split_window() {
    tmux splitw -v -p 50
    tmux splitw -h -p 50
    tmux selectp -t 0
    tmux splitw -h -p 50
}

function cluster_start() {
    echo "===> Starting chainmaker cluster (Sharding Mode)"

    if [[ $START_TYPE == "normal" ]] ; then
        start_normal
    else
        start_tmux
    fi
}

function prepare() {
    echo "===> Unzip chainmaker installation packages"
    if [ ! -d "$RELEASE_PATH" ]; then
        echo "Error: Release path not found: $RELEASE_PATH"
        exit 1
    fi

    # 遍历 release 下的分片目录
    for shard_dir in $(ls $RELEASE_PATH); do
        SHARD_FULL_PATH="$RELEASE_PATH/$shard_dir"
        
        # 排除非目录或特殊目录
        if [ ! -d "$SHARD_FULL_PATH" ] || [ "$shard_dir" == "backup" ]; then
            continue
        fi

        echo "Checking shard: $shard_dir"
        cd $SHARD_FULL_PATH
        
        # 根据 chainmaker-* 节点目录判断是否已解压，排除压缩包。
        has_unzipped=false
        for item in *; do
            if [[ -d "$item" ]] && [[ "$item" == chainmaker* ]]; then
                has_unzipped=true
                break
            fi
        done

        if [ "$has_unzipped" = true ]; then
            echo "  - Packages already unzipped in $shard_dir"
        else
            echo "  - Unzipping packages in $shard_dir..."
            for file in *.tar.gz; do
                # 排除 crypto-config 备份包，只解压节点包
                if [[ "$file" == chainmaker-*.tar.gz ]]; then
                    tar -zxvf "$file" > /dev/null 2>&1
                fi
            done
        fi
    done
}

function start_normal() {
    for shard_dir in $(ls $RELEASE_PATH); do
        SHARD_FULL_PATH="$RELEASE_PATH/$shard_dir"
        if [ ! -d "$SHARD_FULL_PATH" ] || [ "$shard_dir" == "backup" ]; then continue; fi

        echo "===> Starting Shard: $shard_dir"
        cd $SHARD_FULL_PATH
        
        for node_dir in chainmaker-*; do
            if [ -d "$node_dir" ]; then
                echo "START ==> $shard_dir / $node_dir"
                cd "$node_dir/bin" && ./start.sh -f full && cd - > /dev/null
            fi
        done
    done

    start_gateway
}

function start_tmux() {
    SESSION_NAME="chainmaker-sharding"
    
    # 如果会话已存在，先杀掉
    tmux kill-session -t $SESSION_NAME 2>/dev/null || true
    
    # 创建新会话 (先不依附，后台创建)
    # 初始窗口使用 overview 名称，处理首个分片时重命名。
    tmux new-session -d -s $SESSION_NAME -n "overview"
    
    # 标记：是否是第一个有效的分片窗口 (用于决定是否覆盖初始窗口)
    is_first_window=true

    for shard_dir in $(ls $RELEASE_PATH); do
        SHARD_FULL_PATH="$RELEASE_PATH/$shard_dir"
        if [ ! -d "$SHARD_FULL_PATH" ] || [ "$shard_dir" == "backup" ]; then continue; fi

        # 获取该分片下的所有节点目录
        cd $SHARD_FULL_PATH
        # 使用数组存储节点目录名
        node_dirs=($(ls -d chainmaker-* 2>/dev/null | grep -v "\.tar\.gz"))
        
        if [ ${#node_dirs[@]} -eq 0 ]; then
            echo "Warning: No unzipped nodes found in $shard_dir"
            continue
        fi

        # 为当前分片创建一个新窗口 (或者重命名第一个窗口)
        if [ "$is_first_window" = true ]; then
            tmux rename-window -t $SESSION_NAME:0 "$shard_dir"
            is_first_window=false
        else
            tmux new-window -t $SESSION_NAME -n "$shard_dir"
        fi

        # 每个分片使用四个面板，分别启动前四个节点。
        split_window

        # 遍历节点并启动
        cnt=0
        for node_dir in "${node_dirs[@]}"; do
            # 选择面板 (0-3)
            pane_idx=$((cnt % 4))
            tmux select-pane -t $pane_idx
            
            # 构造启动命令
            # 1. 设置 PS1 提示符，显示 [Shard-Node]
            # 2. 进入目录并启动
            # 3. 打印版本、日志和进程信息
            # 提取简单的节点名 (从 chainmaker-v2.4.0-business-shard1-wx-org1... 中提取 org1)
            # 这里的提取逻辑依赖包名格式
            simple_name=$(echo $node_dir | grep -o 'wx-org[0-9]*' || echo "node$((cnt+1))")
            
            CMD="export PS1=\"\[\e[32m\][${shard_dir}-${simple_name}] \W> \[\e[m\]\" && \
                 cd ${SHARD_FULL_PATH}/${node_dir}/bin && \
                 ./start.sh -f full && \
                 reset && \
                 ./chainmaker version && \
                 echo \"Waiting for startup...\" && sleep 3 && \
                 echo -e \"\\n>>> Latest System Log <<<\" && tail -n 2 ../log/system.log && \
                 echo -e \"\\n>>> Process Info <<<\" && ps axo pid,cmd | grep -v grep | grep \"chainmaker start\" | grep \"${node_dir}\""

            tmux send-keys "$CMD" C-m
            
            cnt=$((cnt+1))
            
            # 四个面板使用完毕后结束当前分片的节点启动循环。
            if [ $cnt -ge 4 ]; then
                break
            fi
        done
        
        # 切回左上角面板
        tmux select-pane -t 0
    done

    # 为 gateway 单独开一个窗口
    tmux new-window -t $SESSION_NAME -n "gateway"
    tmux send-keys "export PS1=\"\[\e[36m\][gateway] \W> \[\e[m\]\" && \
        $GATEWAY_BIN --conf $GATEWAY_CONF 2>&1 | tee $GATEWAY_LOG" C-m

    # 选中第一个窗口并接入会话
    tmux select-window -t $SESSION_NAME:0
    tmux attach-session -t $SESSION_NAME
}

prepare
cluster_start

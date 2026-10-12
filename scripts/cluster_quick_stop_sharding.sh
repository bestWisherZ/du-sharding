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
GATEWAY_PID=${PROJECT_PATH}/build/gateway.pid
ARG1=$1

function stop_gateway() {
    if [ ! -f "$GATEWAY_PID" ]; then
        echo "[gateway] no PID file found, gateway may not be running"
        return
    fi
    pid=$(cat "$GATEWAY_PID")
    if kill -0 "$pid" 2>/dev/null; then
        echo "===> Stopping Gateway (pid=$pid)"
        kill "$pid"
        sleep 1
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null || true
        fi
        echo "    Gateway stopped."
    else
        echo "[gateway] process $pid not found, already stopped"
    fi
    rm -f "$GATEWAY_PID"
}

function cluster_stop() {
    echo "===> Stoping chainmaker cluster (Sharding Mode)"
    # 先停 gateway（在节点之前，避免停节点后 gateway 还在接收请求）
    stop_gateway

    # stop all shard nodes
    stop_all
    
    # Kill tmux session if exists
    SESSION_NAME="chainmaker-sharding"
    if tmux has-session -t $SESSION_NAME 2>/dev/null; then
        echo "Detected running tmux session: $SESSION_NAME. Killing it..."
        tmux kill-session -t $SESSION_NAME
        echo "Tmux session killed."
    fi

    # second clean data and log if input clean arg
    if [[ $ARG1 == "clean" ]] ; then
        clean
    fi
}

function stop_all() {
    stop_count=0
    
    if [ ! -d "$RELEASE_PATH" ]; then
        echo "[WARN] Release path not found: $RELEASE_PATH"
        return
    fi

    # 遍历分片目录
    for shard_dir in $(ls $RELEASE_PATH); do
        SHARD_FULL_PATH="$RELEASE_PATH/$shard_dir"
        
        # 排除非目录或特殊目录
        if [ ! -d "$SHARD_FULL_PATH" ] || [ "$shard_dir" == "backup" ]; then
            continue
        fi

        echo "Checking shard: $shard_dir"
        cd $SHARD_FULL_PATH

        # 遍历节点目录
        for node_dir in chainmaker-*; do
            if [ -d "$node_dir" ]; then
                echo "STOP ==> $shard_dir / $node_dir"
                # 检查 bin 目录是否存在，防止误操作
                if [ -d "$node_dir/bin" ]; then
                    cd "$node_dir/bin" && ./stop.sh full && cd - > /dev/null
                    stop_count=$((stop_count + 1))
                else
                    echo "[WARN] bin directory not found in $node_dir"
                fi
            fi
        done
    done

    if [ $stop_count -eq 0 ]; then
        echo "[WARN] No process is stopped. You need to check whether the chainmaker is running on the path."
    else
        echo " $stop_count chainmaker process is stopped."
    fi
}

function clean() {
    echo "===> Cleaning data and logs"
    
    if [ ! -d "$RELEASE_PATH" ]; then
        return
    fi

    # 遍历分片目录
    for shard_dir in $(ls $RELEASE_PATH); do
        SHARD_FULL_PATH="$RELEASE_PATH/$shard_dir"
        if [ ! -d "$SHARD_FULL_PATH" ] || [ "$shard_dir" == "backup" ]; then continue; fi

        cd $SHARD_FULL_PATH
        
        # 遍历节点目录
        for node_dir in chainmaker-*; do
            if [ -d "$node_dir" ]; then
                echo "CLEAN data and log ==> $shard_dir / $node_dir"
                cd "$node_dir"
                # 安全清理：只清理 data 目录和 log 下的文件
                rm -rf data
                # 如果 log 目录存在，清空其内容但保留目录
                if [ -d "log" ]; then
                    rm -rf log/*
                fi
                cd - > /dev/null
            fi
        done
    done
}

cluster_stop

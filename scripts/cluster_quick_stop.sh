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
ARG1=$1

function cluster_stop() {
    echo "===> Stoping chainmaker cluster"
    # first stop node
    stop_all
    # second clean data and log if input clean arg
    if [[ $ARG1 == "clean" ]] ; then
        clean
    fi
}

function stop_all() {
    stop_count=0
    cd $RELEASE_PATH
    for file in `ls $RELEASE_PATH`
    do
        if [ -d $file ]; then
            echo "STOP ==> " $RELEASE_PATH/$file
            cd $file/bin && ./stop.sh full && cd - > /dev/null
            stop_count=$((stop_count + 1))
        fi
    done
    if [ $stop_count -eq 0 ]; then
        echo "[WARN] No process is stopped. You need to check whether the chainmaker is running on the path."
    else
        echo " $stop_count chainmaker process is stopped."
    fi
}

function clean() {
    cd $RELEASE_PATH
    for file in `ls $RELEASE_PATH`
    do
        if [ -d $file ]; then
            echo "CLEAN data and log ==> " $RELEASE_PATH/$file/data $RELEASE_PATH/$file/log
            cd $file && rm -rf data && rm -rf log/* && cd - > /dev/null
        fi
    done
}

cluster_stop

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
BUILD_PATH=${PROJECT_PATH}/build
# RELEASE_PATH 和其他 PATH 在 package_one_shard 函数中动态设置
# RELEASE_PATH=${PROJECT_PATH}/build/release
# BACKUP_PATH=${PROJECT_PATH}/build/backup
# BUILD_CRYPTO_CONFIG_PATH=${BUILD_PATH}/crypto-config
# BUILD_CONFIG_PATH=${BUILD_PATH}/config

VERSION=v2.4.0_alpha
DATETIME=$(date "+%Y%m%d%H%M%S")
PLATFORM=$(uname -m)
system=$(uname)
PACKAGE_ORG=${PACKAGE_ORG:-}

function xsed() {

    if [ "${system}" = "Linux" ]; then
        sed -i "$@"
    else
        sed -i '' "$@"
    fi
}

function check_env() {
    if  [ ! -d $BUILD_PATH ] ;then
        echo $BUILD_PATH" is missing"
        exit 1
    fi
}

function build() {
    cd $PROJECT_PATH
    echo "build chainmaker ${PROJECT_PATH}..."
    make
}

function build_gateway() {
    echo "build chainmaker-gateway..."
    cd $PROJECT_PATH
    go build -o $PROJECT_PATH/bin/chainmaker-gateway ./module/sharding/gateway/cmd/
    echo "chainmaker-gateway built: $PROJECT_PATH/bin/chainmaker-gateway"
}

# 封装单个分片的打包逻辑
function package_one_shard() {
    local SHARD_NAME=$1
    local CURRENT_SHARD_PATH="${BUILD_PATH}/${SHARD_NAME}"
    # 修改：将 release 路径指向分片专属子目录
    local CURRENT_RELEASE_PATH="${BUILD_PATH}/release/${SHARD_NAME}"
    local CURRENT_CRYPTO_CONFIG_PATH="${CURRENT_SHARD_PATH}/crypto-config"
    local CURRENT_CONFIG_PATH="${CURRENT_SHARD_PATH}/config"

    # 检查分片目录结构是否完整
    if [ ! -d "$CURRENT_CONFIG_PATH" ] || [ ! -d "$CURRENT_CRYPTO_CONFIG_PATH" ]; then
        echo "Warning: Shard ${SHARD_NAME} config or crypto-config missing, skipping..."
        return
    fi

    echo "=================================================================="
    echo "Packaging Shard: ${SHARD_NAME}"
    echo "Config Path: ${CURRENT_CONFIG_PATH}"
    echo "=================================================================="

    mkdir -p $CURRENT_RELEASE_PATH
    cd $CURRENT_RELEASE_PATH

    # 为每个分片单独打包证书 (带分片前缀)
    echo "tar zcf crypto-config for ${SHARD_NAME}..."
    # 在构建目录中打包，保留相对路径结构。
    tar -zcf ${SHARD_NAME}-crypto-config-$DATETIME.tar.gz -C ${CURRENT_SHARD_PATH} crypto-config

    c=0
    dirNames[0]=""
    for file in $(find "$CURRENT_CRYPTO_CONFIG_PATH" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V)
    do
        c=$(($c+1))
        if [ -n "$PACKAGE_ORG" ] && [ "$file" != "$PACKAGE_ORG" ]; then
            continue
        fi
        node_index=$c
        if [[ "$file" =~ wx-org([0-9]+)\.chainmaker\.org ]]; then
            node_index="${BASH_REMATCH[1]}"
        fi
        # 修改包名，增加 SHARD_NAME 前缀
        chainmaker_file=chainmaker-$VERSION-$SHARD_NAME-$file
        dirNames[$c]=$chainmaker_file
        
        mkdir $chainmaker_file
        mkdir $chainmaker_file/bin
        mkdir $chainmaker_file/lib
        mkdir -p $chainmaker_file/config/$file
        mkdir $chainmaker_file/log
        
        cp $PROJECT_PATH/bin/chainmaker   $chainmaker_file/bin
        cp $CURRENT_PATH/bin/start.sh     $chainmaker_file/bin
        cp $CURRENT_PATH/bin/stop.sh      $chainmaker_file/bin
        cp $CURRENT_PATH/bin/restart.sh   $chainmaker_file/bin
        cp $CURRENT_PATH/bin/version.sh   $chainmaker_file/bin
        cp $CURRENT_PATH/bin/docker-vm-standalone-start.sh   $chainmaker_file/bin
        cp $CURRENT_PATH/bin/docker-vm-standalone-stop.sh   $chainmaker_file/bin
        cp $CURRENT_PATH/service/*        $chainmaker_file/bin
        
        if [ "${system}" = "Linux" ]; then
          cp -r $PROJECT_PATH/main/libwasmer_runtime_c_api.so     $chainmaker_file/lib/libwasmer.so
          cp -r $PROJECT_PATH/main/prebuilt/linux/wxdec           $chainmaker_file/lib/
        else
          cp -r $PROJECT_PATH/main/libwasmer.dylib                $chainmaker_file/lib/
          cp -r $PROJECT_PATH/main/prebuilt/mac/wxdec             $chainmaker_file/lib/
        fi
        
        chmod 644 $chainmaker_file/lib/*
        chmod 700 $chainmaker_file/lib/wxdec
        chmod 700 $chainmaker_file/bin/*
        
        # 从当前分片的配置目录复制节点配置。
        cp -r $CURRENT_CONFIG_PATH/node$node_index/* $chainmaker_file/config/$file
        
        # 替换 org_id
        xsed "s%{org_id}%$file%g"         $chainmaker_file/bin/start.sh
        xsed "s%{org_id}%$file%g"         $chainmaker_file/bin/stop.sh
        xsed "s%{org_id}%$file%g"         $chainmaker_file/bin/restart.sh
        xsed "s%{org_id}%$file%g"         $chainmaker_file/bin/run.sh
        
        d=$(date "+%Y%m%d%H%M%S")
        xsed "s%{tagName}%name-$d%g"         $chainmaker_file/bin/*.sh
        
        echo "tar zcf ${chainmaker_file}..."
        tar -zcf chainmaker-$VERSION-$SHARD_NAME-$file-$DATETIME-$PLATFORM.tar.gz $chainmaker_file &
    done
    
    echo "wait tar for ${SHARD_NAME}..."
    wait
    
    for dirName in ${dirNames[@]}
    do
      rm -rf $CURRENT_RELEASE_PATH/$dirName
    done
}

function package_all() {
    # 备份旧 release (如果有)
    RELEASE_PATH=${PROJECT_PATH}/build/release
    BACKUP_PATH=${PROJECT_PATH}/build/backup
    if [ -d $RELEASE_PATH ]; then
        mkdir -p $BACKUP_PATH/backup_release
        mv $RELEASE_PATH $BACKUP_PATH/backup_release/release_$(date "+%Y%m%d%H%M%S")
    fi

    # 遍历 build 下的所有子目录
    for shard_dir in $(find "$BUILD_PATH" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V); do
        # 排除非分片目录 (如 release, backup)
        if [ "$shard_dir" != "release" ] && [ "$shard_dir" != "backup" ]; then
             # 进一步确认它是不是一个有效的分片目录 (包含 config 子目录)
             if [ -d "$BUILD_PATH/$shard_dir/config" ]; then
                 package_one_shard $shard_dir
             fi
        fi
    done
}

check_env
build
build_gateway
package_all

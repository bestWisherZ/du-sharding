#!/usr/bin/env bash
#
# Sync sharding certs from build/ to tools/cmc/testdata/
#

set -e

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
BUILD_PATH=${PROJECT_PATH}/build
CMC_TESTDATA_PATH=${PROJECT_PATH}/tools/cmc/testdata

function sync_certs() {
    echo "===> Syncing certs to CMC testdata..."
    
    if [ ! -d "$BUILD_PATH" ]; then
        echo "Error: Build path not found: $BUILD_PATH"
        exit 1
    fi

    # 创建 CMC 证书根目录。
    mkdir -p "${CMC_TESTDATA_PATH}"

    # 2. 遍历所有分片
    for shard_dir in $(ls $BUILD_PATH); do
        SRC_CRYPTO_PATH="${BUILD_PATH}/${shard_dir}/crypto-config"
        
        # 检查是否为有效的分片目录
        if [ -d "$SRC_CRYPTO_PATH" ]; then
            TARGET_DIR="${CMC_TESTDATA_PATH}/${shard_dir}"
            
            echo "Syncing ${shard_dir} -> ${TARGET_DIR} ..."
            
            # 创建目标分片目录
            mkdir -p "${TARGET_DIR}"
            
            # 按分片目录隔离各自的 crypto-config。
            cp -rf "${SRC_CRYPTO_PATH}" "${TARGET_DIR}/"
            
            echo "  [OK] Certs copied."
        fi
    done
    
    echo "===> Sync complete."
    echo "CMC certs path: ${CMC_TESTDATA_PATH}"
}

sync_certs

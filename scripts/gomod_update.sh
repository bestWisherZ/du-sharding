#!/usr/bin/env bash
#
# Copyright (C) BABEC. All rights reserved.
# Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

set -x
BRANCH=$1
LWS_BRANCH='v1.2.1'
PRE_BRANCH="v2.3.4_qc"
LAST_PRE_BRANCH="v2.3.5_qc"
NET_COMMON_BRANCH='v1.2.7_qc'
NET_LIBP2P_BRANCH='v1.2.9_qc'
NET_LIQUID_BRANCH='v1.1.4_qc'

if [[ ! -n $BRANCH ]]; then
				  BRANCH="v2.4.0_qc"
fi
cd ..

go get chainmaker.org/chainmaker/lws@${LWS_BRANCH}
go get chainmaker.org/chainmaker/chainconf/v2@${BRANCH}
go get chainmaker.org/chainmaker/common/v2@${BRANCH}
go get chainmaker.org/chainmaker/localconf/v2@${BRANCH}
go get chainmaker.org/chainmaker/logger/v2@${BRANCH}
go get chainmaker.org/chainmaker/pb-go/v2@${BRANCH}
go get chainmaker.org/chainmaker/protocol/v2@${BRANCH}
go get chainmaker.org/chainmaker/utils/v2@${BRANCH}
go get chainmaker.org/chainmaker/net-common@${NET_COMMON_BRANCH}
go get chainmaker.org/chainmaker/net-libp2p@${NET_LIBP2P_BRANCH}
go get chainmaker.org/chainmaker/net-liquid@${NET_LIQUID_BRANCH}
go get chainmaker.org/chainmaker/consensus-dpos/v2@${BRANCH}
go get chainmaker.org/chainmaker/consensus-raft/v2@${BRANCH}
go get chainmaker.org/chainmaker/consensus-solo/v2@${BRANCH}
go get chainmaker.org/chainmaker/consensus-utils/v2@${BRANCH}
go get chainmaker.org/chainmaker/consensus-maxbft/v2@${BRANCH}
go get chainmaker.org/chainmaker/consensus-tbft/v2@${BRANCH}
go get chainmaker.org/chainmaker/store/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-docker-go/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-engine/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-evm/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-gasm/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-native/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-wasmer/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm-wxvm/v2@${BRANCH}
go get chainmaker.org/chainmaker/vm/v2@${BRANCH}
go get chainmaker.org/chainmaker/txpool-batch/v2@${BRANCH}
go get chainmaker.org/chainmaker/txpool-normal/v2@${BRANCH}
go get chainmaker.org/chainmaker/txpool-single/v2@${BRANCH}
go get chainmaker.org/chainmaker/sync/v2@${BRANCH}
go get chainmaker.org/chainmaker/sdk-go/v2@${BRANCH}

go mod tidy

make
make cmc

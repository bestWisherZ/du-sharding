#!/bin/bash
#
# Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

if [ "$1" = "" ]; then
  vm_type="go"
else
  vm_type="$1"
fi

if [[ "$vm_type" != "go" && "$vm_type" != "java" ]]; then
  echo "parameter must be go or java or leave it empty(default go)"
  exit 1
fi

CONTAINER_NAME=chainmaker-vm-$vm_type

docker ps

echo

read -r -p "input container name to stop(default '$CONTAINER_NAME'): " tmp
if  [ -n "$tmp" ] ;then
  CONTAINER_NAME=$tmp
else
  echo "container name use default: '$CONTAINER_NAME'"
fi

echo "stop docker vm container"

docker stop "$CONTAINER_NAME"

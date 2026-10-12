/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package sync

import (
	"sync"
	"time"

	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
)

// NodeState Aliases
type NodeState = syncPb.NodeState

// NodeList maintain the state of nodes except node self
type NodeList struct {
	mutex sync.Mutex
	nodes map[string]*NodeState
}

// NewNodeList create a new NodeList
func NewNodeList() *NodeList {
	return &NodeList{
		nodes: make(map[string]*NodeState),
	}
}

// AddNode add/update the state of node
func (nl *NodeList) AddNode(id string, height, archived uint64) {
	nl.mutex.Lock()
	defer nl.mutex.Unlock()
	nl.nodes[id] = &NodeState{
		NodeId:         id,
		Height:         height,
		ArchivedHeight: archived,
		ReceiveTime:    time.Now().Unix(),
	}
}

// GetAll list the state of all nodes
func (nl *NodeList) GetAll() []*NodeState {
	nl.mutex.Lock()
	defer nl.mutex.Unlock()
	nodes := make([]*NodeState, 0, len(nl.nodes))
	for _, node := range nl.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

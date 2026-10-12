/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/
package sync

import (
	"fmt"
	"sync"

	netPb "chainmaker.org/chainmaker/pb-go/v2/net"
	syncPb "chainmaker.org/chainmaker/pb-go/v2/sync"
	"chainmaker.org/chainmaker/protocol/v2"
	"github.com/gogo/protobuf/proto"
)

type syncMessageHandler interface {
	HandleSyncMsg(string, *syncPb.SyncMsg) error
}

// syncNetHandler syncNetHandler is designed to replace different handles to handle the same type of message,
// because net does not have the cancel capability before.
type syncNetHandler struct {
	// receive/broadcast messages from net module
	net protocol.NetService
	log protocol.Logger

	mu      sync.RWMutex
	handler syncMessageHandler
}

func newSyncNetHandler(net protocol.NetService, log protocol.Logger) *syncNetHandler {
	return &syncNetHandler{
		net: net,
		log: log,
	}
}

// RegisterHandler register the handler to handle the sync message.
func (h *syncNetHandler) RegisterHandler(handler syncMessageHandler) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handler != nil && h.handler != handler {
		return fmt.Errorf("other handler already registered")
	}
	h.handler = handler
	return nil
}

// UnregisterHandler unregister the handler.
func (h *syncNetHandler) UnregisterHandler(handler syncMessageHandler) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handler != handler {
		return fmt.Errorf("the handler to be unregistered is not a registered handler")
	}
	h.handler = nil
	return nil
}

// Start subscribe and receive the NetMsg_SYNC_BLOCK_MSG message from net.
func (h *syncNetHandler) Start() error {
	if err := h.net.Subscribe(netPb.NetMsg_SYNC_BLOCK_MSG, h.netMessageHandle); err != nil {
		return err
	}
	return h.net.ReceiveMsg(netPb.NetMsg_SYNC_BLOCK_MSG, h.netMessageHandle)
}

// Stop stop the handler.
func (h *syncNetHandler) Stop() error {
	if err := h.net.CancelSubscribe(netPb.NetMsg_SYNC_BLOCK_MSG); err != nil {
		return err
	}
	return h.net.CancelReceiveMsg(netPb.NetMsg_SYNC_BLOCK_MSG)
}

func (h *syncNetHandler) netMessageHandle(from string, msg []byte, msgType netPb.NetMsg_MsgType) error {
	if msgType != netPb.NetMsg_SYNC_BLOCK_MSG {
		return nil
	}
	var (
		err     error
		syncMsg = syncPb.SyncMsg{}
	)
	if err = proto.Unmarshal(msg, &syncMsg); err != nil {
		h.log.Errorf("fail to proto.Unmarshal the syncPb.SyncMsg:%s", err.Error())
		return err
	}
	h.log.Debugf("receive the NetMsg_SYNC_BLOCK_MSG:the Type is %d", syncMsg.Type)
	var handler syncMessageHandler
	h.mu.RLock()
	handler = h.handler
	h.mu.RUnlock()
	if handler == nil {
		return nil
	}
	return handler.HandleSyncMsg(from, &syncMsg)
}

// SendMsg send msg to any nodes.
// func (h *syncNetHandler) SendMsg(msg []byte, msgType net.NetMsg_MsgType, to ...string) error {
// 	return h.net.SendMsg(msg, msgType, to...)
// }

/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rpcserver

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chainmaker.org/chainmaker-go/module/blockchain"
	"chainmaker.org/chainmaker-go/module/subscriber"
	"chainmaker.org/chainmaker-go/module/subscriber/model"
	"chainmaker.org/chainmaker/localconf/v2"
	commonPb "chainmaker.org/chainmaker/pb-go/v2/common"
)

var (
	// singleton tx result dispatcher
	dispatcher *RootDispatcher
)

// TxResultExt extend Result
type TxResultExt struct {
	Result        *commonPb.Result
	TxTimestamp   int64
	TxBlockHeight uint64
}

//////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////// RootDispatcher ///////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////

// RootDispatcher is the root dispatcher for tx result management
// Note: only one RootDispatcher instance
type RootDispatcher struct {
	// chainMakerServer instance
	chainMakerServer *blockchain.ChainMakerServer
	// mu protect childs
	mu sync.Mutex
	// childs stored childDispatchers
	// use atomic.Value for copy-on-write
	childs atomic.Value

	// stopC for stop RootDispatcher
	stopC chan struct{}
}

// NewRootDispatcher returns a new RootDispatcher
func NewRootDispatcher(chainMakerServer *blockchain.ChainMakerServer) (*RootDispatcher, error) {
	childs := make(childDispatchers)
	for _, chainConfig := range localconf.ChainMakerConfig.BlockChainConfig {
		eventSubscriber, err := chainMakerServer.GetEventSubscribe(chainConfig.ChainId)
		if err != nil {
			return nil, err
		}

		childs[chainConfig.ChainId] = newChildDispatcher(chainConfig.ChainId, eventSubscriber)
	}

	var atomicValue atomic.Value
	atomicValue.Store(childs)

	return &RootDispatcher{
		chainMakerServer: chainMakerServer,
		childs:           atomicValue,
		stopC:            make(chan struct{}),
	}, nil
}

// Start all child dispatchers
func (root *RootDispatcher) Start() {
	go root.loggingStatistics()

	childs := root.loadChilds()
	for i := range childs {
		child := childs[i]
		go child.start()
	}
}

// Stop all child dispatchers
func (root *RootDispatcher) Stop() {
	// stop components of root dispatcher,
	// include logging statistics goroutine
	close(root.stopC)
	// stop all child dispatchers
	for _, child := range root.loadChilds() {
		child.stop()
	}
}

// loadChilds get childs map from RootDispatcher
func (root *RootDispatcher) loadChilds() childDispatchers {
	return root.childs.Load().(childDispatchers)
}

// loadChild get child from root.childs
func (root *RootDispatcher) loadChild(chainId string) (child *childDispatcher, ok bool) {
	child, ok = root.childs.Load().(childDispatchers)[chainId]
	return
}

// storeChild store child to root.childs, using copy-on-write
func (root *RootDispatcher) storeChild(chainId string, child *childDispatcher) {
	root.mu.Lock()
	defer root.mu.Unlock()

	m1, ok := root.childs.Load().(childDispatchers)
	if !ok {
		panic("RootDispatcher.childs Type must be childDispatchers")
	}
	m2 := make(childDispatchers)
	for k, v := range m1 {
		m2[k] = v
	}
	m2[chainId] = child
	root.childs.Store(m2)
}

// deleteChild delete child from root.childs, using copy-on-write
func (root *RootDispatcher) deleteChild(chainId string) {
	root.mu.Lock()
	defer root.mu.Unlock()

	m1, ok := root.childs.Load().(childDispatchers)
	if !ok {
		panic("RootDispatcher.childs Type must be childDispatchers")
	}
	m2 := make(childDispatchers)
	for k, v := range m1 {
		m2[k] = v
	}
	delete(m2, chainId)
	root.childs.Store(m2)
}

// CheckAndUpdate check and update child dispatchers based on the newest chainconfig
func (root *RootDispatcher) CheckAndUpdate() error {
	// start and add child dispatchers based on the newest chainconfig
	oldChilds := root.loadChilds()
	currentChainIds := make(map[string]struct{})
	for _, chainConfig := range localconf.ChainMakerConfig.BlockChainConfig {
		chainId := chainConfig.ChainId
		currentChainIds[chainId] = struct{}{}
		if _, exists := oldChilds[chainId]; !exists {
			eventSubscriber, err := root.chainMakerServer.GetEventSubscribe(chainId)
			if err != nil {
				return err
			}
			child := newChildDispatcher(chainId, eventSubscriber)
			go child.start()
			root.storeChild(chainId, child)
		}
	}

	// stop and delete child dispatchers based on the newest chainconfig
	for chainId, child := range root.loadChilds() {
		if _, exists := currentChainIds[chainId]; !exists {
			child.stop()
			root.deleteChild(chainId)
		}
	}

	return nil
}

// Register for transaction result events.
// Note that Unregister must be called when the registration is no longer needed.
// chainId is the chain ID for which events are to be received
// txId is the transaction ID for which events are to be received
// Returns the channel that is used to receive result. The channel
// is closed when Unregister is called.
func (root *RootDispatcher) Register(chainId, txId string) (chan *TxResultExt, error) {
	child, ok := root.loadChild(chainId)
	if !ok {
		log.Warnf("Register tx [%s] failed, child dispatcher [%s] not exists", txId, chainId)
		return nil, fmt.Errorf("child dispatcher [%s] not exists", chainId)
	}

	return child.register(txId)
}

// Unregister removes the given registration and closes the event channel.
func (root *RootDispatcher) Unregister(chainId, txId string) {
	child, ok := root.loadChild(chainId)
	if !ok {
		log.Warnf("Unregister tx [%s] failed, child dispatcher [%s] not exists", txId, chainId)
		return
	}

	child.unregister(txId)
}

// loggingStatistics logging statistics
// logging count of transactions are waiting for results
func (root *RootDispatcher) loggingStatistics() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var totalCount int64
			var childTxCounts []string
			for _, child := range root.loadChilds() {
				count := atomic.LoadInt64(&child.txCount)
				totalCount += count
				childTxCounts = append(childTxCounts, fmt.Sprintf("%s:%d", child.chainId, count))
			}
			log.Infof("total [%d] txs are waiting for results, [%s]", totalCount,
				strings.Join(childTxCounts, ","))
		case <-root.stopC:
			return
		}
	}
}

///////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////// childDispatcher ///////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////

// childDispatcher is a child dispatcher for a chain
// Note: one childDispatcher instance per chain
type childDispatcher struct {
	chainId string
	// eventSubscriber is block event subscriber for realtime block event
	eventSubscriber *subscriber.EventSubscriber
	// mu protect txRegs
	mu sync.RWMutex
	// txRegs key: txId value: chan *TxResultExt
	txRegs txRegistrations
	// count of transactions of this chain are waiting for results
	txCount int64

	// stopC channel for stop signal
	stopC chan struct{}
}

// childDispatchers key: chainId value: *childDispatcher
type childDispatchers map[string]*childDispatcher

// txRegistrations key: txId value: chan *TxResultExt, store txIds that registered
// for each txId, there is only one result channel
type txRegistrations map[string]chan *TxResultExt

// newChildDispatcher returns a new childDispatcher
func newChildDispatcher(chainId string, eventSubscriber *subscriber.EventSubscriber) *childDispatcher {
	return &childDispatcher{
		chainId:         chainId,
		eventSubscriber: eventSubscriber,
		txRegs:          make(txRegistrations),
		stopC:           make(chan struct{}),
	}
}

// start run dispatcher for a chain until stop
func (child *childDispatcher) start() {
	blockEventC := make(chan model.NewBlockEvent, 1)
	sub := child.eventSubscriber.SubscribeBlockEvent(blockEventC)
	defer sub.Unsubscribe()

	for {
		select {
		case ev := <-blockEventC:
			log.Debugf("child dispatcher [%s] received block [%d]", child.chainId,
				ev.BlockInfo.Block.Header.BlockHeight)
			for _, tx := range ev.BlockInfo.Block.Txs {
				child.trySendTxResult(tx, ev.BlockInfo.Block.Header.BlockHeight)
			}
		case <-child.stopC:
			log.Debugf("child dispatcher [%s] stopped", child.chainId)
			return
		}
	}
}

// stop child dispatcher
func (child *childDispatcher) stop() {
	close(child.stopC)
}

// register for transaction result events.
// txId is the transaction ID for which events are to be received
// Returns the channel that is used to receive result. The channel
// is closed when unregister is called.
func (child *childDispatcher) register(txId string) (chan *TxResultExt, error) {
	child.mu.Lock()
	defer child.mu.Unlock()

	if _, exists := child.txRegs[txId]; exists {
		return nil, errors.New("txId duplicated")
	}
	atomic.AddInt64(&child.txCount, 1)
	txResultC := make(chan *TxResultExt, 1)
	child.txRegs[txId] = txResultC
	return txResultC, nil
}

// unregister removes the given registration and closes the event channel.
func (child *childDispatcher) unregister(txId string) {
	child.mu.Lock()
	defer child.mu.Unlock()

	if txResultC, exists := child.txRegs[txId]; exists {
		atomic.AddInt64(&child.txCount, -1)
		delete(child.txRegs, txId)
		close(txResultC)
	}
}

// trySendTxResult try to send a tx result to channel
// if the tx not registered, do nothing
func (child *childDispatcher) trySendTxResult(tx *commonPb.Transaction, blockHeight uint64) {
	child.mu.RLock()
	defer child.mu.RUnlock()

	if txResultC, exists := child.txRegs[tx.Payload.TxId]; exists {
		result := &TxResultExt{
			Result:        tx.Result,
			TxTimestamp:   tx.Payload.Timestamp,
			TxBlockHeight: blockHeight,
		}
		// non-blocking write to channel to ignore txResultC buffer is full in extreme cases
		select {
		case txResultC <- result:
		default:
			log.Warnf("tx [%s] result channel is full, result dropped", tx.Payload.TxId)
		}
	}
}

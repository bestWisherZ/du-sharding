/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 */

package snapshot

import (
	"sync"
)

const (
	Prime32             = uint32(16777619)
	ShardNum            = 64
	KeysPerShardDefault = 1024
)

type ShardSet struct {
	shardNum int
	shards   []*Shard
}

func newShardSet() *ShardSet {
	shards := make([]*Shard, ShardNum)
	for i := 0; i < ShardNum; i++ {
		shards[i] = newShard()
	}
	return &ShardSet{
		shardNum: ShardNum,
		shards:   shards,
	}
}

func (s *ShardSet) getByLock(k string) (*sv, bool) {
	shard := s.shards[shardNum(k, s.shardNum)]
	return shard.getByLock(k)
}

func (s *ShardSet) putByLock(k string, sv *sv) {
	shard := s.shards[shardNum(k, s.shardNum)]
	shard.putByLock(k, sv)
}

type Shard struct {
	sync.RWMutex
	m map[string]*sv
}

func newShard() *Shard {
	return &Shard{
		m: make(map[string]*sv, KeysPerShardDefault),
	}
}

func (s *Shard) getByLock(k string) (*sv, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.get(k)
}

func (s *Shard) putByLock(k string, kv *sv) {
	s.Lock()
	defer s.Unlock()
	s.put(k, kv)
}

func (s *Shard) get(k string) (*sv, bool) {
	if kv, exist := s.m[k]; exist {
		return kv, exist
	}
	return nil, false
}

func (s *Shard) put(k string, sv *sv) {
	s.m[k] = sv
}

func shardNum(key string, shardedNum int) int {
	return int(fnv32(key) % uint32(shardedNum))
}

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	keyLength := len(key)
	for i := 0; i < keyLength; i++ {
		hash *= Prime32
		hash ^= uint32(key[i])
	}
	return hash
}

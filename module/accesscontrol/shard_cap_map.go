/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 */

package accesscontrol

import (
	"sync"
)

const (
	Prime32             = uint32(16777619)
	ShardNum            = 64
	KeysPerShardDefault = 1024
)

type ShardCache struct {
	shards []*Shard
}

func NewShardCache(totalCap int) *ShardCache {
	shards := make([]*Shard, ShardNum)
	capPerShard := totalCap / ShardNum
	if capPerShard < 64 {
		capPerShard = 64
	}
	for i := 0; i < ShardNum; i++ {
		shards[i] = newShard(capPerShard)
	}
	return &ShardCache{
		shards: shards,
	}
}

func (s *ShardCache) Get(k string) (interface{}, bool) {
	shard := s.shards[shardNum(k, ShardNum)]
	return shard.Get(k)
}

func (s *ShardCache) Add(k string, v interface{}) {
	shard := s.shards[shardNum(k, ShardNum)]
	shard.Put(k, v)
}

func (s *ShardCache) Clear() {
	for i := 0; i < ShardNum; i++ {
		s.shards[i].Clear()
	}
}

func (s *ShardCache) Remove(k string) {
	for i := 0; i < ShardNum; i++ {
		s.shards[i].Remove(k)
	}
}

// Iterate 遍历 ShardCache 中的所有键值对，并调用回调函数处理每个键值对
func (s *ShardCache) Iterate(f func(key string, value interface{})) {
	for i := 0; i < ShardNum; i++ {
		shard := s.shards[i]
		shard.RLock()
		for k, v := range shard.m {
			f(k, v)
		}
		shard.RUnlock()
	}
}

func (s *ShardCache) Long() int {
	long := 0
	for i := 0; i < ShardNum; i++ {
		shard := s.shards[i]
		shard.RLock()
		long += len(shard.m)
		shard.RUnlock()
	}
	return long
}

type Shard struct {
	sync.RWMutex
	cap int
	m   map[string]interface{}
}

func newShard(cap int) *Shard {
	return &Shard{
		cap: cap,
		m:   make(map[string]interface{}, KeysPerShardDefault),
	}
}

func (s *Shard) Get(k string) (interface{}, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.get(k)
}

func (s *Shard) Remove(k string) {
	s.Lock()
	defer s.Unlock()
	delete(s.m, k)
}

func (s *Shard) Put(k string, v interface{}) {
	s.Lock()
	defer s.Unlock()
	// 判断容量是否已经达到
	if len(s.m) > s.cap {
		// 清理map
		s.m = make(map[string]interface{}, KeysPerShardDefault)
	}
	s.put(k, v)
}

func (s *Shard) Clear() {
	s.Lock()
	defer s.Unlock()
	// 清理map
	s.m = make(map[string]interface{}, KeysPerShardDefault)
}

func (s *Shard) get(k string) (interface{}, bool) {
	if v, exist := s.m[k]; exist {
		return v, exist
	}
	return nil, false
}

func (s *Shard) put(k string, v interface{}) {
	s.m[k] = v
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

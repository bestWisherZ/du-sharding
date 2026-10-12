/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 */

package snapshot

import "sync"

// mapper is the interface for snapshot mapper.
type mapper interface {
	// GetByLock get the value by key, and lock the mapper.
	getByLock(k string) (*sv, bool)

	// PutByLock put the value by key, and lock the mapper.
	putByLock(k string, sv *sv)
}

type InnerMapper struct {
	sync.RWMutex
	data map[string]*sv
}

func NewInnerMapper() *InnerMapper {
	return &InnerMapper{
		data: make(map[string]*sv),
	}
}

func (m *InnerMapper) getByLock(k string) (*sv, bool) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.data[k]
	return v, ok
}

func (m *InnerMapper) putByLock(k string, sv *sv) {
	m.Lock()
	defer m.Unlock()
	m.data[k] = sv
}

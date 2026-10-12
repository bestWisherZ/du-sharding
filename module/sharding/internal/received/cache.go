package received

import "sync"

type entry struct {
	done     chan struct{}
	accepted bool
}

// Cache coalesces duplicate content only after an authenticated handler has
// accepted it. Invalid first arrivals never suppress a valid retransmission.
type Cache struct {
	mu       sync.Mutex
	entries  map[[32]byte]*entry
	order    [][32]byte
	capacity int
}

func New(capacity int) *Cache {
	if capacity < 1 {
		capacity = 1
	}
	return &Cache{entries: make(map[[32]byte]*entry), capacity: capacity}
}

func (c *Cache) Do(key [32]byte, accept func() bool) bool {
	for {
		c.mu.Lock()
		if existing := c.entries[key]; existing != nil {
			c.mu.Unlock()
			<-existing.done
			if existing.accepted {
				return true
			}
			continue
		}
		break
	}
	current := &entry{done: make(chan struct{})}
	c.entries[key] = current
	c.mu.Unlock()
	accepted := false
	defer func() {
		c.mu.Lock()
		current.accepted = accepted
		if !accepted {
			delete(c.entries, key)
		} else {
			c.order = append(c.order, key)
			if len(c.order) > c.capacity {
				delete(c.entries, c.order[0])
				c.order = c.order[1:]
			}
		}
		close(current.done)
		c.mu.Unlock()
	}()
	accepted = accept()
	return accepted
}

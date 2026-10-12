package received

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentCopiesAreAcceptedOnce(t *testing.T) {
	c := New(2)
	var calls int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !c.Do([32]byte{1}, func() bool { atomic.AddInt32(&calls, 1); return true }) {
				t.Error("valid message rejected")
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("accepted duplicate %d times", calls)
	}
}

func TestInvalidFirstMessageDoesNotSuppressValidRetry(t *testing.T) {
	c := New(2)
	key := [32]byte{1}
	if c.Do(key, func() bool { return false }) {
		t.Fatal("invalid message cached")
	}
	if !c.Do(key, func() bool { return true }) {
		t.Fatal("valid retry was lost")
	}
	if !c.Do(key, func() bool { t.Fatal("duplicate reprocessed"); return false }) {
		t.Fatal("accepted duplicate forgotten")
	}
}

func TestEvictionAndPanicAllowReverification(t *testing.T) {
	c := New(1)
	c.Do([32]byte{1}, func() bool { return true })
	c.Do([32]byte{2}, func() bool { return true })
	called := false
	c.Do([32]byte{1}, func() bool { called = true; return true })
	if !called || len(c.entries) != 1 {
		t.Fatal("cache capacity was not enforced")
	}
	func() {
		defer func() { _ = recover() }()
		c.Do([32]byte{3}, func() bool { panic("handler failed") })
	}()
	if !c.Do([32]byte{3}, func() bool { return true }) {
		t.Fatal("panic left content permanently in flight")
	}
}

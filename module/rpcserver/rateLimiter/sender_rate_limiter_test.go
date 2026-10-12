package rateLimiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"chainmaker.org/chainmaker/localconf/v2"

	"chainmaker.org/chainmaker/logger/v2"
	"github.com/stretchr/testify/require"
)

func TestSenderSubscriberRateLimiter_Concurrency(t *testing.T) {
	// Mock logger
	logger := logger.GetLoggerByChain(logger.MODULE_RPC, "Chain1")

	localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.RateLimitConfig.Enabled = true

	// Create rate limiter
	limiter := NewSenderRateLimiter(logger)
	require.NotNil(t, limiter, "limiter should not be nil")

	// Define test cases
	senders := []string{"sender1", "sender2", "sender3"}
	var wg sync.WaitGroup

	// Run concurrent tests
	for _, sender := range senders {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				opts := &LimiterOptions{Sender: s}
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				err := limiter.Wait(ctx, opts)
				require.Nil(t, err, "expected no error, got %v", err)
				cancel()
			}
		}(sender)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func TestSenderSubscriberRateLimiter_NilOptions(t *testing.T) {
	// Mock logger
	logger := logger.GetLoggerByChain(logger.MODULE_RPC, "Chain1")

	// Create rate limiter
	limiter := NewSenderRateLimiter(logger)
	require.NotNil(t, limiter, "limiter should not be nil")

	// Test nil LimiterOptions
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := limiter.Wait(ctx, nil)
	require.NotNil(t, err, "expected error for nil options")
	require.Contains(t, err.Error(), "sender subscriber rate limiter's opts should not be nil")
}

package rateLimiter

import (
	"testing"

	"chainmaker.org/chainmaker/logger/v2"
	"github.com/stretchr/testify/require"
)

func TestSubscriberRateLimiter_Disable(t *testing.T) {
	// Set up logger and rate limiter
	logger := logger.GetLoggerByChain(logger.MODULE_RPC, "Chain1")
	limiter := NewSubscriberRateLimiter(logger)
	require.Nil(t, limiter)
}

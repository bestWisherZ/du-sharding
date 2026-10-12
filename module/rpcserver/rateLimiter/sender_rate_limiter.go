/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rateLimiter

import (
	"context"
	"fmt"
	"sync"

	"chainmaker.org/chainmaker/logger/v2"
	"golang.org/x/time/rate"
)

// senderSubscriberRateLimiter is a rate limiter for subscribers that limits the rate
// at which senders can make subscription requests. It manages a map of rate limiters,
// one per sender, to ensure fair distribution of tokens across different senders.
type senderSubscriberRateLimiter struct {
	limiters map[string]*rate.Limiter // A map of rate limiters for each sender
	mu       sync.RWMutex             // Read/Write mutex to synchronize access to the limiters map
	logger   *logger.CMLogger         // Logger instance for logging information
}

// NewSenderRateLimiter creates a new senderSubscriberRateLimiter instance.
func NewSenderRateLimiter(log *logger.CMLogger) SubscriberRateLimiter {
	return &senderSubscriberRateLimiter{limiters: make(map[string]*rate.Limiter), logger: log}
}

// GetSubscriberRateLimiterType returns the type of the rate limiter, which indicates
// that it is a sender-based rate limiter.
func (ssr *senderSubscriberRateLimiter) GetSubscriberRateLimiterType() int {
	return SenderRateLimiterType
}

// Wait waits for permission to proceed according to the rate limits set for the sender.
// If LimiterOptions is nil, it returns an error indicating that the options must not be nil.
func (ssr *senderSubscriberRateLimiter) Wait(ctx context.Context, opts *LimiterOptions) error {
	if opts == nil {
		return fmt.Errorf("sender subscriber rate limiter's opts should not be nil")
	}

	limiter := ssr.getOrCreateLimiter(opts.Sender)
	return limiter.Wait(ctx)
}

// getOrCreateLimiter retrieves the rate limiter for a given sender or creates a new
// one if none exists. The creation of new limiters is synchronized to avoid race conditions.
func (ssr *senderSubscriberRateLimiter) getOrCreateLimiter(sender string) *rate.Limiter {
	ssr.mu.RLock()
	limiter, exists := ssr.limiters[sender]
	ssr.mu.RUnlock()

	// If the rate limiter does not exist for the sender, create a new one
	if !exists {
		ssr.mu.Lock()
		defer ssr.mu.Unlock()

		// Double-check existence after acquiring the write lock
		limiter, exists = ssr.limiters[sender]
		if !exists {
			limiter = newRateLimiter()
			ssr.limiters[sender] = limiter
		}
	}
	return limiter
}

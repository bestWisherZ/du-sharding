/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rateLimiter

import (
	"context"

	"chainmaker.org/chainmaker/localconf/v2"
	"chainmaker.org/chainmaker/logger/v2"
	"golang.org/x/time/rate"
)

const (
	// GlobalRateLimiterType shared across all subscribers
	GlobalRateLimiterType = 0
	// SenderRateLimiterType separate rate limiter for each sender
	SenderRateLimiterType = 1

	// Default rate limit configurations
	// rateLimitDefaultTokenPerSecond tokens generated per second
	rateLimitDefaultTokenPerSecond = 1000
	// rateLimitDefaultTokenBucketSize maximum size of the token bucket
	rateLimitDefaultTokenBucketSize = 1000
)

// LimiterOptions contains options for rate limiting
type LimiterOptions struct {
	// sender addr
	Sender string
}

// SubscriberRateLimiter is an interface that controls rate limiting for subscriptions
type SubscriberRateLimiter interface {
	GetSubscriberRateLimiterType() int
	Wait(ctx context.Context, opts *LimiterOptions) error
}

// NewSubscriberRateLimiter returns a new rate limiter based on the configuration
// It supports two types of rate limiters:
// - GlobalRateLimiter: shared across all subscribers
// - SenderRateLimiter: each sender has its own rate limiter
func NewSubscriberRateLimiter(logger *logger.CMLogger) SubscriberRateLimiter {
	// Check if rate limiting is enabled in the configuration
	if !localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.RateLimitConfig.Enabled {
		return nil
	}

	// Determine the rate limiter type from the configuration
	rateLimiterType := localconf.ChainMakerConfig.RpcConfig.RateLimitConfig.Type
	switch rateLimiterType {
	case GlobalRateLimiterType:
		return NewGlobalRateLimiter(logger)
	case SenderRateLimiterType:
		return NewSenderRateLimiter(logger)
	default:
		// Log a warning if the rate limiter type is invalid, and default to GlobalRateLimiter
		logger.Warnf("invalid subscriber rate limiter type[%d], using default type!", rateLimiterType)
		return NewGlobalRateLimiter(logger)
	}
}

// newRateLimiter creates a new rate limiter instance based on the rate limit configuration.
// It initializes the limiter with a token bucket size and tokens per second, falling back to
// default values if the configuration is invalid (e.g., negative or zero values).
func newRateLimiter() *rate.Limiter {
	// Retrieve the token bucket size and tokens per second from the configuration
	tokenBucketSize := localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.RateLimitConfig.TokenBucketSize
	tokenPerSecond := localconf.ChainMakerConfig.RpcConfig.SubscriberConfig.RateLimitConfig.TokenPerSecond

	// If the token bucket size is invalid, use the default size
	if tokenBucketSize <= 0 {
		tokenBucketSize = rateLimitDefaultTokenBucketSize
	}

	// If the token per second rate is invalid, use the default rate
	if tokenPerSecond <= 0 {
		tokenPerSecond = rateLimitDefaultTokenPerSecond
	}

	// Create and return a new rate limiter with the configured or default values
	return rate.NewLimiter(rate.Limit(tokenPerSecond), tokenBucketSize)
}

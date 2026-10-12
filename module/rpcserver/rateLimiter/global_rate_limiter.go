/*
Copyright (C) BABEC. All rights reserved.
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rateLimiter

import (
	"context"
	"errors"
	"fmt"

	"chainmaker.org/chainmaker/logger/v2"
	"golang.org/x/time/rate"
)

// globalSubscriberRateLimiter implements the SubscriberRateLimiter interface
// It applies a global rate limiter shared across all subscribers.
type globalSubscriberRateLimiter struct {
	limiter *rate.Limiter
	logger  *logger.CMLogger
}

// NewGlobalRateLimiter creates a new global rate limiter based on the configuration.
// The limiter restricts the number of tokens (requests) that can be processed per second globally.
func NewGlobalRateLimiter(log *logger.CMLogger) SubscriberRateLimiter {
	return &globalSubscriberRateLimiter{limiter: newRateLimiter(), logger: log}
}

func (gsr *globalSubscriberRateLimiter) GetSubscriberRateLimiterType() int {
	return GlobalRateLimiterType
}

// Wait applies the rate limiting, blocking until the request is allowed or context is done.
// It ensures that the global rate limiter's token bucket has enough tokens for the request to proceed.
func (gsr *globalSubscriberRateLimiter) Wait(ctx context.Context, opts *LimiterOptions) error {
	if err := gsr.limiter.Wait(ctx); err != nil {
		errMsg := fmt.Sprintf("subscriber rateLimiter wait token failed, %s", err.Error())
		gsr.logger.Error(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

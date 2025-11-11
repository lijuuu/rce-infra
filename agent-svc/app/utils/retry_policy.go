package utils

import (
	"math"
	"time"
)

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy returns a default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: 5,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// CalculateDelay calculates exponential backoff delay for retry attempt
func (r *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= r.MaxRetries {
		return r.MaxDelay
	}

	delay := time.Duration(math.Pow(2, float64(attempt))) * r.BaseDelay
	if delay > r.MaxDelay {
		return r.MaxDelay
	}
	return delay
}

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

// NewRetryPolicy creates a new retry policy
func NewRetryPolicy(maxRetries int, baseDelay, maxDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
		MaxDelay:   maxDelay,
	}
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

// Execute executes a function with retry policy
func (r *RetryPolicy) Execute(fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < r.MaxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt < r.MaxRetries-1 {
			delay := r.CalculateDelay(attempt)
			time.Sleep(delay)
		}
	}
	return lastErr
}

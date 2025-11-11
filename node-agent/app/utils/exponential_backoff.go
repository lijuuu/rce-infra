package utils

import (
	"math"
	"time"
)

// ExponentialBackoff calculates exponential backoff delay
func ExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(maxRetries int, baseDelay time.Duration, maxDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt < maxRetries-1 {
			delay := ExponentialBackoff(attempt, baseDelay, maxDelay)
			time.Sleep(delay)
		}
	}
	return lastErr
}

package utils

import (
	"fmt"
	"time"
)

// RetryOperation is a function that returns an error
type RetryOperation func() error

// RetryWithBackoff retries an operation with exponential backoff
// maxRetries: maximum number of retries
// initialBackoff: starting delay
// maxBackoff: maximum delay cap
func RetryWithBackoff(op RetryOperation, maxRetries int, initialBackoff time.Duration, maxBackoff time.Duration) error {
	backoff := initialBackoff
	var err error

	for i := 0; i <= maxRetries; i++ {
		err = op()
		if err == nil {
			return nil
		}

		if i == maxRetries {
			break
		}

		// Check if we should stop? (Context checking could be added here)

		// Log if needed (caller handles logging usually, but we could accept a logger)
		// For now we keep it simple.

		time.Sleep(backoff)

		// Exponential increase
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}

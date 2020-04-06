package util

import (
	"time"

	"github.com/cenkalti/backoff"
)

// Retry retries with exponential backoff the given function
func Retry(maxElapsedTime time.Duration, f func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = maxElapsedTime
	bo.Reset()
	return backoff.Retry(f, bo)

}

// RetryWithInitialDelay retires with exponential backoff and initial delay the given function
func RetryWithInitialDelay(initialDelay, maxElapsedTime time.Duration, f func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = initialDelay
	bo.MaxElapsedTime = maxElapsedTime
	bo.Multiplier = 2.0
	bo.Reset()
	return backoff.Retry(f, bo)
}

// RetryWithInitialDelaySlower retries with exponential backoff, an initial delay and with a slower rate
func RetryWithInitialDelaySlower(initialDelay, maxElapsedTime time.Duration, f func() error) error {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = initialDelay
	bo.MaxElapsedTime = maxElapsedTime
	bo.Multiplier = 2.0
	bo.Reset()
	return backoff.Retry(f, bo)
}

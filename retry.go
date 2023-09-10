package pl

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

var DefaultRetryOption = RetryOption{
	MaxCount: 5,
	Timeout:  backoff.DefaultMaxElapsedTime,
	Backoff:  backoff.NewExponentialBackOff(),
	StopIf: func(n int, since time.Duration, err error) bool {
		return err == nil
	},
}

type RetryOption struct {
	MaxCount int
	Timeout  time.Duration
	Backoff  backoff.BackOff
	StopIf   func(n int, since time.Duration, err error) bool
}

package pl

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type RetryStopIfFunc func(n uint64, since time.Duration, err error) bool

var DefaultRetryOption = RetryOption{
	MaxCount: 5,
	Backoff:  backoff.NewExponentialBackOff(),
	StopIf:   nil,
	Timer:    nil,
}

type RetryOption struct {
	MaxCount uint64
	Backoff  backoff.BackOff
	StopIf   RetryStopIfFunc
	Timer    backoff.Timer
}

func (opt *RetryOption) Run(ctx context.Context, fn func(context.Context) error, notAfter time.Time) error {
	maxCount := DefaultRetryOption.MaxCount
	if opt.MaxCount > 0 {
		maxCount = opt.MaxCount
	}
	b := DefaultRetryOption.Backoff
	if opt.Backoff != nil {
		b = opt.Backoff
	}
	stopIf := DefaultRetryOption.StopIf
	if opt.StopIf != nil {
		stopIf = opt.StopIf
	}
	timer := DefaultRetryOption.Timer
	if opt.Timer != nil {
		timer = opt.Timer
	}

	count := uint64(0)
	start := time.Now()
	var err error

	return backoff.RetryNotifyWithTimer(
		func() error {
			err = fn(ctx)
			if !notAfter.IsZero() && time.Now().After(notAfter) { // timeouted
				err = backoff.Permanent(err)
			}
			if stopIf != nil && stopIf(count, time.Since(start), err) {
				err = backoff.Permanent(err)
			}
			count++
			return err
		},
		backoff.WithMaxRetries(b, maxCount),
		nil,
		timer,
	)
}

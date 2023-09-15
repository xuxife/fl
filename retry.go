package pl

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var DefaultRetryOption = RetryOption{
	Backoff:  backoff.NewExponentialBackOff(),
	MaxCount: 10,
	StopIf:   nil,
	Timer:    nil,
}

type RetryOption struct {
	Backoff  backoff.BackOff
	MaxCount uint64
	StopIf   func(n uint64, since time.Duration, err error) bool
	Timer    backoff.Timer
}

func (opt *RetryOption) Run(
	ctx context.Context,
	fn func(context.Context) error,
	notAfter time.Time, // the timeout ddl
) error {
	b := DefaultRetryOption.Backoff
	if opt.Backoff != nil {
		b = opt.Backoff
	}
	maxCount := DefaultRetryOption.MaxCount
	if opt.MaxCount > 0 {
		maxCount = opt.MaxCount
	}
	if maxCount > 0 {
		b = backoff.WithMaxRetries(b, maxCount)
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
		b,
		nil,
		timer,
	)
}

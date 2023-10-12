package pl

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var DefaultRetryOption = RetryOption{
	Backoff:  backoff.NewExponentialBackOff(),
	Attempts: 10,
	StopIf:   nil,
	Timer:    nil,
}

type RetryOption struct {
	Backoff  backoff.BackOff
	Attempts uint64 // 0 means no limit
	StopIf   func(ctx context.Context, attempt uint64, since time.Duration, err error) bool
	Timer    backoff.Timer
}

func (opt *RetryOption) Default() {
	if opt.Backoff == nil {
		opt.Backoff = DefaultRetryOption.Backoff
	}
	if opt.Attempts == 0 {
		opt.Attempts = DefaultRetryOption.Attempts
	}
	if opt.StopIf == nil {
		opt.StopIf = DefaultRetryOption.StopIf
	}
	if opt.Timer == nil {
		opt.Timer = DefaultRetryOption.Timer
	}
}

func (s *Workflow) retry(opt *RetryOption) func(
	ctx context.Context,
	fn func(context.Context) error,
	notAfter time.Time, // the Step level timeout ddl
) error {
	return func(ctx context.Context, fn func(context.Context) error, notAfter time.Time) error {
		opt.Default()
		if opt.Attempts > 0 {
			opt.Backoff = backoff.WithMaxRetries(opt.Backoff, opt.Attempts)
		}
		attempt := uint64(0)
		start := time.Now()
		return backoff.RetryNotifyWithTimer(
			func() error {
				err := fn(ctx)
				if !notAfter.IsZero() && time.Now().After(notAfter) { // timeouted
					err = backoff.Permanent(err)
				}
				if opt.StopIf != nil && opt.StopIf(ctx, attempt, time.Since(start), err) {
					err = backoff.Permanent(err)
				}
				attempt++
				return err
			},
			opt.Backoff,
			nil,
			opt.Timer,
		)
	}
}

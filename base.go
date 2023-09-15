package pl

import (
	"sync"
	"time"
)

type base interface {
	GetStatus() JobStatus
	setStatus(JobStatus)

	getCondition() Condition
	setCondition(Condition)

	getRetry() *RetryOption
	setRetry(*RetryOption)

	getWhen() When
	setWhen(When)

	getTimeout() time.Duration
	setTimeout(time.Duration)
}

var _ base = &Base{}

type Base struct {
	mutex   sync.RWMutex
	status  JobStatus
	cond    Condition
	retry   *RetryOption
	when    When
	timeout time.Duration
}

func (b *Base) GetStatus() JobStatus {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.status
}

func (b *Base) setStatus(status JobStatus) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.status = status
}

func (b *Base) getCondition() Condition {
	return b.cond
}

func (b *Base) setCondition(cond Condition) {
	b.cond = cond
}

func (b *Base) getRetry() *RetryOption {
	return b.retry
}

func (b *Base) setRetry(opt *RetryOption) {
	b.retry = opt
}

func (b *Base) getWhen() When {
	return b.when
}

func (b *Base) setWhen(when When) {
	b.when = when
}

func (b *Base) getTimeout() time.Duration {
	return b.timeout
}

func (b *Base) setTimeout(timeout time.Duration) {
	b.timeout = timeout
}

// BaseIn[I] is a helper struct that can be embeded into your Job implement struct,
// such that you can avoid `Input() *I` implement.
type BaseIn[I any] struct {
	Base
	In I
}

func (i *BaseIn[I]) Input() *I {
	return &i.In
}

func GetOutput[T any](out Outputer[T]) T {
	var v T
	out.Output(&v)
	return v
}

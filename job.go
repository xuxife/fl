package pl

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Jober[I, O any] is the basic unit of a workflow.
//
//	I: input type
//	O: output type
//
// A Jober implement should always embed Base.
//
//	type SomeTask struct {
//		Base // always embed Base
//	}
//
// Then please implement following interfaces:
//
//	String() string	// give this job a name
//	Input() *I // return the reference input of this job, used to accept input.
//	Output(*O)  // fill the output into pointer, used to send output.
//	Do(context.Context) error // the main logic of this job, basically it transform input to output.
//
// Tip: you can avoid nasty `Input()` implement by embed `BaseIn[I]`
//
//	type SomeTask struct {
//		BaseIn[TaskInput] // inherit `Input() *TaskInput`
//	}
type Jober[I, O any] interface {
	// methods to be implemented
	Inputer[I]
	Outputer[O]
	Doer
	fmt.Stringer // please give this job a name

	// methods that inherit from Base
	base
}

type Inputer[I any] interface {
	Input() *I
}

type Outputer[O any] interface {
	Output(*O)
}

type Doer interface {
	Do(context.Context) error
}

type Reporter interface {
	fmt.Stringer
	GetStatus() JobStatus
}

type base interface {
	GetStatus() JobStatus
	setStatus(JobStatus)

	getCondition() Condition
	setCondition(Condition)

	getRetry() *RetryOption
	setRetry(*RetryOption)

	getWhen() WhenFunc
	setWhen(WhenFunc)

	getTimeout() time.Duration
	setTimeout(time.Duration)
}

var _ base = &Base{}

type Base struct {
	mutex   sync.RWMutex
	status  JobStatus
	cond    Condition
	retry   *RetryOption
	when    WhenFunc
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

func (b *Base) getWhen() WhenFunc {
	return b.when
}

func (b *Base) setWhen(when WhenFunc) {
	b.when = when
}

func (b *Base) getTimeout() time.Duration {
	return b.timeout
}

func (b *Base) setTimeout(timeout time.Duration) {
	b.timeout = timeout
}

// Inp is a help struct that can be embeded into your Job implement,
// such that you can skip Input() implement.
type BaseIn[I any] struct {
	Base
	In I
}

func (i *BaseIn[I]) Input() *I {
	return &i.In
}

func SetInput[T any](in Inputer[T], v T) {
	*in.Input() = v
}

func GetOutput[T any](out Outputer[T]) T {
	var v T
	out.Output(&v)
	return v
}

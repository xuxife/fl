package pl

import (
	"sync"
	"time"
)

// stepBase is the stepBase interface for an e2e Step.
// Embed `StepBase` to inherit the implementation for `stepBase`.
type stepBase interface {
	GetStatus() StepStatus
	setStatus(StepStatus)

	getCondition() Condition
	setCondition(Condition)

	getRetry() *RetryOption
	setRetry(*RetryOption)

	getWhen() When
	setWhen(When)

	getTimeout() time.Duration
	setTimeout(time.Duration)
}

var _ stepBase = &StepBase{}

// StepBase is to be embeded into your Step implement struct.
type StepBase struct {
	mutex   sync.RWMutex
	status  StepStatus
	cond    Condition
	retry   *RetryOption
	when    When
	timeout time.Duration
}

func (b *StepBase) GetStatus() StepStatus {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.status
}

func (b *StepBase) setStatus(status StepStatus) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.status = status
}

func (b *StepBase) getCondition() Condition {
	return b.cond
}

func (b *StepBase) setCondition(cond Condition) {
	b.cond = cond
}

func (b *StepBase) getRetry() *RetryOption {
	return b.retry
}

func (b *StepBase) setRetry(opt *RetryOption) {
	b.retry = opt
}

func (b *StepBase) getWhen() When {
	return b.when
}

func (b *StepBase) setWhen(when When) {
	b.when = when
}

func (b *StepBase) getTimeout() time.Duration {
	return b.timeout
}

func (b *StepBase) setTimeout(timeout time.Duration) {
	b.timeout = timeout
}

// StepBaseIn[I] is to be embeded into your Step implement struct,
// with the sepcified input type `I`.
type StepBaseIn[I any] struct {
	StepBase
	In I
}

func (i *StepBaseIn[I]) Input() *I {
	return &i.In
}

// StepBaseInOut[I, O] is to be embeded into your Step implement struct,
// with the sepcified input type `I`, output type `O`.
type StepBaseInOut[I, O any] struct {
	StepBase
	In  I
	Out O
}

func (i *StepBaseInOut[I, O]) Input() *I {
	return &i.In
}

func (i *StepBaseInOut[I, O]) Output(out *O) {
	*out = i.Out
}

// StepBaseNoInOut is to be embeded into your Step implement struct,
// if the Step don't have Input or Output
type StepBaseNoInOut = StepBaseInOut[struct{}, struct{}]

// GetOutput gets the output from a Step.
func GetOutput[A any](out outputer[A]) A {
	var v A
	out.Output(&v)
	return v
}

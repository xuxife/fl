package pl

import (
	"context"
	"fmt"
	"sync"
)

// Job[I, O any] is the basic unit of a workflow.
//
//	I: input type
//	O: output type
//
// A Job implement should always embed Base.
//
//	type SomeTask struct {
//		Base // always embed Base
//		TaskInput
//		TaskOnput
//	}
//
//	var _ Job[TaskInput, TaskOutput] = &SomeTask{}
//
// Then please implement following interfaces:
//
//	String() string	// give this job a name
//	Input() *I // return the reference input of this job, used to accept input.
//	Output(*O)  // fill the output into pointer, used to send output.
//	Do(context.Context) error // the main logic of this job, basically it transform input to output.
//
// Tip: you can avoid nasty `Input()` implement by embed `Inp[I]`
//
//	type SomeTask struct {
//		Base // always embed Base
//		InP[TaskInput] // inherit Input()
//	}
type Job[I, O any] interface {
	// methods to be implemented
	Inputer[I]
	Outputer[O]
	Doer
	fmt.Stringer // please give this job a name

	// methods that inherit from Base
	job
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

type job interface {
	GetStatus() JobStatus
	setStatus(JobStatus) // do not export set status
	GetCond() Cond
	When(Cond) // SetCond
}

var _ job = &Base{}

type Base struct {
	mutex  sync.RWMutex
	status JobStatus
	cond   Cond
}

func (b *Base) setStatus(status JobStatus) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.status = status
}

func (b *Base) GetStatus() JobStatus {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.status
}

func (b *Base) GetCond() Cond {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if b.cond == nil {
		return DefaultCond
	}
	return b.cond
}

func (b *Base) When(cond Cond) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.cond = cond
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

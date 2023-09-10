package pl

import (
	"context"
	"fmt"
	"reflect"
)

// Func constructs a Job from an arbitrary function
func Func[I, O any](name string, do func(context.Context, I) (func(*O), error)) Jober[I, O] {
	return &func_[I, O]{name: name, do: do}
}

// Adapter constructs a Job that adapts the Output from one job to the Input of another job.
// Adapter job.Do() will never return error.
func Adapter[I, O any](name string, fn func(I) func(*O)) Jober[I, O] {
	if name == "" {
		name = fmt.Sprintf("Adapter(%s->%s)", typeOf[I](), typeOf[O]())
	}
	return Func(
		name,
		func(_ context.Context, i I) (func(*O), error) {
			return fn(i), nil
		},
	)
}

// Input constructs a Job that modifies the Input for other jobs.
func Input[O any](name string, fn func(*O)) Jober[struct{}, O] {
	if name == "" {
		name = fmt.Sprintf("Input(->%s)", typeOf[O]())
	}
	return Func(
		name,
		func(_ context.Context, _ struct{}) (func(*O), error) {
			return fn, nil
		},
	)
}

// Producer constructs a Job that produce the Input for other jobs.
func Producer[O any](name string, fn func(context.Context) (func(*O), error)) Jober[struct{}, O] {
	if name == "" {
		name = fmt.Sprintf("Producer(->%s)", typeOf[O]())
	}
	return Func(
		name,
		func(ctx context.Context, _ struct{}) (func(*O), error) {
			return fn(ctx)
		},
	)
}

// Consumer constructs a Job that consumes the Output from other jobs.
func Consumer[I any](name string, fn func(context.Context, I) error) Jober[I, struct{}] {
	if name == "" {
		name = fmt.Sprintf("Consumer(%s->)", typeOf[I]())
	}
	return Func(
		name,
		func(ctx context.Context, i I) (func(*struct{}), error) {
			return nil, fn(ctx, i)
		},
	)
}

type func_[I, O any] struct {
	BaseIn[I]
	name   string
	do     func(context.Context, I) (func(*O), error)
	output func(*O)
}

func (f *func_[I, O]) String() string {
	if f.name != "" {
		return f.name
	}
	return fmt.Sprintf("Func(%s->%s)", typeOf[I](), typeOf[O]())
}

func (f *func_[I, O]) Do(ctx context.Context) error {
	var err error
	f.output, err = f.do(ctx, f.In)
	return err
}

func (f *func_[I, O]) Output(o *O) {
	if f.output != nil {
		f.output(o)
	}
}

func typeOf[T any]() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

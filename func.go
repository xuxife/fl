package pl

import (
	"context"
	"fmt"
	"reflect"
)

// Input constructs a Job that output the given value
func Input[T any](v T) Job[struct{}, T] {
	return Func(
		fmt.Sprintf("Input(%s)", typeOf[T]()),
		func(ctx context.Context, _ struct{}) (T, error) {
			return v, nil
		},
	)
}

// Func constructs a Job from an arbitrary function
func Func[I, O any](name string, do func(context.Context, I) (O, error)) Job[I, O] {
	return &func_[I, O]{name: name, do: do}
}

type func_[I, O any] struct {
	Base
	InOut[I, O]
	name string
	do   func(context.Context, I) (O, error)
}

func (f *func_[I, O]) String() string {
	if f.name != "" {
		return f.name
	}
	return fmt.Sprintf("Func(%s -> %s)", typeOf[I](), typeOf[O]())
}

func (f *func_[I, O]) Do(ctx context.Context) error {
	var err error
	f.Out, err = f.do(ctx, f.In)
	return err
}

func typeOf[T any]() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

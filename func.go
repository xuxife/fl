package pl

import (
	"context"
	"fmt"
	"reflect"
)

// Func constructs a Step from an arbitrary function
func Func[I, O any](name string, do func(context.Context, I) (func(*O), error)) Steper[I, O] {
	return &func_[I, O]{name: name, do: do}
}

func FuncIn[I any](name string, do func(context.Context, I) error) Steper[I, struct{}] {
	return Func[I, struct{}](name, func(ctx context.Context, i I) (func(*struct{}), error) {
		return nil, do(ctx, i)
	})
}

func FuncOut[O any](name string, do func(context.Context) (func(*O), error)) Steper[struct{}, O] {
	return Func[struct{}, O](name, func(ctx context.Context, _ struct{}) (func(*O), error) {
		return do(ctx)
	})
}

func FuncNoInOut(name string, do func(context.Context) error) Steper[struct{}, struct{}] {
	return Func[struct{}, struct{}](name, func(ctx context.Context, s struct{}) (func(*struct{}), error) {
		return nil, do(ctx)
	})
}

type func_[I, O any] struct {
	StepBaseIn[I]
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

func typeOf[A any]() reflect.Type {
	var a A
	return reflect.TypeOf(a)
}

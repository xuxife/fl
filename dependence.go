package pl

import (
	"context"
	"fmt"
)

// NoDependency declares a Job without any dependency
func NoDependency[T any](t dependent[T]) *dependence {
	return DirectDependsOn(t)
}

// DependsOn connects two Jobs with adapter function.
//
// Usage:
//
//	DependsOn(
//		jobA, // Job[I, _]
//		jobB, // Job[_, O]
//	).WithAdapter(
//		func(O, *I) {}, // adapter
//	)
//
// The logic relationship is:
//
// _ -> jobB -> O -> adapter -> I -> jobA -> _
func DependsOn[I, O any](t dependent[I], cy dependency[O]) *adapterBuilder[I, O] {
	return &adapterBuilder[I, O]{t, cy}
}

type adapterBuilder[I, O any] struct {
	t  dependent[I]
	cy dependency[O]
}

func (a *adapterBuilder[I, O]) WithAdapter(adapter func(O, *I)) *dependence {
	return &dependence{
		t: a.t,
		cys: []struct {
			job   jobDoer
			apply func()
		}{
			{
				job: a.cy,
				apply: func() {
					var o O
					a.cy.Output(&o)
					adapter(o, a.t.Input())
				},
			},
		},
	}
}

// WithAdapter is used to inject an adapter before adding dependency.
//
// Usage:
//
//	DirectDependsOn(
//		jobA, // Job[I, _]
//		WithAdapter(
//			jobB, // Job[_, O]
//			func(O, *I) {}, // adapter
//		), // dependency[I]
//	)
func WithAdapter[I, O any](cy dependency[O], adapter func(O, *I)) dependency[I] {
	return Func(
		fmt.Sprintf("%s WithAdapter(%s -> %s)", cy, typeOf[O](), typeOf[I]()),
		func(ctx context.Context, o O) (I, error) {
			var i I
			if err := cy.Do(ctx); err != nil {
				return i, err
			}
			cy.Output(&o)
			adapter(o, &i)
			return i, nil
		},
	)
}

// DependsOn connects Jobs into dependence, which would feed into Workflow.Add().
//
// Usage:
//
//	DirectDependsOn(
//		jobA, // Job[T, _]
//		jobB, // Job[_, T]
//	)
//
// The logic relationship is:
//
// _ -> jobB -> T -> jobA -> _
func DirectDependsOn[T any](t dependent[T], cys ...dependency[T]) *dependence {
	ce := &dependence{t: t}
	for _, cy := range cys {
		ce.cys = append(ce.cys, struct {
			job   jobDoer
			apply func()
		}{
			job: cy,
			apply: func() {
				cy.Output(t.Input())
			},
		})
	}
	return ce
}

// dependenT
type dependent[I any] interface {
	Inputer[I]
	jobDoer
}

// dependenCY
type dependency[O any] interface {
	Outputer[O]
	jobDoer
}

// dependenCE
//
//	`dependency[T]` -> (then) -> `depdendent[T]`: this relation is `dependence`
type dependence struct {
	t   jobDoer // dependenT
	cys []struct {
		job   jobDoer
		apply func()
	} // dependenCYs
}

type jobDoer interface {
	Doer
	Reporter
	job
}

// ListDependencies list all depdencies in this depdendence
func (d *dependence) ListDepedencies() []Reporter {
	reporters := make([]Reporter, 0, len(d.cys))
	for _, cy := range d.cys {
		reporters = append(reporters, cy.job)
	}
	return reporters
}

func (d *dependence) applyAll() {
	for _, cy := range d.cys {
		cy.apply()
	}
}

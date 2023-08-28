package fl

import (
	"context"
	"fmt"
)

var Parallel = NoDependency

// NoDependency declares Job(s) without any dependency,
// which means they can be run in parallel.
func NoDependency(jobs ...jobDoer) dependence {
	ce := make(dependence)
	for _, j := range jobs {
		ce[j] = nil
	}
	return ce
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

func (a *adapterBuilder[I, O]) WithAdapter(adapter func(O, *I)) dependence {
	return dependence{
		a.t: []struct {
			job       jobDoer
			getOutput func()
		}{
			{
				job: a.cy,
				getOutput: func() {
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
func DirectDependsOn[T any](t dependent[T], cys ...dependency[T]) dependence {
	ce := make(dependence)
	for _, cy := range cys {
		ce[t] = append(ce[t], struct {
			job       jobDoer
			getOutput func()
		}{
			job: cy,
			getOutput: func() {
				cy.Output(t.Input())
			},
		})
	}
	return ce
}

type jobDoer interface {
	Doer
	Reporter
	job
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
type dependence map[jobDoer][]struct {
	job       jobDoer
	getOutput func()
}

func (d dependence) setInputFor(j jobDoer) {
	for _, cy := range d[j] {
		cy.getOutput()
	}
}

// ListDependencies list all depdencies in this depdendence
func (d dependence) ListDepedencies(j jobDoer) []Reporter {
	reporters := make([]Reporter, 0, len(d[j]))
	for _, cy := range d[j] {
		reporters = append(reporters, cy.job)
	}
	return reporters
}

func (d dependence) Merge(other dependence) {
	for j, cys := range other {
		d[j] = append(d[j], cys...)
		// track dependency jobs
		for _, cy := range cys {
			// this dependency job hasn't been tracked and has no dependency
			if _, ok := d[cy.job]; !ok {
				d[cy.job] = nil
			}
		}
	}
}

package pl

import (
	"context"
	"time"
)

// WorkflowStep adds Step(s) with or without dependency into a Workflow.
type WorkflowStep interface {
	Done() dependency
}

// Step declares a Step for Workflow.Add()
func Step[I any](r depender[I]) *addStep[I] {
	return &addStep[I]{
		r:  r,
		cy: make(dependency),
	}
}

type addStep[I any] struct {
	r  depender[I]
	cy dependency
}

// DependsOn declares dependency between Steps.
//
// DependsOn is used for that Dependee's Output != Depender's Input type.
//
// Use Adapt function to convert the Dependee's Output to Depender's Input.
//
// Usage:
//
//	// `a` depends on `as`
//	Step(a).DependsOn(
//		Adapt(as, func(o O, i *I) error {
//			// o is the Output of as
//			// i is the Input of a
//			// check o, fill i
//		}),
//	)
func (as *addStep[I]) DependsOn(adapts ...*adapt[I]) *addStep[I] {
	for _, adapt := range adapts {
		as.cy[as.r] = append(as.cy[as.r], link{
			Dependee: adapt.Dependee,
			Flow: func(ctx context.Context) error {
				return adapt.Flow(ctx, as.r.Input())
			},
		})
	}
	return as
}

// AdaptFunc bridges Dependee's Output to Depender's Input.
type AdaptFunc[I, O any] func(context.Context, O, *I) error

// Adapt is the bridge between Dependee and Depender.
//
// Adapt is only used in Step().DependsOn().
//
// Adapt function is used to convert the Dependee's Output to Depender's Input,
// with basic value validation.
//
// Usage:
//
//	// `a` depends on `as`
//	Step(a).DependsOn(
//		Adapt(as, func(o O, i *I) error {
//			// o is the Output of as
//			// i is the Input of a
//			// check o, fill i
//		}),
//	)
func Adapt[I, O any](e dependee[O], fn AdaptFunc[I, O]) *adapt[I] {
	return &adapt[I]{
		Dependee: e,
		Flow: func(ctx context.Context, i *I) error {
			return fn(ctx, GetOutput(e), i)
		},
	}
}

type adapt[I any] struct {
	Dependee StepDoer
	Flow     func(context.Context, *I) error
}

// DirectDependsOn declares dependency between Steps.
//
// DirectDependsOn is for Dependee's Output == Depender's Input type.
//
// Usage:
//
//	// `a` depends on `as` and `c`
//	Step(a).DirectDependsOn(as, c)
func (as *addStep[I]) DirectDependsOn(es ...dependee[I]) *addStep[I] {
	for _, e := range es {
		as.cy[as.r] = append(as.cy[as.r], link{
			Dependee: e,
			Flow: func(context.Context) error {
				e.Output(as.r.Input())
				return nil
			},
		})
	}
	return as
}

// ExtraDependsOn declares dependency between Steps WITHOUT any data flow.
//
// It means the Dependee(s) will still be executed BEFORE the Depender,
// but their Output will not be sent to Depender's Input.
func (as *addStep[I]) ExtraDependsOn(dependees ...StepDoer) *addStep[I] {
	for _, j := range dependees {
		as.cy[as.r] = append(as.cy[as.r], link{
			Dependee: j,
		})
	}
	return as
}

// Input sets the Input for the Step.
//
// If the Input function returns error, the Step will return a ErrFlow.
//
// Input respects the order in building calls, because it's actually a empty Dependee.
//
// i.e.
//
//	// `a` depends on `as`
//	Step(a).
//		Input(func(i *I) { ... }).	// this Input will be executed first
//		DependsOn(as, ...).			// then receive the Output from as
//		Input(func(i *I) { ... }),	// this Input is after as's Output set
func (as *addStep[I]) Input(fns ...func(context.Context, *I) error) *addStep[I] {
	as.cy[as.r] = append(as.cy[as.r], link{
		Flow: func(ctx context.Context) error {
			for _, fn := range fns {
				if err := fn(ctx, as.r.Input()); err != nil {
					return err
				}
			}
			return nil
		},
	})
	return as
}

// Timeout sets the Step timeout.
//
// It's the Step level timeout (beyond retry),
// add timeout to the context of Do(context.Context) if you need timeout for one retry.
func (as *addStep[I]) Timeout(timeout time.Duration) *addStep[I] {
	as.r.setTimeout(timeout)
	return as
}

// Condition decides whether the Step should be Canceled.
func (as *addStep[I]) Condition(cond Condition) *addStep[I] {
	as.r.setCondition(cond)
	return as
}

// When decides whether the Step should be Skipped.
func (as *addStep[I]) When(when When) *addStep[I] {
	as.r.setWhen(when)
	return as
}

// Retry sets the RetryOption for the Step.
func (as *addStep[I]) Retry(opt RetryOption) *addStep[I] {
	as.r.setRetry(&opt)
	return as
}

func (as *addStep[I]) Done() dependency {
	if _, ok := as.cy[as.r]; !ok {
		as.cy[as.r] = nil
	}
	return as.cy
}

// Steps declares a series of Steps.
//
// The Steps are mutually independent, and will be executed in parallel.
//
// Usage:
//
// - A series of Steps in parallel:
//
//	Steps(a, as, c) // a, as, c will be executed in parallel
//
// - A series of Steps in parallel, but after some other Steps:
//
//	Steps(a, as, c).DependsOn(d, e) // d, e will be executed in parallel, then a, as, c in parallel
func Steps(dependers ...StepDoer) addSteps {
	d := make(dependency)
	for _, r := range dependers {
		d[r] = nil
	}
	return addSteps(d)
}

// ToStepDoer converts []<StepDoer implemention> to []StepDoer.
//
// Usage:
//
//	steps := []someStepImpl{ ... }
//	suite.Add(
//		Steps(ToStepDoer(steps)...),
//	)
func ToStepDoer[S StepDoer](steps []S) []StepDoer {
	rv := []StepDoer{}
	for _, s := range steps {
		rv = append(rv, s)
	}
	return rv
}

type addSteps dependency

// DependsOn declares dependency with another group of Steps.
func (as addSteps) DependsOn(dependees ...StepDoer) addSteps {
	links := []link{}
	for _, e := range dependees {
		links = append(links, link{Dependee: e})
	}
	for r := range as {
		as[r] = append(as[r], links...)
	}
	return as
}

// Timeout sets the Step timeout.
func (as addSteps) Timeout(timeout time.Duration) addSteps {
	for j := range as {
		j.setTimeout(timeout)
	}
	return as
}

// Condition decides whether the Step should be Canceled.
func (as addSteps) Condition(cond Condition) addSteps {
	for j := range as {
		j.setCondition(cond)
	}
	return as
}

// When decides whether the Step should be Skipped.
func (as addSteps) When(when When) addSteps {
	for j := range as {
		j.setWhen(when)
	}
	return as
}

// Retry sets the RetryOption for the Step.
func (as addSteps) Retry(opt RetryOption) addSteps {
	for j := range as {
		j.setRetry(&opt)
	}
	return as
}

func (as addSteps) Done() dependency {
	return dependency(as)
}

// TSteps is Typed-Steps, which is used to declare Steps with the same Input type.
func TSteps[I any, T depender[I]](rs ...T) addTypedSteps[I] {
	as := []*addStep[I]{}
	for _, r := range rs {
		as = append(as, Step(r))
	}
	return as
}

type addTypedSteps[I any] []*addStep[I]

// DependsOn declares dependency between Steps.
//
// All steps are Dependers at the same time.
//
// Usage:
//
//	TSteps(a, as). // a, as share the same Input type
//		DependsOn(c, d) // c, d share the same Output type
//
// Then the order is: parallel(c, d) --then-> parallel(a, as),
// and a, as both will receive the Output from c and d.
func (as addTypedSteps[I]) DependsOn(adapts ...*adapt[I]) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.DependsOn(adapts...)
	}
	return as
}

// DirectDependsOn declares dependency between Steps.
func (as addTypedSteps[I]) DirectDependsOn(es ...dependee[I]) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.DirectDependsOn(es...)
	}
	return as
}

// ExtraDependsOn declares dependency between Steps WITHOUT any data flow.
func (as addTypedSteps[I]) ExtraDependsOn(steps ...StepDoer) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.ExtraDependsOn(steps...)
	}
	return as
}

// Input sets the Input for the Steps.
func (as addTypedSteps[I]) Input(fns ...func(context.Context, *I) error) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.Input(fns...)
	}
	return as
}

// Timeout sets the Steps timeout.
func (as addTypedSteps[I]) Timeout(timeout time.Duration) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.Timeout(timeout)
	}
	return as
}

// Condition decides whether the Steps should be Canceled.
func (as addTypedSteps[I]) Condition(cond Condition) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.Condition(cond)
	}
	return as
}

// When decides whether the Steps should be Skipped.
func (as addTypedSteps[I]) When(when When) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.When(when)
	}
	return as
}

// Retry sets the RetryOption for the Steps.
func (as addTypedSteps[I]) Retry(opt RetryOption) addTypedSteps[I] {
	for _, addStep := range as {
		addStep.Retry(opt)
	}
	return as
}

func (as addTypedSteps[I]) Done() dependency {
	d := make(dependency)
	for _, addStep := range as {
		d.merge(addStep.Done())
	}
	return d
}

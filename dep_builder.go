package pl

import "time"

type depBuilder interface {
	Done() Dependency
}

// Job declares a Job for Workflow.Add()
func Job[T any](r Depender[T]) *jobBuilder[T] {
	return &jobBuilder[T]{
		r:  r,
		cy: make(Dependency),
	}
}

type jobBuilder[T any] struct {
	r  Depender[T]
	cy Dependency
}

// DependsOn declares dependency between Jobs.
// DependsOn is used for that Dependee's Output != Depender's Input type,
// please use Adapt with a function to convert the Dependee's Output to Depender's Input.
//
// Usage:
//
//	Job(j1).DependsOn(
//		Adapt(j2, func(o O, i *I) {
//			// o is the Output of j2
//			// i is the Input of j1
//			// fill i from o
//		}),
//	)
func (jb *jobBuilder[T]) DependsOn(adapts ...*adapt[T]) *jobBuilder[T] {
	for _, a := range adapts {
		jb.cy[jb.r] = append(jb.cy[jb.r], link{
			Dependee: a.Dependee,
			Flow: func() {
				a.Flow(jb.r.Input())
			},
		})
	}
	return jb
}

// Adapt is only used in Job().DependsOn()
func Adapt[I, O any](e Dependee[O], fn func(O, *I)) *adapt[I] {
	return &adapt[I]{
		Dependee: e,
		Flow: func(i *I) {
			fn(GetOutput(e), i)
		},
	}
}

type adapt[I any] struct {
	Dependee job
	Flow     func(*I)
}

// DirectDependsOn declares dependency between Jobs.
// DirectDependsOn is used for that Dependee's Output == Depender's Input type.
//
// Usage:
//
//	Job(j1).DirectDependsOn(j2, j3)
func (jb *jobBuilder[T]) DirectDependsOn(es ...Dependee[T]) *jobBuilder[T] {
	for _, e := range es {
		jb.cy[jb.r] = append(jb.cy[jb.r], link{
			Dependee: e,
			Flow: func() {
				e.Output(jb.r.Input())
			},
		})
	}
	return jb
}

// ExtraDependsOn declares dependency between Jobs, but without any data flow.
// It means the Dependee(s) will still be executed before the Depender,
// but their Output will not be sent to Depender's Input.
func (jb *jobBuilder[T]) ExtraDependsOn(jobs ...job) *jobBuilder[T] {
	for _, j := range jobs {
		jb.cy[jb.r] = append(jb.cy[jb.r], link{
			Dependee: j,
		})
	}
	return jb
}

// Input sets the Input of the Job.
//
// Input respects the order in builder,
// because it's actually a empty Dependee.
// i.e.
//
//	Job(j1).
//		Input(func(i *I) { ... }).	// this Input will be executed first
//		DependsOn(j2).				// then receive the Output of j2
//		Input(func(i *I) { ... }).	// this Input is able to modify the Input again
func (jb *jobBuilder[T]) Input(fns ...func(*T)) *jobBuilder[T] {
	jb.cy[jb.r] = append(jb.cy[jb.r], link{
		Flow: func() {
			for _, fn := range fns {
				fn(jb.r.Input())
			}
		},
	})
	return jb
}

// Timeout sets the timeout of the Job.
// It's the overall timeout outside of retry, add timeout inside the .Do() if you need it for one try.
func (jb *jobBuilder[T]) Timeout(timeout time.Duration) *jobBuilder[T] {
	jb.r.setTimeout(timeout)
	return jb
}

// Condition sets the Condition of the Job, which decides whether the Job should be canceled based on Dependee(s)' status.
// If Condition is omitted, the default Condition is "all Dependee(s) are succeeded".
func (jb *jobBuilder[T]) Condition(cond Condition) *jobBuilder[T] {
	jb.r.setCondition(cond)
	return jb
}

// When sets the When of the Job, which decides when the Job should be skipped.
// If When is omitted, the default When is "never skip".
func (jb *jobBuilder[T]) When(when When) *jobBuilder[T] {
	jb.r.setWhen(when)
	return jb
}

// Retry sets the RetryOption of the Job.
func (jb *jobBuilder[T]) Retry(opt RetryOption) *jobBuilder[T] {
	jb.r.setRetry(&opt)
	return jb
}

func (jb *jobBuilder[T]) Done() Dependency {
	return jb.cy
}

// Jobs make a series of Jobs ready to connect other Jobs with dependency.
// It can also be used to declare Jobs without any dependency into a Workflow.
//
// Usage:
//
// - A series of Jobs in parallel:
//
//	Jobs(j1, j2, j3) // j1, j2, j3 will be executed in parallel
//
// - A series of Jobs in parallel, and connect them with other Jobs:
//
//	Jobs(j1, j2, j3).DependsOn(j4, j5) // j4, j5 will be executed in parallel, then j1, j2, j3
func Jobs(dependers ...job) jobsBuilder {
	d := make(Dependency)
	for _, r := range dependers {
		d[r] = nil
	}
	return jobsBuilder(d)
}

type jobsBuilder Dependency

func (jb jobsBuilder) DependsOn(dependees ...job) jobsBuilder {
	links := []link{}
	for _, e := range dependees {
		links = append(links, link{Dependee: e})
	}
	for r := range jb {
		jb[r] = append(jb[r], links...)
	}
	return jb
}

func (jb jobsBuilder) Timeout(timeout time.Duration) jobsBuilder {
	for j := range jb {
		j.setTimeout(timeout)
	}
	return jb
}

func (jb jobsBuilder) Condition(cond Condition) jobsBuilder {
	for j := range jb {
		j.setCondition(cond)
	}
	return jb
}

func (jb jobsBuilder) When(when When) jobsBuilder {
	for j := range jb {
		j.setWhen(when)
	}
	return jb
}

func (jb jobsBuilder) Retry(opt RetryOption) jobsBuilder {
	for j := range jb {
		j.setRetry(&opt)
	}
	return jb
}

func (jb jobsBuilder) Done() Dependency {
	return Dependency(jb)
}

package pl

// Job declares a Job ready to connect other Job with dependency, only use in Workflow.Add()
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

// Input sets the Input of the Job.
//
// This `Input` method differs from `Input` function (`Input(string, func(*T))`) in that:
//
//  1. The `Input` method is executed immediately during the build phase,
//     while the `Input` function is executed during the run phase.
//
//  2. The `Input` method not create a new Job instance, while the `Input` function does.
func (jb *jobBuilder[T]) Input(fn func(*T)) *jobBuilder[T] {
	fn(jb.r.Input())
	return jb
}

// DependsOn declares dependency between Jobs, that Dependee's Output doesn't match Depender's Input type,
// then use an adapter function to do the conversion.
//
// Usage:
//
//	Job(j1).DependsOn(
//		Adapt(j2, func(o O, i *I) {
//			// o is the Output of j2
//			// i is the Input of j1
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

// Adapt is only used in Job().DependsOn().
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

// DirectDependsOn declares dependency between Jobs, that Dependee's Output matches Depender's Input type.
//
// Usage:
//
//	Job(j1).DirectDependsOn(j2)
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

// ExtraDependsOn declares dependency between Jobs, and no data will flow between them.
func (jb *jobBuilder[T]) ExtraDependsOn(jobs ...job) *jobBuilder[T] {
	for _, j := range jobs {
		jb.cy[jb.r] = append(jb.cy[jb.r], link{
			Dependee: j,
		})
	}
	return jb
}

func (jb *jobBuilder[T]) Condition(cond Condition) *jobBuilder[T] {
	jb.r.SetCondition(cond)
	return jb
}

func (jb *jobBuilder[T]) Retry(opt RetryOption) *jobBuilder[T] {
	jb.r.SetRetryOption(opt)
	return jb
}

func (jb *jobBuilder[T]) When(when WhenFunc) *jobBuilder[T] {
	jb.r.SetWhen(when)
	return jb
}

func (jb *jobBuilder[T]) Done() Dependency {
	return jb.cy
}

// Jobs declares a bunch of Jobs ready to connect other Job with dependency.
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

func (jb jobsBuilder) Condition(cond Condition) jobsBuilder {
	for j := range jb {
		j.SetCondition(cond)
	}
	return jb
}

func (jb jobsBuilder) Retry(opt RetryOption) jobsBuilder {
	for j := range jb {
		j.SetRetryOption(opt)
	}
	return jb
}

func (jb jobsBuilder) When(when WhenFunc) jobsBuilder {
	for j := range jb {
		j.SetWhen(when)
	}
	return jb
}

func (jb jobsBuilder) Done() Dependency {
	return Dependency(jb)
}

type depBuilder interface {
	Done() Dependency
}

type job interface {
	base
	Doer
	Reporter
}

type Depender[I any] interface {
	Inputer[I]
	job
}

type Dependee[O any] interface {
	Outputer[O]
	job
}

type link struct {
	Dependee job
	Flow     func() // Flow sends Dependee's Output to Depender's Input
}

// Dependency is a relationship between Depender(s) and Dependee(s).
// We say "A depends on B", then A is Depender, B is Dependee.
type Dependency map[job][]link

func (d Dependency) ListDependeeOf(r job) []job {
	var dependees []job
	for _, l := range d[r] {
		dependees = append(dependees, l.Dependee)
	}
	return dependees
}

func (d Dependency) listDepedeeReporterOf(r job) []Reporter {
	var dependees []Reporter
	for _, l := range d[r] {
		dependees = append(dependees, l.Dependee)
	}
	return dependees
}

func (d Dependency) ListDependerOf(e job) []job {
	var dependers []job
	for r, links := range d {
		for _, l := range links {
			if l.Dependee == e {
				dependers = append(dependers, r)
				break
			}
		}
	}
	return dependers
}

// FlowInto flows all Depdenee(s)' Output(s) into the Depender's Input
func (d Dependency) FlowInto(r job) {
	for _, l := range d[r] {
		if l.Flow != nil {
			l.Flow()
		}
	}
}

func (d Dependency) Merge(other Dependency) {
	for r, links := range other {
		d[r] = append(d[r], links...)
		// we also need to add the Dependee(s) as 'Depender(s) without any Dependee'
		for _, l := range links {
			if _, ok := d[l.Dependee]; !ok {
				d[l.Dependee] = nil
			}
		}
	}
}

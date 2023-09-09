package pl

var Parallel = NoDependency

// NoDependency declares Job(s) without any dependency,
// which means they are mutually independent, and can be run in parallel.
func NoDependency(jobs ...jobDoer) Dependency {
	ce := make(Dependency)
	for _, j := range jobs {
		ce[j] = nil
	}
	return ce
}

// DependsOn connects two Jobs with an adapter function.
//
// Usage:
//
//	DependsOn(
//		jobThen, // Job[I, _]
//		jobFirst, // Job[_, O]
//	).WithAdapter(
//		func(O, *I) {}, // adapter
//	)
//
// The logic relationship is:
//
// _ -> jobFirst -> O -> adapter -> I -> jobThen -> _
func DependsOn[I, O any](r Depender[I], e Dependee[O]) *depBuilder[I, O] {
	return &depBuilder[I, O]{r, e}
}

type depBuilder[I, O any] struct {
	r Depender[I]
	e Dependee[O]
}

func (a *depBuilder[I, O]) WithAdapter(adapter func(O, *I)) Dependency {
	return Dependency{
		a.r: []link{
			{
				Depender: a.r,
				Dependee: a.e,
				Flow: func() {
					var o O
					a.e.Output(&o)
					adapter(o, a.r.Input())
				},
			},
		},
	}
}

// DirectDependsOn connects Jobs into a Dependency, which would be feeded into Workflow.Add().
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
func DirectDependsOn[T any](r Depender[T], es ...Dependee[T]) Dependency {
	d := make(Dependency)
	for _, e := range es {
		d[r] = append(d[r], link{
			Depender: r,
			Dependee: e,
			Flow: func() {
				e.Output(r.Input())
			},
		})
	}
	return d
}

// NoFlowDependsOn connects Jobs without any in/out flow.
func NoFlowDependsOn(r jobDoer, es ...jobDoer) Dependency {
	d := make(Dependency)
	for _, e := range es {
		d[r] = append(d[r], link{
			Depender: r,
			Dependee: e,
		})
	}
	return d
}

type jobDoer interface {
	job
	Doer
	Reporter
}

type Depender[I any] interface {
	Inputer[I]
	jobDoer
}

type Dependee[O any] interface {
	Outputer[O]
	jobDoer
}

type link struct {
	Depender jobDoer
	Dependee jobDoer
	Flow     func() // Flow sends Dependee's Output to Depender's Input
}

// Dependency is a relationship between Depender(s) and Dependee(s).
// We say "A depends on B", then A is Depender, B is Dependee.
type Dependency map[jobDoer][]link

func (d Dependency) ListDependeeOf(r jobDoer) []jobDoer {
	var dependees []jobDoer
	for _, l := range d[r] {
		dependees = append(dependees, l.Dependee)
	}
	return dependees
}

func (d Dependency) listDepedeeReporterOf(r jobDoer) []Reporter {
	var dependees []Reporter
	for _, l := range d[r] {
		dependees = append(dependees, l.Dependee)
	}
	return dependees
}

func (d Dependency) ListDependerOf(e jobDoer) []jobDoer {
	var dependers []jobDoer
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
func (d Dependency) FlowInto(r jobDoer) {
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

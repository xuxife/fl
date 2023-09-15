package pl

import (
	"context"
	"fmt"
)

// Jober[I, O any] is the basic unit of a Workflow.
//
//	I: input type
//	O: output type
//
// A Jober implement should always embed Base.
//
//	type SomeTask struct {
//		Base // always embed Base
//	}
//
// Then please implement the following interfaces:
//
//	String() string				// give this job a name
//	Input() *I					// return the reference to input of this job
//	Output(*O)					// fill the output
//	Do(context.Context) error	// the main logic of this job
//
// Tip: you can avoid nasty `Input() *I` implement by embed `BaseIn[I]` instead.
//
//	type SomeTask struct {
//		BaseIn[TaskInput] // inherit `Input() *TaskInput`
//	}
type Jober[I, O any] interface {
	base
	fmt.Stringer
	Inputer[I]
	Outputer[O]
	Doer
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
	base
	Doer
	Reporter
}

type Depender[I any] interface {
	job
	Inputer[I]
}

type Dependee[O any] interface {
	job
	Outputer[O]
}

// Dependency is a relationship between Depender(s) and Dependee(s).
// We say "A depends on B", then A is Depender, B is Dependee.
type Dependency map[job][]link

type link struct {
	Dependee job
	Flow     func() // Flow sends Dependee's Output to Depender's Input
}

// ListDependeeOf returns all Dependee(s) of a Depender.
func (d Dependency) ListDependeeOf(depender job) []job {
	var dependees []job
	for _, l := range d[depender] {
		if l.Dependee != nil {
			dependees = append(dependees, l.Dependee)
		}
	}
	return dependees
}

// ListDependerOf returns all Depender(s) of a Dependee.
func (d Dependency) ListDependerOf(dependee job) []job {
	var dependers []job
	for r, links := range d {
		for _, l := range links {
			if l.Dependee == dependee {
				dependers = append(dependers, r)
				break
			}
		}
	}
	return dependers
}

// Merge merges other Dependency into this Dependency.
func (d Dependency) Merge(other Dependency) {
	for r, links := range other {
		d[r] = append(d[r], links...)
		// need to add the Dependee(s) as key(s) also
		for _, l := range links {
			if l.Dependee != nil {
				if _, ok := d[l.Dependee]; !ok {
					d[l.Dependee] = nil
				}
			}
		}
	}
}

// this is for Workflow checking Condition
func (d Dependency) listDepedeeReporterOf(r job) []Reporter {
	var dependees []Reporter
	for _, l := range d[r] {
		if l.Dependee != nil {
			dependees = append(dependees, l.Dependee)
		}
	}
	return dependees
}

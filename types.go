package pl

import (
	"context"
)

// Steper[I, O any] is the basic unit of a Workflow.
//
//	I: input type
//	O: output type
//
// A Steper implement should always embed StepBase.
//
//	type SomeTask struct {
//		StepBase // always embed StepBase
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
type Steper[I, O any] interface {
	StepDoer
	inputer[I]
	outputer[O]
}

type inputer[I any] interface {
	Input() *I
}

type outputer[O any] interface {
	Output(*O)
}

// StepDoer is the non-generic version of Steper.
type StepDoer interface {
	stepBase
	Do(context.Context) error
	StepReader
}

// aka. downstream / client
type depender[I any] interface {
	StepDoer
	inputer[I]
}

// aka. upstream / provider
type dependee[O any] interface {
	StepDoer
	outputer[O]
}

// dependency is a relationship between Depender(s) and Dependee(s).
// We say "A depends on B", or "B happened-before A", then A is Depender, B is Dependee.
type dependency map[StepDoer][]link

// link represents one connection between a Depender and a Dependee,
// with the data Flow function.
type link struct {
	Dependee StepDoer
	Flow     func(context.Context) error // Flow sends Dependee's Output to Depender's Input
}

// UpstreamOf returns all Dependee(s) of a Depender.
func (d dependency) UpstreamOf(depender StepDoer) []StepDoer {
	var dependees []StepDoer
	for _, l := range d[depender] {
		if l.Dependee != nil {
			dependees = append(dependees, l.Dependee)
		}
	}
	return dependees
}

// DownstreamOf returns all Depender(s) of a Dependee.
// WARNING: this is expensive
func (d dependency) DownstreamOf(dependee StepDoer) []StepDoer {
	var dependers []StepDoer
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

// Steps returns all Steps in this Workflow.
func (d dependency) Steps() []StepDoer {
	var steps []StepDoer
	for s := range d {
		steps = append(steps, s)
	}
	return steps
}

// merge merges other Dependency into this Dependency.
func (d dependency) merge(other dependency) {
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
func (d dependency) listUpstreamReporterOf(r StepDoer) []StepReader {
	var dependees []StepReader
	for _, l := range d[r] {
		if l.Dependee != nil {
			dependees = append(dependees, l.Dependee)
		}
	}
	return dependees
}

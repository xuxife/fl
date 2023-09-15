package pl

import "fmt"

// JobStatus describes the status of a Job.
//
// The relations between JobStatus, Condition and When are:
//
//													/--false-> JobStatusSkipped
//	JobStatusPending -> [Condition] ---true--> [When] --true--> JobStatusRunning --err == nil--> JobStatusSucceeded
//									\--false-> JobStatusCanceled				\--err != nil--> JobStatusFailed
type JobStatus string

const (
	JobStatusPending   = ""
	JobStatusRunning   = "Running"
	JobStatusFailed    = "Failed"
	JobStatusSucceeded = "Succeeded"
	JobStatusCanceled  = "Canceled"
	JobStatusSkipped   = "Skipped"
)

func (s JobStatus) IsTerminated() bool {
	return s == JobStatusFailed || s == JobStatusSucceeded || s == JobStatusCanceled || s == JobStatusSkipped
}

func (s JobStatus) String() string {
	switch s {
	case JobStatusPending:
		return "Pending"
	case JobStatusRunning, JobStatusFailed, JobStatusSucceeded, JobStatusCanceled, JobStatusSkipped:
		return string(s)
	default:
		return "Unknown"
	}
}

type Reporter interface {
	fmt.Stringer
	GetStatus() JobStatus
}

// Condition is a function to determine whether the Job should be Canceled.
// Condition makes the decision based on the status of the Dependee Jobs.
// Condition is only called when all Dependees are terminated.
type Condition func(dependees []Reporter) bool

var DefaultCondition Condition = Succeeded

func Always(deps []Reporter) bool {
	return true
}

// Succeeded: all Dependees are Succeeded (or Skipped)
func Succeeded(dependees []Reporter) bool {
	for _, e := range dependees {
		switch e.GetStatus() {
		case JobStatusSucceeded, JobStatusSkipped:
			// do nothing
		case JobStatusFailed, JobStatusCanceled:
			return false
		}
	}
	return true
}

// Failed: at least one Dependee is Failed
func Failed(dependees []Reporter) bool {
	hasFailed := false
	for _, e := range dependees {
		switch e.GetStatus() {
		case JobStatusSucceeded, JobStatusSkipped:
			// do nothing
		case JobStatusFailed:
			hasFailed = true
		case JobStatusCanceled:
			return false
		}
	}
	return hasFailed
}

// SucceededOrFailed: all Dependees are Succeeded or Failed (or Skipped)
func SucceededOrFailed(deps []Reporter) bool {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusSucceeded, JobStatusFailed, JobStatusSkipped:
			// do nothing
		case JobStatusCanceled:
			return false
		}
	}
	return true
}

func Never(deps []Reporter) bool {
	return false
}

// When is a function to determine whether the Job should be Skipped.
// When makes the decesion according to the context and environment, so it's an arbitrary function.
// When is only called after Condition is true.
type When func() bool

var DefaultWhenFunc = When(func() bool {
	return true
})

func Skip() bool {
	return false
}

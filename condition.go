package pl

import (
	"context"
	"fmt"
)

// StepStatus describes the status of a Step.
//
// The relations between StepStatus, Condition and When are:
//
//													  /--false-> StepStatusSkipped
//	StepStatusPending -> [Condition] ---true--> [When] --true--> StepStatusRunning --err == nil--> StepStatusSucceeded
//									 \--false-> StepStatusCanceled				  \--err != nil--> StepStatusFailed
type StepStatus string

const (
	StepStatusPending   = ""
	StepStatusRunning   = "Running"
	StepStatusFailed    = "Failed"
	StepStatusSucceeded = "Succeeded"
	StepStatusCanceled  = "Canceled" // Canceled will be propagated through dependency
	StepStatusSkipped   = "Skipped"
)

func (s StepStatus) IsTerminated() bool {
	return s == StepStatusFailed || s == StepStatusSucceeded || s == StepStatusCanceled || s == StepStatusSkipped
}

func (s StepStatus) String() string {
	switch s {
	case StepStatusPending:
		return "Pending"
	case StepStatusRunning, StepStatusFailed, StepStatusSucceeded, StepStatusCanceled, StepStatusSkipped:
		return string(s)
	default:
		return "Unknown"
	}
}

// StepReader allows you to read the name and status of a Step.
type StepReader interface {
	fmt.Stringer
	GetStatus() StepStatus
}

// Condition is a function to determine whether the Step should be Canceled.
// Condition makes the decision based on the status of all the Dependee Steps.
// Condition is only called when all Dependees are terminated.
type Condition func(dependees []StepReader) bool

var DefaultCondition Condition = Succeeded

// Always: as long as all Dependees are terminated
func Always(deps []StepReader) bool {
	return true
}

// Succeeded: all Dependees are Succeeded (or Skipped)
func Succeeded(dependees []StepReader) bool {
	for _, e := range dependees {
		switch e.GetStatus() {
		case StepStatusSucceeded, StepStatusSkipped:
			// do nothing
		case StepStatusFailed, StepStatusCanceled:
			return false
		}
	}
	return true
}

// Failed: at least one Dependee is Failed
func Failed(dependees []StepReader) bool {
	hasFailed := false
	for _, e := range dependees {
		switch e.GetStatus() {
		case StepStatusSucceeded, StepStatusSkipped:
			// do nothing
		case StepStatusFailed:
			hasFailed = true
		case StepStatusCanceled:
			return false
		}
	}
	return hasFailed
}

// SucceededOrFailed: all Dependees are Succeeded or Failed (or Skipped)
func SucceededOrFailed(deps []StepReader) bool {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case StepStatusSucceeded, StepStatusFailed, StepStatusSkipped:
			// do nothing
		case StepStatusCanceled:
			return false
		}
	}
	return true
}

// Never: this step will always be Canceled
func Never(deps []StepReader) bool {
	return false
}

// When is a function to determine whether the Step should be Skipped.
// When makes the decesion according to the context and environment, so it's an arbitrary function.
// When is called after Condition.
type When func(context.Context) bool

var DefaultWhenFunc = When(func(context.Context) bool {
	return true
})

// Skip: this step will always be Skipped
func Skip(context.Context) bool {
	return false
}

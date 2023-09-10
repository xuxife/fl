package pl

import "fmt"

// JobStatus is a state machine defined as below:
//
//	Pending -> Running | Cancled | Skipped
//	Running -> Succeeded | Failed
type JobStatus string

const (
	JobStatusPending   = ""
	JobStatusRunning   = "Running"
	JobStatusFailed    = "Failed"
	JobStatusSucceeded = "Succeeded"
	JobStatusCanceled  = "Canceled" // Canceled is determined by Condition, will be propagated to Dependers except `Always` Condition
	JobStatusSkipped   = "Skipped"  // Skipped is determined by When or Condition, will be ignored for Dependers
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
	GetCondition() Condition
	GetRetryOption() RetryOption
	GetWhen() WhenFunc
}

// Condition is a condition function to determine the next status of a Job,
// based on dependency Jobs' statuses.
//
// Condition should only return
//
//	JobStatusPending
//	JobStatusRunning
//	JobStatusCanceled
//	JobStatusSkipped
type Condition func([]Reporter) JobStatus

var condReturnStatus = []JobStatus{
	JobStatusPending,
	JobStatusRunning,
	JobStatusCanceled,
	JobStatusSkipped,
}

// DefaultCondition is the default Cond for Job without calling SetCond()
var DefaultCondition Condition = Succeeded

// Always: all dependencies are succeeded, failed, or canceled
func Always(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		}
	}
	return JobStatusRunning
}

// Succeeded: all dependencies are succeeded
func Succeeded(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		case JobStatusFailed, JobStatusCanceled:
			return JobStatusCanceled
		}
	}
	return JobStatusRunning
}

// Failed: at least one dependency has failed
func Failed(deps []Reporter) JobStatus {
	hasFailed := false
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		case JobStatusFailed:
			hasFailed = true
		case JobStatusCanceled:
			return JobStatusCanceled
		}
	}
	if hasFailed {
		return JobStatusRunning
	}
	return JobStatusCanceled
}

// SucceededOrFailed: all dependencies are succeeded or failed, but cancel if a dependency is canceled
func SucceededOrFailed(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		case JobStatusCanceled:
			return JobStatusCanceled
		}
	}
	return JobStatusRunning
}

func Never(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		}
	}
	return JobStatusCanceled
}

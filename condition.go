package fl

import "fmt"

// JobStatus
//
//	Pending -> Running or Cancled
//	Running -> Succeeded or Failed
type JobStatus string

const (
	JobStatusPending   = ""
	JobStatusRunning   = "Running"
	JobStatusFailed    = "Failed"
	JobStatusSucceeded = "Succeeded"
	JobStatusCanceled  = "Canceled"
)

func (s JobStatus) IsTerminated() bool {
	return s == JobStatusFailed || s == JobStatusSucceeded || s == JobStatusCanceled
}

func (s JobStatus) String() string {
	switch s {
	case JobStatusPending:
		return "Pending"
	case JobStatusRunning, JobStatusFailed, JobStatusSucceeded, JobStatusCanceled:
		return string(s)
	default:
		return "Unknown"
	}
}

type Reporter interface {
	fmt.Stringer
	GetStatus() JobStatus
	GetCond() Cond
}

// Cond is a condition function to determine
// next status of a Job based on dependency Jobs' statuses.
//
// Cond should only return
//
//	JobStatusPending
//	JobStatusRunning
//	JobStatusCanceled
type Cond func([]Reporter) JobStatus

var condReturnStatus = []JobStatus{
	JobStatusPending,
	JobStatusRunning,
	JobStatusCanceled,
}

// DefaultCond is the default Cond for Job without calling SetCond()
var DefaultCond Cond = CondSucceeded

// CondAlways: all dependencies are succeeded, failed, or canceled
func CondAlways(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		}
	}
	return JobStatusRunning
}

// CondSucceeded: all dependencies are succeeded
func CondSucceeded(deps []Reporter) JobStatus {
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

// CondFailed: at least one dependency has failed
func CondFailed(deps []Reporter) JobStatus {
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

// CondSucceededOrFailed: all dependencies are succeeded or failed, but cancel if a dependency is canceled
func CondSucceededOrFailed(deps []Reporter) JobStatus {
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

func CondNever(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusPending, JobStatusRunning:
			return JobStatusPending
		}
	}
	return JobStatusCanceled
}

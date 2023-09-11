package pl

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
	JobStatusSkipped   = "Skipped"  // Skipped is determined by When, will be ignored for Dependers
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

// Condition is a condition function to determine whether the Job should be Run or Cancel,
// based on Dependee Jobs' statuses, you can assume the Dependee(s) are terminated.
//
//	true -> run
//	false -> cancel
type Condition func([]Reporter) bool

// DefaultCondition is the default Cond for Job without calling SetCond()
var DefaultCondition Condition = Succeeded

// Always: all dependencies are succeeded, failed, or canceled
func Always(deps []Reporter) bool {
	return true
}

// Succeeded: all dependencies are succeeded
func Succeeded(deps []Reporter) bool {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusFailed, JobStatusCanceled:
			return false
		}
	}
	return true
}

// Failed: at least one dependency has failed
func Failed(deps []Reporter) bool {
	hasFailed := false
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusFailed:
			hasFailed = true
		case JobStatusCanceled:
			return false
		}
	}
	return hasFailed
}

// SucceededOrFailed: all dependencies are succeeded or failed, but cancel if a dependency is canceled
func SucceededOrFailed(deps []Reporter) bool {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case JobStatusCanceled:
			return false
		}
	}
	return true
}

func Never(deps []Reporter) bool {
	return false
}

package pl

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Workflow interface {
	// Add appends dependences into Workflow.
	Add(...dependence) Workflow

	// GetJobs returns the jobs and its depedencies in Workflow.
	GetJobs() map[Reporter][]Reporter

	// Run starts and waits the Workflow terminated (blocking current goroutine).
	Run(context.Context) error

	// IsTerminated returns true if all jobs terminated.
	IsTerminated() bool

	// GetErrors returns the result errors of jobs in Workflow.
	//
	// Usage:
	//
	//  werr := workflow.GetErrors()
	//  if werr == nil {
	//      // all jobs succeeded or workflow has not run
	//  } else {
	//      jobAerr, ok := werr[jobA]
	//      switch {
	//      case !ok:
	//          // jobA has not finished or jobA is not in workflow
	//      case ok && jobAerr == nil:
	//          // jobA succeeded
	//      case ok && jobAerr != nil:
	//          // jobA failed
	//      }
	//  }
	GetErrors() ErrWorkflow

	// Reset resets every job's status to JobStatusPending,
	// will not reset input/output.
	// Reset will return ErrWorkflowIsRunning if the workflow is running.
	Reset() error
}

type workflow struct {
	mutex sync.Mutex
	jobs  dependence
	errs  ErrWorkflow

	oneJobTerminated chan struct{}
}

// NewWorkflow constructs a Workflow, append Jobs with depdencies into it with Add.
func NewWorkflow(ces ...dependence) Workflow {
	w := &workflow{
		mutex: sync.Mutex{},
		jobs:  make(dependence),
	}
	w.Add(ces...)
	return w
}

func (w *workflow) Add(ces ...dependence) Workflow {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.jobs == nil {
		w.jobs = make(dependence)
	}
	for _, ce := range ces {
		w.jobs.Merge(ce)
	}
	return w
}

func (w *workflow) GetJobs() map[Reporter][]Reporter {
	rv := map[Reporter][]Reporter{}
	for j := range w.jobs {
		rv[j] = w.jobs.ListDepedencies(j)
	}
	return rv
}

func (w *workflow) GetErrors() ErrWorkflow {
	if w.errs.IsNil() {
		return nil
	}
	rv := make(ErrWorkflow)
	for j, err := range w.errs {
		rv[j] = err
	}
	return rv
}

func (w *workflow) Run(ctx context.Context) error {
	if !w.mutex.TryLock() {
		return ErrWorkflowIsRunning
	}
	defer w.mutex.Unlock()

	// assert the dag of all jobs, whether there is a cycle dependency
	if err := w.preflight(); err != nil {
		return err
	}

	w.errs = make(ErrWorkflow)
	w.oneJobTerminated = make(chan struct{})
	defer close(w.oneJobTerminated)
	go func() {
		// send the first signal to start tick
		w.oneJobTerminated <- struct{}{}
	}()

	for range w.oneJobTerminated {
		if w.IsTerminated() {
			break
		}
		w.tick(ctx)
	}

	// check whether all jobs succeeded without error
	if w.errs.IsNil() {
		return nil
	}
	return w.errs
}

const jobStatusScaned = "Scaned" // a private status for dryrun

func condScan(deps []Reporter) JobStatus {
	for _, dep := range deps {
		switch dep.GetStatus() {
		case jobStatusScaned:
			// continue for
		default:
			return JobStatusPending
		}
	}
	return jobStatusScaned
}

func (w *workflow) preflight() error {
	// check whether the workflow has been run
	if w.errs != nil {
		return ErrWorkflowHasRun
	}

	// assert all jobs' status is Pending
	unexpectStatusJobs := []Reporter{}
	for j := range w.jobs {
		if j.GetStatus() != JobStatusPending {
			unexpectStatusJobs = append(unexpectStatusJobs, j)
		}
	}
	if len(unexpectStatusJobs) > 0 {
		return ErrUnexpectJobInitStatus(unexpectStatusJobs)
	}

	// start scanning, mark job as Scanned when its all depdencies are Scanned
	for {
		hasNewScanned := false // whether has new job being marked as Scanned this turn
		for j := range w.jobs {
			if j.GetStatus() != JobStatusPending {
				continue
			}
			if condScan(w.jobs.ListDepedencies(j)) == jobStatusScaned {
				hasNewScanned = true
				j.setStatus(jobStatusScaned)
			}
		}
		if !hasNewScanned { // break when no new job being Scanned
			break
		}
	}

	// check whether still have jobs not in Scanned,
	// not Scanned jobs are in a cycle.
	jobsInCycle := map[Reporter][]Reporter{}
	for j := range w.jobs {
		if j.GetStatus() != jobStatusScaned {
			for _, cy := range w.jobs.ListDepedencies(j) {
				if cy.GetStatus() != jobStatusScaned {
					jobsInCycle[j] = append(jobsInCycle[j], cy)
				}
			}
		}
	}
	if len(jobsInCycle) > 0 {
		return ErrCycleDependency(jobsInCycle)
	}

	// reset all jobs' status to Pending
	for j := range w.jobs {
		j.setStatus(JobStatusPending)
	}
	return nil
}

func (w *workflow) tick(ctx context.Context) {
	for j := range w.jobs {
		if j.GetStatus() != JobStatusPending {
			continue
		}
		newStatus := j.GetCond()(w.jobs.ListDepedencies(j))
		switch newStatus {
		case JobStatusPending:
			// do nothing
		case JobStatusRunning:
			j.setStatus(JobStatusRunning)
			w.jobs.setInputFor(j) // apply dependency's output to current job's input
			go func(j jobDoer) {
				w.errs[j] = j.Do(ctx)
				if w.errs[j] != nil {
					j.setStatus(JobStatusFailed)
				} else {
					j.setStatus(JobStatusSucceeded)
				}
				w.oneJobTerminated <- struct{}{}
			}(j)
		case JobStatusCanceled:
			go func(j jobDoer) {
				j.setStatus(JobStatusCanceled)
				w.oneJobTerminated <- struct{}{}
			}(j)
		default:
			panic(ErrUnexpectConditionResult(newStatus))
		}
	}
}

func (w *workflow) IsTerminated() bool {
	for j := range w.jobs {
		if !j.GetStatus().IsTerminated() {
			return false
		}
	}
	return true
}

func (w *workflow) Reset() error {
	if !w.mutex.TryLock() {
		return ErrWorkflowIsRunning
	}
	defer w.mutex.Unlock()

	w.errs = nil
	w.oneJobTerminated = nil
	for j := range w.jobs {
		j.setStatus(JobStatusPending)
	}
	return nil
}

type ErrWorkflow map[Reporter]error

func (e ErrWorkflow) Error() string {
	builder := new(strings.Builder)
	for reporter, err := range e {
		if err != nil {
			builder.WriteString(fmt.Sprintf(
				"%s [%s]: %s\n",
				reporter.String(), reporter.GetStatus().String(), err.Error(),
			))
		}
	}
	return builder.String()
}

func (e ErrWorkflow) IsNil() bool {
	for _, err := range e {
		if err != nil {
			return false
		}
	}
	return true
}

var ErrWorkflowIsRunning = fmt.Errorf("workflow is running, please wait for it terminated")
var ErrWorkflowHasRun = fmt.Errorf("workflow has run, fetch errors with GetErrors() and results from job.Output()")

type ErrUnexpectJobInitStatus []Reporter

func (e ErrUnexpectJobInitStatus) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("unexpect job init status: ")
	jStr := []string{}
	for _, j := range e {
		jStr = append(jStr, fmt.Sprintf("%s [%s]", j, j.GetStatus()))
	}
	builder.WriteString(strings.Join(jStr, ", "))
	builder.WriteRune('\n')
	return builder.String()
}

type ErrUnexpectConditionResult JobStatus

func (e ErrUnexpectConditionResult) Error() string {
	return fmt.Sprintf("unexpect condition result: %s, expect only %v", string(e), condReturnStatus)
}

type ErrCycleDependency map[Reporter][]Reporter

func (e ErrCycleDependency) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("CycleDependency, following jobs introduce cycle dependency:\n")
	for j, cys := range e {
		builder.WriteString(j.String())
		builder.WriteString(": [")
		cyStr := []string{}
		for _, cy := range cys {
			cyStr = append(cyStr, cy.String())
		}
		builder.WriteString(strings.Join(cyStr, ", "))
		builder.WriteString("]\n")
	}
	return builder.String()
}

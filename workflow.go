package pl

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Workflow interface {
	// Add appends dependences into Workflow
	Add(...*dependence) Workflow

	// GetJobs returns the jobs and its depedencies in Workflow
	GetJobs() map[Reporter][]Reporter

	// Run starts the Workflow (blocking)
	Run(context.Context) error

	// IsTerminated returns true if all jobs terminated
	IsTerminated() bool

	// GetErrors returns the result errors of jobs in Workflow
	GetErrors() map[Reporter]error

	// Reset resets every job's status to JobStatusPending,
	// will not reset input/output.
	Reset()
}

type workflow struct {
	mutex sync.Mutex
	jobs  map[jobDoer]*dependence
	errs  ErrWorkflow

	oneJobTerminated chan struct{}
}

func NewWorkflow(ces ...*dependence) Workflow {
	w := &workflow{
		mutex: sync.Mutex{},
		jobs:  make(map[jobDoer]*dependence),
	}
	w.Add(ces...)
	return w
}

func (w *workflow) Add(dpdces ...*dependence) Workflow {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.jobs == nil {
		w.jobs = make(map[jobDoer]*dependence)
	}
	for _, ce := range dpdces {
		var newCe *dependence
		t, cys := ce.t, ce.cys
		if oldCe, ok := w.jobs[t]; ok {
			// when the dependent is already tracked in w.jobs
			// append the depdencies after the previous ones
			newCe = &dependence{
				t:   t,
				cys: append(oldCe.cys, cys...),
			}
		} else {
			newCe = ce
		}
		w.jobs[t] = newCe
		// track dependency jobs
		for _, cy := range cys {
			if _, ok := w.jobs[cy.job]; !ok {
				// this dependency job hasn't been tracked and has no dependency
				w.jobs[cy.job] = &dependence{
					t:   cy.job,
					cys: nil,
				}
			}
		}
	}
	return w
}

func (w *workflow) GetJobs() map[Reporter][]Reporter {
	rv := map[Reporter][]Reporter{}
	for j, ce := range w.jobs {
		rv[j] = ce.ListDepedencies()
	}
	return rv
}

func (w *workflow) GetErrors() map[Reporter]error {
	rv := map[Reporter]error{}
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

	// check whether the workflow has been run
	if w.errs != nil {
		return ErrWorkflowHasRun
	}

	// scan the dag, check whether there is a cycle dependency
	if err := w.scan(); err != nil {
		return err
	}

	w.errs = make(ErrWorkflow)
	w.oneJobTerminated = make(chan struct{})
	defer close(w.oneJobTerminated)
	go func() {
		// send the first signal to start tick
		w.oneJobTerminated <- struct{}{}
	}()

loop:
	for {
		select {
		case <-w.oneJobTerminated: // some job is terminated
			if w.IsTerminated() {
				break loop
			}
			w.tick(ctx)
		}
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

func (w *workflow) scan() error {
	// set all jobs' status to Pending
	for j := range w.jobs {
		j.setStatus(JobStatusPending)
	}

	// start scanning, mark job as Scanned when all depdencies are Scanned
	for {
		hasNewScanned := false // whether has new job being marked as Scanned this turn
		for j, ce := range w.jobs {
			if j.GetStatus() != JobStatusPending {
				continue
			}
			switch condScan(ce.ListDepedencies()) {
			case jobStatusScaned:
				hasNewScanned = true
				j.setStatus(jobStatusScaned)
			}
		}
		if !hasNewScanned { // break when no new job being Scanned
			break
		}
	}

	// check whether still have jobs not in Scanned
	jobsInCycle := map[Reporter][]Reporter{}
	for j := range w.jobs {
		switch j.GetStatus() {
		case jobStatusScaned:
			// continue for
		default:
			jobsInCycle[j] = w.jobs[j].ListDepedencies()
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

func (w *workflow) IsTerminated() bool {
	for j := range w.jobs {
		if !j.GetStatus().IsTerminated() {
			return false
		}
	}
	return true
}

func (w *workflow) tick(ctx context.Context) {
	for j, ce := range w.jobs {
		if j.GetStatus() != JobStatusPending {
			continue
		}
		newStatus := j.GetCond()(ce.ListDepedencies())
		switch newStatus {
		case JobStatusPending:
			// do nothing
		case JobStatusRunning:
			j.setStatus(newStatus)
			ce.applyAll() // apply dependency's output to current input
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
			j.setStatus(newStatus)
			go func(j jobDoer) {
				w.errs[j] = ErrJobCanceled
				w.oneJobTerminated <- struct{}{}
			}(j)
		default:
			panic(ErrUnexpectConditionResult(newStatus))
		}
	}
}

func (w *workflow) Reset() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.errs = nil
	w.oneJobTerminated = nil
	for j := range w.jobs {
		j.setStatus(JobStatusPending)
	}
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

var ErrWorkflowIsRunning = fmt.Errorf("workflow is running")
var ErrWorkflowHasRun = fmt.Errorf("workflow has run")
var ErrJobCanceled = fmt.Errorf("job canceled")

type ErrUnexpectConditionResult JobStatus

func (e ErrUnexpectConditionResult) Error() string {
	return fmt.Sprintf("unexpect condition result: %s, expect only %v", string(e), condReturnStatus)
}

type ErrCycleDependency map[Reporter][]Reporter

func (e ErrCycleDependency) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("CycleDependency, following jobs introduce cycle dependency:\n")
	for j, cys := range e {
		builder.WriteString(fmt.Sprintf("%s: %v\n", j, cys))
	}
	return builder.String()
}

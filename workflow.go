package pl

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Workflow represents a collection of connected jobs that form a directed acyclic graph.
// It tracks the status of each job and executing them in a topological order.
type Workflow struct {
	deps      Dependency
	errs      ErrWorkflow
	errsMutex sync.RWMutex // need this because errs are written from each job's goroutine

	leaseBucket      chan struct{}  // constraint max concurrency of running jobs
	waitGroup        sync.WaitGroup // to prevent goroutine leak
	isRunning        sync.Mutex
	oneJobTerminated chan struct{} // signals for next tick
}

// Add appends dependences into Workflow.
func (w *Workflow) Add(dbs ...depBuilder) *Workflow {
	if w.deps == nil {
		w.deps = make(Dependency)
	}
	for _, db := range dbs {
		w.deps.Merge(db.Done())
	}
	return w
}

// Dep returns the jobs and its depedencies in this Workflow.
//
// Iterate all jobs and its dependencies:
//
//	for j, deps := range workflow.Dep() {
//		// do something with j
//		for _, link := range deps {
//			link.Dependee // do something with j's Dependee
//		}
//	}
func (w *Workflow) Dep() Dependency {
	// make a copy to prevent w.deps being modified
	d := make(Dependency)
	d.Merge(w.deps)
	return d
}

// Run starts the job execution in topological order,
// and waits until all jobs terminated.
//
// Run blocks the current goroutine.
func (w *Workflow) Run(ctx context.Context) error {
	if !w.isRunning.TryLock() {
		return ErrWorkflowIsRunning
	}
	defer w.isRunning.Unlock()

	// preflight check the initial state of workflow
	if err := w.preflight(); err != nil {
		return err
	}

	w.errs = make(ErrWorkflow)
	w.oneJobTerminated = make(chan struct{})
	// send the first signal to start tick
	w.waitGroup.Add(1)
	go w.signalTick()
	// everytime one job terminated, tick
	for range w.oneJobTerminated {
		if w.IsTerminated() {
			break
		}
		w.tick(ctx)
	}
	// consume all the following singals cooperataed with waitGroup
	go func() {
		for range w.oneJobTerminated {
		}
	}()
	w.waitGroup.Wait()
	close(w.oneJobTerminated)

	// check whether all jobs succeeded without error
	if w.errs.IsNil() {
		return nil
	}
	return w.errs
}

const scanned JobStatus = "scanned" // a private status for preflight

func isAllDependeeScanned(deps []Reporter) bool {
	for _, dep := range deps {
		if dep.GetStatus() != scanned {
			return false
		}
	}
	return true
}

func (w *Workflow) preflight() error {
	// check whether the workflow has been run
	if w.errs != nil {
		return ErrWorkflowHasRun
	}

	// assert all jobs' status is Pending
	unexpectStatusJobs := []Reporter{}
	for j := range w.deps {
		if j.GetStatus() != JobStatusPending {
			unexpectStatusJobs = append(unexpectStatusJobs, j)
		}
	}
	if len(unexpectStatusJobs) > 0 {
		return ErrUnexpectJobInitStatus(unexpectStatusJobs)
	}

	// assert all dependency would not form a cycle
	// start scanning, mark job as Scanned only when its all depdencies are Scanned
	for {
		hasNewScanned := false // whether a new job being marked as Scanned this turn
		for j := range w.deps {
			if j.GetStatus() == scanned {
				continue
			}
			if isAllDependeeScanned(w.deps.listDepedeeReporterOf(j)) {
				hasNewScanned = true
				j.setStatus(scanned)
			}
		}
		if !hasNewScanned { // break when no new job being Scanned
			break
		}
	}

	// check whether still have jobs not in Scanned,
	// not Scanned jobs are in a cycle.
	jobsInCycle := map[Reporter][]Reporter{}
	for j := range w.deps {
		if j.GetStatus() != scanned {
			for _, dep := range w.deps.listDepedeeReporterOf(j) {
				if dep.GetStatus() != scanned {
					jobsInCycle[j] = append(jobsInCycle[j], dep)
				}
			}
		}
	}
	if len(jobsInCycle) > 0 {
		return ErrCycleDependency(jobsInCycle)
	}

	// reset all jobs' status to Pending
	for j := range w.deps {
		j.setStatus(JobStatusPending)
	}
	return nil
}

func (w *Workflow) signalTick() {
	w.oneJobTerminated <- struct{}{}
	w.waitGroup.Done()
}

func (w *Workflow) tick(ctx context.Context) {
tick:
	for j := range w.deps {
		if j.GetStatus() != JobStatusPending {
			continue
		}
		// check whether all Dependee(s) are terminated
		es := w.deps.listDepedeeReporterOf(j)
		for _, e := range es {
			if !e.GetStatus().IsTerminated() {
				continue tick
			}
		}
		// check whether the job should be cancel via Condition
		cond := j.getCondition()
		if cond == nil {
			cond = DefaultCondition
		}
		if !cond(es) {
			j.setStatus(JobStatusCanceled)
			w.waitGroup.Add(1)
			go w.signalTick()
			continue
		}
		// check whether the job should be skip via When
		when := j.getWhen()
		if when == nil {
			when = DefaultWhenFunc
		}
		if !when() {
			j.setStatus(JobStatusSkipped)
			w.waitGroup.Add(1)
			go w.signalTick()
			continue
		}
		// if WithMaxConcurrency is set
		if w.leaseBucket != nil {
			w.leaseBucket <- struct{}{} // lease
		}
		// start the job
		j.setStatus(JobStatusRunning)
		w.waitGroup.Add(1)
		go w.kickoff(ctx, j)
	}
}

func (w *Workflow) kickoff(ctx context.Context, j job) {
	// set timeout for the job
	var notAfter time.Time
	timeout := j.getTimeout()
	if timeout > 0 {
		notAfter = time.Now().Add(timeout)
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// run the job with retry
	do := w.do(j)
	var err error
	if retry := j.getRetry(); retry == nil { // disable retry
		err = do(ctx)
	} else {
		err = retry.Run(ctx, do, notAfter)
	}
	// use mutex to guard errs because
	// for a job not run, the job would not be in errs
	w.errsMutex.Lock()
	w.errs[j] = err
	w.errsMutex.Unlock()
	// mark the job as succeeded or failed
	if err != nil {
		j.setStatus(JobStatusFailed)
	} else {
		j.setStatus(JobStatusSucceeded)
	}
	// unlease
	if w.leaseBucket != nil {
		<-w.leaseBucket
	}
	w.signalTick()
}

func (w *Workflow) do(j job) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// apply dependee's output to current job's input
		for _, l := range w.deps[j] {
			if l.Dependee != nil {
				switch l.Dependee.GetStatus() {
				case JobStatusSucceeded, JobStatusFailed:
					// only flow data from succeeded or failed job
					// TODO(xuxife): is this a good decision?
				default:
					continue
				}
			} // or flow data from Dependee == nil (it's Input)
			if l.Flow != nil {
				l.Flow()
			}
		}
		return j.Do(ctx)
	}
}

// IsTerminated returns true if all jobs terminated.
func (w *Workflow) IsTerminated() bool {
	for j := range w.deps {
		if !j.GetStatus().IsTerminated() {
			return false
		}
	}
	return true
}

// Err returns the result errors of jobs in Workflow.
//
// Usage:
//
//	werr := workflow.Err()
//	if werr == nil {
//	    // all jobs succeeded or workflow has not run
//	} else {
//	    jobErr, ok := werr[job]
//	    switch {
//	    case !ok:
//	        // jobA has not finished or jobA is not in workflow
//	    case jobErr == nil:
//	        // jobA succeeded
//	    case jobErr != nil:
//	        // jobA failed
//	    }
//	}
func (w *Workflow) Err() ErrWorkflow {
	w.errsMutex.RLock()
	defer w.errsMutex.RUnlock()
	if w.errs.IsNil() {
		return nil
	}
	werr := make(ErrWorkflow)
	for j, err := range w.errs {
		werr[j] = err
	}
	return werr
}

// Reset resets every job's status to JobStatusPending,
// will not reset input/output.
// Reset will return ErrWorkflowIsRunning if the workflow is running.
func (w *Workflow) Reset() error {
	if !w.isRunning.TryLock() {
		return ErrWorkflowIsRunning
	}
	w.isRunning.Unlock()

	for j := range w.deps {
		j.setStatus(JobStatusPending)
	}
	w.errs = nil
	w.leaseBucket = nil
	w.oneJobTerminated = nil
	return nil
}

// Option alters the behavior of Workflow.
type Option func(*Workflow)

func (w *Workflow) WithOptions(opts ...Option) *Workflow {
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// WithMaxConcurrency limits the max concurrency of running jobs.
func WithMaxConcurrency(n int) Option {
	return func(w *Workflow) {
		// use buffered channel as a sized bucket
		// a job needs to create a lease in the bucket to run,
		// and remove the lease from the bucket when it's done.
		w.leaseBucket = make(chan struct{}, n)
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

var ErrWorkflowIsRunning = fmt.Errorf("workflow is running, please wait for it terminated")
var ErrWorkflowHasRun = fmt.Errorf("workflow has run, check result error with Err(), reset the Workflow with Reset()")

type ErrUnexpectJobInitStatus []Reporter

func (e ErrUnexpectJobInitStatus) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("unexpect job init status:\n")
	for _, j := range e {
		builder.WriteString(fmt.Sprintf(
			"%s [%s]\n",
			j, j.GetStatus(),
		))
	}
	return builder.String()
}

type ErrCycleDependency map[Reporter][]Reporter

func (e ErrCycleDependency) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("following jobs introduce cycle dependency:\n")
	for j, deps := range e {
		depsStr := []string{}
		for _, dep := range deps {
			depsStr = append(depsStr, dep.String())
		}
		builder.WriteString(fmt.Sprintf(
			"%s: [%s]\n",
			j, strings.Join(depsStr, ", "),
		))
	}
	return builder.String()
}

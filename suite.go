package pl

import (
	"context"
	"sync"
	"time"
)

// Workflow represents a collection of connected Steps that form a directed acyclic graph (DAG).
//
// Workflow executes Steps in a topological order,
// and flow the Output(s) from Dependee(s) to Input(s) of Depender(s).
type Workflow struct {
	deps              dependency
	errs              ErrWorkflow
	errsMu            sync.RWMutex   // need this because errs are written from each Step's goroutine
	when              When           // Workflow level When
	leaseBucket       chan struct{}  // constraint max concurrency of running Steps
	waitGroup         sync.WaitGroup // to prevent goroutine leak, only Add(1) when a Step start running
	isRunning         sync.Mutex
	oneStepTerminated chan struct{} // signals for next tick
}

// Add appends Steps into Workflow.
func (s *Workflow) Add(dbs ...WorkflowStep) *Workflow {
	if s.deps == nil {
		s.deps = make(dependency)
	}
	for _, db := range dbs {
		s.deps.merge(db.Done())
	}
	return s
}

// Dep returns the Steps and its depedencies in this Workflow.
//
// Iterate all Steps and its dependencies:
//
//	for step, deps := range workflow.Dep() {
//		// do something with step
//		for _, link := range deps {
//			link.Dependee // do something with step's Dependee
//		}
//	}
func (s *Workflow) Dep() dependency {
	// make a copy to prevent w.deps being modified
	d := make(dependency)
	d.merge(s.deps)
	return d
}

// Run starts the Step execution in topological order,
// and waits until all Steps terminated.
//
// Run will block the current goroutine.
func (s *Workflow) Run(ctx context.Context) error {
	if !s.isRunning.TryLock() {
		return ErrWorkflowIsRunning
	}
	defer s.isRunning.Unlock()

	if s.when != nil && !s.when(ctx) {
		for step := range s.deps {
			step.setStatus(StepStatusSkipped)
		}
		return nil
	}

	// preflight check the initial state of workflow
	if err := s.preflight(); err != nil {
		return err
	}

	s.errs = make(ErrWorkflow)
	s.oneStepTerminated = make(chan struct{}, len(s.deps))
	// first tick
	s.tick(ctx)
	// each time one Step terminated, tick forward
	for range s.oneStepTerminated {
		if s.IsTerminated() {
			break
		}
		s.tick(ctx)
	}
	// consume all the following singals cooperataed with waitGroup
	s.waitGroup.Wait()
	close(s.oneStepTerminated)

	// check whether all Steps succeeded without error
	if s.errs.IsNil() {
		return nil
	}
	return s.errs
}

const scanned StepStatus = "scanned" // a private status for preflight

func isAllDependeeScanned(deps []StepReader) bool {
	for _, dep := range deps {
		if dep.GetStatus() != scanned {
			return false
		}
	}
	return true
}

func (s *Workflow) preflight() error {
	// check whether the workflow has been run
	if s.errs != nil {
		return ErrWorkflowHasRun
	}

	// assert all Steps' status is Pending
	unexpectStatusSteps := []StepReader{}
	for step := range s.deps {
		if step.GetStatus() != StepStatusPending {
			unexpectStatusSteps = append(unexpectStatusSteps, step)
		}
	}
	if len(unexpectStatusSteps) > 0 {
		return ErrUnexpectStepInitStatus(unexpectStatusSteps)
	}

	// assert all dependency would not form a cycle
	// start scanning, mark Step as Scanned only when its all depdencies are Scanned
	for {
		hasNewScanned := false // whether a new Step being marked as Scanned this turn
		for step := range s.deps {
			if step.GetStatus() == scanned {
				continue
			}
			if isAllDependeeScanned(s.deps.listUpstreamReporterOf(step)) {
				hasNewScanned = true
				step.setStatus(scanned)
			}
		}
		if !hasNewScanned { // break when no new Step being Scanned
			break
		}
	}
	// check whether still have Steps not Scanned,
	// not Scanned Steps are in a cycle.
	stepsInCycle := map[StepReader][]StepReader{}
	for step := range s.deps {
		if step.GetStatus() != scanned {
			for _, dep := range s.deps.listUpstreamReporterOf(step) {
				if dep.GetStatus() != scanned {
					stepsInCycle[step] = append(stepsInCycle[step], dep)
				}
			}
		}
	}
	if len(stepsInCycle) > 0 {
		return ErrCycleDependency(stepsInCycle)
	}

	// reset all Steps' status to Pending
	for step := range s.deps {
		step.setStatus(StepStatusPending)
	}
	return nil
}

func (s *Workflow) signalTick() {
	s.oneStepTerminated <- struct{}{}
}

// tick will not block, it starts a goroutine for each runnable Step.
func (s *Workflow) tick(ctx context.Context) {
tick:
	for step := range s.deps {
		// skip if the Step is not Pending
		if step.GetStatus() != StepStatusPending {
			continue
		}
		// check whether all Dependees / Upstreams are terminated
		es := s.deps.listUpstreamReporterOf(step)
		for _, e := range es {
			if !e.GetStatus().IsTerminated() {
				continue tick
			}
		}
		// check whether the Step should be Canceled via Condition
		cond := step.getCondition()
		if cond == nil {
			cond = DefaultCondition
		}
		if !cond(es) {
			step.setStatus(StepStatusCanceled)
			s.signalTick()
			continue
		}
		// check whether the Step should be skip via When
		when := step.getWhen()
		if when == nil {
			when = DefaultWhenFunc
		}
		if !when(ctx) {
			step.setStatus(StepStatusSkipped)
			s.signalTick()
			continue
		}
		// if WithMaxConcurrency is set
		if s.leaseBucket != nil {
			s.leaseBucket <- struct{}{} // lease
		}
		// start the Step
		step.setStatus(StepStatusRunning)
		s.waitGroup.Add(1)
		go func(ctx context.Context, step StepDoer) {
			defer s.waitGroup.Done()
			err := s.runStep(ctx, step)
			// mark the Step as succeeded or failed
			if err != nil {
				step.setStatus(StepStatusFailed)
			} else {
				step.setStatus(StepStatusSucceeded)
			}
			if s.leaseBucket != nil {
				<-s.leaseBucket // unlease
			}
			s.signalTick()
		}(ctx, step)
	}
}

func (s *Workflow) runStep(ctx context.Context, step StepDoer) error {
	// set timeout for the Step
	var notAfter time.Time
	timeout := step.getTimeout()
	if timeout > 0 {
		notAfter = time.Now().Add(timeout)
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// run the Step with or without retry
	do := s.makeDoForStep(step)
	var err error
	if retryOpt := step.getRetry(); retryOpt == nil {
		err = do(ctx)
	} else {
		err = s.retry(retryOpt)(ctx, do, notAfter)
	}
	// use mutex to guard errs
	s.errsMu.Lock()
	s.errs[step] = err
	s.errsMu.Unlock()
	return err
}

// makeDoForStep is panic-free from Step's Do and Input.
func (s *Workflow) makeDoForStep(step StepDoer) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return catchPanicAsError(
			func() error {
				// apply dependee's output to current Step's input
				for _, l := range s.deps[step] {
					if l.Dependee != nil {
						switch l.Dependee.GetStatus() {
						case StepStatusSucceeded, StepStatusFailed:
							// only flow data from succeeded or failed Step
							// TODO(xuxife): is this a good decision?
						default:
							continue
						}
					} // or flow data from Dependee == nil (it's Input)
					if l.Flow != nil {
						if ferr := catchPanicAsError(func() error {
							return l.Flow(ctx)
						}); ferr != nil {
							return &ErrFlow{
								Err:  ferr,
								From: l.Dependee,
							}
						}
					}
				}
				return step.Do(ctx)
			},
		)
	}
}

// IsTerminated returns true if all Steps terminated.
func (s *Workflow) IsTerminated() bool {
	for step := range s.deps {
		if !step.GetStatus().IsTerminated() {
			return false
		}
	}
	return true
}

// Err returns the errors of all Steps in Workflow.
//
// Usage:
//
//	suiteErr := suite.Err()
//	if suiteErr == nil {
//	    // all Steps succeeded or workflow has not run
//	} else {
//	    stepErr, ok := suiteErr[StepA]
//	    switch {
//	    case !ok:
//	        // StepA has not finished or StepA is not in Workflow
//	    case stepErr == nil:
//	        // StepA succeeded
//	    case stepErr != nil:
//	        // StepA failed
//	    }
//	}
func (s *Workflow) Err() ErrWorkflow {
	s.errsMu.RLock()
	defer s.errsMu.RUnlock()
	if s.errs.IsNil() {
		return nil
	}
	werr := make(ErrWorkflow)
	for step, err := range s.errs {
		werr[step] = err
	}
	return werr
}

// Reset resets every Step's status to StepStatusPending,
// will not reset input/output.
// Reset will return ErrWorkflowIsRunning if the workflow is running.
func (s *Workflow) Reset() error {
	if !s.isRunning.TryLock() {
		return ErrWorkflowIsRunning
	}
	s.isRunning.Unlock()

	for step := range s.deps {
		step.setStatus(StepStatusPending)
	}
	s.errs = nil
	s.leaseBucket = nil
	s.oneStepTerminated = nil
	return nil
}

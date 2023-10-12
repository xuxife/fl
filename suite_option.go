package pl

// WorkflowOption alters the behavior of a Workflow.
type WorkflowOption func(*Workflow)

func (s *Workflow) WithOptions(opts ...WorkflowOption) *Workflow {
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WorkflowMaxConcurrency limits the max concurrency of running Steps.
func WorkflowMaxConcurrency(n int) WorkflowOption {
	return func(s *Workflow) {
		// use buffered channel as a sized bucket
		// a Step needs to create a lease in the bucket to run,
		// and remove the lease from the bucket when it's done.
		s.leaseBucket = make(chan struct{}, n)
	}
}

// WorkflowWhen sets the Workflow-level When condition.
func WorkflowWhen(when When) WorkflowOption {
	return func(s *Workflow) {
		s.when = when
	}
}

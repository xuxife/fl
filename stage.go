package pl

import (
	"context"
	"fmt"
)

// Stage wraps a Workflow into a Job.
// It's feasible to use Stage inside Workflow.Add()
type Stage[I, O any] struct {
	BaseIn[I]
	Name       string
	Workflow   *Workflow
	SetInput   func(I)  // SetInput from Stage Input to the inside Jobs' Input
	FillOutput func(*O) // FillOutput fills the Stage Output from the inside Jobs' Output
}

func (s *Stage[I, O]) String() string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("Stage(%s->%s)", typeOf[I](), typeOf[O]())
}

func (s *Stage[I, O]) Output(o *O) {
	if s.FillOutput != nil {
		s.FillOutput(o)
	}
}

func (s *Stage[I, O]) Do(ctx context.Context) error {
	if s.SetInput != nil {
		s.SetInput(s.In)
	}
	return s.Workflow.Run(ctx)
}

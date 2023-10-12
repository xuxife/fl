package pl

import (
	"context"
	"fmt"
)

// Stage wraps a Workflow into a Step.
//
// This is tricky, because we actually can use Workflow as a Step.
//
// Usage:
//
//	s := new(Workflow)
//	// build s with many Steps.
//	// then we can GROUP Workflow s as a Step and put it into a global Workflow
//	stage := &Stage[I, O]{
//		Name: "StageBuild"
//		Workflow: s,
//	}
type Stage[I, O any] struct {
	StepBaseIn[I]
	Name      string
	Workflow  *Workflow
	SetInput  func(I)  // SetInput sets the inside Steps' Input from Stage Input
	SetOutput func(*O) // SetOutput sets the Stage Output from the inside Steps' Output
}

func (s *Stage[I, O]) String() string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("Stage(%s->%s)", typeOf[I](), typeOf[O]())
}

func (s *Stage[I, O]) Output(o *O) {
	if s.SetOutput != nil {
		s.SetOutput(o)
	}
}

func (s *Stage[I, O]) Do(ctx context.Context) error {
	if s.SetInput != nil {
		s.SetInput(s.In)
	}
	return s.Workflow.Run(ctx)
}

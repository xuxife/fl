package pl

import (
	"context"
	"fmt"
)

// Connect connects two Jobs into a new Job.
// The execution order of new Job would be
//
//	I -> first.Do -> M -> then.Do -> O
//
// Notice that user should NOT use the original `First` and `Then` any more to prevent unexpected behavior.
func Connect[I, M, O any](name string, first Job[I, M], then Job[M, O]) Job[I, O] {
	return &connect[I, M, O]{name: name, first: first, then: then}
}

type connect[I, M, O any] struct {
	Base
	name  string
	first Job[I, M]
	then  Job[M, O]
}

func (c *connect[I, M, O]) String() string {
	if c.name != "" {
		return c.name
	}
	return fmt.Sprintf("Connect(%s->%s)", c.first, c.then)
}

func (c *connect[I, M, O]) Input() *I {
	return c.first.Input()
}

func (c *connect[I, M, O]) Output(o *O) {
	c.then.Output(o)
}

func (c *connect[I, M, O]) Do(ctx context.Context) error {
	if err := c.first.Do(ctx); err != nil {
		return err
	}
	c.first.Output(c.then.Input())
	return c.then.Do(ctx)
}

package pl

import (
	"context"
	"fmt"
)

// Connect connects two Jobs into a new Job.
// The execution order of new Job would be
//
//	First.Input() -> First.Do() -> First.Output() -> Then.Input() -> Then.Do() -> Then.Output
//
// Notice that user should NOT use the original `First` and `Then` any more to prevent unexpected behavior.
func Connect[I1, O1, O2 any](name string, first Job[I1, O1], then Job[O1, O2]) Job[I1, O2] {
	return &connect[I1, O1, O2]{name: name, first: first, then: then}
}

type connect[I1, O1, O2 any] struct {
	Base
	name  string
	first Job[I1, O1]
	then  Job[O1, O2]
}

func (c *connect[I1, O1, O2]) String() string {
	if c.name != "" {
		return c.name
	}
	return fmt.Sprintf("(%s -> %s)", c.first, c.then)
}

func (c *connect[I1, O1, O2]) Input() *I1 {
	return c.first.Input()
}

func (c *connect[I1, O1, O2]) Output(o *O2) {
	c.then.Output(o)
}

func (c *connect[I1, O1, O2]) Do(ctx context.Context) error {
	if err := c.first.Do(ctx); err != nil {
		return err
	}
	c.first.Output(c.then.Input())
	return c.then.Do(ctx)
}

package fl

import (
	"context"
	"fmt"
	"strings"
)

// Start your Job implement
type Concat struct {
	Base
	JobInput  []string
	JobOutput string
}

func (j *Concat) Input() *[]string {
	return &j.JobInput
}

func (j *Concat) Output(s *string) {
	*s = j.JobOutput
}

func (j *Concat) Do(ctx context.Context) error {
	j.JobOutput = ""
	for _, s := range j.JobInput {
		j.JobOutput += s
	}
	return nil
}

func (j *Concat) String() string {
	return "Concat"
}

type ToUpper struct {
	Base // embed Base
	InOut[string, string]
}

func (j *ToUpper) Do(ctx context.Context) error {
	j.Out = strings.ToUpper(j.In)
	return nil
}

func (j *ToUpper) String() string {
	return "ToUpper"
}

func ExampleWorkflow() {
	// initialize your job
	concat := &Concat{}
	toUpper := &ToUpper{}

	// new a workflow and add job and dependencies into it
	w := NewWorkflow().Add(
		DirectDependsOn(toUpper, concat),
	)

	// set input
	concat.JobInput = []string{
		"hello", ",", "world",
	}

	werr := w.Run(context.Background())
	fmt.Println(werr)
	fmt.Println(toUpper.Out)
	// Output:
	// <nil>
	// HELLO,WORLD
}

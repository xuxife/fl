package fl

import (
	"context"
	"fmt"
	"strings"
)

func ExampleConnect() {
	splitByComma := Func("splitByComma", func(ctx context.Context, s string) ([]string, error) {
		return strings.Split(s, ","), nil
	})
	joinByDot := Func("jobByDot", func(ctx context.Context, ss []string) (string, error) {
		return strings.Join(ss, "."), nil
	})
	replaceCommaWithDot := Connect("replaceCommaWithDot", splitByComma, joinByDot)
	SetInput(replaceCommaWithDot, "hello,world")
	fmt.Println(replaceCommaWithDot.Do(context.Background()))
	fmt.Println(GetOutput(replaceCommaWithDot))
	// Output:
	// <nil>
	// hello.world
}

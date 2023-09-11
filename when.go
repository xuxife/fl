package pl

type WhenFunc func(*Workflow) bool

var DefaultWhenFunc = WhenFunc(func(*Workflow) bool {
	return true
})

func Skip(*Workflow) bool {
	return false
}

package pl

type WhenFunc func() bool

var DefaultWhenFunc = WhenFunc(func() bool {
	return true
})

func Skip() bool {
	return false
}

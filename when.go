package pl

type WhenFunc func(dep Dependency) bool

var DefaultWhenFunc = WhenFunc(func(dep Dependency) bool {
	return true
})

func Skip(dep Dependency) bool {
	return false
}

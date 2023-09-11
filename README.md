# pl - pipeline running in a single Go process

`pl` provides a simple implement of pipeline in a single Go process, like GitHub Actions, Azure DevOps Pipeline, etc.

`pl` supports a minimum set of features:
- [x] Generic job
- [x] Connect jobs with dependency relationship
- [x] Data flow between connected jobs
- [x] Condition based on dependency status `[ always | succeeded | failed | succeedOrFailed | never ]`
- [x] Job retry

# Usage

`pl` is implemented with the mind of simplicity and customizability. You can define your own job with strong typed input and output, then connect these jobs into an executable workflow.

## Define your job

```go
type MyJob struct {
    pl.Base // must embed pl.Base, it records your job's status and condition function
    MyJobInput // define your input
    // other intermidate fields
}

type MyJobInput struct {
    // define your input here
}

type MyJobOutput struct {
    // define your output here
}

// implement your job to satisfy pl.Jober interface
//
//  type Jober[I, O any] interface {
//  	Inputer[I]
//  	Outputer[O]
//  	Doer
//  	fmt.Stringer
//      ...
//  }

// String defines what to display of this Job in Workflow
func (j *MyJob) String() string {
    return "MyJob"
}

// Input returns a pointer to input type, which will be filled by preceding jobs or user
func (j *MyJob) Input() *MyJobInput {
    return &j.MyJobInput
}

// Output accepts a pointer to output type, please fill what need to be outputted to it
func (j *MyJob) Output(o *MyJobOutput) {
    // fill o here
}

// Do is your job's main logic
func (j *MyJob) Do(ctx context.Context) error {
    // do your job here
}
```

### Toolkit to help you define your job

#### `BaseIn[I]`

`BaseIn[I]` is a helper generic struct to define your job's input and implements `Input() *I` methods.

```go
type MyJob struct {
    pl.BaseIn[MyJobInput]
    // other intermidate fields
}

// now you can skip implementation of 
//  Input() *MyJobInput
```

#### `Func`

`Func` is what to use when your job can be implemented in a single function.

```go
func Func[I, O any](name string, do func(context.Context, I) (func(*O), error)) Job[I, O]

// example
var myJob = Func("MyJobName", func(ctx context.Context, input MyJobInput) (func(*MyJobOutput), error) {
    // do your job here with input
    return func(output *MyJobOutput) {
        // fill output here
    }, nil
})
```

## Connect your jobs into Workflow

`pl` uses a mental model of this pattern:
```go
w.Add(
    pl.Job(A).
        Input(func(i *InputA) { /* fill A's input here */ }).
        DependsOn(B, /* adapter function */).  // if A Depends On B, then
        DirectDependsOn(C, D).  // multiple dependencies
        Retry(pl.RetryOption{ /* retry options */ }).
        Timeout(10*time.Minute). // set timeout
        Condition(pl.SucceededOrFailed). // set condition
        When(pl.Skip), // set when to skip this job
    pl.Jobs(C, D).
        DependsOn(E, F),
)
```

Check [example_test.go](./example_test.go) for a real world example.

## Condition and When

Job's condition is a function to determine whether this job should be executed or cancel, based on the status of its dependencies.

Following conditions are available:
- `Always`: job will always be executed, even its dependencies failed or canceled
- `Succeeded`: job will be executed only if all its dependencies succeeded, be canceled if any of them failed or canceled.
- `Failed`: job will be executed only if any of its dependencies failed, be canceled if all of them succeeded or any of them canceled
- `SucceededOrFailed`: job will be executed only if all its dependencies succeeded or failed, be canceled if any of them canceled
- `Never`: job will never be executed

Job's when is a function to determine whether this job should be executed or skipped.

### Skip vs Cancel

- Skip: an expected status that the job should not executed.
- Cancel: an unexpected status that the job should not executed.

Their relations are defined as below:

<img src="https://github.com/xuxife/pl/assets/28257575/bc398419-471e-49ba-854b-8b039d33e467" width=400>

## Run your workflow

```go
// run your workflow,
// blocks the current goroutine until all job terminated.
err := w.Run(context.Background())

// handle error
if err == nil {
    // workflow finished successfully
}
switch werr := err.(type) {
    case pl.ErrWorkflow:
        // workflow failed
        for job, jobErr := range werr {
            if jobErr == nil {
                // job finished successfully
            } else {
                // job failed
                job.String() // get failed job's name
                job.GetStatus() // get failed job's status
            }
        }
    case pl.ErrWorkflowIsRunning:
        // workflow is running, you can't run it again
    case pl.ErrWorkflowHasRun:
        // workflow has run, all jobs are terminated,
        // you can't run it again unless you reset it by `w.Reset()`
    case pl.ErrCycleDependency:
        // workflow has cycle dependency, can't run
        // 
        //  type ErrCycleDependency map[Reporter][]Reporter
        for job, jobDependency := range werr {
            job.String() // get job's name
            for _, dependency := range jobDependency {
                dependency.String() // get job's dependency's name
            }
        }
}
```

## Get Output

```go
var o COut
jobC.Output(&o)
// or
o = pl.GetOutput(jobC)
```
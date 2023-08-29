# pl - pipeline running in a single Go process

`pl` provides a simple implement of pipeline in a single Go process, like Gitlab Actions, Azure DevOps Pipeline, etc.

`pl` supports a minimum set of features:
- [x] Generic Job
- [x] Connect Jobs with Dependence
- [x] Data Flow between Connected Jobs
- [x] Condition based On Dependency `[ always | succeeded | failed | succeedOrFailed | never ]`
- [ ] Job Retry

# Usage

`pl` is implemented with the mind of simplicity and customizability. You can define your own job with strong typed input and output, then connect these jobs into a executable workflow.

## Define your job

```go
type MyJob struct {
    pl.Base // must embed pl.Base, it records your job's status and condition function
    MyJobInput // define your input
    MyJobOutput // define your output
    // other intermidate fields
}

type MyJobInput struct {
    // define your input here
}

type MyJobOutput struct {
    // define your output here
}

// implement your job to satisfy pl.Job interface
//
//  type Job[I, O any] interface {
//  	// methods to be implemented
//  	Inputer[I]
//  	Outputer[O]
//  	Doer
//  	fmt.Stringer // please give this job a name
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
    *o = j.MyJobOutput
}

// Do is your job's main logic
func (j *MyJob) Do(ctx context.Context) error {
    // do your job here
}
```

### Toolkit to help you define your job

#### `InOut[I, O]`

`InOut[I, O]` is a helper generic struct to define your job's input and output and related methods.

```go
type MyJob struct {
    pl.Base
    pl.InOut[MyJobInput, MyJobOutput]
    // other intermidate fields
}

// now you can skip implementation of 
//  Input() *MyJobInput
//  Output(o *MyJobOutput
```

#### `Func`

`Func` is what to use when your job can be implemented in a single function.

```go
func Func[I, O any](name string, do func(context.Context, I) (O, error)) Job[I, O]

// example
var myJob = Func("MyJobName", func(ctx context.Context, input MyJobInput) (MyJobOutput, error) {
    // do your job here
})
```

## Connect your jobs into Workflow

```go
var w = pl.NewWorkflow()

// assume you have defined your jobs
var (
    jobA Job[AIn, AOut] = nil
    jobB Job[BIn, BOut] = nil
    jobC Job[BOut, COut] = nil

    jobD = nil
    jobE = nil
)

// connect your jobs by `DependsOn`, `DirectDependsOn`, ...
w.Add(
    // jobB depends on jobA,
    // means jobA must be executed before jobB,
    // and jobA's output will be flowed into jobB's input
    // via an adapter function you define.
    DependsOn(jobB, jobA).WithAdapter(func(a AOut, b *BIn) {
        // define your adapter function here
    }),
    // jobC depends on jobB,
    // and jobC's input is exactly jobB's output.
    DirectDependsOn(jobC, jobB),
    // use `Parallel` or `NoDependency` to add jobs without dependency.
    Parallel(jobD, jobE),
)

w.Add(
    // Add is idempotent, you can add jobs to workflow multiple times.
    Parallel(jobD, jobE),
    // `DependsOn` with adapter function and `DirectDependsOn` will be executed in order, FIFO.
    DependsOn(jobB, jobA).WithAdapter(func(a AOut, b *BIn) {
        // here `b` already have been filled by adapter function of previous `DependsOn`
    }),
)
```

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
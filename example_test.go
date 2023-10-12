package pl_test

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/xuxife/pl"
)

func ExampleWorkflow() {
	suite := new(pl.Workflow)

	{
		// create jobs
		createResourceGroup := new(CreateResourceGroup)
		createAKSCluster := new(CreateAKSCluster)
		getKubeConfig := new(GetAKSClusterCredential)

		// connect steps into Workflow
		suite.Add(
			pl.Step(createResourceGroup).
				// use Input to set the Input of a Job.
				Input(func(ctx context.Context, i *CreateResourceGroupInput) error {
					i.Name = "rg"
					i.Region = "eastus"
					i.SubscriptionID = "sub"
					return nil
				}).
				Retry(pl.RetryOption{
					Attempts: 10,
					Backoff:  backoff.NewExponentialBackOff(),
					Timer:    new(testTimer),
				}).
				Condition(pl.Always),

			pl.Step(createAKSCluster).
				Input(func(_ context.Context, i *CreateAKSClusterInput) error {
					i.Name = "aks-cluster"
					return nil
				}).
				// use DependsOn to connects two Jobs with an adapter function.
				DependsOn(
					pl.Adapt(createResourceGroup, func(_ context.Context, o CreateResourceGroupOutput, i *CreateAKSClusterInput) error {
						i.ResourceGroupName = o.Name
						i.SubscriptionID = o.SubscriptionID
						return nil
					}),
				),

			pl.Step(getKubeConfig).
				DependsOn(
					pl.Adapt(createAKSCluster, func(_ context.Context, o CreateAKSClusterOutput, i *GetKubeConfigInput) error {
						i.ClusterName = path.Join(o.SubscriptionID, o.Region, o.ResourceGroupName, o.Name)
						i.Type = "Admin"
						return nil
					}),
				),
		)

		// use Func to create a Job from a function.
		preCheck := pl.Func("precheck", func(ctx context.Context, in string) (func(*string), error) {
			return func(s *string) {
				// set the Output in this callback function
				*s = "hello world " + in
			}, nil
		})
		preWorkflow := new(pl.Workflow).Add(
			pl.Step(
				pl.FuncIn("helloworld", func(_ context.Context, in string) error {
					fmt.Println(in)
					return nil
				}),
			).DirectDependsOn(preCheck),
		)
		// use Stage to wrap a Workflow into a Job.
		preStage := &pl.Stage[PreCheckInput, PreCheckOutput]{
			Name:     "PreStage",
			Workflow: preWorkflow,
			SetInput: func(pci PreCheckInput) {
				// PreCheckInput is already be filled,
				// use the input to fill the Input of Jobs inside your Workflow.
				*preCheck.Input() = pci.BuildID
			},
			SetOutput: func(pco *PreCheckOutput) {
				// fill values into the Output of the Stage
				pco.Message = pl.GetOutput(preCheck)
			},
		}

		// Stage can be used as a Job in a Workflow
		suite.Add(
			pl.Step(preStage).
				Input(func(_ context.Context, in *PreCheckInput) error {
					in.BuildID = "321"
					return nil
				}),
			pl.Step(createResourceGroup).
				ExtraDependsOn(preStage),
		)

		// still able to modify the Workflow if still hold reference to jobs.
		passRegion := pl.Func("forget to pass region", func(_ context.Context, o CreateResourceGroupOutput) (func(i *CreateAKSClusterInput), error) {
			return func(i *CreateAKSClusterInput) {
				i.Region = o.Region
			}, nil
		})
		suite.Add(
			// use DirectDependsOn to connect two jobs with matched Input and Output
			pl.Step(createAKSCluster).DirectDependsOn(passRegion),
			pl.Step(passRegion).DirectDependsOn(createResourceGroup),
		)
	}

	var getKubeConfig *GetAKSClusterCredential
	// if already lose reference to the original jobs,
	// use w.Dep() to get the Dependency and jobs inside.
	for step := range suite.Dep() {
		switch typedStep := step.(type) {
		case *CreateAKSCluster:
			// still able to inject jobs between createAKSCluster and its Dependers.
			createAKSCluster := typedStep
			patchCVE := pl.FuncIn("PatchCVE", func(ctx context.Context, o CreateAKSClusterOutput) error {
				// update the aks cluster with CVE patch
				fmt.Println("patched!")
				return nil
			})
			suite.Add(
				// in this case, dependers of createAKSCluster will wait for patchCVE to finish.
				pl.Steps(suite.Dep().DownstreamOf(createAKSCluster)...).DependsOn(patchCVE),
				pl.Step(patchCVE).DirectDependsOn(createAKSCluster),
			)
		case *GetAKSClusterCredential:
			getKubeConfig = typedStep
			// use Input() to add a dependency that modifies the Input of a Job.
			suite.Add(
				pl.Step(getKubeConfig).
					Input(func(_ context.Context, i *GetKubeConfigInput) error {
						i.Type = "User"
						return nil
					}),
			)
		}
	}

	// use Run(context.Context) to kick off the Workflow,
	// it will block the current goroutine.
	err := suite.Run(context.Background())

	// the err returned from Run() is nil-able,
	// if the Workflow succeeded without error.
	fmt.Println(err)

	// use IsTerminated() to check the Workflow status,
	// it may only be helpful when the Workflow is running in another goroutine.
	fmt.Println(suite.IsTerminated())

	// use Err() to get the ErrWorkflow, do not compare it with nil, since it's always non-nil.
	// use IsNil() to check whether the ErrWorkflow is empty.
	suiteErr := suite.Err()
	fmt.Println(suiteErr.IsNil())

	// get the output from the original jobs.
	fmt.Println(pl.GetOutput(getKubeConfig))

	// Output:
	// hello world 321
	// patched!
	// <nil>
	// true
	// true
	// User kubeconfig for sub/eastus/rg/aks-cluster
}

type CreateResourceGroup struct {
	pl.StepBaseIn[CreateResourceGroupInput]
}

var count = 0

func (c *CreateResourceGroup) Do(ctx context.Context) error {
	// call azure api to create resource group
	count++
	if count < 5 {
		return fmt.Errorf("retry")
	}
	return nil
}

func (c *CreateResourceGroup) String() string {
	return fmt.Sprintf("CreateResourceGroup(/subscriptions/%s/resourceGroup/%s,%s)", c.In.SubscriptionID, c.In.Name, c.In.Region)
}

func (c *CreateResourceGroup) Output(o *CreateResourceGroupOutput) {
	o.Name = c.In.Name
	o.Region = c.In.Region
	o.SubscriptionID = c.In.SubscriptionID
}

type CreateResourceGroupInput struct {
	SubscriptionID string
	Region         string
	Name           string
	EnableCache    bool
}

type CreateResourceGroupOutput struct {
	SubscriptionID string
	Region         string
	Name           string
}

type CreateAKSCluster struct {
	pl.StepBaseIn[CreateAKSClusterInput]
}

func (c *CreateAKSCluster) Do(ctx context.Context) error {
	// call azure api to create aks cluster
	return nil
}

func (c *CreateAKSCluster) String() string {
	return "CreateAKSCluster"
}

func (c *CreateAKSCluster) Output(o *CreateAKSClusterOutput) {
	o.Name = c.In.Name
	o.Region = c.In.Region
	o.SubscriptionID = c.In.SubscriptionID
	o.ResourceGroupName = c.In.ResourceGroupName
}

type CreateAKSClusterInput struct {
	SubscriptionID    string
	ResourceGroupName string
	Name              string
	Region            string
	// Spec              armcontainerservice.ManagedCluster
}

type CreateAKSClusterOutput struct {
	SubscriptionID    string
	ResourceGroupName string
	Name              string
	Region            string
	// Spec              armcontainerservice.ManagedCluster
}

type GetAKSClusterCredential struct {
	pl.StepBaseIn[GetKubeConfigInput] // output is kubeconfig
}

func (c *GetAKSClusterCredential) Do(ctx context.Context) error {
	// call azure api to get kubeconfig
	return nil
}

func (c *GetAKSClusterCredential) Output(o *string) {
	*o = fmt.Sprintf("%s kubeconfig for %s", c.In.Type, c.In.ClusterName)
}

func (c *GetAKSClusterCredential) String() string {
	return "GetAKSClusterAdminCredential"
}

type GetKubeConfigInput struct {
	ClusterName string
	Type        string // Admin | User | Monitor
}

type testTimer struct {
	timer *time.Timer
}

func (t *testTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *testTimer) Start(duration time.Duration) {
	if t.timer == nil {
		t.timer = time.NewTimer(0)
	} else {
		t.timer.Reset(0)
	}
}

func (t *testTimer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
}

type PreCheckInput struct {
	BuildID string
}

type PreCheckOutput struct {
	Message string
}

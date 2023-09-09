package pl_test

import (
	"context"
	"fmt"
	"path"

	"github.com/xuxife/pl"
)

func ExampleWorkflow() {
	var w *pl.Workflow

	{
		// create jobs
		createResourceGroup := new(CreateResourceGroup)
		createAKSCluster := new(CreateAKSCluster)
		getKubeConfig := new(GetAKSClusterCredential)

		// set input
		createResourceGroup.Input().Name = "rg"
		createAKSCluster.Input().Name = "aks-cluster"

		w = pl.NewWorkflow(
			// or use Input to modify the Input of a Job.
			pl.DirectDependsOn(createResourceGroup, pl.Input("", func(i *CreateResourceGroupInput) {
				i.Region = "eastus"
				i.SubscriptionID = "sub"
			})),
			// use DependsOn to connects two Jobs with an adapter function.
			pl.DependsOn(createAKSCluster, createResourceGroup).
				WithAdapter(func(o CreateResourceGroupOutput, i *CreateAKSClusterInput) {
					i.ResourceGroupName = o.Name
					i.SubscriptionID = o.SubscriptionID
				}),
			pl.DependsOn(getKubeConfig, createAKSCluster).
				WithAdapter(func(o CreateAKSClusterOutput, i *GetKubeConfigInput) {
					i.ClusterName = path.Join(o.SubscriptionID, o.Region, o.ResourceGroupName, o.Name)
					i.Type = "Admin"
				}),
			// use NoDependency or Parallel to declare Job(s) without any dependency
			pl.NoDependency(
				// use Func to create a Job from a function.
				pl.Func("I'm alone", func(ctx context.Context, in struct{}) (func(*string), error) {
					// use struct{} to present no Input/Output
					return func(s *string) {
						// set the Output in this callback function
						*s = "hello world"
					}, nil
				}),
			),
		)

		// still able to modify the Workflow if still hold reference to jobs.
		passRegion := pl.Adapter("forget to pass region", func(o CreateResourceGroupOutput) func(i *CreateAKSClusterInput) {
			return func(i *CreateAKSClusterInput) {
				i.Region = o.Region
			}
		})
		w.Add(
			pl.DirectDependsOn(passRegion, createResourceGroup),
			pl.DirectDependsOn(createAKSCluster, passRegion),
		)
	}

	var getKubeConfig *GetAKSClusterCredential
	// if already lose reference to the original jobs,
	// use w.Dep() to get the Dependency and jobs inside.
	for job := range w.Dep() {
		switch typedJob := job.(type) {
		case *CreateAKSCluster:
			// still able to inject jobs between createAKSCluster and its Dependers.
			createAKSCluster := typedJob
			patchCVE := pl.Consumer("PatchCVE", func(ctx context.Context, o CreateAKSClusterOutput) error {
				// update the aks cluster with CVE patch
				fmt.Println("patched!")
				return nil
			})
			for _, depender := range w.Dep().ListDependerOf(createAKSCluster) {
				w.Add(
					// use NoFlowDependsOn if a data flow is not necessary,
					// in this case, dependers of createAKSCluster will wait for patchCVE to finish.
					pl.NoFlowDependsOn(depender, patchCVE),
				)
			}
			// add the patchCVE depends on createAKSCluster AFTER the above loop,
			// otherwise, the above loop will also add the patchCVE depends on itself.
			w.Add(pl.DirectDependsOn(patchCVE, createAKSCluster))
		case *GetAKSClusterCredential:
			getKubeConfig = typedJob
			// use Input to modify the Input of a Job.
			w.Add(pl.DirectDependsOn(
				getKubeConfig,
				pl.Input("get User kubeconfig instead", func(i *GetKubeConfigInput) {
					i.Type = "User"
				}),
			))
		}
	}

	// use Run(context.Context) to kick off the workflow,
	// it will block the current goroutine.
	err := w.Run(context.Background())

	// the err returned from Run() is nil-able,
	// if the workflow succeeded without error.
	fmt.Println(err)

	// use IsTerminated() to check the workflow status,
	// it may only be helpful when the workflow is running in another goroutine.
	fmt.Println(w.IsTerminated())

	// use Err() to get the ErrWorkflow, do not compare it with nil, since it's always non-nil.
	// use IsNil() to check whether the ErrWorkflow is empty.
	werr := w.Err()
	fmt.Println(werr.IsNil())

	// get the output from the original jobs.
	fmt.Println(pl.GetOutput(getKubeConfig))

	// Output:
	// patched!
	// <nil>
	// true
	// true
	// User kubeconfig for sub/eastus/rg/aks-cluster
}

type CreateResourceGroup struct {
	pl.BaseIn[CreateResourceGroupInput]
}

func (c *CreateResourceGroup) Do(ctx context.Context) error {
	// call azure api to create resource group
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
	pl.BaseIn[CreateAKSClusterInput]
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
	pl.BaseIn[GetKubeConfigInput] // output is kubeconfig
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

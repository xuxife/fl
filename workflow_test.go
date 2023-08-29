package pl

import "context"

type CreateResourceGroup struct {
	Base
	InOut[CreateResourceGroupInput, CreateResourceGroupOutput]
}

func (c *CreateResourceGroup) Do(ctx context.Context) error {
	// do something
	return nil
}

func (c *CreateResourceGroup) String() string {
	return "CreateResourceGroup"
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
	Base
	InOut[CreateAKSClusterInput, CreateAKSClusterOutput]
}

func (c *CreateAKSCluster) Do(ctx context.Context) error {
	// do something
	return nil
}

func (c *CreateAKSCluster) String() string {
	return "CreateAKSCluster"
}

type CreateAKSClusterInput struct {
	SubscriptionID    string
	ResourceGroupName string
	Name              string
	// Spec              armcontainerservice.ManagedCluster
}

type CreateAKSClusterOutput struct {
	SubscriptionID    string
	ResourceGroupName string
	Name              string
	// Spec              armcontainerservice.ManagedCluster
}

func ExampleWorkflow() {
	createRG := new(CreateResourceGroup)
	createAKS := new(CreateAKSCluster)

	NewWorkflow(
		DirectDependsOn(createRG, Input(CreateResourceGroupInput{})),
		DependsOn(createAKS, createRG).
			WithAdapter(func(o CreateResourceGroupOutput, i *CreateAKSClusterInput) {
				// pass fields
			}),
	)
}

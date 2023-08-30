package pl

import (
	"context"
	"fmt"
)

type CreateResourceGroup struct {
	Base
	InOut[CreateResourceGroupInput, CreateResourceGroupOutput]
}

func (c *CreateResourceGroup) Do(ctx context.Context) error {
	// do something
	c.Out.Name = c.In.Name
	c.Out.SubscriptionID = c.In.SubscriptionID
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
	c.Out.ResourceGroupName = c.In.ResourceGroupName
	c.Out.SubscriptionID = c.In.SubscriptionID
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
	A := new(CreateResourceGroup)
	B := new(CreateResourceGroup)
	C := new(CreateAKSCluster)

	// A.Input().Name = "rg1"
	B.Input().SubscriptionID = "rg2"
	err := NewWorkflow(
		DirectDependsOn(A, Input(CreateResourceGroupInput{
			Name: "rg1",
		})),
		DependsOn(C, A).
			WithAdapter(func(o CreateResourceGroupOutput, i *CreateAKSClusterInput) {
				i.ResourceGroupName = o.Name
			}),
		DependsOn(C, B).
			WithAdapter(func(o CreateResourceGroupOutput, i *CreateAKSClusterInput) {
				i.SubscriptionID = o.SubscriptionID
			}),
	).Run(context.Background())

	fmt.Println(err)
	fmt.Println(C.GetOutput().ResourceGroupName)
	fmt.Println(C.GetOutput().SubscriptionID)
	// Output:
	// <nil>
	// rg1
	// rg2
}

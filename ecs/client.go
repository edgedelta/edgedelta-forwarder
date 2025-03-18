package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go/aws"
)

type Client interface {
	GetTaskDetails(ctx context.Context, clusterName, taskID string) (*ecs.DescribeTasksOutput, error)
}

type DefaultClient struct {
	svc *ecs.Client
}

func NewClient() (*DefaultClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config, err: %v", err)
	}

	svc := ecs.NewFromConfig(cfg)
	return &DefaultClient{svc: svc}, nil
}

func (c *DefaultClient) GetTaskDetails(ctx context.Context, clusterName, taskID string) (*ecs.DescribeTasksOutput, error) {
	input := &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   []string{taskID},
	}

	return c.svc.DescribeTasks(ctx, input)
}

type NoOpClient struct{}

func NewNoOpClient() *NoOpClient {
	return &NoOpClient{}
}

func (c *NoOpClient) GetTaskDetails(ctx context.Context, clusterName, taskID string) (*ecs.DescribeTasksOutput, error) {
	return nil, nil
}

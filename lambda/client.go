package lambda

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
)

type Client interface {
	GetFunction(functionARN string) (*lambda.GetFunctionOutput, error)
}

type DefaultClient struct {
	svc *lambda.Lambda
}

func NewClient(region string) (*DefaultClient, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS Lambda client, err: %v", err)
	}
	return &DefaultClient{svc: lambda.New(sess, &aws.Config{Region: aws.String(region)})}, nil
}

func (c *DefaultClient) GetFunction(functionARN string) (*lambda.GetFunctionOutput, error) {
	result, err := c.svc.GetFunction(&lambda.GetFunctionInput{
		FunctionName: aws.String(functionARN),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type NoOpClient struct{}

func NewNoOpClient() *NoOpClient {
	return &NoOpClient{}
}

func (c *NoOpClient) GetFunction(functionARN string) (*lambda.GetFunctionOutput, error) {
	return nil, nil
}

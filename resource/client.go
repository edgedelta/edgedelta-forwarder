package resource

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
)

type Client interface {
	GetResourceTags(ctx context.Context, resourceARNs ...string) (map[string]map[string]string, error)
}

type DefaultClient struct {
	svc *resourcegroupstaggingapi.Client
}

func NewAWSClient() (Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config, err: %v", err)
	}

	svc := resourcegroupstaggingapi.NewFromConfig(cfg)
	return &DefaultClient{svc: svc}, nil

}

func (c *DefaultClient) GetResourceTags(ctx context.Context, resourceARNs ...string) (map[string]map[string]string, error) {
	input := &resourcegroupstaggingapi.GetResourcesInput{
		ResourceARNList: resourceARNs,
	}
	output, err := c.svc.GetResources(ctx, input)
	if err != nil {
		return nil, err
	}
	res := make(map[string]map[string]string)
	for _, m := range output.ResourceTagMappingList {
		tags := make(map[string]string)
		for _, t := range m.Tags {
			k := aws.ToString(t.Key)
			v := aws.ToString(t.Value)
			tags[k] = v
		}
		arn := aws.ToString(m.ResourceARN)
		res[arn] = tags
	}
	return res, nil
}

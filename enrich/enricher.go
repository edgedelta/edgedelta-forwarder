package enrich

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/parser"
	"github.com/edgedelta/edgedelta-forwarder/resource"
)

var resourceTagsCache = make(map[string]map[string]string)

type faas struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cloud struct {
	ResourceID string `json:"resource_id"`
}

type Common struct {
	Cloud      *cloud            `json:"cloud"`
	Faas       *faas             `json:"faas"`
	LambdaTags map[string]string `json:"lambda_tags,omitempty"`
}

type Enricher struct {
	resourceCl           *resource.DefaultClient
	region               string
	forwardForwarderTags bool
	forwardSourceTags    bool
	forwardLogGroupTags  bool
}

func NewEnricher(conf *cfg.Config, resourceCl *resource.DefaultClient) *Enricher {
	return &Enricher{
		forwardForwarderTags: conf.ForwardForwarderTags,
		forwardSourceTags:    conf.ForwardSourceTags,
		forwardLogGroupTags:  conf.ForwardLogGroupTags,
		region:               conf.Region,
		resourceCl:           resourceCl,
	}
}

func (e *Enricher) GetEDCommon(ctx context.Context, logGroup, accountID string) *Common {
	var forwarderARN string
	lc, ok := lambdacontext.FromContext(ctx)
	if !ok {
		log.Printf("Failed to create lambda context")
	} else {
		forwarderARN = lc.InvokedFunctionArn
	}

	var arnsToGetTags []string
	if forwarderARN != "" && e.forwardForwarderTags {
		arnsToGetTags = append(arnsToGetTags, forwarderARN)
	}

	if e.forwardLogGroupTags {
		arn := parser.BuildServiceARN("logs", accountID, e.region, fmt.Sprintf("log-group:%s:*", logGroup))
		arnsToGetTags = append(arnsToGetTags, arn)
	}

	functionName := lambdacontext.FunctionName
	functionVersion := lambdacontext.FunctionVersion

	if e.forwardSourceTags {
		if arns, ok := parser.GetSourceARNsFromLogGroup(accountID, e.region, logGroup); ok {
			arnsToGetTags = append(arnsToGetTags, arns...)
		} else {
			log.Printf("Failed to get source ARNs from log group: %s", logGroup)
		}
	}

	return &Common{
		Cloud: &cloud{ResourceID: forwarderARN},
		Faas: &faas{
			Name:    functionName,
			Version: functionVersion,
		},
		LambdaTags: e.getResourceTags(ctx, arnsToGetTags),
	}
}

func (e *Enricher) getResourceTags(ctx context.Context, arns []string) map[string]string {
	tagsCacheKey := getTagsCacheKey(arns...)
	if tags, ok := resourceTagsCache[tagsCacheKey]; ok {
		return tags
	}
	tagsMap, err := e.resourceCl.GetResourceTags(ctx, arns...)
	if err != nil {
		log.Printf("Failed to get resource tags for ARNs: %v, err: %v", arns, err)
		return nil
	}
	if len(tagsMap) == 0 {
		log.Printf("Failed to find tags for ARNs: %v", arns)
		return nil
	}

	resourceTagsCache[tagsCacheKey] = map[string]string{}
	for r, t := range tagsMap {
		log.Printf("Found tags: %v for ARN: %s", t, r)
		for k, v := range t {
			resourceTagsCache[tagsCacheKey][k] = v
		}
	}

	return resourceTagsCache[tagsCacheKey]
}

func getTagsCacheKey(arns ...string) string {
	return strings.Join(arns, ",")
}

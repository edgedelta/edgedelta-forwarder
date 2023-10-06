package enrich

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/lambda"
	"github.com/edgedelta/edgedelta-forwarder/parser"
	"github.com/edgedelta/edgedelta-forwarder/resource"
	"github.com/edgedelta/edgedelta-forwarder/utils"
)

var resourceTagsCache = make(map[string]map[string]string)

type faas struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	RequestID string `json:"request_id,omitempty"`
}

type cloud struct {
	ResourceID string `json:"resource_id"`
	AccountID  string `json:"account_id"`
	Region     string `json:"region"`
}

type Common struct {
	Cloud      *cloud            `json:"cloud"`
	Faas       *faas             `json:"faas"`
	LambdaTags map[string]string `json:"lambda_tags,omitempty"`
}

type Enricher struct {
	resourceCl           *resource.DefaultClient
	lambdaCl             *lambda.DefaultClient
	region               string
	forwardForwarderTags bool
	forwardSourceTags    bool
	forwardLogGroupTags  bool
}

func NewEnricher(conf *cfg.Config, resourceCl *resource.DefaultClient, lambdaCl *lambda.DefaultClient) *Enricher {
	return &Enricher{
		forwardForwarderTags: conf.ForwardForwarderTags,
		forwardSourceTags:    conf.ForwardSourceTags,
		forwardLogGroupTags:  conf.ForwardLogGroupTags,
		region:               conf.Region,
		resourceCl:           resourceCl,
		lambdaCl:             lambdaCl,
	}
}

func (e *Enricher) GetEDCommon(ctx context.Context, logGroup, logStream, accountID string) *Common {
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

	logGroupARN := parser.BuildServiceARN("logs", accountID, e.region, fmt.Sprintf("log-group:%s:*", logGroup))
	if e.forwardLogGroupTags {
		arnsToGetTags = append(arnsToGetTags, logGroupARN)
	}

	if e.forwardSourceTags {
		if arns, ok := parser.GetSourceARNsFromLogGroup(accountID, e.region, logGroup, logStream); ok {
			arnsToGetTags = append(arnsToGetTags, arns...)
		} else {
			log.Printf("Failed to get source ARNs from log group: %s", logGroup)
		}
	}

	tags := e.getResourceTags(ctx, arnsToGetTags)
	if tags == nil {
		tags = make(map[string]string)
	}

	functionARN := forwarderARN
	functionName := lambdacontext.FunctionName
	functionVersion := lambdacontext.FunctionVersion
	// Assign function arn name if source is lambda
	if arn, name, ok := parser.GetFunctionARNAndNameIfSourceIsLambda(logGroup, accountID, e.region); ok {
		functionARN = arn
		functionName = name
		functionVersion = ""
	}

	if functionARN != "" {
		config, err := e.lambdaCl.GetFunctionConfiguration(functionARN)
		if err != nil {
			log.Printf("Failed to get function configuration for ARN: %s, err: %v", functionARN, err)
		} else {
			tags["process.runtime.name"] = *config.Runtime
			tags["faas.memory_size"] = fmt.Sprintf("%d", *config.MemorySize)
			tags["host.arch"] = getRuntimeArchitecture(functionARN, forwarderARN, config.Architectures)
		}
	}

	return &Common{
		Cloud: &cloud{ResourceID: getResourceID(arnsToGetTags, forwarderARN, logGroupARN), AccountID: accountID, Region: e.region},
		Faas: &faas{
			Name:      functionName,
			Version:   functionVersion,
			RequestID: lc.AwsRequestID,
		},
		LambdaTags: tags,
	}
}

func (e *Enricher) getResourceTags(ctx context.Context, arns []string) map[string]string {
	tagsCacheKey := getTagsCacheKey(arns...)
	if tags, ok := resourceTagsCache[tagsCacheKey]; ok {
		return tags
	}
	log.Printf("Getting resource tags for ARNs: %v", arns)
	tagsMap, err := e.resourceCl.GetResourceTags(context.Background(), arns...)
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
		resourceTagsCache[r] = t
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

func getRuntimeArchitecture(functionARN, forwarderARN string, archs []*string) string {
	if len(archs) == 0 || functionARN == forwarderARN {
		return utils.GetRuntimeArchitecture()
	}
	var architectures []string
	for _, a := range archs {
		architectures = append(architectures, *a)
	}
	return strings.Join(architectures, ",")
}

func getResourceID(arns []string, forwarderARN, logGroupARN string) string {
	// If any tag exists for an ARN other than forwarder or log group, return that ARN as resource ID
	for _, arn := range arns {
		if arn != forwarderARN && arn != logGroupARN {
			if m, ok := resourceTagsCache[arn]; ok {
				if len(m) > 0 {
					return arn
				}
			}
		}
	}
	return forwarderARN
}

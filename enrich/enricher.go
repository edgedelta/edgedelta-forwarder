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

var resourceARNToTagsCache = make(map[string]map[string]string)

type faas struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	RequestID  string            `json:"request_id,omitempty"`
	MemorySize string            `json:"memory_size,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type cloud struct {
	ResourceID string `json:"resource_id"`
	AccountID  string `json:"account_id"`
	Region     string `json:"region"`
}

type awsCommon struct {
	LogGroup     string            `json:"log.group.name"`
	LogGroupARN  string            `json:"log.group.arn"`
	LogGroupTags map[string]string `json:"log.group.tags,omitempty"`
	LogStream    string            `json:"log.stream.name"`
	ServiceTags  map[string]string `json:"service.tags,omitempty"`
}

type Common struct {
	Cloud              *cloud     `json:"cloud"`
	Faas               *faas      `json:"faas"`
	AwsCommon          *awsCommon `json:"aws"`
	HostArchitecture   string     `json:"host.arch,omitempty"`
	ProcessRuntimeName string     `json:"process.runtime.name,omitempty"`
}

type Enricher struct {
	resourceCl           resource.Client
	lambdaCl             lambda.Client
	region               string
	forwardForwarderTags bool
	forwardSourceTags    bool
	forwardLogGroupTags  bool
}

func NewEnricher(conf *cfg.Config, resourceCl resource.Client, lambdaCl lambda.Client) *Enricher {
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

	logGroupARN := parser.BuildServiceARN("logs", accountID, e.region, fmt.Sprintf("log-group:%s", logGroup))
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

	isSourceLambda := false
	functionARN := forwarderARN
	functionName := lambdacontext.FunctionName
	functionVersion := lambdacontext.FunctionVersion
	// Assign function arn name if source is lambda
	if arn, name, ok := parser.GetFunctionARNAndNameIfSourceIsLambda(logGroup, accountID, e.region); ok {
		functionARN = arn
		functionName = name
		functionVersion = ""
		isSourceLambda = true
	}

	var memorySize string
	var hostArchitecture string
	var processRuntimeName string
	if functionARN != "" {
		function, err := e.lambdaCl.GetFunction(functionARN)
		if err != nil {
			log.Printf("Failed to get function for ARN: %s, err: %v", functionARN, err)
		} else {
			functionVersion = *function.Configuration.Version
			processRuntimeName = *function.Configuration.Runtime
			hostArchitecture = getRuntimeArchitecture(functionARN, forwarderARN, function.Configuration.Architectures)
			memorySize = fmt.Sprintf("%d", *function.Configuration.MemorySize)
		}
	}

	sourceTags, faasTags, logGroupTags := e.getAllTags(ctx, forwarderARN, logGroupARN, arnsToGetTags, isSourceLambda)

	return &Common{
		Cloud: &cloud{ResourceID: getResourceID(arnsToGetTags, forwarderARN, logGroupARN), AccountID: accountID, Region: e.region},
		Faas: &faas{
			Name:       functionName,
			Version:    functionVersion,
			RequestID:  lc.AwsRequestID,
			MemorySize: memorySize,
			Tags:       faasTags,
		},
		AwsCommon: &awsCommon{
			LogGroup:     logGroup,
			LogGroupARN:  logGroupARN,
			LogGroupTags: logGroupTags,
			LogStream:    logStream,
			ServiceTags:  sourceTags,
		},
		HostArchitecture:   hostArchitecture,
		ProcessRuntimeName: processRuntimeName,
	}
}

func (e *Enricher) getAllTags(ctx context.Context, forwarderARN, logGroupARN string, allARNs []string, isSourceLambda bool) (sourceTags, faasTags, logGroupTags map[string]string) {
	e.prepareResourceTags(ctx, allARNs)
	if m, ok := resourceARNToTagsCache[logGroupARN]; ok {
		logGroupTags = m
	}
	if m, ok := resourceARNToTagsCache[forwarderARN]; ok {
		faasTags = m
	}

	var tags map[string]string
	if isSourceLambda {
		if faasTags == nil {
			faasTags = make(map[string]string)
		}
		tags = faasTags
	} else {
		if sourceTags == nil {
			sourceTags = make(map[string]string)
		}
		tags = sourceTags
	}

	for _, arn := range allARNs {
		if arn == forwarderARN || arn == logGroupARN {
			continue
		}
		if m, ok := resourceARNToTagsCache[arn]; ok {
			for k, v := range m {
				tags[k] = v
			}
		}
	}

	return
}

func (e *Enricher) prepareResourceTags(ctx context.Context, arns []string) {
	tagsCacheKey := getTagsCacheKey(arns...)
	if _, ok := resourceARNToTagsCache[tagsCacheKey]; ok {
		return
	}
	log.Printf("Getting resource tags for ARNs: %v", arns)
	tagsMap, err := e.resourceCl.GetResourceTags(ctx, arns...)
	if err != nil {
		log.Printf("Failed to get resource tags for ARNs: %v, err: %v", arns, err)
		return
	}
	if len(tagsMap) == 0 {
		log.Printf("Failed to find tags for ARNs: %v", arns)
		return
	}

	for _, arn := range arns {
		if m, ok := tagsMap[arn]; ok {
			resourceARNToTagsCache[arn] = m
		}
	}

	// Empty map with key as all ARNs is added to cache to avoid repeated calls to resource service
	resourceARNToTagsCache[tagsCacheKey] = map[string]string{}
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
			if m, ok := resourceARNToTagsCache[arn]; ok {
				if len(m) > 0 {
					return arn
				}
			}
		}
	}
	return forwarderARN
}

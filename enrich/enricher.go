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
	"github.com/edgedelta/edgedelta-forwarder/tag"
	"github.com/edgedelta/edgedelta-forwarder/utils"
)

var resourceARNToTagsCache = make(map[string]map[string]string)

func NewEnricher(conf *cfg.Config, resourceCl resource.Client, lambdaCl lambda.Client) *Enricher {
	return &Enricher{
		forwardForwarderTags: conf.ForwardForwarderTags,
		forwardSourceTags:    conf.ForwardSourceTags,
		forwardLogGroupTags:  conf.ForwardLogGroupTags,
		sourcePrefixMap:      prepareSourcePrefixMap(conf.SourceEnvironmentPrefixes),
		region:               conf.Region,
		resourceCl:           resourceCl,
		lambdaCl:             lambdaCl,
	}
}

func (e *Enricher) GetEDCommon(ctx context.Context, subscriptionFilters []string, messageType, logGroup, logStream, accountID string) *Common {
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

	logGroupARN := parser.BuildResourceARN("logs", accountID, e.region, fmt.Sprintf("log-group:%s", logGroup))
	if e.forwardLogGroupTags {
		arnsToGetTags = append(arnsToGetTags, logGroupARN)
	}

	arnToTagSourceMap := map[string]tag.Source{}
	if e.forwardSourceTags {
		if sources, ok := parser.GetSourceARNsFromLogGroup(accountID, e.region, logGroup, logStream); ok {
			for _, s := range sources {
				arnsToGetTags = append(arnsToGetTags, s.ARN)
				arnToTagSourceMap[s.ARN] = s.Name
			}
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

	sourceTags, faasTags, logGroupTags := e.getAllTags(ctx, forwarderARN, logGroupARN, arnsToGetTags, arnToTagSourceMap, isSourceLambda)

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
			awsLogs: awsLogs{
				LogGroup:               logGroup,
				LogGroupARN:            logGroupARN,
				LogGroupTags:           logGroupTags,
				LogStream:              logStream,
				LogMessageType:         messageType,
				LogSubscriptionFilters: subscriptionFilters,
			},
			ServiceTags: sourceTags,
		},
		HostArchitecture:   hostArchitecture,
		ProcessRuntimeName: processRuntimeName,
	}
}

// getAllTags retrieves all the tags for the specified ARNs and populates the tag maps.
func (e *Enricher) getAllTags(ctx context.Context, forwarderARN, logGroupARN string, allARNs []string, arnToService map[string]tag.Source, isSourceLambda bool) (sourceTags, faasTags, logGroupTags map[string]string) {
	e.prepareResourceTags(ctx, allARNs)

	if prefix, ok := e.sourcePrefixMap[tag.SourceLogGroup]; !ok {
		logGroupTags = resourceARNToTagsCache[logGroupARN]
	} else if m, ok := resourceARNToTagsCache[logGroupARN]; ok {
		logGroupTags = make(map[string]string, len(m))
		for k, v := range m {
			utils.SetKeyWithPrefix(logGroupTags, prefix, k, v)
		}
	}

	if prefix, ok := e.sourcePrefixMap[tag.SourceForwarder]; !ok {
		faasTags = resourceARNToTagsCache[forwarderARN]
	} else if m, ok := resourceARNToTagsCache[forwarderARN]; ok {
		faasTags = make(map[string]string, len(m))
		for k, v := range m {
			utils.SetKeyWithPrefix(faasTags, prefix, k, v)
		}
	}

	var tags map[string]string
	if isSourceLambda {
		faasTags = initializeMapIfEmpty(faasTags)
		tags = faasTags
	} else {
		sourceTags = initializeMapIfEmpty(sourceTags)
		tags = sourceTags
	}

	for _, arn := range allARNs {
		if arn == forwarderARN || arn == logGroupARN {
			continue
		}
		if m, ok := resourceARNToTagsCache[arn]; ok {
			prefix := e.sourcePrefixMap[arnToService[arn]]
			for k, v := range m {
				utils.SetKeyWithPrefix(tags, prefix, k, v)
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
	for _, arn := range arns {
		if arn != forwarderARN && arn != logGroupARN {
			if m, ok := resourceARNToTagsCache[arn]; ok && len(m) > 0 {
				return arn
			}
		}
	}

	return forwarderARN
}

func initializeMapIfEmpty(m map[string]string) map[string]string {
	if m == nil {
		return make(map[string]string)
	}
	return m

}

func prepareSourcePrefixMap(prefixes string) map[tag.Source]string {
	if prefixes == "" {
		return nil
	}
	prefixMap := make(map[tag.Source]string)
	parts := strings.Split(prefixes, ",")
	for _, p := range parts {
		parts := strings.Split(strings.TrimSpace(p), "=")
		if len(parts) == 2 {
			key := tag.Source(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			prefixMap[key] = value
		}
	}

	return prefixMap
}

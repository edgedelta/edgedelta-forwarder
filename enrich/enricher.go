package enrich

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/resource"
)

const lambdaLogGroupPrefix = "/aws/lambda/"

var resourceARNToTagsCache = make(map[string]map[string]string)

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
	forwardLambdaTags    bool
	forwardForwarderTags bool
}

func NewEnricher(conf *cfg.Config, resourceCl *resource.DefaultClient) *Enricher {
	return &Enricher{
		forwardLambdaTags:    conf.ForwardLambdaTags,
		forwardForwarderTags: conf.ForwardForwarderTags,
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

	var tags map[string]string
	if forwarderARN != "" && e.forwardForwarderTags {
		tags = e.getLambdaTags(ctx, forwarderARN)
	} else {
		tags = make(map[string]string)
	}

	var functionName, functionVersion, functionARN string
	foundLambdaLogGroup := false
	if name, ok := getFunctionName(logGroup); ok {
		log.Printf("Got lambda name: %s from log group: %s", name, logGroup)
		foundLambdaLogGroup = true
		functionName = name
		functionVersion = ""
		functionARN = buildFunctionARN(name, accountID, e.region)
	} else {
		functionName = lambdacontext.FunctionName
		functionVersion = lambdacontext.FunctionVersion
		functionARN = forwarderARN
	}

	if foundLambdaLogGroup && e.forwardLambdaTags {
		t := e.getLambdaTags(ctx, functionARN)
		for k, v := range t {
			tags[k] = v
		}
	}
	return &Common{
		Cloud: &cloud{ResourceID: functionARN},
		Faas: &faas{
			Name:    functionName,
			Version: functionVersion,
		},
		LambdaTags: tags,
	}
}

func (e *Enricher) getLambdaTags(ctx context.Context, functionARN string) map[string]string {
	if tags, ok := resourceARNToTagsCache[functionARN]; ok {
		return tags
	}
	tagsMap, err := e.resourceCl.GetResourceTags(ctx, functionARN)
	if err != nil {
		log.Printf("Failed to get resource tags for ARN: %s, err: %v", functionARN, err)
	} else if len(tagsMap) == 0 {
		log.Printf("Failed to find tags for ARN: %s", functionARN)
	} else {
		for r, t := range tagsMap {
			log.Printf("Found tags: %v for ARN: %s", t, r)
			resourceARNToTagsCache[r] = t
		}
	}
	return resourceARNToTagsCache[functionARN]
}

func getFunctionName(logGroup string) (string, bool) {
	name := strings.TrimPrefix(logGroup, lambdaLogGroupPrefix)
	if len(name) == len(logGroup) {
		return "", false
	}
	return name, true
}

func buildFunctionARN(functionName, accountID, region string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", region, accountID, functionName)
}

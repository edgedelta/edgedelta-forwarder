package enrich

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/ecs"
	"github.com/edgedelta/edgedelta-forwarder/lambda"
	"github.com/edgedelta/edgedelta-forwarder/parser"
	"github.com/edgedelta/edgedelta-forwarder/resource"
	"github.com/edgedelta/edgedelta-forwarder/tag"
	"github.com/edgedelta/edgedelta-forwarder/utils"

	sLambda "github.com/aws/aws-sdk-go/service/lambda"
)

var resourceARNToTagsCache = make(map[string]map[string]string)

func NewEnricher(conf *cfg.Config, resourceCl resource.Client, lambdaCl lambda.Client, ecsCl ecs.Client) *Enricher {
	return &Enricher{
		forwardForwarderTags: conf.ForwardForwarderTags,
		forwardSourceTags:    conf.ForwardSourceTags,
		forwardLogGroupTags:  conf.ForwardLogGroupTags,
		sourcePrefixMap:      prepareSourcePrefixMap(conf.SourceEnvironmentPrefixes),
		region:               conf.Region,
		resourceCl:           resourceCl,
		lambdaCl:             lambdaCl,
		ecsCl:                ecsCl,
		ecsContainerCacheMap: make(map[ecsContainerCacheKey]ecsContainerCachedResult),
		ecsContainerCacheTTL: conf.ECSContainerCacheTTL,
		ecsClusterOverride:   conf.ECSClusterOverride,
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

	var functionOutput *sLambda.GetFunctionOutput
	if functionARN != "" {
		function, err := e.lambdaCl.GetFunction(functionARN)
		if err != nil {
			log.Printf("Failed to get function for ARN: %s, err: %v", functionARN, err)
		} else {
			functionOutput = function
		}
	}

	// Overwrite function version if it exists in the function output
	if functionOutput != nil && functionOutput.Configuration != nil && functionOutput.Configuration.Version != nil {
		functionVersion = *functionOutput.Configuration.Version
	}

	var memorySize string
	if functionOutput != nil && functionOutput.Configuration != nil && functionOutput.Configuration.MemorySize != nil {
		memorySize = fmt.Sprintf("%d", *functionOutput.Configuration.MemorySize)
	}

	var processRuntimeName string
	if functionOutput != nil && functionOutput.Configuration != nil && functionOutput.Configuration.Runtime != nil {
		processRuntimeName = *functionOutput.Configuration.Runtime
	}

	var hostArchitecture string
	if functionOutput != nil && functionOutput.Configuration != nil && functionOutput.Configuration.Architectures != nil {
		hostArchitecture = getRuntimeArchitecture(functionARN, forwarderARN, functionOutput.Configuration.Architectures)
	}

	sourceTags, faasTags, logGroupTags := e.getAllTags(ctx, forwarderARN, logGroupARN, arnsToGetTags, arnToTagSourceMap, isSourceLambda)
	cm := &Common{
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

	ecsCluster, ecsContainerFromStream, ecsTaskID := parser.GetClusterContainerAndTaskIfSourceIsECS(logGroup, logStream, e.ecsClusterOverride)
	if ecsCluster != "" {
		container, containerList, err := e.GetECSContainerDetails(ctx, ecsCluster, ecsTaskID, ecsContainerFromStream)
		if err != nil {
			log.Printf("Failed to get container ID for cluster: %s task: %s container: %s, err: %v", ecsCluster, ecsTaskID, ecsContainerFromStream, err)
			// fallback to container name from log stream, in case DescribeTask permission is missing
			container = &ecsContainer{Name: ecsContainerFromStream}
		}
		cm.AwsCommon.ECS = &ecsContainerWrapper{Container: container, ContainerList: containerList}
	}

	return cm
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

func (e *Enricher) GetECSContainerDetails(ctx context.Context, clusterName, taskID, containerName string) (*ecsContainer, []*ecsContainer, error) {
	cKey := ecsContainerCacheKey{
		clusterName: clusterName,
		taskID:      taskID,
	}

	// Check cache first
	e.ecsContainerCacheLock.RLock()
	if cached, ok := e.ecsContainerCacheMap[cKey]; ok && time.Now().Before(cached.expiry) {
		// Cache hit
		var foundContainer *ecsContainer
		if cached.containerInfo != nil && cached.containerInfo.Name == containerName {
			foundContainer = cached.containerInfo
		} else {
			// Search through container list if containerName doesn't match the cached containerInfo
			for _, container := range cached.containerList {
				if container.Name == containerName {
					foundContainer = container
					break
				}
			}
		}
		e.ecsContainerCacheLock.RUnlock()
		return foundContainer, cached.containerList, nil
	}
	e.ecsContainerCacheLock.RUnlock()

	// Cache miss, fetch from ECS Service
	taskOutput, err := e.ecsCl.GetTaskDetails(ctx, clusterName, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get task details, err: %v", err)
	}

	if len(taskOutput.Tasks) == 0 {
		return nil, nil, fmt.Errorf("task: %s not found in cluster: %s", taskID, clusterName)
	}

	var containerList []*ecsContainer
	var containerInfo *ecsContainer

	for _, container := range taskOutput.Tasks[0].Containers {
		container := &ecsContainer{
			Name:   safeDeref(container.Name),
			ID:     safeDeref(container.RuntimeId),
			Image:  safeDeref(container.Image),
			Status: safeDeref(container.LastStatus),
		}

		containerList = append(containerList, container)

		if container.Name == containerName {
			containerInfo = container
		}
	}

	// Update cache
	e.ecsContainerCacheLock.Lock()
	e.ecsContainerCacheMap[cKey] = ecsContainerCachedResult{
		containerInfo: containerInfo,
		containerList: containerList,
		expiry:        time.Now().Add(e.ecsContainerCacheTTL),
	}
	e.ecsContainerCacheLock.Unlock()

	return containerInfo, containerList, nil
}

func (e *Enricher) StartECSContainerCacheCleanup() {
	go func() {
		ticker := time.NewTicker(e.ecsContainerCacheTTL / 3)
		defer ticker.Stop()

		for range ticker.C {
			e.CleanupExpiredEntries()
		}
	}()
}

func (e *Enricher) CleanupExpiredEntries() {
	e.ecsContainerCacheLock.Lock()
	defer e.ecsContainerCacheLock.Unlock()

	now := time.Now()
	for k, v := range e.ecsContainerCacheMap {
		if now.After(v.expiry) {
			delete(e.ecsContainerCacheMap, k)
		}
	}
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

func safeDeref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

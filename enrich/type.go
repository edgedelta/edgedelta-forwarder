package enrich

import (
	"sync"
	"time"

	"github.com/edgedelta/edgedelta-forwarder/ecs"
	"github.com/edgedelta/edgedelta-forwarder/lambda"
	"github.com/edgedelta/edgedelta-forwarder/resource"
	"github.com/edgedelta/edgedelta-forwarder/tag"
)

type ecsContainerCacheKey struct {
	clusterName string
	taskID      string
}

type ecsContainerCachedResult struct {
	containerInfo *ecsContainer
	containerList []*ecsContainer
	expiry        time.Time
}

type Enricher struct {
	resourceCl            resource.Client
	lambdaCl              lambda.Client
	ecsCl                 ecs.Client
	region                string
	sourcePrefixMap       map[tag.Source]string
	forwardForwarderTags  bool
	forwardSourceTags     bool
	forwardLogGroupTags   bool
	ecsContainerCacheMap  map[ecsContainerCacheKey]ecsContainerCachedResult
	ecsContainerCacheLock sync.RWMutex
	ecsContainerCacheTTL  time.Duration
	ecsClusterOverride    string
}

type Common struct {
	Cloud              *cloud     `json:"cloud"`
	Faas               *faas      `json:"faas"`
	AwsCommon          *awsCommon `json:"aws"`
	HostArchitecture   string     `json:"host.arch,omitempty"`
	ProcessRuntimeName string     `json:"process.runtime.name,omitempty"`
}

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

type awsLogs struct {
	LogGroup               string            `json:"log.group.name"`
	LogGroupARN            string            `json:"log.group.arn"`
	LogGroupTags           map[string]string `json:"log.group.tags,omitempty"`
	LogStream              string            `json:"log.stream.name"`
	LogMessageType         string            `json:"log.message_type"`
	LogSubscriptionFilters []string          `json:"log.subscription_filters"`
}

type ecsContainer struct {
	Name   string `json:"name,omitempty"`
	ID     string `json:"id,omitempty"`
	Image  string `json:"image,omitempty"`
	Status string `json:"status,omitempty"`
}

type ecsContainerWrapper struct {
	Container     *ecsContainer   `json:"container,omitempty"`
	ContainerList []*ecsContainer `json:"container_list,omitempty"`
}

type awsCommon struct {
	awsLogs
	ServiceTags map[string]string    `json:"service.tags,omitempty"`
	ECS         *ecsContainerWrapper `json:"ecs,omitempty"`
}

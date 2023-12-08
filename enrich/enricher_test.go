package enrich

import (
	"context"
	"testing"

	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/lambda"
	"github.com/edgedelta/edgedelta-forwarder/resource"
	"github.com/edgedelta/edgedelta-forwarder/tag"
	"github.com/edgedelta/edgedelta-forwarder/utils"
	"github.com/google/go-cmp/cmp"
)

const (
	functionARN   = "arn:aws:lambda:us-west-2:123456789012:function:my-function"
	forwarderARN  = "arn:aws:lambda:us-west-2:123456789012:function:forwarder"
	logGroupARN   = "arn:aws:logs:us-west-2:123456789012:log-group:my-log-group"
	ecsTaskARN    = "arn:aws:ecs:us-west-2:123456789012:task/my-cluster/1234567890123456789"
	ecsClusterARN = "arn:aws:ecs:us-west-2:123456789012:cluster/my-cluster"
	ecsServiceARN = "arn:aws:ecs:us-west-2:123456789012:service/my-cluster/my-service"
)

func copyMap(m map[string]string) map[string]string {
	var newMap = make(map[string]string)
	for k, v := range m {
		newMap[k] = v
	}
	return newMap
}

var (
	logGroupTags = map[string]string{
		"customer_log_group_tag":   "test_1",
		"customer_log_group_tag_2": "log_group_tag_2",
	}

	functionTags = map[string]string{
		"customer_lambda_tag":   "test_1",
		"customer_lambda_tag_2": "lambda_tag_2",
		"environment":           "dev",
	}
	forwarderTags = map[string]string{
		"customer_forwarder_tag":   "test_1",
		"customer_forwarder_tag_2": "forwarder_tag_2",
		"environment":              "dev",
	}

	ecsTaskTags = map[string]string{
		"customer_ecs_task_tag":   "test_1",
		"customer_ecs_task_tag_2": "ecs_task_tag_2",
		"environment":             "dev",
	}

	ecsClusterTags = map[string]string{
		"customer_ecs_cluster_tag":   "test_1",
		"customer_ecs_cluster_tag_2": "ecs_cluster_tag_2",
		"environment":                "dev",
	}

	ecsServiceTags = map[string]string{
		"customer_ecs_service_tag":   "test_1",
		"customer_ecs_service_tag_2": "ecs_service_tag_2",
		"environment":                "dev",
	}
)

var (
	arnToSourceMap = map[string]tag.Source{
		functionARN:   "lambda",
		logGroupARN:   tag.SourceLogGroup,
		forwarderARN:  tag.SourceForwarder,
		ecsClusterARN: tag.SourceECSCluster,
		ecsTaskARN:    tag.SourceECSTask,
		ecsServiceARN: tag.SourceECSService,
	}
)

type mockResourceClient struct {
	tags map[string]map[string]string
	err  error
}

func (m *mockResourceClient) GetResourceTags(ctx context.Context, resourceARNs ...string) (map[string]map[string]string, error) {
	return m.tags, m.err
}

func TestGetLambdaTags(t *testing.T) {
	type testCase struct {
		desc             string
		config           *cfg.Config
		isSourceLambda   bool
		resourceClient   resource.Client
		arnsToGetTags    []string
		wantFaasTags     map[string]string
		wantSourceTags   map[string]string
		wantLogGroupTags map[string]string
	}

	tests := []func() testCase{
		func() testCase {
			return testCase{

				desc: "Get tags when source is lambda and without forwarder tags",
				config: &cfg.Config{
					ForwardSourceTags:         true,
					ForwardForwarderTags:      false,
					ForwardLogGroupTags:       true,
					Region:                    "us-west-2",
					SourceEnvironmentPrefixes: "",
				},
				resourceClient: &mockResourceClient{
					tags: map[string]map[string]string{
						functionARN:  copyMap(functionTags),
						logGroupARN:  copyMap(logGroupTags),
						forwarderARN: copyMap(forwarderTags),
					},
				},
				arnsToGetTags:    []string{functionARN, logGroupARN},
				isSourceLambda:   true,
				wantSourceTags:   nil,
				wantLogGroupTags: copyMap(logGroupTags),
				wantFaasTags:     copyMap(functionTags),
			}
		},
		func() testCase {
			return testCase{
				desc: "Get tags when source is lambda and with forwarder tags",
				config: &cfg.Config{
					ForwardSourceTags:         true,
					ForwardForwarderTags:      true,
					ForwardLogGroupTags:       true,
					Region:                    "us-west-2",
					SourceEnvironmentPrefixes: "",
				},
				arnsToGetTags:  []string{functionARN, logGroupARN, forwarderARN},
				isSourceLambda: true,
				resourceClient: &mockResourceClient{
					tags: map[string]map[string]string{
						functionARN:  copyMap(functionTags),
						logGroupARN:  copyMap(logGroupTags),
						forwarderARN: copyMap(forwarderTags),
					},
				},
				wantSourceTags:   nil,
				wantLogGroupTags: copyMap(logGroupTags),
				wantFaasTags: func() map[string]string {
					var m = make(map[string]string)
					for k, v := range copyMap(functionTags) {
						m[k] = v
					}
					for k, v := range copyMap(forwarderTags) {
						m[k] = v
					}
					return m
				}(),
			}
		},
		func() testCase {
			return testCase{
				desc: "Get tags when source is lambda and with forwarder tags and with prefix to prevent duplication",
				config: &cfg.Config{
					ForwardSourceTags:         true,
					ForwardForwarderTags:      true,
					ForwardLogGroupTags:       true,
					Region:                    "us-west-2",
					SourceEnvironmentPrefixes: "ed_forwarder=ed_fwd_",
				},
				arnsToGetTags:  []string{functionARN, logGroupARN, forwarderARN},
				isSourceLambda: true,
				resourceClient: &mockResourceClient{
					tags: map[string]map[string]string{
						functionARN:  copyMap(functionTags),
						logGroupARN:  copyMap(logGroupTags),
						forwarderARN: copyMap(forwarderTags),
					},
				},
				wantSourceTags:   nil,
				wantLogGroupTags: copyMap(logGroupTags),
				wantFaasTags: func() map[string]string {
					var m = make(map[string]string)
					for k, v := range copyMap(functionTags) {
						m[k] = v
					}
					for k, v := range copyMap(forwarderTags) {
						utils.SetKeyWithPrefix(m, "ed_fwd_", k, v)
					}
					return m
				}(),
			}
		},
		func() testCase {
			return testCase{
				desc: "Get tags when source is ECS and with forwarder tags and with prefix to prevent duplication",
				config: &cfg.Config{
					ForwardSourceTags:         true,
					ForwardForwarderTags:      true,
					ForwardLogGroupTags:       true,
					Region:                    "us-west-2",
					SourceEnvironmentPrefixes: "ecs_task=task_prefix_,ecs_cluster=cluster_prefix_,ecs_service=service_prefix_",
				},
				arnsToGetTags: []string{logGroupARN, forwarderARN, ecsClusterARN, ecsTaskARN, ecsServiceARN},
				resourceClient: &mockResourceClient{
					tags: map[string]map[string]string{
						logGroupARN:   copyMap(logGroupTags),
						forwarderARN:  copyMap(forwarderTags),
						ecsClusterARN: copyMap(ecsClusterTags),
						ecsTaskARN:    copyMap(ecsTaskTags),
						ecsServiceARN: copyMap(ecsServiceTags),
					},
				},
				wantSourceTags: func() map[string]string {
					var m = make(map[string]string)
					for k, v := range copyMap(ecsClusterTags) {
						utils.SetKeyWithPrefix(m, "cluster_prefix_", k, v)
					}
					for k, v := range copyMap(ecsTaskTags) {
						utils.SetKeyWithPrefix(m, "task_prefix_", k, v)
					}
					for k, v := range copyMap(ecsServiceTags) {
						utils.SetKeyWithPrefix(m, "service_prefix_", k, v)
					}
					return m
				}(),
				wantLogGroupTags: copyMap(logGroupTags),
				wantFaasTags:     copyMap(forwarderTags),
			}
		},
		func() testCase {
			return testCase{
				desc: "Get tags when source is ECS and with forwarder tags and without prefix",
				config: &cfg.Config{
					ForwardSourceTags:         true,
					ForwardForwarderTags:      true,
					ForwardLogGroupTags:       true,
					Region:                    "us-west-2",
					SourceEnvironmentPrefixes: "",
				},
				arnsToGetTags: []string{logGroupARN, forwarderARN, ecsClusterARN, ecsTaskARN, ecsServiceARN},
				resourceClient: &mockResourceClient{
					tags: map[string]map[string]string{
						logGroupARN:   copyMap(logGroupTags),
						forwarderARN:  copyMap(forwarderTags),
						ecsClusterARN: copyMap(ecsClusterTags),
						ecsTaskARN:    copyMap(ecsTaskTags),
						ecsServiceARN: copyMap(ecsServiceTags),
					},
				},
				wantSourceTags: func() map[string]string {
					var m = make(map[string]string)
					for k, v := range copyMap(ecsClusterTags) {
						m[k] = v
					}
					for k, v := range copyMap(ecsTaskTags) {
						m[k] = v
					}
					for k, v := range copyMap(ecsServiceTags) {
						m[k] = v
					}
					return m
				}(),
				wantLogGroupTags: copyMap(logGroupTags),
				wantFaasTags:     copyMap(forwarderTags),
			}
		},
	}

	for _, tc := range tests {
		tc := tc()
		t.Run(tc.desc, func(t *testing.T) {
			// Clear global cache
			defer func() {
				resourceARNToTagsCache = make(map[string]map[string]string)
			}()

			enricher := NewEnricher(tc.config, tc.resourceClient, lambda.NewNoOpClient())
			sourceTags, faasTags, logGroupTags := enricher.getAllTags(context.Background(), forwarderARN, logGroupARN, tc.arnsToGetTags, arnToSourceMap, tc.isSourceLambda)
			if diff := cmp.Diff(tc.wantFaasTags, faasTags); diff != "" {
				t.Errorf("Faas tags mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantSourceTags, sourceTags); diff != "" {
				t.Errorf("Source tags mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantLogGroupTags, logGroupTags); diff != "" {
				t.Errorf("Lambda tags mismatch (-want +got):\n%s", diff)
			}
		})
	}

}

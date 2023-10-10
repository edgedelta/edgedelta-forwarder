package enrich

import (
	"context"
	"errors"
	"testing"

	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/lambda"
	"github.com/edgedelta/edgedelta-forwarder/resource"
	"github.com/google/go-cmp/cmp"
)

type mockResourceClient struct {
	tags map[string]map[string]string
	err  error
}

func (m *mockResourceClient) GetResourceTags(ctx context.Context, resourceARNs ...string) (map[string]map[string]string, error) {
	return m.tags, m.err
}

func TestGetLambdaTags(t *testing.T) {
	functionARN := "function:1"
	forwarderARN := "function:2"
	logGroupARN := "logGroup:1"

	tests := []struct {
		desc           string
		config         *cfg.Config
		resourceClient resource.Client
		wantFaasTags   map[string]string
	}{
		{
			desc: "got tags from resources",
			config: &cfg.Config{
				ForwardSourceTags:    true,
				ForwardForwarderTags: true,
				Region:               "us-west-2",
			},
			resourceClient: &mockResourceClient{
				tags: map[string]map[string]string{
					functionARN: {
						"tag1": "val1",
					},
				},
			},
			wantFaasTags: map[string]string{
				"tag1": "val1",
			},
		},
		{
			desc: "error while getting tags from resources",
			config: &cfg.Config{
				ForwardSourceTags:    true,
				ForwardForwarderTags: true,
				Region:               "us-west-2",
			},
			resourceClient: &mockResourceClient{
				tags: nil,
				err:  errors.New("no tags for this function"),
			},
			wantFaasTags: map[string]string{},
		},
		{
			desc: "got empty tags from resources",
			config: &cfg.Config{
				ForwardSourceTags:    true,
				ForwardForwarderTags: true,
				Region:               "us-west-2",
			},
			resourceClient: &mockResourceClient{
				tags: map[string]map[string]string{},
				err:  nil,
			},
			wantFaasTags: map[string]string{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			// Clear global cache
			defer delete(resourceARNToTagsCache, functionARN)

			enricher := NewEnricher(tc.config, tc.resourceClient, lambda.NewNoOpClient())
			_, faasTags := enricher.getAllTags(context.TODO(), forwarderARN, logGroupARN, []string{functionARN}, true)
			if diff := cmp.Diff(tc.wantFaasTags, faasTags); diff != "" {
				t.Errorf("Tags mismatch (-want +got):\n%s", diff)
			}
		})
	}

}

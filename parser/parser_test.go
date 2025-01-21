package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindSourceFromLogGroup(t *testing.T) {
	tests := []struct {
		logGroup      string
		expectedSvc   string
		expectedRes   string
		expectedFound bool
	}{
		{"/aws/lambda/my-function", "lambda", "my-function", true},
		{"/aws/codebuild/my-project", "codebuild", "my-project", true},
		{"/aws/kinesis/my-stream", "kinesis", "my-stream", true},
		{"/aws/eks/my-cluster", "eks", "my-cluster", true},
		{"/aws/docdb/my-cluster", "docdb", "my-cluster", true},
		{"/aws/rds/mariadb/my-db", "mariadb", "my-db", true},
		{"/aws/rds/mysql/my-db", "mysql", "my-db", true},
		{"/aws/rds/postgresql/my-db", "postgresql", "my-db", true},
		{"/aws/rds/other-db", "rds", "other-db", true},
		{"/aws/unknown-service/resource", "unknown-service", "resource", true},
		{"/aws/unknown-service/resource/type", "unknown-service", "resource/type", true},
		{"/aws/", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.logGroup, func(t *testing.T) {
			svc, res, found := findSourceFromLogGroup(tt.logGroup)
			assert.Equal(t, tt.expectedSvc, svc)
			assert.Equal(t, tt.expectedRes, res)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

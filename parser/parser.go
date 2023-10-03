package parser

import (
	"fmt"
	"strings"
)

const (
	NoSuffix = ""
)

var (
	AvailableResourceSuffixesForServices = map[string][]string{
		"lambda":           {"function:"},
		"eks":              {"cluster/"},
		"sns":              {NoSuffix},
		"sqs":              {NoSuffix},
		"ecs":              {"cluster/", "service/", "task/"},
		"ec2":              {"instance/", "vpc/", "image/"},
		"rds":              {"cluster:", "instance:", "snapshot:", "pg:", "cluster-pg:"},
		"sagemaker":        {"project/", "training-job/", "transform-job/", "pipeline/", "context/", "notebook-instance/"},
		"emr-serverless":   {"/applications/"},
		"codebuild":        {"project/", "build/"},
		"codecatalyst":     {"/connections/"},
		"codedeploy":       {"application:", "instance:"},
		"firehose":         {"deliverystream/"},
		"kinesis":          {"stream/"},
		"docdb":            {"cluster/"},
		"network-firewall": {"firewall"},
		"route53":          {"hostedzone", "change"},
		"vpc":              {NoSuffix},
		"cloudtrail":       {"trail"},
		"msk":              {"cluster"},
		"elasticsearch":    {"es"},
		"transitgateway":   {"tgw"},
		"verified-access":  {"vpc"},
	}

	ResourceSuffixToDeleteFromLogGroup = map[string]string{
		"eks": "/cluster",
	}
)

func findSourceFromLogGroup(logGroup string) (string, string, bool) {
	hasPrefixFunc := func(prefix string) bool {
		return strings.HasPrefix(logGroup, prefix)
	}
	trimPrefixFunc := func(prefix string) string {
		return strings.TrimPrefix(logGroup, prefix)
	}
	containsFunc := func(str string) bool {
		return strings.Contains(logGroup, str)
	}

	for _, str := range []string{"lambda", "codebuild", "kinesis", "eks", "docdb", "sns", "sqs"} {
		if hasPrefixFunc("/aws/" + str) {
			return str, trimPrefixFunc(fmt.Sprintf("/aws/%s/", str)), true
		}
	}

	if hasPrefixFunc("/aws/rds") {
		for _, db := range []string{"mariadb", "mysql", "postgresql"} {
			if containsFunc(db) {
				return db, trimPrefixFunc(fmt.Sprintf("/aws/rds/%s/", db)), true
			}
		}
		return "rds", trimPrefixFunc("/aws/rds/"), true
	}

	for _, str := range []string{"sns", "sqs"} {
		if withSlash := str + "/"; hasPrefixFunc(withSlash) {
			return str, trimPrefixFunc(withSlash), true
		}
	}

	// For the rest assume that log group has format /aws/<service>/<resource_name> or /aws/<service>/<resource_type>/<resource_name>/...
	withoutAWS := trimPrefixFunc("/aws/")
	parts := strings.Split(withoutAWS, "/")
	if len(parts) > 1 {
		return parts[0], strings.Join(parts[1:], "/"), true
	}
	return "", "", false
}

func GetSourceARNsFromLogGroup(accountID, region, logGroup string) ([]string, bool) {
	service, resourceName, ok := findSourceFromLogGroup(logGroup)
	if !ok {
		return nil, false
	}

	suffixToDelete := ResourceSuffixToDeleteFromLogGroup[service]
	if suffixToDelete != "" {
		resourceName = strings.TrimSuffix(resourceName, suffixToDelete)
	}

	suffixes, ok := AvailableResourceSuffixesForServices[service]
	if !ok {
		return []string{BuildServiceARN(service, accountID, region, resourceName)}, true
	}

	var arns []string
	for _, suffix := range suffixes {
		arns = append(arns, BuildServiceARN(service, accountID, region, suffix+resourceName))
	}
	return arns, true
}

func BuildServiceARN(service, accountID, region, suffix string) string {
	return fmt.Sprintf("arn:aws:%s:%s:%s:%s", service, region, accountID, suffix)
}

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
		"ecs":              {"cluster/", "service/", "task/"},
		"ec2":              {"instance/", "vpc/", "image/"},
		"rds":              {"cluster:", "instance:", "snapshot:", "pg:", "cluster-pg:"},
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
	trimPrefixFunc := func(prefix string) string {
		return strings.TrimPrefix(logGroup, prefix)
	}

	if strings.HasPrefix(logGroup, "/ecs/") {
		return "ecs", trimPrefixFunc("/ecs/"), true
	}

	for _, str := range []string{"lambda", "codebuild", "kinesis", "eks", "docdb"} {
		if strings.HasPrefix(logGroup, "/aws/"+str) {
			return str, trimPrefixFunc(fmt.Sprintf("/aws/%s/", str)), true
		}
	}

	if strings.HasPrefix(logGroup, "/aws/rds") {
		for _, db := range []string{"mariadb", "mysql", "postgresql"} {
			if strings.Contains(logGroup, db) {
				return db, trimPrefixFunc(fmt.Sprintf("/aws/rds/%s/", db)), true
			}
		}
		return "rds", trimPrefixFunc("/aws/rds/"), true
	}

	// For the rest assume that log group has format /aws/<service>/<resource_name> or /aws/<service>/<resource_type>/<resource_name>/...
	withoutAWS := trimPrefixFunc("/aws/")
	parts := strings.Split(withoutAWS, "/")
	if len(parts) > 1 {
		return parts[0], strings.Join(parts[1:], "/"), true
	}
	return "", "", false
}

func GetSourceARNsFromLogGroup(accountID, region, logGroup, logStream string) ([]string, bool) {
	if strings.HasPrefix(logGroup, "/aws/sagemaker") {
		return []string{buildSagemakerARN(strings.TrimPrefix(logGroup, "/aws/sagemaker/"), logStream, accountID, region)}, true
	}
	if strings.HasPrefix(logGroup, "sns/") {
		return []string{buildSNSARN(strings.TrimPrefix(logGroup, "sns/"), logStream, accountID, region)}, true
	}
	return buildGenericARN(logGroup, logStream, accountID, region)
}

func buildSagemakerARN(trimmedGroup, logStream, accountID, region string) string {
	containsFunc := func(str string) bool {
		return strings.Contains(trimmedGroup, str)
	}
	groupParts := strings.Split(trimmedGroup, "/")
	streamParts := strings.Split(logStream, "/")
	buildServiceArnFunc := func(suffix string) string {
		return BuildServiceARN("sagemaker", accountID, region, suffix)
	}
	if containsFunc("CompilationJobs") {
		return buildServiceArnFunc(fmt.Sprintf("CompilationJob/%s", logStream))
	}
	if containsFunc("Endpoints") && len(groupParts) == 2 {
		return buildServiceArnFunc(fmt.Sprintf("endpoint/%s", groupParts[1]))
	}
	if containsFunc("InferenceRecommendationsJobs") && len(streamParts) == 2 {
		if len(streamParts) == 2 {
			return buildServiceArnFunc(fmt.Sprintf("InferenceRecommendationsJob/%s", streamParts[0]))
		}
	}
	if containsFunc("LabelingJobs") {
		return buildServiceArnFunc(fmt.Sprintf("LabelingJob/%s", logStream))
	}
	if containsFunc("NotebookInstances") && len(streamParts) == 2 {
		if len(streamParts) == 2 {
			return buildServiceArnFunc(fmt.Sprintf("NotebookInstance/%s", streamParts[0]))
		}
	}
	if containsFunc("ProcessingJobs") && len(streamParts) == 2 {
		return buildServiceArnFunc(fmt.Sprintf("ProcessingJob/%s", streamParts[0]))
	}

	// as fallback sagemaker/{resource_name}
	return buildServiceArnFunc(trimmedGroup)
}

func buildSNSARN(trimmedGroup, logStream, accountID, region string) string {
	// sns/{region}/{account_id}/{topic_name}
	parts := strings.Split(trimmedGroup, "/")
	if len(parts) == 3 {
		return BuildServiceARN("sns", accountID, region, parts[2])
	}
	// as fallback sns/{topic_name}
	return BuildServiceARN("sns", accountID, region, trimmedGroup)
}

func buildGenericARN(logGroup, logStream, accountID, region string) ([]string, bool) {
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

func GetFunctionARNAndNameIfSourceIsLambda(logGroup, accountID, region string) (string, string, bool) {
	if service, resourceName, ok := findSourceFromLogGroup(logGroup); ok && service == "lambda" {
		return BuildServiceARN(service, accountID, region, resourceName), resourceName, true
	}
	return "", "", false
}

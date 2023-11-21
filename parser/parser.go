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

func buildSagemakerARN(trimmedGroup, logStream, accountID, region string) string {
	containsFunc := func(str string) bool {
		return strings.Contains(trimmedGroup, str)
	}
	groupParts := strings.Split(trimmedGroup, "/")
	streamParts := strings.Split(logStream, "/")
	buildServiceArnFunc := func(suffix string) string {
		return BuildResourceARN("sagemaker", accountID, region, suffix)
	}

	if containsFunc("CompilationJobs") {
		return buildServiceArnFunc(fmt.Sprintf("compilation-job/%s", logStream))
	}
	if containsFunc("Endpoints") && len(groupParts) == 2 {
		return buildServiceArnFunc(fmt.Sprintf("endpoint/%s", groupParts[1]))
	}
	if containsFunc("LabelingJobs") {
		return buildServiceArnFunc(fmt.Sprintf("labeling-job/%s", streamParts[0]))
	}

	if len(streamParts) == 2 {
		if containsFunc("InferenceRecommendationsJobs") {
			return buildServiceArnFunc(fmt.Sprintf("inference-recommendations-job/%s", streamParts[0]))
		}
		if containsFunc("NotebookInstances") {
			return buildServiceArnFunc(fmt.Sprintf("notebook-instance/%s", streamParts[0]))
		}
		if containsFunc("ProcessingJobs") {
			return buildServiceArnFunc(fmt.Sprintf("processing-job/%s", streamParts[0]))
		}
		if containsFunc("TrainingJobs") {
			return buildServiceArnFunc(fmt.Sprintf("training-job/%s", streamParts[0]))
		}
	}

	// as fallback sagemaker/{resource_name} from log group
	return buildServiceArnFunc(trimmedGroup)
}

func buildEC2ARN(trimmedGroup, accountID, region string) string {
	// expect vpc/{vpc_id} or instance/{instance_id}
	parts := strings.Split(trimmedGroup, "/")
	if len(parts) == 2 {
		return BuildResourceARN("ec2", accountID, region, fmt.Sprintf("%s/%s", parts[0], parts[1]))
	}

	// as fallback assume logs are from an ec2 instance and use ec2/{instance_id}
	return BuildResourceARN("ec2", accountID, region, fmt.Sprintf("instance/%s", trimmedGroup))
}

func buildECSARNs(trimmedGroup, logStream, accountID, region string) []string {
	groupParts := strings.Split(trimmedGroup, "/")
	streamParts := strings.Split(logStream, "/")

	arns := make([]string, 0)
	if len(streamParts) == 3 {
		arns = append(arns, BuildResourceARN("ecs", accountID, region, fmt.Sprintf("task/%s/%s", groupParts[0], streamParts[2])))
	}

	arns = append(arns, BuildResourceARN("ecs", accountID, region, fmt.Sprintf("cluster/%s", groupParts[0])))
	if len(groupParts) == 2 {
		arns = append(arns, BuildResourceARN("ecs", accountID, region, fmt.Sprintf("service/%s/%s", groupParts[0], groupParts[1])))
	}

	// as fallback also add ecs/{resource_name}
	arns = append(arns, BuildResourceARN("ecs", accountID, region, trimmedGroup))

	return arns
}

func buildSNSARN(trimmedGroup, accountID, region string) string {
	// {region}/{account_id}/{topic_name}
	parts := strings.Split(trimmedGroup, "/")
	if len(parts) == 3 {
		return BuildResourceARN("sns", accountID, region, parts[2])
	}
	// {region}/{account_id}/{topic_name}/Failure
	if len(parts) == 4 {
		return BuildResourceARN("sns", accountID, region, parts[2])
	}
	// as fallback sns/{topic_name}
	return BuildResourceARN("sns", accountID, region, trimmedGroup)
}

func findSourceFromLogGroup(logGroup string) (string, string, bool) {
	trimPrefixFunc := func(prefix string) string {
		return strings.TrimPrefix(logGroup, prefix)
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

func buildGenericARN(logGroup, accountID, region string) ([]string, bool) {
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
		return []string{BuildResourceARN(service, accountID, region, resourceName)}, true
	}

	var arns []string
	for _, suffix := range suffixes {
		arns = append(arns, BuildResourceARN(service, accountID, region, suffix+resourceName))
	}
	return arns, true
}

func GetSourceARNsFromLogGroup(accountID, region, logGroup, logStream string) ([]string, bool) {
	trimPrefixFunc := func(prefix string) string {
		return strings.TrimPrefix(logGroup, prefix)
	}
	hasPrefixFunc := func(prefix string) bool {
		return strings.HasPrefix(logGroup, prefix)
	}

	if hasPrefixFunc("/aws/sagemaker/") {
		return []string{buildSagemakerARN(trimPrefixFunc("/aws/sagemaker/"), logStream, accountID, region)}, true
	}
	if hasPrefixFunc("sns/") {
		return []string{buildSNSARN(trimPrefixFunc("sns/"), accountID, region)}, true
	}
	if hasPrefixFunc("/ecs/") {
		return buildECSARNs(trimPrefixFunc("/ecs/"), logStream, accountID, region), true
	}
	if hasPrefixFunc("/ec2/") {
		return []string{buildEC2ARN(trimPrefixFunc("/ec2/"), accountID, region)}, true
	}

	return buildGenericARN(logGroup, accountID, region)
}

func BuildResourceARN(service, accountID, region, resource string) string {
	return fmt.Sprintf("arn:aws:%s:%s:%s:%s", service, region, accountID, resource)
}

func GetFunctionARNAndNameIfSourceIsLambda(logGroup, accountID, region string) (string, string, bool) {
	if service, resourceName, ok := findSourceFromLogGroup(logGroup); ok && service == "lambda" {
		return BuildResourceARN(service, accountID, region, resourceName), resourceName, true
	}
	return "", "", false
}

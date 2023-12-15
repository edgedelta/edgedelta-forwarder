package parser

import (
	"fmt"
	"strings"

	"github.com/edgedelta/edgedelta-forwarder/tag"
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

func buildSagemakerARN(trimmedGroup, logStream, accountID, region string) tag.ServiceInfo {
	containsFunc := func(str string) bool {
		return strings.Contains(trimmedGroup, str)
	}
	groupParts := strings.Split(trimmedGroup, "/")
	streamParts := strings.Split(logStream, "/")
	buildServiceArnFunc := func(suffix string) string {
		return BuildResourceARN("sagemaker", accountID, region, suffix)
	}

	service := tag.ServiceInfo{
		Name: tag.SourceSagemaker,
	}

	if containsFunc("CompilationJobs") {
		service.ARN = buildServiceArnFunc(fmt.Sprintf("compilation-job/%s", logStream))
	} else if containsFunc("Endpoints") && len(groupParts) == 2 {
		service.ARN = buildServiceArnFunc(fmt.Sprintf("endpoint/%s", groupParts[1]))
	} else if containsFunc("LabelingJobs") {
		service.ARN = buildServiceArnFunc(fmt.Sprintf("labeling-job/%s", streamParts[0]))
	} else if len(streamParts) == 2 {
		if containsFunc("InferenceRecommendationsJobs") {
			service.ARN = buildServiceArnFunc(fmt.Sprintf("inference-recommendations-job/%s", streamParts[0]))
		} else if containsFunc("NotebookInstances") {
			service.ARN = buildServiceArnFunc(fmt.Sprintf("notebook-instance/%s", streamParts[0]))
		} else if containsFunc("ProcessingJobs") {
			service.ARN = buildServiceArnFunc(fmt.Sprintf("processing-job/%s", streamParts[0]))
		} else if containsFunc("TrainingJobs") {
			service.ARN = buildServiceArnFunc(fmt.Sprintf("training-job/%s", streamParts[0]))
		}
	}

	// as fallback sagemaker/{resource_name} from log group
	if service.ARN == "" {
		service.ARN = buildServiceArnFunc(trimmedGroup)
	}

	return service
}

func buildEC2ARN(trimmedGroup, accountID, region string) tag.ServiceInfo {
	// expect vpc/{vpc_id} or instance/{instance_id}
	parts := strings.Split(trimmedGroup, "/")
	if len(parts) == 2 {
		return tag.ServiceInfo{
			Name: tag.SourceEC2,
			ARN:  BuildResourceARN("ec2", accountID, region, fmt.Sprintf("%s/%s", parts[0], parts[1])),
		}
	}

	// as fallback assume logs are from an ec2 instance and use ec2/{instance_id}
	return tag.ServiceInfo{
		Name: tag.SourceEC2,
		ARN:  BuildResourceARN("ec2", accountID, region, fmt.Sprintf("instance/%s", trimmedGroup)),
	}
}

func buildECSARNs(trimmedGroup, logStream, accountID, region string) []tag.ServiceInfo {
	groupParts := strings.Split(trimmedGroup, "/")
	streamParts := strings.Split(logStream, "/")

	services := make([]tag.ServiceInfo, 0)
	if len(streamParts) == 3 {
		services = append(services, tag.ServiceInfo{
			Name: tag.SourceECSTask,
			ARN:  BuildResourceARN("ecs", accountID, region, fmt.Sprintf("task/%s/%s", groupParts[0], streamParts[2])),
		})
	}

	services = append(services, tag.ServiceInfo{
		Name: tag.SourceECSCluster,
		ARN:  BuildResourceARN("ecs", accountID, region, fmt.Sprintf("cluster/%s", groupParts[0])),
	})

	if len(groupParts) == 2 {
		services = append(services, tag.ServiceInfo{
			Name: tag.SourceECSService,
			ARN:  BuildResourceARN("ecs", accountID, region, fmt.Sprintf("service/%s/%s", groupParts[0], groupParts[1])),
		})
	}

	return services
}

func buildSNSARN(trimmedGroup, accountID, region string) tag.ServiceInfo {
	parts := strings.Split(trimmedGroup, "/")
	service := tag.ServiceInfo{
		Name: tag.SourceSNS,
		ARN:  BuildResourceARN("sns", accountID, region, trimmedGroup), // fallback if not parsed
	}

	// {region}/{account_id}/{topic_name}/Failure or {region}/{account_id}/{topic_name}
	if len(parts) == 3 || len(parts) == 4 {
		service.ARN = BuildResourceARN("sns", accountID, region, parts[2])
	}

	return service
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

func buildGenericARN(logGroup, accountID, region string) ([]tag.ServiceInfo, bool) {
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
		return []tag.ServiceInfo{
			{
				Name: tag.Source(service),
				ARN:  BuildResourceARN(service, accountID, region, resourceName),
			},
		}, true
	}

	var services []tag.ServiceInfo
	for _, suffix := range suffixes {
		services = append(services, tag.ServiceInfo{
			Name: tag.Source(service),
			ARN:  BuildResourceARN(service, accountID, region, suffix+resourceName),
		})
	}

	return services, true
}

func GetSourceARNsFromLogGroup(accountID, region, logGroup, logStream string) ([]tag.ServiceInfo, bool) {
	trimPrefixFunc := func(prefix string) string {
		return strings.TrimPrefix(logGroup, prefix)
	}
	hasPrefixFunc := func(prefix string) bool {
		return strings.HasPrefix(logGroup, prefix)
	}

	if hasPrefixFunc("/aws/sagemaker/") {
		return []tag.ServiceInfo{buildSagemakerARN(trimPrefixFunc("/aws/sagemaker/"), logStream, accountID, region)}, true
	}
	if hasPrefixFunc("sns/") {
		return []tag.ServiceInfo{buildSNSARN(trimPrefixFunc("sns/"), accountID, region)}, true
	}
	if hasPrefixFunc("/ecs/") {
		return buildECSARNs(trimPrefixFunc("/ecs/"), logStream, accountID, region), true
	}
	if hasPrefixFunc("/ec2/") {
		return []tag.ServiceInfo{buildEC2ARN(trimPrefixFunc("/ec2/"), accountID, region)}, true
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

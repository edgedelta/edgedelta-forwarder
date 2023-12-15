package tag

type ServiceInfo struct {
	Name Source
	ARN  string
}

type Source string

const (
	SourceForwarder  Source = "ed_forwarder"
	SourceLogGroup   Source = "log_group"
	SourceSagemaker  Source = "sagemaker"
	SourceECSTask    Source = "ecs_task"
	SourceECSCluster Source = "ecs_cluster"
	SourceECSService Source = "ecs_service"
	SourceEC2        Source = "ec2"
	SourceSNS        Source = "sns"
)

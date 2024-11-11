# edgedelta-forwarder
AWS lambda function to forward logs from AWS to Edge Delta agent.


## Environment Variables

- ED_ENDPOINT: Edge Delta hosted agent endpoint. (Required)
- ED_FORWARD_SOURCE_TAGS: If set to true, source log group's tags are fetched. Forwarder tries to build ARN of the source by using log group's name. Requires "tag:GetResources" permission. 
    This only works if the log group name is in the correct format (i.e. /aws/lambda/<lambda_name>).
- ED_FORWARD_FORWARDER_TAGS: If set to true, forwarder lambda's own tags are fetched. Requires "tag:GetResources" permission.
- ED_FORWARD_LOG_GROUP_TAGS: If set to true, log group tags are fetched. Requires "tag:GetResources" permission.
- ED_PUSH_TIMEOUT_SEC: Push timeout is the total duration of waiting for to send one batch of logs (in seconds). Default is 10.
- ED_BATCH_SIZE: Batch size is the max allowed size of data to send in one batch. The default (and also the maximum) batch size is 1MB, minimum is 50KB.
- ED_RETRY_INTERVAL_MS: RetryInterval is the initial interval to wait until next retry (in milliseconds). It is increased exponentially until our process is shut down. Default is 100.
- ED_SOURCE_TAG_PREFIXES: Comma separated list of tag prefixes to be added to the source tags. For example, if ED_SOURCE_TAG_PREFIXES has "ed_forwarder=ed_fwd_", then all the forwarder tags will be prefixed with "ed_fwd_". Default is empty. Refer to mapping below for the list of tags keys.


## Manual Build

### Step 1 - Build executable
```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
```
Executable’s name must be “bootstrap”

### Step 2 - Zip
```
zip "<zipped_forwarder_lambda_path>" bootstrap
```

### Step 3 - Create IAM Role for lambda
If lambda role has "lambda:GetFunction" permission, forwarder will also get lambda runtime, architectures, memory size, and version from lambda function.

### Step 4 - Create Lambda function
```
aws lambda create-function \
    --function-name "<name_of_the_forwarder_lambda>" \
    --runtime provided.al2 \
    --handler bootstrap \
    --role "<forwarder_lambda_role_arn>" \
    --zip-file "fileb://<zipped_forwarder_lambda_path>"
```

### Step 5 - Allow AWS Cloudwatch to invoke the lambda function
```
aws lambda add-permission \
    --function-name “<name_of_the_forwarder_lambda>” \
    --statement-id “<sid_for_policy>” \
    --principal “logs.amazonaws.com” \
    --action “lambda:InvokeFunction” \
    --source-arn “<arn_of_the_log_group_you_want_to_consume>” \
    --source-account ”<aws_account_id>”
```

### Step 6 - Setup AWS Cloudwatch subscription
```
aws logs put-subscription-filter \
    --log-group-name “<the_log_group_you_want_to_consume>” \
    --filter-name “<name_of_the_filter_just_for_display_purpose>” \
    --filter-pattern “<filter_pattern_for_logs_if_needed_to_send_logs_matching_with_pattern>” \
    --destination-arn “<arn_of_the_forwarder_lambda>”
```

## Log Format

Forwarder lambda function sends logs in the following format:
```
{
    "cloud": {
        "resource_id": "<arn_of_the_forwarder_lambda>"
        "account_id": "<account_id_of_the_log_group>",
        "region": "<region_of_the_log_group>"
    },
    "faas":{
        "name":"<name_of_the_forwarder_lambda>",
        "version":"<version_of_the_forwarder_lambda>",
        "request_id":"<request_id_of_the_forwarder_lambda>",
        "memory_size":<memory_size_of_the_forwarder_lambda>,
        "tags": {
        <Populated with the tags of the lambda function if ED_FORWARD_FORWARDER_TAGS is set to true or ED_FORWARD_SOURCE_TAGS is true and source is lambda>
        }
    },
    "aws": {
        "log.group.name": "<Cloudwatch_log_group_name>",
        "log.group.arn": "<Cloudwatch_log_group_arn>",
        "log.group.tags": {
            <Populated with the tags of the log group if ED_FORWARD_LOG_GROUP_TAGS is set to true>
        },
        "log.group.message_type": "<message_type_of_the_log_group>",
        "log.group.subscription_filters": [
            "<subscription_filter_1>",
            ...
        ],
        "log.stream.name": "<Cloudwatch_log_stream_name>",
        "service.tags": {
            <Populated with the tags of the log group if ED_FORWARD_SOURCE_TAGS is set to true>
        },

    },
    "logEvents":[
       {
            "id":"<log_id>",
            "timestamp":<timestamp>,
            "message":"<log_message>"
        },
        ...
    ]
}
```

## Source Tags Prefix Mapping
- Edge Delta Forwarder: ed_forwarder
- Cloudwatch Log Group: log_group
- Lambda: lambda
- Sagemaker: sagemaker
- ECS Task: ecs_task
- ECS Cluster: ecs_cluster
- ECS Service: ecs_service
- EC2: ec2
- SNS: sns
... 
The rest of the tag prefix keys are the same with the Amazon service name. For example, if the source is EKS, then the tag prefix key is eks. Thus, to prefix EKS tags ED_SOURCE_TAG_PREFIXES should have "eks=eks_prefix_".

Example:
```
ED_SOURCE_TAG_PREFIXES=ed_forwarder=ed_fwd_,lambda=lambda_prefix_,sagemaker=sagemaker_prefix_
```
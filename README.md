# edgedelta-forwarder
AWS lambda function to forward logs from AWS to Edge Delta agent.


## Environment Variables

- ED_ENDPOINT: Edge Delta hosted agent endpoint. (Required)

- ED_FORWARD_SOURCE_TAGS: If set to true, source log group's tags are fetched. Forwarder tries to build ARN of the source by using log group's name. Requires "tag.GetResources" permission.
    This only works if the log group name is in the correct format (i.e. /aws/lambda/<lambda_name>).
- ED_FORWARD_FORWARDER_TAGS: If set to true, forwarder lambda's own tags are fetched. Requires "tag.GetResources" permission.
- ED_FORWARD_LOG_GROUP_TAGS: If set to true, log group tags are fetched. Requires "tag.GetResources" permission.
- ED_PUSH_TIMEOUT_SEC: Push timeout is the total duration of waiting for to send one batch of logs (in seconds). Default is 10.
- ED_RETRY_INTERVAL_MS: RetryInterval is the initial interval to wait until next retry (in milliseconds). It is increased exponentially until our process is shut down. Default is 100.


## Manuel Build

### Build executable: 
```
GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap main.go
```
Executable’s name must be “bootstrap”

### Zip:
```
zip "<zipped_forwarder_lambda_path>" bootstrap
```
### Create IAM Role for lambda:

### Create Lambda function:
```
aws lambda create-function \
    --function-name "<name_of_the_forwarder_lambda>" \
    --runtime provided.al2 \
    --handler bootstrap \
    --role "<forwarder_lambda_role_arn>" \
    --zip-file "fileb://<zipped_forwarder_lambda_path>"
```

### Allow AWS Cloudwatch to invoke the lambda function:
```
aws lambda add-permission \
    --function-name “<name_of_the_forwarder_lambda>” \
    --statement-id “<sid_for_policy>” \
    --principal “logs.amazonaws.com” \
    --action “lambda:InvokeFunction” \
    --source-arn “<arn_of_the_log_group_you_want_to_consume>” \
    --source-account ”<aws_account_id>”
```
### Setup AWS Cloudwatch subscription:
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
    },
    "faas":{
        "name":"<name_of_the_forwarder_lambda>",
        "version":"<version_of_the_forwarder_lambda>"},
    },
    "owner":"<account_id_of_the_log_group>",
    "logGroup":"<Cloudwatch_log_group_name>",
    "logStream":"<Cloudwatch_log_stream_name>",
    "subscriptionFilters":[
        <subscription_filter_name>"
    ],
    "messageType":"<message_type>",    // i.e. "DATA_MESSAGE"
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

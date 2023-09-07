package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/push"
	"github.com/edgedelta/edgedelta-forwarder/resource"
)

const lambdaLogGroupPrefix = "/aws/lambda/"

type faas struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cloud struct {
	ResourceID string `json:"resource_id"`
}

type common struct {
	Cloud      *cloud            `json:"cloud"`
	Faas       *faas             `json:"faas"`
	LambdaTags map[string]string `json:"lambda_tags,omitempty"`
}

var (
	region                 string
	config                 *cfg.Config
	pusher                 *push.Pusher
	resourceARNToTagsCache map[string]map[string]string
	resourceCl             *resource.DefaultClient
)

type logsData events.CloudwatchLogsData

type edLog struct {
	common
	logsData
}

type HandlerFn func(context.Context, events.CloudwatchLogsEvent) error

func withGracefulShutdown(handler HandlerFn, gracePeriod time.Duration) HandlerFn {
	return func(ctx context.Context, logsEvent events.CloudwatchLogsEvent) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			return handler(ctx, logsEvent)
		}
		shorterDeadline := deadline.Add(-gracePeriod)
		graceCtx, cancel := context.WithDeadline(ctx, shorterDeadline)
		defer cancel()
		return handler(graceCtx, logsEvent)
	}
}

func main() {
	lambda.Start(withGracefulShutdown(handleRequest, time.Second*5))
}

func init() {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		log.Fatalf("Failed to get AWS region from environment")
	}
	c, err := cfg.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get config from environment variables, err: %v", err)
	}
	config = c
	resCl, err := resource.NewAWSClient()
	if err != nil {
		log.Fatalf("Failed to create AWS resourcegroupstaggingapi client, err: %v", err)
	}
	resourceCl = resCl
	resourceARNToTagsCache = make(map[string]map[string]string)
	pusher = push.NewPusher(config)
}

func buildFunctionARN(functionName, accountID string) string {
	return fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", region, accountID, functionName)
}

func getFunctionName(logGroup string) (string, bool) {
	name := strings.TrimPrefix(logGroup, lambdaLogGroupPrefix)
	if len(name) == len(logGroup) {
		return "", false
	}
	return name, true
}

func handleRequest(ctx context.Context, logsEvent events.CloudwatchLogsEvent) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovering from panic in handleRequest, err: %v", r)
		}
	}()
	var functionARN string
	lc, ok := lambdacontext.FromContext(ctx)
	if !ok {
		log.Printf("Failed to create lambda context")
	} else {
		functionARN = lc.InvokedFunctionArn
	}
	data, err := logsEvent.AWSLogs.Parse()
	if err != nil {
		log.Printf("Failed to parse logs event, err: %v", err)
		return err
	}
	functionName := lambdacontext.FunctionName
	functionVersion := lambdacontext.FunctionVersion
	foundLambdaLogGroup := false
	if name, ok := getFunctionName(data.LogGroup); ok {
		foundLambdaLogGroup = true
		functionName = name
		functionVersion = ""
		functionARN = buildFunctionARN(name, data.Owner)
	}
	if foundLambdaLogGroup && config.ForwardLambdaTags {
		tagsMap, err := resourceCl.GetResourceTags(ctx)
		if err != nil {
			log.Printf("Failed to get resource tags for ARN: %s, err: %v", functionARN, err)
		} else if len(tagsMap) == 0 {
			log.Printf("Failed to find tags for ARN: %s", functionARN)
		} else {
			for r, t := range tagsMap {
				resourceARNToTagsCache[r] = t
			}
		}
	}

	edLog := &edLog{
		common: common{
			Cloud: &cloud{ResourceID: functionARN},
			Faas: &faas{
				Name:    functionName,
				Version: functionVersion,
			},
			LambdaTags: resourceARNToTagsCache[functionARN],
		},
		logsData: logsData(data),
	}
	b, err := json.Marshal(edLog)
	if err != nil {
		log.Printf("Failed to marshal logs, err: %v", err)
		return err
	}
	// blocks until context deadline
	if err := pusher.Push(ctx, b); err != nil {
		return err
	}
	log.Printf("Successfully pushed logs")
	return nil
}

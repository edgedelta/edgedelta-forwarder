package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/push"
)

type faas struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cloud struct {
	ResourceID string `json:"resource_id"`
}

type common struct {
	Cloud *cloud `json:"cloud"`
	Faas  *faas  `json:"faas"`
}

var (
	pusher *push.Pusher
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
	config, err := cfg.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get config from environment variables, err: %v", err)
	}
	pusher = push.NewPusher(config)
}

func handleRequest(ctx context.Context, logsEvent events.CloudwatchLogsEvent) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovering from panic in handleRequest, err: %v", r)
		}
	}()
	var functionArn string
	lc, ok := lambdacontext.FromContext(ctx)
	if !ok {
		log.Printf("Failed to create lambda context")
	} else {
		functionArn = lc.InvokedFunctionArn
	}
	data, err := logsEvent.AWSLogs.Parse()
	if err != nil {
		log.Printf("Failed to parse logs event, err: %v", err)
		return err
	}
	edLog := &edLog{
		common: common{
			Cloud: &cloud{ResourceID: functionArn},
			Faas: &faas{
				Name:    lambdacontext.FunctionName,
				Version: lambdacontext.FunctionVersion,
			},
		},
		logsData: logsData(data),
	}
	b, err := json.Marshal(edLog)
	if err != nil {
		log.Printf("Failed to marshal logs data, err: %v", err)
		return err
	}
	// blocks until context deadline
	if err := pusher.Push(ctx, b); err != nil {
		return err
	}
	log.Printf("Successfully pushed logs")
	return nil
}

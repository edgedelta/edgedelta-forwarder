package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/enrich"
	"github.com/edgedelta/edgedelta-forwarder/push"
	"github.com/edgedelta/edgedelta-forwarder/resource"

	lambdaCl "github.com/edgedelta/edgedelta-forwarder/lambda"
)

var (
	config   *cfg.Config
	pusher   *push.Pusher
	enricher *enrich.Enricher
)

type edCommon enrich.Common

type edLogsData struct {
	SubscriptionFilters []string                        `json:"subscriptionFilters"`
	MessageType         string                          `json:"messageType"`
	LogEvents           []events.CloudwatchLogsLogEvent `json:"logEvents"`
}

type edLog struct {
	edCommon
	edLogsData
}

type HandlerFn func(context.Context, events.CloudwatchLogsEvent) error

func withGracefulShutdown(handler HandlerFn, gracePeriod time.Duration) HandlerFn {
	return func(ctx context.Context, logsEvent events.CloudwatchLogsEvent) error {
		deadline, ok := ctx.Deadline()
		log.Printf("Deadline: %v, ok: %v", deadline, ok)
		if !ok {
			log.Printf("No deadline set, running handler without graceful shutdown")
			return handler(ctx, logsEvent)
		}
		shorterDeadline := deadline.Add(-gracePeriod)
		log.Printf("Shorter deadline: %v", shorterDeadline)
		graceCtx, cancel := context.WithDeadline(ctx, shorterDeadline)
		defer cancel()
		log.Printf("Running handler with graceful shutdown")
		return handler(graceCtx, logsEvent)
	}
}

func main() {
	lambda.Start(withGracefulShutdown(handleRequest, time.Second*5))
}

func init() {
	c, err := cfg.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get config from environment variables, err: %v", err)
	}
	config = c
	resCl, err := resource.NewAWSClient()
	if err != nil {
		log.Fatalf("Failed to create AWS resourcegroupstaggingapi client, err: %v", err)
	}
	lambdaClient, err := lambdaCl.NewClient(config.Region)
	if err != nil {
		log.Fatalf("Failed to create AWS lambda client, err: %v", err)
	}
	enricher = enrich.NewEnricher(config, resCl, lambdaClient)

	pusher = push.NewPusher(config)
}

func handleRequest(ctx context.Context, logsEvent events.CloudwatchLogsEvent) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovering from panic in handleRequest, err: %v", r)
		}
	}()

	data, err := logsEvent.AWSLogs.Parse()
	if err != nil {
		log.Printf("Failed to parse logs event, err: %v", err)
		return err
	}
	common := enricher.GetEDCommon(ctx, data.LogGroup, data.LogStream, data.Owner)

	edLog := &edLog{
		edCommon: edCommon(*common),
		edLogsData: edLogsData{
			SubscriptionFilters: data.SubscriptionFilters,
			MessageType:         data.MessageType,
			LogEvents:           data.LogEvents,
		},
	}

	b, err := json.Marshal(edLog)
	if err != nil {
		log.Printf("Failed to marshal logs, err: %v", err)
		return err
	}
	log.Printf("Sending %d bytes of logs", len(b))
	// blocks until context deadline
	if err := pusher.Push(ctx, b); err != nil {
		return err
	}
	log.Printf("Successfully pushed logs")
	return nil
}

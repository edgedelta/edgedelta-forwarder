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
)

var (
	config   *cfg.Config
	pusher   *push.Pusher
	enricher *enrich.Enricher
)

type edLogsData events.CloudwatchLogsData
type edCommon enrich.Common

type edLog struct {
	edCommon
	edLogsData
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
	c, err := cfg.GetConfig()
	if err != nil {
		log.Fatalf("Failed to get config from environment variables, err: %v", err)
	}
	config = c
	resCl, err := resource.NewAWSClient()
	if err != nil {
		log.Fatalf("Failed to create AWS resourcegroupstaggingapi client, err: %v", err)
	}
	enricher = enrich.NewEnricher(config, resCl)

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
	common := enricher.GetEDCommon(ctx, data.LogGroup, data.Owner)

	edLog := &edLog{
		edCommon:   edCommon(*common),
		edLogsData: edLogsData(data),
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

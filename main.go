package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/chunker"
	"github.com/edgedelta/edgedelta-forwarder/core"
	"github.com/edgedelta/edgedelta-forwarder/ecs"
	"github.com/edgedelta/edgedelta-forwarder/enrich"
	"github.com/edgedelta/edgedelta-forwarder/push"
	"github.com/edgedelta/edgedelta-forwarder/resource"

	lambdaCl "github.com/edgedelta/edgedelta-forwarder/lambda"
)

var (
	config     *cfg.Config
	pusher     *push.Pusher
	enricher   *enrich.Enricher
	logChunker *chunker.Chunker
)

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
	lambdaClient, err := lambdaCl.NewClient(config.Region)
	if err != nil {
		log.Fatalf("Failed to create AWS lambda client, err: %v", err)
	}
	ecsClient, err := ecs.NewClient()
	if err != nil {
		log.Fatalf("Failed to create AWS ECS client, err: %v", err)
	}

	enricher = enrich.NewEnricher(config, resCl, lambdaClient, ecsClient)
	enricher.StartECSContainerCacheCleanup()

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
	common := enricher.GetEDCommon(ctx, data.SubscriptionFilters, data.MessageType, data.LogGroup, data.LogStream, data.Owner)

	edLog := &core.Log{
		Common: core.Common(*common),
		Data: core.Data{
			LogEvents: data.LogEvents,
		},
	}

	logChunker, err := chunker.NewChunker(config.BatchSize, edLog)

	chunks, err := logChunker.ChunkLogs()
	if err != nil {
		log.Printf("Failed to chunk logs, err: %v", err)
		return err
	}

	for i, chunk := range chunks {
		log.Printf("Sending chunk %d of %d, size: %d bytes", i+1, len(chunks), len(chunk))
		// blocks until context deadline
		if err := pusher.Push(ctx, chunk); err != nil {
			log.Printf("Failed to push chunk %d, err: %v", i+1, err)
			return err
		}
	}

	log.Printf("Successfully pushed %d log chunks", len(chunks))
	return nil
}

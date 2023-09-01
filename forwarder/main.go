package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/edgedelta/edgedelta-forwarder/forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/forwarder/push"
)

var (
	pusher *push.Pusher
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

	logsData, err := logsEvent.AWSLogs.Parse()
	if err != nil {
		log.Printf("Failed to parse logs event, err: %v", err)
	}
	var buf bytes.Buffer
	for _, event := range logsData.LogEvents {
		b, err := json.Marshal(event)
		if err != nil {
			log.Printf("Failed to marshal log event: %v, err: %v", event, err)
			continue
		}
		buf.Write(b)
		buf.WriteRune('\n')
	}
	// blocks until context deadline
	return pusher.Push(ctx, &buf)
}

package edlog

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/edgedelta/edgedelta-forwarder/enrich"
)

type Common enrich.Common

type Data struct {
	LogEvents []events.CloudwatchLogsLogEvent `json:"logEvents"`
}

type Log struct {
	Common
	Data
}

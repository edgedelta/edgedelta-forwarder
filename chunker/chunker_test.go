package chunker

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/core"
)

func TestChunkLogs(t *testing.T) {
	tests := []struct {
		name               string
		chunkSize          int
		common             core.Common
		logEvents          []events.CloudwatchLogsLogEvent
		expectedChunks     int
		chunksShouldExceed []bool
		exceptsError       bool
	}{
		{
			name:               "Empty log",
			chunkSize:          1024,
			common:             core.Common{},
			logEvents:          []events.CloudwatchLogsLogEvent{},
			expectedChunks:     1,
			chunksShouldExceed: []bool{false},
		},
		{
			name:      "empty log but common data exceeds chunk size",
			chunkSize: 50,
			common: core.Common{ // common object size is 63 bytes
				HostArchitecture: "test arch 1",
			},
			logEvents:    []events.CloudwatchLogsLogEvent{},
			exceptsError: true,
		},
		{
			name:      "Single small log event",
			chunkSize: 1024,
			common: core.Common{ // common object size is 63 bytes
				HostArchitecture: "test arch 1",
			},
			logEvents:          []events.CloudwatchLogsLogEvent{{Message: "Small log"}},
			expectedChunks:     1,
			chunksShouldExceed: []bool{false},
		},
		{
			name:      "Multiple log events within chunk size",
			chunkSize: 1024,
			common: core.Common{ // common object size is 100 bytes
				HostArchitecture:   "test arch 2",
				ProcessRuntimeName: "test source",
			},
			logEvents: []events.CloudwatchLogsLogEvent{
				{Message: "Log 1"},
				{Message: "Log 2"},
				{Message: "Log 3"},
			},
			expectedChunks:     1,
			chunksShouldExceed: []bool{false},
		},
		{
			name:      "Log events exceeding chunk size",
			chunkSize: 128,
			common: core.Common{ // common object size is 105 bytes
				HostArchitecture:   "test arch 3",
				ProcessRuntimeName: "test-container-1",
			},
			logEvents: []events.CloudwatchLogsLogEvent{
				{Message: "This is a long log message that will exceed the chunk size"},
				{Message: "Another long log message to ensure multiple chunks"},
			},
			expectedChunks:     2,
			chunksShouldExceed: []bool{true, true}, // Both chunks should exceed the chunk size
		},
		{
			name:      "Many small log events",
			chunkSize: 256,
			common: core.Common{ // common object size is 99 bytes
				HostArchitecture:   "test arch 4",
				ProcessRuntimeName: "test-pod-1",
			},
			logEvents:          generateLogEvents(100, 10), // 100 events of 10 bytes each
			expectedChunks:     100,
			chunksShouldExceed: make([]bool, 100),
		},
		{
			name:      "Single large log event exceeding max chunk size",
			chunkSize: cfg.MaxChunkSize,
			common: core.Common{ // common object size is 103 bytes
				HostArchitecture:   "test arch 5",
				ProcessRuntimeName: "test-namespace",
			},
			logEvents:          []events.CloudwatchLogsLogEvent{{Message: string(make([]byte, cfg.MaxChunkSize))}},
			expectedChunks:     1,
			chunksShouldExceed: []bool{true},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			log := &core.Log{
				Common: tt.common,
				Data:   core.Data{LogEvents: tt.logEvents},
			}
			chunker, err := NewChunker(tt.chunkSize, log)
			if err != nil {
				if !tt.exceptsError {
					t.Errorf("Failed to create chunker: %v", err)
				}
				return
			}

			chunks, err := chunker.ChunkLogs()
			if err != nil {
				t.Errorf("Failed to chunk logs: %v", err)
			}

			if len(chunks) != tt.expectedChunks {
				t.Errorf("Expected %d chunks, but got %d", tt.expectedChunks, len(chunks))
			}

			// Verify each chunk
			totalEvents := 0
			for i, chunk := range chunks {
				var decodedLog core.Log
				if err := json.Unmarshal(chunk, &decodedLog); err != nil {
					t.Errorf("Failed to unmarshal chunk %d: %v", i, err)
					continue
				}
				if !reflect.DeepEqual(tt.common, decodedLog.Common) {
					t.Errorf("Common data mismatch in chunk %d", i)
				}
				gotSize := len(chunk)

				// Verify log events
				if tt.chunksShouldExceed[i] && gotSize <= tt.chunkSize {
					t.Errorf("Chunk %d size should exceed chunk size, max chunk size: %d, got %d bytes", i, tt.chunkSize, gotSize)
				} else if !tt.chunksShouldExceed[i] && gotSize > tt.chunkSize {
					t.Errorf("Chunk %d size should not exceed chunk size, max chunk size: %d, got %d bytes", i, tt.chunkSize, gotSize)
				}

				totalEvents += len(decodedLog.Data.LogEvents)
			}

			// Verify total number of log events
			if totalEvents != len(tt.logEvents) {
				t.Errorf("Total number of log events mismatch: expected %d, got %d", len(tt.logEvents), totalEvents)
			}
		})
	}
}

// Helper function to generate multiple log events
func generateLogEvents(count, size int) []events.CloudwatchLogsLogEvent {
	logEvents := make([]events.CloudwatchLogsLogEvent, count)
	for i := 0; i < count; i++ {
		logEvents[i] = events.CloudwatchLogsLogEvent{Message: string(make([]byte, size))}
	}
	return logEvents
}

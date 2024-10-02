package chunker

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/edgedelta/edgedelta-forwarder/edlog"
)

func TestChunkLogs(t *testing.T) {
	chunker := NewChunker(125) // Small size for testing

	tests := []struct {
		name           string
		input          *edlog.Log
		expectedChunks int
		expectOverflow bool
	}{
		{
			name: "Single small log",
			input: &edlog.Log{
				Common: edlog.Common{
					HostArchitecture: "test arch 1",
				},
				Data: edlog.Data{
					LogEvents: []events.CloudwatchLogsLogEvent{
						{Message: "Short log"},
					},
				},
			},
			expectedChunks: 1,
			expectOverflow: false,
		},
		{
			name: "Multiple logs requiring chunks",
			input: &edlog.Log{
				Common: edlog.Common{
					HostArchitecture: "test arch 2",
				},
				Data: edlog.Data{
					LogEvents: []events.CloudwatchLogsLogEvent{
						{Message: "Log 1"},
						{Message: "Log 2"},
						{Message: "Log 3"},
					},
				},
			},
			expectedChunks: 3,
			expectOverflow: false,
		},
		{
			name: "Log exceeding chunk size",
			input: &edlog.Log{
				Common: edlog.Common{
					HostArchitecture: "test arch 3",
				},
				Data: edlog.Data{
					LogEvents: []events.CloudwatchLogsLogEvent{
						{Message: "This is a very long log that exceeds the chunk size limit"},
					},
				},
			},
			expectedChunks: 1,
			expectOverflow: true,
		},
		{
			name: "Multiple logs exceeding chunk size",
			input: &edlog.Log{
				Common: edlog.Common{
					HostArchitecture: "test arch 4",
				},
				Data: edlog.Data{
					LogEvents: []events.CloudwatchLogsLogEvent{
						{Message: "Log 1"},
						{Message: "Log 2"},
						{Message: "Log 3"},
						{Message: "This is a very long log that exceeds the chunk size limit"},
					},
				},
			},
			expectedChunks: 2,
			expectOverflow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.ChunkLogs(tt.input)
			if err != nil {
				t.Errorf("Failed to chunk logs: %v", err)
			}

			if len(chunks) != tt.expectedChunks {
				t.Errorf("Expected %d chunks, got %d", tt.expectedChunks, len(chunks))
			}

			// Validate each chunk
			for i, chunk := range chunks {
				var log edlog.Log
				err := json.Unmarshal(chunk, &log)
				if err != nil {
					t.Errorf("Failed to unmarshal chunk %d: %v", i, err)
				}
				if log.Common != tt.input.Common {
					t.Errorf("Chunk %d: Common data mismatch", i)
				}

				t.Logf("Chunk %d size: %d bytes", i, len(chunk))

				if !tt.expectOverflow && len(chunk) > chunker.maxChunkSize {
					t.Errorf("Chunk %d exceeds max size: %d > %d", i, len(chunk), chunker.maxChunkSize)
				}
			}
		})
	}
}

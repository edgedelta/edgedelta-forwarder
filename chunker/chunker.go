package chunker

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/edgedelta/edgedelta-forwarder/edlog"
)

const MaxChunkSize = 1 * 1024 * 1024 // 1MB

type Chunker struct {
	maxChunkSize int
}

func NewChunker(maxChunkSize int) *Chunker {
	if maxChunkSize <= 0 || maxChunkSize > MaxChunkSize {
		maxChunkSize = MaxChunkSize
	}
	return &Chunker{maxChunkSize: maxChunkSize}
}

func (c *Chunker) ChunkLogs(log *edlog.Log) ([][]byte, error) {
	var chunks [][]byte
	var currentChunk edlog.Log
	currentChunk.Common = log.Common

	// Estimate the size of the common data
	commonJSON, err := json.Marshal(log.Common)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal common data: %w", err)
	}
	commonSize := len(commonJSON)
	currentSize := commonSize
	for _, logEvent := range log.LogEvents {
		logEventJSON, err := json.Marshal(logEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal log event: %w", err)
		}
		logEventSize := len(logEventJSON)

		currentChunk.LogEvents = append(currentChunk.LogEvents, logEvent)
		currentSize += logEventSize

		if currentSize+logEventSize > c.maxChunkSize {
			// Finalize current chunk
			chunkJSON, err := json.Marshal(currentChunk)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal chunk: %w", err)
			}
			chunks = append(chunks, chunkJSON)

			// Start a new chunk (without overwriting common data)
			currentChunk.LogEvents = []events.CloudwatchLogsLogEvent{}
			currentSize = commonSize
		}
	}

	// Add the last chunk if it's not empty
	if len(currentChunk.LogEvents) > 0 {
		chunkJSON, err := json.Marshal(currentChunk)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal final chunk: %w", err)
		}
		chunks = append(chunks, chunkJSON)
	}

	return chunks, nil
}

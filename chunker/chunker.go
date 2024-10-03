package chunker

import (
	"encoding/json"
	"fmt"

	"github.com/edgedelta/edgedelta-forwarder/cfg"
	"github.com/edgedelta/edgedelta-forwarder/core"
)

const (
	maxDepth = 9 // Maximum recursion depth
)

type Chunker struct {
	chunkSize int
	log       *core.Log
}

func NewChunker(chunkSize int, logEntry *core.Log) (*Chunker, error) {
	if logEntry == nil {
		return nil, fmt.Errorf("log object is nil")
	}

	commonJson, err := json.Marshal(logEntry.Common)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal common object: %w", err)
	}

	if len(commonJson) > cfg.MaxChunkSize {
		return nil, fmt.Errorf("common object is too large for specified chunk size. Try increasing the chunk size, given chunk size: %d, detected object size: %d", cfg.MaxChunkSize, len(commonJson))
	}

	return &Chunker{chunkSize: chunkSize, log: logEntry}, nil
}

// ChunkLogs splits a large log into smaller chunks that fit within the specified max chunk size.
// It uses a recursive divide-and-conquer approach.
func (c *Chunker) ChunkLogs() ([][]byte, error) {
	return c.chunkLogs(c.log, 0, len(c.log.LogEvents), 0)
}

func (c *Chunker) chunkLogs(log *core.Log, start, end, depth int) ([][]byte, error) {
	currentLogObject := core.Log{
		Common: log.Common,
		Data:   core.Data{LogEvents: log.LogEvents[start:end]},
	}

	currentBytes, err := json.Marshal(currentLogObject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal log object: %w", err)
	}

	shouldChunk := len(currentBytes) > c.chunkSize && // should chunk further if current log object is too large
		start != end-1 && // except when there is no more log events to chunk
		depth < maxDepth // except when we have reached the maximum recursion depth

	if !shouldChunk {
		return [][]byte{currentBytes}, nil
	}

	// Find the middle of the log events
	mid := start + (end-start)/2

	// Recursively chunk the left and right halves
	leftChunks, err := c.chunkLogs(log, start, mid, depth+1)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk left half (start: %d, mid: %d, depth: %d): %w", start, mid, depth+1, err)
	}
	rightChunks, err := c.chunkLogs(log, mid, end, depth+1)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk right half (mid: %d, end: %d, depth: %d): %w", mid, end, depth+1, err)
	}

	return append(leftChunks, rightChunks...), nil
}

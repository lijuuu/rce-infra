package executor

import (
	"bufio"
	"context"
	"io"
	"time"
)

// Chunk represents a log chunk
type Chunk struct {
	ChunkIndex int64
	Stream     string
	Data       string
	IsFinal    bool // true if this is the final chunk (work is done)
}

// Chunker handles chunking of stdout/stderr streams
type Chunker struct {
	chunkSize         int
	chunkInterval     time.Duration
	chunkChan         chan Chunk
	currentChunkIndex int64
	stdoutBuffer      []byte
	stderrBuffer      []byte
	lastFlush         time.Time
}

// NewChunker creates a new chunker
func NewChunker(chunkSize int, chunkIntervalSec int) *Chunker {
	return &Chunker{
		chunkSize:     chunkSize,
		chunkInterval: time.Duration(chunkIntervalSec) * time.Second,
		chunkChan:     make(chan Chunk, 100),
		lastFlush:     time.Now(),
	}
}

// StartChunking starts chunking from stdout and stderr readers
func (c *Chunker) StartChunking(ctx context.Context, stdout, stderr io.Reader) <-chan Chunk {
	go c.readStream(ctx, stdout, "stdout")
	go c.readStream(ctx, stderr, "stderr")

	// Start flush ticker
	ticker := time.NewTicker(c.chunkInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.flushBuffers()
			}
		}
	}()

	return c.chunkChan
}

// readStream reads from a stream and buffers data
func (c *Chunker) readStream(ctx context.Context, reader io.Reader, stream string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Bytes()
			c.appendToBuffer(stream, line)
		}
	}

	// Flush remaining on EOF
	c.flushStream(stream)
}

// appendToBuffer appends data to the appropriate buffer
func (c *Chunker) appendToBuffer(stream string, data []byte) {
	if stream == "stdout" {
		c.stdoutBuffer = append(c.stdoutBuffer, data...)
		c.stdoutBuffer = append(c.stdoutBuffer, '\n')
	} else {
		c.stderrBuffer = append(c.stderrBuffer, data...)
		c.stderrBuffer = append(c.stderrBuffer, '\n')
	}

	// Check if we should flush due to size
	if len(c.stdoutBuffer) >= c.chunkSize {
		c.flushStream("stdout")
	}
	if len(c.stderrBuffer) >= c.chunkSize {
		c.flushStream("stderr")
	}
}

// flushBuffers flushes all buffers if interval has passed
func (c *Chunker) flushBuffers() {
	now := time.Now()
	if now.Sub(c.lastFlush) >= c.chunkInterval {
		c.flushStream("stdout")
		c.flushStream("stderr")
		c.lastFlush = now
	}
}

// flushStream flushes a specific stream buffer
func (c *Chunker) flushStream(stream string) {
	var buffer []byte
	if stream == "stdout" {
		if len(c.stdoutBuffer) == 0 {
			return
		}
		buffer = c.stdoutBuffer
		c.stdoutBuffer = nil
	} else {
		if len(c.stderrBuffer) == 0 {
			return
		}
		buffer = c.stderrBuffer
		c.stderrBuffer = nil
	}

	if len(buffer) > 0 {
		chunk := Chunk{
			ChunkIndex: c.currentChunkIndex,
			Stream:     stream,
			Data:       string(buffer),
		}
		c.currentChunkIndex++

		select {
		case c.chunkChan <- chunk:
		default:
			// Channel full, drop chunk (shouldn't happen with buffered channel)
		}
	}
}

// FinalFlush flushes all remaining buffers and marks them as final
func (c *Chunker) FinalFlush() {
	// Flush stdout and stderr as final chunks
	c.flushStreamFinal("stdout")
	c.flushStreamFinal("stderr")
	close(c.chunkChan)
}

// flushStreamFinal flushes a specific stream buffer and marks it as final
func (c *Chunker) flushStreamFinal(stream string) {
	var buffer []byte
	if stream == "stdout" {
		if len(c.stdoutBuffer) == 0 {
			return
		}
		buffer = c.stdoutBuffer
		c.stdoutBuffer = nil
	} else {
		if len(c.stderrBuffer) == 0 {
			return
		}
		buffer = c.stderrBuffer
		c.stderrBuffer = nil
	}

	if len(buffer) > 0 {
		chunk := Chunk{
			ChunkIndex: c.currentChunkIndex,
			Stream:     stream,
			Data:       string(buffer),
			IsFinal:    true, // Mark as final chunk
		}
		c.currentChunkIndex++

		select {
		case c.chunkChan <- chunk:
		default:
			// Channel full, drop chunk (shouldn't happen with buffered channel)
		}
	}
}

// ChunkSize returns the chunk size
func (c *Chunker) ChunkSize() int {
	return c.chunkSize
}

// ChunkInterval returns the chunk interval in seconds
func (c *Chunker) ChunkInterval() int {
	return int(c.chunkInterval.Seconds())
}

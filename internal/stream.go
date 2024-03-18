package ndog

import (
	"io"
)

type Stream struct {
	Reader io.ReadCloser
	Writer io.WriteCloser
}

func (stream Stream) Close() error {
	stream.Reader.Close()
	stream.Writer.Close()
	return nil
}

type StreamManager interface {
	NewStream(name string) Stream
}

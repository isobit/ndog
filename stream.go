package ndog

import (
	"encoding/json"
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

func ReadJSON[T any](stream Stream) (*T, error) {
	readData, err := io.ReadAll(stream.Reader)
	if err != nil {
		return nil, err
	}
	var v T
	if err := json.Unmarshal(readData, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func WriteJSON[T any](stream Stream, v T) error {
	writeData, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w := stream.Writer
	w.Write(writeData)
	io.WriteString(w, "\n")
	w.Close()
	return nil
}

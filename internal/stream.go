package ndog

import (
	"io"
	"strconv"

	"github.com/isobit/ndog/internal/log"
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

type LogStreamManager struct {
	StreamManager
}

func NewLogStreamManager(delegate StreamManager) *LogStreamManager {
	return &LogStreamManager{
		StreamManager: delegate,
	}
}

func (f *LogStreamManager) NewStream(name string) Stream {
	stream := f.StreamManager.NewStream(name)
	return streamWithLogging(
		stream,
		func(p []byte) {
			log.Logf(0, "<-%s %s", name, strconv.Quote(string(p)))
		},
		func(p []byte) {
			log.Logf(0, "->%s %s", name, strconv.Quote(string(p)))
		},
	)
}

func streamWithLogging(stream Stream, logRecv func([]byte), logSend func([]byte)) Stream {
	recvReader, recvWriter := io.Pipe()
	sendReader, sendWriter := io.Pipe()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := recvReader.Read(buf)
			if err != nil {
				return
			}
			logRecv(buf[:n])
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := sendReader.Read(buf)
			if err != nil {
				return
			}
			logSend(buf[:n])
		}
	}()
	return Stream{
		Reader: TeeReadCloser(stream.Reader, sendWriter),
		Writer: MultiWriteCloser(stream.Writer, recvWriter),
	}
}

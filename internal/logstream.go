package ndog

import (
	"io"
	"strconv"

	"github.com/isobit/ndog/internal/log"
)

type LogStreamManager struct {
	Delegate StreamManager
}

func NewLogStreamManager(delegate StreamManager) *LogStreamManager {
	return &LogStreamManager{
		Delegate: delegate,
	}
}

func (m *LogStreamManager) NewStream(name string) Stream {
	stream := m.Delegate.NewStream(name)
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

type LogFileStreamManager struct {
	Delegate StreamManager
	RecvOut  io.Writer
	SendOut  io.Writer
}

func (m *LogFileStreamManager) NewStream(name string) Stream {
	stream := m.Delegate.NewStream(name)
	return streamWithLogging(
		stream,
		func(p []byte) {
			if m.RecvOut != nil {
				m.RecvOut.Write(p)
			}
		},
		func(p []byte) {
			if m.SendOut != nil {
				m.SendOut.Write(p)
			}
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

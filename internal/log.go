package ndog

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

var Log io.Writer = os.Stderr
var LogLevel int = 0
var LogColor bool = false

var Logf func(int, string, ...interface{}) (int, error) = defaultLogf

func defaultLogf(level int, format string, v ...interface{}) (int, error) {
	if level > LogLevel {
		return 0, nil
	}
	if LogColor {
		if level >= 0 {
			format = "\u001b[30;1m" + format + "\u001b[0m"
		} else {
			format = "\u001b[31;1m" + format + "\u001b[0m"
		}
	}
	if len(format) > 0 && format[len(format)-1] != '\n' {
		format = format + "\n"
	}
	return fmt.Fprintf(Log, format, v...)
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
			Logf(0, "<-%s %s", name, strconv.Quote(string(p)))
		},
		func(p []byte) {
			Logf(0, "->%s %s", name, strconv.Quote(string(p)))
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
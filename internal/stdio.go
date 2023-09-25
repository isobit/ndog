package ndog

import (
	"bytes"
	"io"
	"os"
)

type StdIOStreamManager struct {
	readCloserFunc func() io.ReadCloser
}

func NewStdIOStreamManager(fixedData []byte) *StdIOStreamManager {
	m := &StdIOStreamManager{}

	if fixedData != nil {
		m.readCloserFunc = func() io.ReadCloser {
			var buf bytes.Buffer
			buf.Write(fixedData)
			return io.NopCloser(&buf)
		}
	} else {
		fanout := FanoutStdin()
		m.readCloserFunc = fanout.Tee
	}

	return m
}

func (m *StdIOStreamManager) NewStream(name string) Stream {
	rc := m.readCloserFunc()
	return Stream{
		Reader: rc,
		Writer: NopWriteCloser(os.Stdout),
	}
}

func FanoutStdin() *Fanout {
	fanout := NewFanout()
	go func() {
		defer fanout.Close()
		if _, err := io.Copy(fanout, os.Stdin); err != nil {
			if !IsIOClosedErr(err) {
				Logf(-1, "stdin read error: %s", err)
			}
		}
		Logf(10, "stdin EOF")
		// os.Exit(1) // TODO clean shutdown
	}()
	return fanout
}

package ndog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
)

type Stream interface {
	io.ReadWriteCloser
	CloseWriter() error
}

type StreamFactory interface {
	NewStream(name string) Stream
}

type genericStream struct {
	io.Reader
	io.Writer
	CloseWriterFunc func() error
	CloseFunc       func() error
}

var _ Stream = genericStream{}

func (rwc genericStream) CloseWriter() error {
	if rwc.CloseWriterFunc == nil {
		return nil
	}
	return rwc.CloseWriterFunc()
}

func (rwc genericStream) Close() error {
	if rwc.CloseFunc == nil {
		return nil
	}
	return rwc.CloseFunc()
}

type StdIOStreamFactory struct {
	readCloserFunc func() io.ReadCloser
}

func NewStdIOStreamFactory(fixedData []byte) *StdIOStreamFactory {
	f := &StdIOStreamFactory{}

	stdinStat, _ := os.Stdin.Stat()
	if stdinStat.Mode()&os.ModeCharDevice == 0 {
		fanout := FanoutStdin()
		f.readCloserFunc = fanout.Tee
	} else {
		f.readCloserFunc = func() io.ReadCloser {
			var buf bytes.Buffer
			if fixedData != nil {
				buf.Write(fixedData)
			}
			return io.NopCloser(&buf)
		}
	}

	return f
}

func (f *StdIOStreamFactory) NewStream(name string) Stream {
	rc := f.readCloserFunc()
	return genericStream{
		Reader:    rc,
		Writer:    os.Stdout,
		CloseFunc: rc.Close,
	}
}

func FanoutStdin() *Fanout {
	fanout := NewFanout()
	go func() {
		defer fanout.Close()
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			data := scanner.Bytes()
			Logf(10, "stdin data: %s", data)
			if _, err := fanout.Write(data); err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			Logf(-1, "stdin scan error: %s", err)
		} else {
			Logf(10, "stdin EOF")
		}
		// os.Exit(1) // TODO clean shutdown
	}()
	return fanout
}

type Fanout struct {
	sync.Mutex
	writers []io.WriteCloser
	wakeCh  chan bool
	closed  bool
}

func NewFanout() *Fanout {
	return &Fanout{
		writers: []io.WriteCloser{},
		wakeCh:  make(chan bool),
	}
}

func (f *Fanout) CloseWriters() {
	f.Lock()
	defer f.Unlock()
	for _, w := range f.writers {
		w.Close()
	}
}

func (f *Fanout) Close() error {
	f.Lock()
	defer f.Unlock()
	for _, w := range f.writers {
		w.Close()
	}
	f.closed = true
	return nil
}

func (f *Fanout) Write(p []byte) (int, error) {
	f.Lock()
	defer f.Unlock()
	if f.closed {
		return 0, fmt.Errorf("fanout is closed")
	}
	if len(f.writers) == 0 {
		f.Unlock()
		<-f.wakeCh
		f.Lock()
	}
	openWriters := []io.WriteCloser{}
	for _, w := range f.writers {
		_, err := w.Write(p)
		if err != nil {
			continue
		}
		w.Write([]byte{'\n'})
		openWriters = append(openWriters, w)
	}
	f.writers = openWriters
	return len(p), nil
}

func (f *Fanout) Tee() io.ReadCloser {
	pr, pw := io.Pipe()

	f.Lock()
	f.writers = append(f.writers, pw)
	f.Unlock()

	// Wake any outstanding write now that there is a writer to handle it.
	// Select with a no-op default to make this non-blocking if there's no
	// write call waiting to receive from the channel.
	select {
	case f.wakeCh <- true:
	default:
	}

	return pr
}

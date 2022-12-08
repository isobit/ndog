package ndog

import (
	"bufio"
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

type LogStreamFactory struct {
	StreamFactory
	Receive io.Writer
	Send    io.Writer
}

func (f *LogStreamFactory) NewStream(name string) Stream {
	stream := f.StreamFactory.NewStream(name)
	return genericStream{
		Reader:    io.TeeReader(stream, f.Send),
		Writer:    io.MultiWriter(stream, f.Receive),
		CloseFunc: stream.Close,
	}
}

type StdIOStreamFactory struct {
	flr *FanoutLineReader
}

func NewStdIOStreamFactory() *StdIOStreamFactory {
	flr := NewFanoutLineReader(os.Stdin)
	go flr.ScanLoop()
	return &StdIOStreamFactory{
		flr: flr,
	}
}

func (f *StdIOStreamFactory) NewStream(name string) Stream {
	r, closeTee := f.flr.Tee()
	return genericStream{
		Reader: r,
		Writer: os.Stdout,
		CloseWriterFunc: func() error {
			closeTee()
			return nil
		},
		CloseFunc: func() error {
			closeTee()
			return nil
		},
	}
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

type FanoutLineReader struct {
	r       io.Reader
	writers []io.Writer
	sync.Mutex
}

func NewFanoutLineReader(r io.Reader) *FanoutLineReader {
	flr := &FanoutLineReader{
		r:       r,
		writers: []io.Writer{},
	}
	return flr
}

func (flr *FanoutLineReader) ScanLoop() {
	scanner := bufio.NewScanner(flr.r)
	for scanner.Scan() {
		p := scanner.Bytes()
		flr.Lock()
		newWriters := []io.Writer{}
		for i, w := range flr.writers {
			Logf(10, "writing %d bytes to writer %d", len(p), i)
			n, err := w.Write(p)
			if err == nil {
				newWriters = append(newWriters, w)
				w.Write([]byte{'\n'})
			} else {
				Logf(10, "writer %d error (n=%d), removing: %s", i, n, err)
			}
		}
		flr.writers = newWriters
		flr.Unlock()
	}
}

func (flr *FanoutLineReader) Tee() (io.Reader, func()) {
	pr, pw := io.Pipe()
	flr.Lock()
	flr.writers = append(flr.writers, pw)
	flr.Unlock()
	closeFunc := func() {
		pr.Close()
		pw.Close()
	}
	return pr, closeFunc
}

func TeeLinesReader(r io.Reader, lineHandler func([]byte)) (io.Reader, func()) {
	pr, pw := io.Pipe()
	go func() {
		defer pr.Close()
		defer pw.Close()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			lineHandler(scanner.Bytes())
		}
	}()
	return io.TeeReader(r, pw), func() { pr.Close() }
}

func BidirectionalCopyWithTee(rw1 io.ReadWriter, rw2 io.ReadWriter, log io.Writer) error {
	doneCh := make(chan struct{})
	errCh := make(chan error)

	go func() {
		tr1, tr1Close := TeeLinesReader(rw1, func(p []byte) {
			io.WriteString(log, "-> ")
			log.Write(p)
			io.WriteString(log, "\n")
		})
		defer tr1Close()

		_, err := io.Copy(rw2, tr1)
		// logf("-> DONE; err=%v", err)
		select {
		case <-doneCh:
			return
		case errCh <- err:
		}
	}()

	go func() {
		tr2, tr2Close := TeeLinesReader(rw2, func(p []byte) {
			io.WriteString(log, "<- ")
			log.Write(p)
			io.WriteString(log, "\n")
		})
		defer tr2Close()

		_, err := io.Copy(rw1, tr2)
		// logf("<- DONE; err=%v", err)
		select {
		case <-doneCh:
			return
		case errCh <- err:
		}
	}()
	err := <-errCh
	// logf("<-> got err; err=%v", err)
	close(doneCh)
	return err
}

package ndog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
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

type StdIOStreamManager struct {
	readCloserFunc func() io.ReadCloser
}

func NewStdIOStreamManager(fixedData []byte) *StdIOStreamManager {
	f := &StdIOStreamManager{}

	if fixedData == nil {
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

func (f *StdIOStreamManager) NewStream(name string) Stream {
	rc := f.readCloserFunc()
	return Stream{
		Reader: rc,
		Writer: NopWriteCloser(os.Stdout),
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

type funcReadCloser struct {
	io.Reader
	closeFunc func() error
	closed    bool
}

func (frc *funcReadCloser) Close() error {
	if frc.closed {
		return nil
	}
	if err := frc.closeFunc(); err != nil {
		return err
	}
	frc.closed = true
	return nil
}

func FuncReadCloser(r io.Reader, f func() error) io.ReadCloser {
	return &funcReadCloser{
		Reader:    r,
		closeFunc: f,
	}
}

func TeeReadCloser(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	tr := io.TeeReader(r, w)
	return FuncReadCloser(tr, func() error {
		r.Close()
		w.Close()
		return nil
	})
}

type funcWriteCloser struct {
	io.Writer
	closeFunc func() error
	closed    bool
}

func (fwc *funcWriteCloser) Close() error {
	if fwc.closed {
		return nil
	}
	if err := fwc.closeFunc(); err != nil {
		return err
	}
	fwc.closed = true
	return nil
}

func FuncWriteCloser(w io.Writer, f func() error) io.WriteCloser {
	return &funcWriteCloser{
		Writer:    w,
		closeFunc: f,
	}
}

func MultiWriteCloser(writerClosers ...io.WriteCloser) io.WriteCloser {
	var writers []io.Writer
	for _, wc := range writerClosers {
		writers = append(writers, wc)
	}
	w := io.MultiWriter(writers...)
	return FuncWriteCloser(w, func() error {
		for _, wc := range writerClosers {
			wc.Close()
		}
		return nil
	})
}
func NopWriteCloser(w io.Writer) io.WriteCloser {
	return nopWriteCloser{Writer: w}
}

type nopWriteCloser struct {
	io.Writer
}

func (nwc nopWriteCloser) Close() error {
	return nil
}

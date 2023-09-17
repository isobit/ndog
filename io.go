package ndog

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"
)

func IsIOClosedErr(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) || errors.Is(err, fs.ErrClosed)
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

type funcReadCloser struct {
	io.Reader
	closeFunc func() error

	sync.Mutex
	closed bool
}

func (frc *funcReadCloser) Close() error {
	frc.Lock()
	defer frc.Unlock()
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

type funcWriteCloser struct {
	io.Writer
	closeFunc func() error

	sync.Mutex
	closed bool
}

func (fwc *funcWriteCloser) Close() error {
	fwc.Lock()
	defer fwc.Unlock()
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

func TeeReadCloser(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	tr := io.TeeReader(r, w)
	return FuncReadCloser(tr, func() error {
		r.Close()
		w.Close()
		return nil
	})
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

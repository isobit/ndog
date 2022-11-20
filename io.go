package main

import (
	"bufio"
	"io"
	"sync"
)

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
		toRemove := []int{}
		for i, w := range flr.writers {
			if verbose {
				logf("writing %d bytes to writer %d", len(p), i)
			}
			n, err := w.Write(p)
			if err != nil {
				logf("writer %d error (n=%d): %s", i, n, err)
			}
			if err == io.ErrClosedPipe {
				toRemove = append(toRemove, i)
			} else {
				w.Write([]byte{'\n'})
			}
		}
		for _, i := range toRemove {
			if verbose {
				logf("removing writer %d", i)
			}
			flr.writers = append(flr.writers[:i], flr.writers[i+1:]...)
		}
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
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			lineHandler(scanner.Bytes())
		}
	}()
	return io.TeeReader(r, pw), func() { pr.Close() }
}

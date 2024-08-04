package ndog

import (
	"bytes"
	"io"
	"os"

	"github.com/isobit/ndog/internal/ioutil"
	"github.com/isobit/ndog/internal/log"
)

type StdIOStreamManager struct {
	fixedData   []byte
	stdinFanout *ioutil.Fanout
}

func NewStdIOStreamManager(fixedData []byte) *StdIOStreamManager {
	m := &StdIOStreamManager{
		fixedData: fixedData,
	}
	if fixedData == nil {
		m.stdinFanout = ioutil.NewFanout()
		go func() {
			defer m.stdinFanout.Close()
			if _, err := io.Copy(m.stdinFanout, os.Stdin); err != nil {
				if !ioutil.IsIOClosedErr(err) {
					log.Logf(-1, "stdin read error: %s", err)
				}
			}
			log.Logf(10, "stdin EOF")
			// os.Exit(1) // TODO clean shutdown
		}()
	}
	return m
}

func (m *StdIOStreamManager) NewStream(name string) Stream {
	var r io.ReadCloser

	if m.fixedData != nil {
		var buf bytes.Buffer
		buf.Write(m.fixedData)
		r = io.NopCloser(&buf)
	} else {
		r = m.stdinFanout.Tee()
	}

	return Stream{
		Reader: r,
		Writer: ioutil.NopWriteCloser(os.Stdout),
	}
}

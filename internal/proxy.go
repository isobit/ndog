package ndog

import (
	"io"

	"github.com/isobit/ndog/internal/log"
)

type ProxyStreamManager struct {
	ConnectConfig Config
	Connect       func(ConnectConfig) error
}

func (f ProxyStreamManager) NewStream(name string) Stream {
	log.Logf(10, "creating proxy pipe: %s", name)

	listenReader, listenWriter := io.Pipe()
	connectReader, connectWriter := io.Pipe()

	listenStream := Stream{
		Reader: connectReader,
		Writer: listenWriter,
	}
	connectStream := Stream{
		Reader: listenReader,
		Writer: connectWriter,
	}

	go func() {
		defer connectWriter.Close()
		cfg := ConnectConfig{
			Config: f.ConnectConfig,
			Stream: connectStream,
		}
		if err := f.Connect(cfg); err != nil {
			log.Logf(-1, "connect error: %s", err)
		}
	}()

	return listenStream
}

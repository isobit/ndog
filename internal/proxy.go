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
		// CloseWriterFunc: listenWriter.Close,
		// CloseFunc: func() error {
		// 	Logf(10, "closing proxy listen stream: %s", name)
		// 	return listenWriter.Close()
		// },
	}
	connectStream := Stream{
		Reader: listenReader,
		Writer: connectWriter,
		// CloseWriterFunc: connectWriter.Close,
		// CloseFunc: func() error {
		// 	Logf(10, "closing proxy connect stream: %s", name)
		// 	return connectWriter.Close()
		// },
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

type proxyPipe struct {
	stream Stream
}

func (p proxyPipe) NewStream(name string) Stream {
	return p.stream
}

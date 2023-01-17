package ssh

import (
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"

	"github.com/isobit/ndog"
)

var Scheme = &ndog.Scheme{
	Names:  []string{"ssh"},
	Listen: Listen,
}

func Listen(cfg ndog.ListenConfig) error {
	handler := func(s ssh.Session) {
		name := fmt.Sprintf("%s@%s", s.User(), s.RemoteAddr())
		ndog.Logf(1, "accepted: %s %s", name, s.RawCommand())
		stream := cfg.StreamFactory.NewStream(name)
		go io.Copy(stream.Writer, s)
		io.Copy(s, stream.Reader)
	}

	server := ssh.Server{
		Addr:    cfg.URL.Host,
		Handler: handler,

		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
	}
	ndog.Logf(0, "listening: %s", cfg.URL.Host)
	return server.ListenAndServe()
}

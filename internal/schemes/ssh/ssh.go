package ssh

import (
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"

	"github.com/isobit/ndog/internal"
	"github.com/isobit/ndog/internal/log"
)

var Scheme = &ndog.Scheme{
	Names:  []string{"ssh"},
	Listen: Listen,

	Description: `
Listen starts an SSH server on the host and port specified in the URL.
	`,
}

func Listen(cfg ndog.ListenConfig) error {
	handler := func(s ssh.Session) {
		name := fmt.Sprintf("%s@%s", s.User(), s.RemoteAddr())
		log.Logf(1, "accepted: %s %s", name, s.RawCommand())
		stream := cfg.StreamManager.NewStream(name)
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
	log.Logf(0, "listening: %s", cfg.URL.Host)
	return server.ListenAndServe()
}

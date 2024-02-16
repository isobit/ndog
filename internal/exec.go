package ndog

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/isobit/ndog/internal/log"
)

type ExecStreamManager struct {
	Args      []string
	TeeWriter io.Writer
}

func NewExecStreamManager(args []string) *ExecStreamManager {
	return &ExecStreamManager{
		Args: args,
	}
}

func (f *ExecStreamManager) NewStream(name string) Stream {
	cmd := exec.Command(f.Args[0], f.Args[1:]...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	log.Logf(10, "exec: starting: %s", cmd)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	log.Logf(10, "exec: started: %d", cmd.Process.Pid)

	// Log stderr
	go func() {
		defer stderr.Close()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Logf(0, "exec: stderr: %d: %s", cmd.Process.Pid, scanner.Text())
		}
	}()

	var w io.WriteCloser = stdin
	if f.TeeWriter != nil {
		w = FuncWriteCloser(io.MultiWriter(w, f.TeeWriter), w.Close)
	}

	stdinClosed := make(chan bool)
	stdoutClosed := make(chan bool)
	go func() {
		// Wait for reader and writer to be closed.
		// Must wait for reader/stdin to be closed first or else we'll
		// deadlock.
		<-stdinClosed
		<-stdoutClosed

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			select {
			case <-time.After(10 * time.Second):
				log.Logf(10, "exec: terminating: %d", cmd.Process.Pid)
				cmd.Process.Signal(syscall.SIGTERM)
			case <-ctx.Done():
				return
			}

			select {
			case <-time.After(10 * time.Second):
				log.Logf(-1, "exec: termination timed out, killing: %d", cmd.Process.Pid)
				cmd.Process.Kill()
			case <-ctx.Done():
				return
			}
		}()

		log.Logf(10, "exec: waiting: %d", cmd.Process.Pid)
		cmd.Wait()
		log.Logf(10, "exec: exited: %d", cmd.Process.Pid)
	}()

	return Stream{
		Reader: FuncReadCloser(stdout, func() error {
			close(stdoutClosed)
			log.Logf(10, "exec: closing stdout: %d", cmd.Process.Pid)
			err := stdout.Close()
			return err
		}),
		Writer: FuncWriteCloser(w, func() error {
			close(stdinClosed)
			log.Logf(10, "exec: closing stdin: %d", cmd.Process.Pid)
			return w.Close()
		}),
	}
}

// type ExecTemplateStreamManager struct {
// 	Name string
// 	Args []string
// }

// func NewExecTemplateStreamManager(name string, args ...string) *ExecTemplateStreamManager {
// 	return &ExecTemplateStreamManager{
// 		Name: name,
// 		Args: args,
// 	}
// }

// func (f *ExecTemplateStreamManager) NewStream(name string) Stream {
// }

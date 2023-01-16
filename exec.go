package ndog

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

type ExecStreamFactory struct {
	Args           []string
	TeeWriteCloser io.WriteCloser
}

func NewExecStreamFactory(args []string) *ExecStreamFactory {
	return &ExecStreamFactory{
		Args: args,
	}
}

func (f *ExecStreamFactory) NewStream(name string) Stream {
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

	Logf(10, "exec: starting: %s", cmd)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	Logf(10, "exec: started: %d", cmd.Process.Pid)

	// Log stderr
	go func() {
		defer stderr.Close()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			Logf(0, "exec: stderr: %d: %s", cmd.Process.Pid, scanner.Text())
		}
	}()

	var w io.WriteCloser = stdin
	if f.TeeWriteCloser != nil {
		w = MultiWriteCloser(w, f.TeeWriteCloser)
	}

	shutdownCh := make(chan bool)
	go func() {
		// Wait for reader and writer to be closed.
		<-shutdownCh
		<-shutdownCh

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			select {
			case <-time.After(10 * time.Second):
				Logf(10, "exec: terminating: %d", cmd.Process.Pid)
				cmd.Process.Signal(syscall.SIGTERM)
			case <-ctx.Done():
				return
			}

			select {
			case <-time.After(10 * time.Second):
				Logf(-1, "exec: termination timed out, killing: %d", cmd.Process.Pid)
				cmd.Process.Kill()
			case <-ctx.Done():
				return
			}
		}()

		Logf(10, "exec: waiting: %d", cmd.Process.Pid)
		cmd.Wait()
		Logf(10, "exec: exited: %d", cmd.Process.Pid)
	}()

	return Stream{
		Reader: FuncReadCloser(stdout, func() error {
			shutdownCh <- true
			return stdout.Close()
		}),
		Writer: FuncWriteCloser(w, func() error {
			shutdownCh <- true
			Logf(10, "exec: closing stdin: %d", cmd.Process.Pid)
			return w.Close()
		}),
	}
}

// type ExecTemplateStreamFactory struct {
// 	Name string
// 	Args []string
// }

// func NewExecTemplateStreamFactory(name string, args ...string) *ExecTemplateStreamFactory {
// 	return &ExecTemplateStreamFactory{
// 		Name: name,
// 		Args: args,
// 	}
// }

// func (f *ExecTemplateStreamFactory) NewStream(name string) Stream {
// }

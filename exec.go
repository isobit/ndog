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
	Args      []string
	TeeWriter io.Writer
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

	var w io.Writer = stdin
	if f.TeeWriter != nil {
		w = io.MultiWriter(w, f.TeeWriter)
	}

	return genericStream{
		Reader: stdout,
		Writer: w,
		CloseWriterFunc: func() error {
			Logf(10, "exec: closing stdin: %d", cmd.Process.Pid)
			return stdin.Close()
		},
		CloseFunc: func() error {
			Logf(10, "exec: terminating: %d", cmd.Process.Pid)
			cmd.Process.Signal(syscall.SIGTERM)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				select {
				case <-time.After(10 * time.Second):
					Logf(-1, "exec: termination timed out, killing: %d", cmd.Process.Pid)
					cmd.Process.Kill()
				case <-ctx.Done():
				}
			}()

			Logf(10, "exec: waiting: %d", cmd.Process.Pid)
			cmd.Wait()

			Logf(10, "exec: exited: %d", cmd.Process.Pid)
			return nil
		},
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

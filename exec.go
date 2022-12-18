package ndog

import (
	"bufio"
	"os/exec"
	"syscall"
)

func execCommandStream(name string, args ...string) Stream {
	cmd := exec.Command(name, args...)

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

	return genericStream{
		Reader:          stdout,
		Writer:          stdin,
		CloseWriterFunc: stdin.Close,
		CloseFunc: func() error {
			defer stderr.Close()
			Logf(10, "exec: closing stdin/stdout: %d", cmd.Process.Pid)
			stdin.Close()
			stdout.Close()
			Logf(10, "exec: terminating: %d", cmd.Process.Pid)
			cmd.Process.Signal(syscall.SIGTERM)
			// TODO kill after timeout
			// cmd.Process.Kill()
			Logf(10, "exec: waiting: %d", cmd.Process.Pid)
			cmd.Wait()
			Logf(10, "exec: exited: %d", cmd.Process.Pid)
			return nil
		},
	}
}

type ExecStreamFactory struct {
	Name string
	Args []string
}

func NewExecStreamFactory(name string, args ...string) *ExecStreamFactory {
	return &ExecStreamFactory{
		Name: name,
		Args: args,
	}
}

func (f *ExecStreamFactory) NewStream(name string) Stream {
	return execCommandStream(f.Name, f.Args...)
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

package commandrunner

import (
	"bytes"
	"os/exec"
)

//go:generate counterfeiter . Runner

type Runner interface {
	Run(outbuf, errbuff *bytes.Buffer) error
	CommandErrorChannel() chan error
	Kill() error
}

type runner struct {
	scriptPath string
	cmdErrChan chan error
	cmd        *exec.Cmd
}

func NewRunner(scriptPath string) Runner {
	return &runner{
		scriptPath: scriptPath,
		cmdErrChan: make(chan error, 1),
	}
}

func (r *runner) CommandErrorChannel() chan error {
	return r.cmdErrChan
}

func (r *runner) Run(outbuf, errbuf *bytes.Buffer) error {
	r.cmd = exec.Command(r.scriptPath)

	r.cmd.Stdout = outbuf
	r.cmd.Stderr = errbuf

	err := r.cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		r.cmdErrChan <- r.cmd.Wait()
	}()

	return nil
}

func (r *runner) Kill() error {
	return r.cmd.Process.Kill()
}

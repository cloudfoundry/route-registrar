package commandrunner

import (
	"bytes"
	"os/exec"
)

type Runner interface {
	Run(outbuf, errbuff *bytes.Buffer) error
	CommandErrorChannel() chan error
}

type runner struct {
	scriptPath string
	cmdErrChan chan error
}

func NewRunner(scriptPath string) Runner {
	return &runner{
		scriptPath: scriptPath,
		cmdErrChan: make(chan error),
	}
}

func (r *runner) CommandErrorChannel() chan error {
	return r.cmdErrChan
}

func (r *runner) Run(outbuf, errbuf *bytes.Buffer) error {
	cmd := exec.Command(r.scriptPath)

	cmd.Stdout = outbuf
	cmd.Stderr = errbuf

	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		r.cmdErrChan <- cmd.Wait()
	}()

	return nil
}

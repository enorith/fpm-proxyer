package pkg

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Command struct {
	logger   *log.Logger
	exec     string
	pid      int
	stopChan chan struct{}
	process  *os.Process
}

func (c *Command) Exec(workDir string) (*exec.Cmd, error) {
	logger := c.logger

	partials := strings.Split(c.exec, " ")
	var args []string
	if len(partials) > 1 {
		args = partials[1:]
	}
	cmd := exec.Command(partials[0], args...)

	stdout, err := cmd.StdoutPipe()

	if err != nil {
		return cmd, err
	}

	cmdErr := cmd.Start()
	if cmdErr != nil {
		return cmd, cmdErr
	}
	c.pid = cmd.Process.Pid
	c.process = cmd.Process
	logger.Println(fmt.Sprintf("execute [%s], work dir [%s], pid [%d]", c.exec, workDir, c.pid))

	reader := bufio.NewReader(stdout)

	for {
		line, err2 := reader.ReadString('\n')
		if io.EOF == err2 {
			break
		}
		if err2 != nil {
			logger.Println(err2)
			break
		}

		logger.Println(line)
	}

	cmd.Wait()

	return cmd, nil
}

func (c *Command) Pid() int {
	return c.pid
}

func (c *Command) GoExec(workDir string) {
	go func() {
		c.Exec(workDir)
	}()
}

func (c *Command) Stop() error {
	if c.process != nil {
		c.logger.Println("Stopping", c.process.Pid)
		return c.process.Kill()
	}
	return nil
}

func NewCommand(exec string, logger *log.Logger) *Command {
	if logger == nil {
		logger = log.Default()
	}
	return &Command{exec: exec, logger: logger, stopChan: make(chan struct{}, 1)}
}

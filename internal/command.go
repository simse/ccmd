package internal

import (
	"os/exec"
	"strings"
)

func RunCommand(command string, workingDirectory string) ([]byte, error) {
	commandParts := strings.Split(command, " ")
	executable := commandParts[0]
	args := commandParts[1:]

	cmd := exec.Command(executable, args...)
	cmd.Dir = workingDirectory

	return cmd.Output()
}

package internal

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
)

var dimGrey = color.RGB(160, 160, 160).PrintlnFunc()

// RunCommand streams a command’s output, printing each stdout line to stdout
// and each stderr line to stderr. It returns an error if the command fails to start
// or exits with a non-zero status.
func RunCommand(command string, workingDirectory string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("no command provided")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workingDirectory

	// get pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("StdoutPipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("StderrPipe: %w", err)
	}

	// start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd.Start: %w", err)
	}

	// scan stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			dimGrey(scanner.Text())
		}
	}()

	// scan stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			dimGrey(os.Stderr, scanner.Text())
		}
	}()

	// wait for it to finish
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("cmd.Wait: %w", err)
	}
	return nil
}

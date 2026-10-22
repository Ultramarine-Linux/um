package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunCommand runs a command with the terminal attached; it routes standard I/O so DNF download progress animations are visible to the user
func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// CommandOutput runs a command and returns its trimmed stdout.
func CommandOutput(name string, args ...string) (string, error) {
	return CommandOutputWithExitCodes(name, nil, args...)
}

// CommandOutputWithExitCodes is CommandOutput, but treats the given exit
// codes as success (e.g. dnf check-update exits with 100 when updates exist).
func CommandOutputWithExitCodes(name string, allowedExitCodes []int, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			for _, allowedExitCode := range allowedExitCodes {
				if exitErr.ExitCode() == allowedExitCode {
					return strings.TrimSpace(string(output)), nil
				}
			}
			return "", fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

package backend

import (
	"fmt"
	"os/exec"
)

// runCommand executes a command and returns an error if it fails
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w\nOutput: %s", name, args, err, string(output))
	}
	return nil
}

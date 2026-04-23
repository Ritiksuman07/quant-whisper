package cmd

import (
	"os"
	"os/exec"
	"strings"
)

func runPythonCommand(args ...string) error {
	command := exec.Command("python", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	return command.Run()
}

func joinArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, value := range args {
		if strings.ContainsRune(value, ' ') {
			quoted = append(quoted, `"`+value+`"`)
			continue
		}
		quoted = append(quoted, value)
	}
	return strings.Join(quoted, " ")
}

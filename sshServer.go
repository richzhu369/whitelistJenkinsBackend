package main

import (
	"fmt"
	"log"
	"os/exec"
)

func executeSSHCommand(server, command string) error {
	cmd := exec.Command("ssh", "-p", "10086", "root@"+server, command)
	log.Println("Executing command: ", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %s, output: %s, error: %w", command, output, err)
	}
	return nil
}

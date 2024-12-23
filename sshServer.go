package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// ssh到服务器执行命令，5秒超时
func executeSSHCommand(server, command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-p", "10086", "root@"+server, command)
	log.Println("Executing command: ", cmd.String())
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		SendToLark("执行命令超时！" + fmt.Sprintf("服务器：%s", server))
		return fmt.Errorf("command timed out")
	}
	if err != nil {
		return fmt.Errorf("failed to execute command: %s, output: %s, error: %w", command, output, err)
	}
	return nil
}

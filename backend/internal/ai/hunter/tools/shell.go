package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	shellDefaultTimeout = 5 * time.Minute
	shellMaxOutputBytes = 1024 * 1024 // 1MB
)

// ShellTool executes shell commands via os/exec
type ShellTool struct{}

func NewShellTool() *ShellTool {
	return &ShellTool{}
}

func (s *ShellTool) Name() string { return "shell" }

func (s *ShellTool) Description() string {
	return `Execute shell commands on the system. Use this to run security tools like:
- nuclei -target <url> -severity critical,high
- httpx -u <domain> -status-code -title -tech-detect
- subfinder -d <domain>
- nmap -sV -sC <target>
- sqlmap -u <url> --batch --level 3
- katana -u <url> -d 2
- curl -v <url>
Always use --batch or non-interactive flags. Set reasonable timeouts.`
}

func (s *ShellTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 300)",
			},
		},
		"required": []string{"command"},
	}
}

func (s *ShellTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeoutSec := 300
	if t, ok := params["timeout_seconds"].(float64); ok && t > 0 {
		timeoutSec = int(t)
	}

	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Split command into program and args
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Safety: block dangerous commands
	dangerous := []string{"rm -rf /", "mkfs", "dd if=", "> /dev/", ":(){ :|:& };:"}
	for _, d := range dangerous {
		if strings.Contains(command, d) {
			return "", fmt.Errorf("blocked dangerous command: %s", d)
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Truncate output if too large
	outStr := stdout.String()
	if len(outStr) > shellMaxOutputBytes {
		outStr = outStr[:shellMaxOutputBytes] + "\n... [truncated]"
	}

	errStr := stderr.String()
	if len(errStr) > shellMaxOutputBytes {
		errStr = errStr[:shellMaxOutputBytes] + "\n... [truncated]"
	}

	result := outStr
	if errStr != "" {
		result += "\n[STDERR]\n" + errStr
	}
	if err != nil {
		result += fmt.Sprintf("\n[EXIT ERROR] %v", err)
	}

	return result, nil
}

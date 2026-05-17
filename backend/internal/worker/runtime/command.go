package runtime

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

// HeartbeatFunc refreshes scan liveness while a long-running process is active.
type HeartbeatFunc func(targetID uint)

// StopRequestedFunc reports whether the user requested the scan to stop.
type StopRequestedFunc func(targetID uint) bool

// RunCommandWithKillSwitch runs an external command and returns stdout.
func RunCommandWithKillSwitch(targetID uint, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	return RunCommandWithStdinAndKillSwitch(targetID, nil, heartbeat, shouldStop, name, args...)
}

// RunCommandWithStdinAndKillSwitch runs an external command with optional stdin
// and kills it when shouldStop returns true.
func RunCommandWithStdinAndKillSwitch(targetID uint, stdin io.Reader, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = EnvWithTargetTmp(targetID)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}

	taskKey := fmt.Sprintf("%d-%d", targetID, cmd.Process.Pid)
	storeRunningProcess(taskKey, TaskInfo{
		TargetID:  targetID,
		Command:   name + " " + strings.Join(args, " "),
		StartTime: time.Now(),
		PID:       cmd.Process.Pid,
	})
	defer deleteRunningProcess(taskKey)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			return outBuf.Bytes(), err
		case <-ticker.C:
			if heartbeat != nil {
				heartbeat(targetID)
			}

			if shouldStop != nil && shouldStop(targetID) {
				log.Printf("🛑 Kill switch activated for target %d. Killing process %s...", targetID, name)
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("⚠️ Failed to kill process: %v", err)
				}
				return nil, fmt.Errorf("process killed by user request")
			}
		}
	}
}

// RunCommandWithKillSwitchCombined runs an external command and returns stdout + stderr.
func RunCommandWithKillSwitchCombined(targetID uint, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = EnvWithTargetTmp(targetID)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}

	taskKey := fmt.Sprintf("%d-%d", targetID, cmd.Process.Pid)
	storeRunningProcess(taskKey, TaskInfo{
		TargetID:  targetID,
		Command:   name + " " + strings.Join(args, " "),
		StartTime: time.Now(),
		PID:       cmd.Process.Pid,
	})
	defer deleteRunningProcess(taskKey)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			return outBuf.Bytes(), err
		case <-ticker.C:
			if heartbeat != nil {
				heartbeat(targetID)
			}

			if shouldStop != nil && shouldStop(targetID) {
				log.Printf("🛑 Kill switch activated for target %d. Killing process %s...", targetID, name)
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("⚠️ Failed to kill process: %v", err)
				}
				return nil, fmt.Errorf("process killed by user request")
			}
		}
	}
}

package runtime

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"syscall"
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

// RunCommandNoCaptureWithKillSwitch runs an external command without buffering stdout/stderr in memory.
// Use this for high-volume tools that write primary output to files.
func RunCommandNoCaptureWithKillSwitch(targetID uint, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %s: %w", name, err)
	}

	var outBuf bytes.Buffer
	_, err := waitCommandWithKillSwitch(targetID, cmd, heartbeat, shouldStop, name, args, &outBuf)
	return err
}

// RunCommandWithStdinAndKillSwitch runs an external command with optional stdin
// and kills the entire process group when shouldStop returns true.
func RunCommandWithStdinAndKillSwitch(targetID uint, stdin io.Reader, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}

	return waitCommandWithKillSwitch(targetID, cmd, heartbeat, shouldStop, name, args, &outBuf)
}

// RunCommandWithKillSwitchCombined runs an external command and returns stdout + stderr.
func RunCommandWithKillSwitchCombined(targetID uint, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}

	return waitCommandWithKillSwitch(targetID, cmd, heartbeat, shouldStop, name, args, &outBuf)
}

func prepareCommand(targetID uint, cmd *exec.Cmd) {
	cmd.Env = EnvWithTargetTmp(targetID)

	// Start every external tool in its own process group. Killing only the
	// parent process can leave child processes behind for tools that spawn
	// helpers or shell pipelines. A process-group kill keeps Stop Scan reliable.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func waitCommandWithKillSwitch(
	targetID uint,
	cmd *exec.Cmd,
	heartbeat HeartbeatFunc,
	shouldStop StopRequestedFunc,
	name string,
	args []string,
	outBuf *bytes.Buffer,
) ([]byte, error) {
	taskKey := fmt.Sprintf("%d-%d", targetID, cmd.Process.Pid)
	storeRunningProcess(taskKey, TaskInfo{
		TargetID:  targetID,
		Command:   name + " " + strings.Join(args, " "),
		StartTime: time.Now(),
		PID:       cmd.Process.Pid,
	})
	defer deleteRunningProcess(taskKey)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

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
				log.Printf("🛑 Kill switch activated for target %d. Killing process group for %s...", targetID, name)

				if err := killProcessGroup(cmd); err != nil {
					log.Printf("⚠️ Failed to kill process group for %s: %v", name, err)
				}

				select {
				case <-done:
					return nil, fmt.Errorf("process killed by user request")
				case <-time.After(5 * time.Second):
					log.Printf("⚠️ Timed out waiting for killed process group to exit for %s", name)
					return nil, fmt.Errorf("process killed by user request")
				}
			}
		}
	}
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Fallback to killing the parent process if the process group is already gone.
		return cmd.Process.Kill()
	}

	// Negative PID targets the whole process group on Unix/Linux.
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		return err
	}

	return nil
}

// RunCommandWithTimeoutAndKillSwitch runs an external command with a maximum wall-clock duration.
// A zero or negative timeout disables timeout handling and behaves like RunCommandWithKillSwitch.
func RunCommandWithTimeoutAndKillSwitch(targetID uint, timeout time.Duration, heartbeat HeartbeatFunc, shouldStop StopRequestedFunc, name string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		return RunCommandWithKillSwitch(targetID, heartbeat, shouldStop, name, args...)
	}

	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}

	return waitCommandWithTimeout(targetID, cmd, timeout, heartbeat, shouldStop, name, args, &outBuf)
}

func waitCommandWithTimeout(
	targetID uint,
	cmd *exec.Cmd,
	timeout time.Duration,
	heartbeat HeartbeatFunc,
	shouldStop StopRequestedFunc,
	name string,
	args []string,
	outBuf *bytes.Buffer,
) ([]byte, error) {
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

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case err := <-done:
			return outBuf.Bytes(), err
		case <-timer.C:
			log.Printf("⏰ Command timeout reached for target %d. Killing process group for %s after %s", targetID, name, timeout)
			if err := killProcessGroup(cmd); err != nil {
				log.Printf("⚠️ Failed to kill timed-out process group for %s: %v", name, err)
			}
			select {
			case <-done:
				return nil, fmt.Errorf("command timed out after %s", timeout)
			case <-time.After(5 * time.Second):
				return nil, fmt.Errorf("command timed out after %s", timeout)
			}
		case <-ticker.C:
			if heartbeat != nil {
				heartbeat(targetID)
			}
			if shouldStop != nil && shouldStop(targetID) {
				log.Printf("Kill switch activated for target %d. Killing process group for %s...", targetID, name)
				if err := killProcessGroup(cmd); err != nil {
					log.Printf("⚠️ Failed to kill process group for %s: %v", name, err)
				}
				select {
				case <-done:
					return nil, fmt.Errorf("process killed by user request")
				case <-time.After(5 * time.Second):
					log.Printf("⚠️ Timed out waiting for killed process group to exit for %s", name)
					return nil, fmt.Errorf("process killed by user request")
				}
			}
		}
	}
}

// RunCommandWithKillSwitchCombinedTimeout runs an external command with stdout+stderr,
// kills the full process group on stop or timeout, and preserves process monitoring.
func RunCommandWithKillSwitchCombinedTimeout(
	targetID uint,
	timeout time.Duration,
	heartbeat HeartbeatFunc,
	shouldStop StopRequestedFunc,
	name string,
	args ...string,
) ([]byte, error) {
	if timeout <= 0 {
		return RunCommandWithKillSwitchCombined(targetID, heartbeat, shouldStop, name, args...)
	}

	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

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
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case err := <-done:
			return outBuf.Bytes(), err

		case <-timer.C:
			log.Printf("⚠️ Timeout reached for target %d. Killing process group for %s...", targetID, name)
			if err := killProcessGroup(cmd); err != nil {
				log.Printf("⚠️ Failed to kill timed-out process group for %s: %v", name, err)
			}

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				log.Printf("⚠️ Timed out waiting for killed process group to exit for %s", name)
			}

			return outBuf.Bytes(), fmt.Errorf("command timed out after %s: %s", timeout, name)

		case <-ticker.C:
			if heartbeat != nil {
				heartbeat(targetID)
			}

			if shouldStop != nil && shouldStop(targetID) {
				log.Printf(" Kill switch activated for target %d. Killing process group for %s...", targetID, name)
				if err := killProcessGroup(cmd); err != nil {
					log.Printf("⚠️ Failed to kill process group for %s: %v", name, err)
				}

				select {
				case <-done:
					return nil, fmt.Errorf("process killed by user request")
				case <-time.After(5 * time.Second):
					log.Printf("⚠️ Timed out waiting for killed process group to exit for %s", name)
					return nil, fmt.Errorf("process killed by user request")
				}
			}
		}
	}
}

// ProgressFunc receives raw stdout/stderr chunks from a running command.
type ProgressFunc func(chunk []byte)

type progressWriter struct {
	progress ProgressFunc
}

func (w progressWriter) Write(p []byte) (int, error) {
	if w.progress != nil && len(p) > 0 {
		cp := make([]byte, len(p))
		copy(cp, p)
		w.progress(cp)
	}
	return len(p), nil
}

// RunCommandWithProgressAndKillSwitch runs an external command without buffering
// full output, but streams stdout/stderr chunks to a progress callback.
func RunCommandWithProgressAndKillSwitch(
	targetID uint,
	heartbeat HeartbeatFunc,
	shouldStop StopRequestedFunc,
	progress ProgressFunc,
	name string,
	args ...string,
) error {
	cmd := exec.Command(name, args...)
	prepareCommand(targetID, cmd)

	var outBuf bytes.Buffer
	writer := progressWriter{progress: progress}
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %s: %w", name, err)
	}

	_, err := waitCommandWithKillSwitch(targetID, cmd, heartbeat, shouldStop, name, args, &outBuf)
	return err
}

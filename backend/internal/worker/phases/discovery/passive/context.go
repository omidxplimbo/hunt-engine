package passive

import "time"

// CommandRunner executes an external tool and returns its combined output.
type CommandRunner func(name string, args ...string) ([]byte, error)

// TimeoutCommandRunner executes an external tool with an optional timeout.
type TimeoutCommandRunner func(timeout time.Duration, name string, args ...string) ([]byte, error)

// Context carries the target-specific execution dependencies used by passive tools.
type Context struct {
	TargetID uint
	Domain   string

	RunCommand                    CommandRunner
	RunCombinedCommand            CommandRunner
	RunCombinedCommandWithTimeout TimeoutCommandRunner
}

type SourceResult struct {
	Subdomain string
	Source    string
}

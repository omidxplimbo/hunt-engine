package tools

import "io"

type Context struct {
	TargetID   uint
	RootDomain string
	TempDir    string

	CheckStop           func(targetID uint) bool
	RunCommand          func(targetID uint, name string, args ...string) ([]byte, error)
	RunCommandWithStdin func(targetID uint, stdin io.Reader, name string, args ...string) ([]byte, error)
}

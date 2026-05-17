package tools

import "fmt"

type CommandRunner func(targetID uint, name string, args ...string) ([]byte, error)
type StopChecker func(targetID uint) bool
type SubdomainNormalizer func(subdomain, rootDomain string) string

type Context struct {
	TargetID   uint
	RootDomain string
	TempDir    string

	RunCommand         CommandRunner
	CheckStop          StopChecker
	NormalizeSubdomain SubdomainNormalizer
}

func (ctx Context) ensureCommandRunner() error {
	if ctx.RunCommand == nil {
		return fmt.Errorf("discovery tool command runner is nil")
	}
	return nil
}

func (ctx Context) stopped() bool {
	if ctx.CheckStop == nil {
		return false
	}
	return ctx.CheckStop(ctx.TargetID)
}

func (ctx Context) normalize(value, rootDomain string) string {
	if ctx.NormalizeSubdomain == nil {
		return value
	}
	return ctx.NormalizeSubdomain(value, rootDomain)
}

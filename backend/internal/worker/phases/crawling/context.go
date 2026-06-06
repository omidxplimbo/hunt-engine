package crawling

import (
	"io"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/crawling/tools"
)

type Context struct {
	TargetID         uint
	RootDomain       string
	VirusTotalAPIKey string

	CheckStop         func(targetID uint) bool
	UpdateTargetPhase func(targetID uint, phase string)
	EnsureScanState   func(targetID uint) (*models.TargetScanState, error)
	ScanIsStepDone    func(targetID uint, module string, step string) bool
	ScanMarkRunning   func(targetID uint, module string, step string)
	ScanMarkStepDone  func(targetID uint, module string, step string)
	TriggerNextModule func(targetID uint, rootDomain string, currentModule string)

	RunCommand          func(targetID uint, name string, args ...string) ([]byte, error)
	RunCommandWithStdin func(targetID uint, stdin io.Reader, name string, args ...string) ([]byte, error)

	AfterCrawling func(targetID uint)
}

func (c Context) toolContext(tempDir string) tools.Context {
	return tools.Context{
		TargetID:            c.TargetID,
		RootDomain:          c.RootDomain,
		VirusTotalAPIKey:    c.VirusTotalAPIKey,
		TempDir:             tempDir,
		CheckStop:           c.CheckStop,
		RunCommand:          c.RunCommand,
		RunCommandWithStdin: c.RunCommandWithStdin,
	}
}

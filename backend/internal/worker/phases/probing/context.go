package probing

// Context contains the worker callbacks and settings required to run the
// probing phase without coupling this package back to the worker package.
type Context struct {
	TargetID   uint
	RootDomain string
	BatchSize  int

	CheckStop         func(targetID uint) bool
	UpdateTargetPhase func(targetID uint, phase string)
	EnsureScanState   func(targetID uint)
	ScanIsStepDone    func(targetID uint, phase, step string) bool
	ScanGetMeta       func(targetID uint) map[string]interface{}
	ScanSetMeta       func(targetID uint, meta map[string]interface{})
	ScanMarkRunning   func(targetID uint, phase, step string)
	ScanMarkStepDone  func(targetID uint, phase, step string)
	TriggerNextModule func(targetID uint, rootDomain, currentModule string)

	RunCommand   func(targetID uint, name string, args ...string) ([]byte, error)
	UpdateAssets func(targetID uint, results map[string]HTTPXResult)

	AfterProbing func(targetID uint)
}

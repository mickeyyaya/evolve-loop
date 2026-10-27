// Package looppreflight is the deterministic readiness gate `evolve loop` runs
// before the first cycle: it accumulates every check and halts iff any halts.
// See docs/architecture/packages/internal-looppreflight.md.
package looppreflight

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type CheckLevel int

const (
	LevelPass CheckLevel = iota
	LevelWarn
	LevelHalt
)

func (l CheckLevel) String() string {
	switch l {
	case LevelPass:
		return "pass"
	case LevelWarn:
		return "warn"
	case LevelHalt:
		return "halt"
	default:
		return "unknown"
	}
}

type CheckResult struct {
	Name    string
	Level   CheckLevel
	Message string
	Detail  string
}

type Result struct {
	Checks       []CheckResult
	ChecksPassed int
	ChecksTotal  int
	OverallLevel CheckLevel
	GeneratedAt  string
	CLIVersions  map[string]string
}

func (r Result) Halted() bool { return r.OverallLevel == LevelHalt }

var DefaultSpinePhases = []string{"build", "scout", "tdd", "audit", "intent", "triage"}

type Options struct {
	ProjectRoot string
	ProfileDir  string
	EvolveDir   string
	Stderr      io.Writer
	Now         func() time.Time

	SkipBoot   bool
	BootBudget time.Duration

	SpinePhases   []string
	FactoryKnown  func(name string) bool
	ContractKnown func(name string) bool
	ProfileLister func() ([]string, error)
	ProfileGetter func(name string) (profiles.Profile, error)
	DriverKnown   func(cli string) bool

	ProbeCLI      func(bin string) (doctor.Result, error)
	HostProbe     func() preflight.Profile
	SandboxMode   func() string
	DirWritable   func(dir string) bool
	DiskFreeBytes func(path string) (uint64, error)
	MinFreeBytes  uint64
	OrphanKill    swarm.TmuxKiller

	NestedFallbackStage config.Stage
	SandboxCanaryProbe  func() bool

	BootTester func(ctx context.Context, driver string, sandbox bool) BootOutcome

	SelfUpdateEvidence func(bin string) (bool, string, error)
	PinnedLister       func() ([]string, error)

	CLIHealthActive func() []clihealth.Entry

	VersionInventory func() map[string]string

	PhaseRoutingWarnings func() []string

	Routing func() (Routing, error)

	UsageEvidence func(driver string) string

	MCPConfig MCPConfigPort

	LatestRelease func(bin string) (string, error)
	ReleaseLag    ReleaseLagConfig
}

type resolved struct {
	projectRoot string
	profileDir  string
	evolveDir   string
	stderr      io.Writer
	now         func() time.Time
	skipBoot    bool
	bootBudget  time.Duration

	spinePhases   []string
	factoryKnown  func(string) bool
	contractKnown func(string) bool
	profileLister func() ([]string, error)
	profileGetter func(string) (profiles.Profile, error)
	driverKnown   func(string) bool

	probeCLI      func(string) (doctor.Result, error)
	hostProbe     func() preflight.Profile
	sandboxMode   func() string
	dirWritable   func(string) bool
	diskFreeBytes func(string) (uint64, error)
	orphanKill    swarm.TmuxKiller
	bootTester    func(context.Context, string, bool) BootOutcome

	nestedFallbackStage config.Stage
	sandboxCanaryProbe  func() bool

	selfUpdateEvidence func(string) (bool, string, error)
	pinnedLister       func() ([]string, error)
	cliHealthActive    func() []clihealth.Entry

	versionInventory func() map[string]string

	phaseRoutingWarnings func() []string

	mcpConfig MCPConfigPort

	latestRelease func(string) (string, error)
	releaseLag    ReleaseLagConfig

	routing *routingOnce
}

const DefaultBootBudget = 90 * time.Second

func resolve(opts Options) (resolved, error) {
	if opts.ProjectRoot == "" {
		return resolved{}, errors.New("looppreflight: ProjectRoot required")
	}
	o := resolved{
		projectRoot:   opts.ProjectRoot,
		profileDir:    opts.ProfileDir,
		evolveDir:     opts.EvolveDir,
		stderr:        opts.Stderr,
		now:           opts.Now,
		skipBoot:      opts.SkipBoot,
		bootBudget:    opts.BootBudget,
		spinePhases:   opts.SpinePhases,
		factoryKnown:  opts.FactoryKnown,
		contractKnown: opts.ContractKnown,
		profileLister: opts.ProfileLister,
		profileGetter: opts.ProfileGetter,
		driverKnown:   opts.DriverKnown,
		probeCLI:      opts.ProbeCLI,
		hostProbe:     opts.HostProbe,
		sandboxMode:   opts.SandboxMode,
		dirWritable:   opts.DirWritable,
		diskFreeBytes: opts.DiskFreeBytes,
		orphanKill:    opts.OrphanKill,
		bootTester:    opts.BootTester,

		nestedFallbackStage: opts.NestedFallbackStage,
		sandboxCanaryProbe:  opts.SandboxCanaryProbe,

		selfUpdateEvidence: opts.SelfUpdateEvidence,
		pinnedLister:       opts.PinnedLister,
		cliHealthActive:    opts.CLIHealthActive,

		versionInventory: opts.VersionInventory,

		phaseRoutingWarnings: opts.PhaseRoutingWarnings,
		mcpConfig:            defaultMCPConfigPort(opts.MCPConfig),
		latestRelease:        opts.LatestRelease,
		releaseLag:           defaultReleaseLagConfig(opts.ReleaseLag),
		routing:              onceRouting(opts.Routing),
	}
	if o.stderr == nil {
		o.stderr = io.Discard
	}
	if o.now == nil {
		o.now = time.Now
	}
	if o.bootBudget <= 0 {
		o.bootBudget = DefaultBootBudget
	}
	if o.evolveDir == "" {
		o.evolveDir = filepath.Join(o.projectRoot, ".evolve")
	}
	if len(o.spinePhases) == 0 {
		o.spinePhases = DefaultSpinePhases
	}
	if o.factoryKnown == nil {
		o.factoryKnown = func(name string) bool { _, ok := registry.For(name); return ok }
	}
	if o.contractKnown == nil {
		o.contractKnown = func(name string) bool { _, ok := phasecontract.For(name); return ok }
	}
	if o.driverKnown == nil {
		o.driverKnown = func(cli string) bool { _, ok := bridge.LookupDriver(cli); return ok }
	}
	if o.profileLister == nil || o.profileGetter == nil {
		l := profiles.NewFromDir(opts.ProfileDir)
		if o.profileLister == nil {
			o.profileLister = l.List
		}
		if o.profileGetter == nil {
			o.profileGetter = l.Get
		}
	}
	if o.probeCLI == nil {
		o.probeCLI = doctor.Probe
	}
	if o.sandboxMode == nil {
		o.sandboxMode = func() string { return "" }
	}
	if o.hostProbe == nil {
		o.hostProbe = func() preflight.Profile {
			return preflight.Probe(preflight.Options{
				ProjectRoot:    o.projectRoot,
				WorktreeBase:   policy.WorktreeBaseFor(o.projectRoot),
				SandboxCapable: preflight.MeasuredSandboxCapability,
			})
		}
	}
	fillProbeDefaults(&o)
	memoizeVersionInventory(&o)
	return o, nil
}

func fillProbeDefaults(o *resolved) {
	if o.dirWritable == nil {
		o.dirWritable = defaultDirWritable
	}
	if o.diskFreeBytes == nil {
		o.diskFreeBytes = defaultDiskFreeBytes
	}
	if o.orphanKill == nil {
		o.orphanKill = swarm.ExecTmuxKill
	}
	if o.bootTester == nil {
		o.bootTester = newDefaultBootTester(o.projectRoot, o.stderr)
	}
	if o.sandboxCanaryProbe == nil {
		o.sandboxCanaryProbe = defaultSandboxCanary(o.projectRoot)
	}
	if o.selfUpdateEvidence == nil {
		o.selfUpdateEvidence = defaultSelfUpdateEvidence
	}
	if o.pinnedLister == nil {
		o.pinnedLister = defaultPinnedLister
	}
	if o.cliHealthActive == nil {
		o.cliHealthActive = defaultCLIHealthActive(o.projectRoot)
	}
	if o.phaseRoutingWarnings == nil {
		o.phaseRoutingWarnings = defaultPhaseRoutingWarnings(o.projectRoot)
	}
}

func memoizeVersionInventory(o *resolved) {
	if o.versionInventory == nil {
		drivers := o.drivers
		o.versionInventory = func() map[string]string {
			seen := map[string]struct{}{}
			var bins []string
			for _, d := range drivers() {
				b := driverBinary(d)
				if _, dup := seen[b]; !dup {
					seen[b] = struct{}{}
					bins = append(bins, b)
				}
			}
			return captureVersionInventory(bins)
		}
	}
	rawInventory := o.versionInventory
	var (
		invOnce sync.Once
		memoInv map[string]string
	)
	o.versionInventory = func() map[string]string {
		if _, err := o.routed(); err != nil {
			return nil
		}
		invOnce.Do(func() {
			memoInv = rawInventory()
		})
		return memoInv
	}
}

func Run(opts Options) (Result, error) {
	o, err := resolve(opts)
	if err != nil {
		return Result{}, err
	}
	checks := []CheckResult{
		checkPipelineStructure(o),
		checkBaseDivergence(o),
		checkLLMCLIStatus(o),
		checkHostCapabilities(o),
		checkDiskSpace(o, opts.MinFreeBytes),
		checkCLIVersionFreeze(o),
		checkCLIHealth(o),
		checkCLIVersionDrift(o),
		checkBridgeBoot(o, opts.UsageEvidence),
		checkSandboxNestedFallback(o),
		checkPhaseRoutingWarnings(o),
		checkMCPConfig(o),
		checkReleaseLag(o),
		checkOverlayTierSelectors(o),
		checkCLIRouting(o),
	}
	r := finalize(checks, o.now())
	r.CLIVersions = o.versionInventory()
	return r, nil
}

func finalize(checks []CheckResult, now time.Time) Result {
	r := Result{
		Checks:       checks,
		ChecksTotal:  len(checks),
		OverallLevel: LevelPass,
		GeneratedAt:  now.UTC().Format(time.RFC3339),
	}
	for _, c := range checks {
		if c.Level == LevelPass {
			r.ChecksPassed++
		}
		if c.Level > r.OverallLevel {
			r.OverallLevel = c.Level
		}
	}
	return r
}

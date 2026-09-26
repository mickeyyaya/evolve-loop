// Package looppreflight is the deterministic readiness gate `evolve loop` runs
// before the first cycle: it accumulates every check and halts iff any halts.
// See docs/architecture/packages/internal-looppreflight.md.
package looppreflight

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

// CheckLevel is a check's severity, ordered so the worst of a set is the maximum.
type CheckLevel int

const (
	// LevelPass means the check found nothing wrong.
	LevelPass CheckLevel = iota
	// LevelWarn is a degraded but runnable condition; it never blocks.
	LevelWarn
	// LevelHalt is a readiness gap; the batch must not start.
	LevelHalt
)

// String returns the stable lowercase token used in JSON and the summary.
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

// CheckResult is one check's outcome: a one-line Message and a multi-line Detail.
type CheckResult struct {
	Name    string
	Level   CheckLevel
	Message string
	Detail  string
}

// Result is the accumulated outcome of a Run; OverallLevel is the maximum check level.
type Result struct {
	Checks       []CheckResult
	ChecksPassed int
	ChecksTotal  int
	OverallLevel CheckLevel
	GeneratedAt  string
	CLIVersions  map[string]string // CLI binary → version token
}

// Halted reports whether any check halted (the batch must not start).
func (r Result) Halted() bool { return r.OverallLevel == LevelHalt }

// DefaultSpinePhases are the phases every cycle dispatches; each needs a factory and a contract.
var DefaultSpinePhases = []string{"build", "scout", "tdd", "audit", "intent", "triage"}

// Options drives a Run; every nil seam defaults to the real implementation.
type Options struct {
	ProjectRoot string // required
	ProfileDir  string // read only by the default profile seams
	EvolveDir   string // default ProjectRoot/.evolve
	Stderr      io.Writer
	Now         func() time.Time

	SkipBoot   bool          // run the cheap checks but skip the real bridge boot
	BootBudget time.Duration // per-driver boot deadline; default DefaultBootBudget

	SpinePhases   []string // nil or empty → DefaultSpinePhases
	FactoryKnown  func(name string) bool
	ContractKnown func(name string) bool
	ProfileLister func() ([]string, error)
	ProfileGetter func(name string) (profiles.Profile, error)
	DriverKnown   func(cli string) bool

	ProbeCLI      func(bin string) (doctor.Result, error)
	HostProbe     func() preflight.Profile
	SandboxMode   func() string // the EVOLVE_SANDBOX value
	DirWritable   func(dir string) bool
	DiskFreeBytes func(path string) (uint64, error) // an error skips the low-disk warning
	OrphanKill    swarm.TmuxKiller                  // used by the boot orphan sweep

	// NestedFallbackStage is the sandbox.nested_fallback stage; the zero value (StageOff) disables the canary.
	NestedFallbackStage config.Stage
	// SandboxCanaryProbe reports true when the outer environment denied an out-of-allowlist write.
	SandboxCanaryProbe func() bool

	// BootTester boots one *-tmux driver's REPL without a prompt and returns the bridge exit code and scrollback.
	BootTester func(ctx context.Context, driver string, sandbox bool) (rc int, scrollback string)

	// SelfUpdateEvidence reports whether bin self-updates on launch; an error means unverifiable (Warn).
	SelfUpdateEvidence func(bin string) (bool, string, error)
	// PinnedLister lists version-frozen package names; an error is ambiguity (Warn).
	PinnedLister func() ([]string, error)

	// CLIHealthActive lists the CLI families with an active bench.
	CLIHealthActive func() []clihealth.Entry

	// VersionInventory returns the current CLI binary → version map.
	VersionInventory func() map[string]string

	// PhaseRoutingWarnings returns the user-phase specs phasespec dropped while merging the catalog.
	PhaseRoutingWarnings func() []string
}

// resolved is Options with every seam and default filled in.
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
	bootTester    func(context.Context, string, bool) (int, string)

	nestedFallbackStage config.Stage
	sandboxCanaryProbe  func() bool

	selfUpdateEvidence func(string) (bool, string, error)
	pinnedLister       func() ([]string, error)
	cliHealthActive    func() []clihealth.Entry

	versionInventory func() map[string]string

	phaseRoutingWarnings func() []string
}

// DefaultBootBudget is the per-driver REPL boot deadline, matching `evolve doctor boot`.
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
		o.sandboxMode = func() string { return os.Getenv("EVOLVE_SANDBOX") }
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
	if o.versionInventory == nil {
		// Resolved after the profile seams so the closure binds the final lister and getter.
		lister, getter := o.profileLister, o.profileGetter
		o.versionInventory = func() map[string]string {
			seen := map[string]struct{}{}
			var bins []string
			for _, d := range distinctDrivers(lister, getter) {
				b := driverBinary(d)
				if _, dup := seen[b]; !dup {
					seen[b] = struct{}{}
					bins = append(bins, b)
				}
			}
			return captureVersionInventory(bins)
		}
	}
	return o, nil
}

// Run executes every check; a failed check halts in the Result, and err reports only harness faults.
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
		checkCLIVersionFreeze(o),
		checkCLIHealth(o),
		checkCLIVersionDrift(o),
		checkBridgeBoot(o),
		checkSandboxNestedFallback(o),
		checkPhaseRoutingWarnings(o),
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

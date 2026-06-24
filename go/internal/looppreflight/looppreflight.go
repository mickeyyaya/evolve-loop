// Package looppreflight is the pre-batch environment-readiness gate for
// `evolve loop`. It runs BEFORE the first cycle dispatches and verifies the
// pipeline can actually run: every spine phase has a factory + deliverable
// contract, the profiles load and name known drivers, the LLM CLIs are present,
// the host has the capabilities the bridge needs, and — the check that matters
// most — each configured *-tmux CLI's REPL really boots.
//
// Motivation (cycle-258): a 3-cycle batch churned ~30 min before anyone
// discovered the bridge could not boot the CLI (exit 80 = ExitREPLBootTimeout).
// This gate catches that at batch start and aborts with a clear diagnostic so a
// doomed run never costs a cycle.
//
// Design: a DETERMINISTIC host-side gate, NOT an LLM agent phase — an env-check
// agent would have to run THROUGH the very bridge it is meant to verify
// (chicken-and-egg), and environment verification is deterministic work. It
// mirrors the releasepreflight blueprint (Options → Run → Result, nil→default
// seams) but ACCUMULATES every check result before deciding, so the operator
// sees all problems at once. The overall verdict halts iff any check halts.
package looppreflight

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// CheckLevel is a check's severity. Ordered so the worst level across a set of
// checks is simply the maximum (LevelHalt > LevelWarn > LevelPass).
type CheckLevel int

const (
	// LevelPass — the check found nothing wrong.
	LevelPass CheckLevel = iota
	// LevelWarn — a degraded-but-runnable condition; surfaced, never blocking.
	LevelWarn
	// LevelHalt — a hard readiness gap; the batch must not start.
	LevelHalt
)

// String renders the level as the stable lowercase token used in JSON and the
// human summary.
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

// CheckResult is one check's outcome. Message is a one-line headline; Detail
// carries the multi-line diagnostic (the gap list, the probe trail, the boot
// scrollback tail) the operator needs to act.
type CheckResult struct {
	Name    string
	Level   CheckLevel
	Message string
	Detail  string
}

// Result is the accumulated outcome of a Run. OverallLevel is the max level
// across Checks; Halted() reports whether the batch must abort.
type Result struct {
	Checks       []CheckResult
	ChecksPassed int
	ChecksTotal  int
	OverallLevel CheckLevel
	GeneratedAt  string
	CLIVersions  map[string]string // CLI binary → version string; populated by drift check
}

// Halted reports whether any check halted (the batch must not start).
func (r Result) Halted() bool { return r.OverallLevel == LevelHalt }

// DefaultSpinePhases are the agent phases a real cycle always dispatches; each
// must have BOTH a registered factory and a deliverable contract or the loop
// cannot run.
var DefaultSpinePhases = []string{"build", "scout", "tdd", "audit", "intent", "triage"}

// Options drives a Run. Seam fields default to real implementations when nil so
// production callers pass only the paths, while tests inject deterministic
// lookups and never touch the real registry/driver/profile-dir state.
type Options struct {
	ProjectRoot string // required; the harness faults if empty
	ProfileDir  string // .evolve/profiles dir; drives the default profile seams
	EvolveDir   string // .evolve dir; used by later host-capability checks
	Stderr      io.Writer
	Now         func() time.Time

	SkipBoot   bool          // run the cheap checks but skip the real bridge boot
	BootBudget time.Duration // per-driver boot deadline (default 90s)

	// Pipeline-structure seams.
	// SpinePhases: nil OR an explicit empty slice both fall back to
	// DefaultSpinePhases. (The integration test passes []string{} to skip the
	// phase-wiring check when the test binary has no phases registered.)
	SpinePhases   []string                                    // default DefaultSpinePhases
	FactoryKnown  func(name string) bool                      // default registry.For
	ContractKnown func(name string) bool                      // default phasecontract.For
	ProfileLister func() ([]string, error)                    // default profiles.NewFromDir(ProfileDir).List
	ProfileGetter func(name string) (profiles.Profile, error) // default ...Get
	DriverKnown   func(cli string) bool                       // default bridge.LookupDriver

	// CLI / host-capability seams.
	ProbeCLI      func(bin string) (doctor.Result, error) // default doctor.Probe
	HostProbe     func() preflight.Profile                // default preflight.Probe(ProjectRoot)
	DirWritable   func(dir string) bool                   // default real touch-probe
	DiskFreeBytes func(path string) (uint64, error)       // default statfs; error → disk check skipped

	// BootTester really boots one *-tmux driver's REPL (boot-only, no prompt)
	// under the supplied context and returns its bridge exit code + scrollback.
	// Default wraps bridge.BootSmokeTest (mirrors `evolve doctor boot`).
	BootTester func(ctx context.Context, driver string, sandbox bool) (rc int, scrollback string)

	// CLI-version-freeze seams (ADR-0044 C5).
	// SelfUpdateEvidence reports whether bin self-updates on launch, plus the
	// host evidence found. A non-nil error means the evidence was
	// UNVERIFIABLE (ambiguity → Warn), distinct from a clean absence.
	// Default: the known-updater registry (codex → ~/.codex/version.json).
	SelfUpdateEvidence func(bin string) (bool, string, error)
	// PinnedLister lists version-frozen package names. Default:
	// `brew list --pinned`; an error is treated as ambiguity (Warn).
	PinnedLister func() ([]string, error)

	// CLIHealthActive lists CLI families with an ACTIVE bench (classified
	// transient wall, e.g. rate_limit). Default reads
	// .evolve/cli-health.json via the clihealth store.
	CLIHealthActive func() []clihealth.Entry

	// VersionInventory returns the current CLI version map (bin→version string).
	// Default: captures versions of all distinct profile CLI binaries via
	// captureVersionInventory. Tests inject a deterministic map closure to avoid
	// shelling out to real CLIs.
	VersionInventory func() map[string]string
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
	dirWritable   func(string) bool
	diskFreeBytes func(string) (uint64, error)
	bootTester    func(context.Context, string, bool) (int, string)

	selfUpdateEvidence func(string) (bool, string, error)
	pinnedLister       func() ([]string, error)
	cliHealthActive    func() []clihealth.Entry

	versionInventory func() map[string]string
}

// DefaultBootBudget is the per-driver REPL boot deadline (mirrors the
// `evolve doctor boot` 90s timeout).
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
		dirWritable:   opts.DirWritable,
		diskFreeBytes: opts.DiskFreeBytes,
		bootTester:    opts.BootTester,

		selfUpdateEvidence: opts.SelfUpdateEvidence,
		pinnedLister:       opts.PinnedLister,
		cliHealthActive:    opts.CLIHealthActive,

		versionInventory: opts.VersionInventory,
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
	if o.hostProbe == nil {
		o.hostProbe = func() preflight.Profile {
			return preflight.Probe(preflight.Options{ProjectRoot: o.projectRoot, WorktreeBase: policy.WorktreeBaseFor(o.projectRoot)})
		}
	}
	if o.dirWritable == nil {
		o.dirWritable = defaultDirWritable
	}
	if o.diskFreeBytes == nil {
		o.diskFreeBytes = defaultDiskFreeBytes
	}
	if o.bootTester == nil {
		o.bootTester = newDefaultBootTester(o.projectRoot, o.stderr)
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
	if o.versionInventory == nil {
		// Capture versions lazily so the closure sees the final resolved state.
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

// Run executes every readiness check, accumulates the results, and returns the
// combined verdict. The error return is reserved for HARNESS faults (e.g. an
// empty ProjectRoot); a failed check lives in the Result as a halt, never an
// error — the caller inspects Result.Halted().
func Run(opts Options) (Result, error) {
	o, err := resolve(opts)
	if err != nil {
		return Result{}, err
	}
	checks := []CheckResult{
		checkPipelineStructure(o),
		checkLLMCLIStatus(o),
		checkHostCapabilities(o),
		checkCLIVersionFreeze(o),
		checkCLIHealth(o),
		checkCLIVersionDrift(o),
		checkBridgeBoot(o),
	}
	r := finalize(checks, o.now())
	r.CLIVersions = o.versionInventory()
	return r, nil
}

// finalize folds the per-check results into the overall Result.
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

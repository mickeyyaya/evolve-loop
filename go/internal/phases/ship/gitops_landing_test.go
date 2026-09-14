package ship

// gitops_landing_test.go — ADR-0103 unit 07: the seam between the ship phase
// and the landing leaf. One wired construction, the once-guard projected
// through pushWithRepair, the Center threaded from Config to Options, the
// happy path silent on the stream, the layout spelled once.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

// Test 33 — `landing.New(` is spelled in exactly ONE non-test file of the
// module (the seam); the leaf itself spells `New(`. moduleRoot is three
// levels up (the carryover copy hard-codes two — a copy at the wrong depth
// scans the wrong root and passes vacuously, so the count of scanned files
// is asserted too).
func TestLanding_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/phases/ship/gitops_landing.go"
	offenders, scanned := nonTestSourcesMentioning(t, "landing.New(", onlySite)
	if len(offenders) > 0 {
		t.Errorf("landing.New( belongs to ONE non-test file (%s); these use it too: %v", onlySite, offenders)
	}
	if scanned < 800 {
		t.Errorf("the scan must cover the whole module (%d non-test files scanned — the root is three levels up)", scanned)
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// landing leaf and the one allowed site whose source contains needle, and
// how many files it scanned.
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) ([]string, int) {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	scanned := 0
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/phases/ship/landing/") {
			return nil
		}
		scanned++
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders, scanned
}

// Test 34 (the flip of the characterization) — a failed tracked-binary reset
// no longer writes the raw `[ship] WARN:` line to opts.Stderr: the Center on
// Options.Signals sees SHIP_LANDING_BINARY_RESET_FAILED under phase "ship"
// with the run identity, and the merge still runs.
func TestWorktreeShipIntegrate_BinaryResetFailureIsTheLandingCodeNotStderr(t *testing.T) {
	opts, _, stderr := pinOptions(t, ClassCycle)
	opts.RunID = "run-7"
	c, got := recordingCenter()
	opts.Signals = c
	r := newArgvRecorder(opts)
	r.on("checkout HEAD -- go/evolve", scriptedCall{exit: 1})
	scriptGreenPost(r)
	res := &RunResult{}
	s := newWorktreeShip(context.Background(), opts, res, pinBranch, filepath.Join(opts.ProjectRoot, "wt"))
	s.cycleBranch = pinCycleBranch
	if err := s.integrate(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "could not reset go/evolve") {
		t.Errorf("the raw stderr line is gone (replaced by the code):\n%s", stderr.String())
	}
	if len(*got) != 1 || (*got)[0].Code != landing.CodeBinaryResetFailed || (*got)[0].Phase != phaseName || (*got)[0].Cycle != 7 || (*got)[0].RunID != "run-7" {
		t.Errorf("the Center sees the landing code under the ship phase with the run identity: %+v", *got)
	}
	if len(r.calls) < 2 || !strings.HasPrefix(r.calls[1].Argv, "merge --ff-only") {
		t.Errorf("the merge still runs: %v", r.argvs())
	}
}

// Test 35 — pushWithRepair projects the host's once-guard: after a declined
// repair the ledger, res.RepairAttempted and res.RepairOutcome are written
// even though an error is returned, res.CommitSHA is untouched; a second
// rejection in the SAME Options returns the original error with zero probes
// (the stage re-run would otherwise repair twice); on success res.CommitSHA
// is the landed head; PhaseResponse.Signals through addRepairSignals are
// unchanged.
func TestPushWithRepair_WritesBackTheLedgerUnconditionally(t *testing.T) {
	opts, _, _ := pinOptions(t, ClassCycle)
	r := newArgvRecorder(opts)
	r.on("push origin "+pinBranch, scriptedCall{exit: 1})
	r.on("fetch origin "+pinBranch, scriptedCall{exit: 1})
	res := &RunResult{CommitSHA: "untouched"}
	err := pushWithRepair(context.Background(), opts, res, pinBranch, landing.SiteDirect)
	if err == nil {
		t.Fatal("a declined repair returns the rejection")
	}
	if res.RepairAttempted != "GIT_PUSH_REJECTED" || res.RepairOutcome != "declined" || res.CommitSHA != "untouched" {
		t.Errorf("result after a declined repair: %+v", res)
	}
	if !opts.repairAttempted[core.CodeGitPushRejected] {
		t.Error("the once-guard is written back to the host's ledger")
	}
	probes := len(r.calls)
	if err := pushWithRepair(context.Background(), opts, res, pinBranch, landing.SiteDirect); err == nil {
		t.Fatal("the second rejection returns the rejection")
	}
	if len(r.calls) != probes+1 {
		t.Errorf("a second rejection in the same Run makes zero repair probes: %v", r.argvs()[probes:])
	}
	signals := map[string]any{}
	addRepairSignals(signals, *res)
	if signals["ship.repair_attempted"] != "GIT_PUSH_REJECTED" || signals["ship.repair_outcome"] != "declined" {
		t.Errorf("PhaseResponse.Signals unchanged: %v", signals)
	}
	fresh, _, _ := pinOptions(t, ClassCycle)
	r2 := newArgvRecorder(fresh)
	scriptGreenPost(r2)
	res2 := &RunResult{}
	if err := pushWithRepair(context.Background(), fresh, res2, pinBranch, landing.SiteDirect); err != nil {
		t.Fatal(err)
	}
	if res2.CommitSHA != pinHead || res2.RepairAttempted != "" || fresh.repairAttempted != nil {
		t.Errorf("a green push sets CommitSHA and touches no ledger: %+v %v", res2, fresh.repairAttempted)
	}
}

// Test 36 — a recording Center threaded through Options.Signals on a
// scripted green worktree ship (run(): resolve, lock, preflight, stage,
// commit, integrate) records ZERO events from module ship: the landing adds
// nothing to the stream on the happy path.
func TestShipFromWorktreeGreen_StreamIsByteIdenticalApartFromTheDeclaredCodes(t *testing.T) {
	opts, _, _ := pinOptions(t, ClassCycle)
	c, got := recordingCenter()
	opts.Signals = c
	r := newArgvRecorder(opts)
	wt := filepath.Join(opts.ProjectRoot, "wt")
	mustMkdir(t, wt)
	r.on("-C "+wt+" symbolic-ref --short HEAD", scriptedCall{stdout: pinCycleBranch + "\n"})
	r.on("-C "+wt+" diff --cached --quiet", scriptedCall{exit: 1})
	scriptGreenPost(r)
	res := &RunResult{}
	if err := newWorktreeShip(context.Background(), opts, res, pinBranch, wt).run(); err != nil {
		t.Fatalf("green worktree ship: %v\n%s", err, strings.Join(res.Logs, "\n"))
	}
	if res.CommitSHA != pinHead || !containsLog(*res, "[ship] OK: pushed to origin/main") {
		t.Errorf("the ship landed: %+v", res)
	}
	if len(*got) != 0 {
		t.Errorf("a green ship emits nothing from module ship: %+v", *got)
	}
}

// Test 37 — shipOptions copies the Phase's Center into Options.Signals and
// (*Phase).signalsWired reports it (the cycle-1064 trap: a Config field the
// translation forgets is silently never wired).
func TestShipOptions_ThreadsSignals(t *testing.T) {
	c, _ := recordingCenter()
	p := New(Config{Runner: execRunner, Signals: c})
	if !p.signalsWired() {
		t.Fatal("signalsWired must be true when Config.Signals is set")
	}
	opts := p.shipOptions(core.PhaseRequest{ProjectRoot: t.TempDir(), Cycle: 7}, "msg")
	if opts.Signals != c {
		t.Error("shipOptions must thread the Center into Options.Signals")
	}
	if New(Config{Runner: execRunner}).signalsWired() {
		t.Error("signalsWired must be false without a Center (the registry factory root)")
	}
}

// Test 38 — the run-workspace layout is spelled once: no non-test ship source
// spells "ship-binding.json" (dossier.ShipBindingFile is the spelling) and
// neither gitops.go nor native.go spells "cycle-%d" (core.RunWorkspacePath).
func TestShipBindingLayout_IsSpelledOnce(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	cycleDir := regexp.MustCompile(`"cycle-%d"`)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), `"ship-binding.json"`) {
			t.Errorf("%s spells the binding file name; dossier.ShipBindingFile is the SSOT", name)
		}
		if (name == "gitops.go" || name == "native.go") && cycleDir.Match(src) {
			t.Errorf("%s spells the run-workspace dir; core.RunWorkspacePath is the SSOT", name)
		}
	}
}

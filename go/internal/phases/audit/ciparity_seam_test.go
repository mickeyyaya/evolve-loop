package audit

// ciparity_seam_test.go — ADR-0103 unit 14: the host seam (ciparity.go) that
// builds the CI-parity gates per call from the package-var seams, projects
// the request, keeps the five Strangler facades, and threads the Signal
// Center from the composition root through Config.Signals / WithSignals.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// Test 33 — ciparitygate.New( is spelled in exactly ONE non-test file (the
// seam), and the five Center-less facades have NO production caller (one
// would drop the unit's events silently).
func TestCIParityGates_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/phases/audit/ciparity.go"
	if offenders := nonTestSourcesMentioning(t, "ciparitygate.New(", onlySite); len(offenders) > 0 {
		t.Errorf("ciparitygate.New( belongs to ONE non-test file (%s); these use it too: %v", onlySite, offenders)
	}
	facades := regexp.MustCompile(`\b(goVetCheckDefault|acsDurableCheckDefault|integrationTierCheckDefault|apicoverEnforceChangedDefault|apicoverNewPackageGraduationDefault)\b`)
	if offenders := nonTestSourcesMatching(t, facades, onlySite); len(offenders) > 0 {
		t.Errorf("the five *Default facades are Center-less, for the by-name tests only; these non-test files reference one (a value reference wires it into production): %v", offenders)
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files outside the
// leaf and the one allowed site whose source contains needle
// (core/carryover_lifecycle_test.go idiom).
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	return nonTestSourcesWhere(t, allowed, func(body string) bool { return strings.Contains(body, needle) })
}

// nonTestSourcesMatching is nonTestSourcesMentioning over a regexp, applied to
// the CODE lines only (comments elsewhere may name a facade as history).
func nonTestSourcesMatching(t *testing.T, re *regexp.Regexp, allowed string) []string {
	t.Helper()
	return nonTestSourcesWhere(t, allowed, func(body string) bool {
		for _, line := range strings.Split(body, "\n") {
			if code, _, _ := strings.Cut(line, "//"); re.MatchString(code) {
				return true
			}
		}
		return false
	})
}

func nonTestSourcesWhere(t *testing.T, allowed string, hit func(body string) bool) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/phases/audit/ciparitygate/") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if hit(string(body)) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// recordingCenter returns a Center whose events are appended to the slice.
func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

// tierRedThenGreenRunner is a runner where the integration tier is red then
// green and every other CI command exits 0 — so a whole audit Run through the
// production constructor reaches the flake-absorbed arm.
func tierRedThenGreenRunner() sysexec.RunFunc {
	tierRuns := 0
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "test" && args[1] == "-race" {
			tierRuns++
			if tierRuns == 1 {
				_, _ = io.WriteString(so, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n")
				return 1, nil
			}
		}
		return 0, nil
	}
}

// Test 34 — WithSignals on the production constructor reaches the gates: a
// whole audit Run over a red-then-green tier records the flake diagnostic
// (text unchanged) AND the Center holds AUDIT_CIPARITY_TIER_FLAKE_ABSORBED
// stamped with the request's cycle and the audit phase; NewDefault (the
// registry path) and New(Config{}) are the declared Null Object.
func TestNewDefaultWithStageCompactSpec_WithSignalsReachesTheGates_NewDefaultIsNullObject(t *testing.T) {
	req := productionRunFixture(t, 7)
	withFakeRunner(t, tierRedThenGreenRunner())
	rec, events := recordingCenter()
	phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: passReport}, fakePromptsFS("body"),
		config.StageOff, false, nil, WithSignals(func() *signalcenter.Center { return rec }))
	if !phase.SignalsWired() {
		t.Fatal("the option must reach the gates: SignalsWired false")
	}
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !hasDiagContaining(resp.Diagnostics, "contention flake absorbed") {
		t.Errorf("the flake WARN diagnostic text is unchanged; got %+v", resp.Diagnostics)
	}
	var flake []signalcenter.Event
	for _, e := range *events {
		if e.Code == ciparitygate.CodeTierFlakeAbsorbed {
			flake = append(flake, e)
		}
	}
	if len(flake) != 1 || flake[0].Cycle != req.Cycle || flake[0].Phase != string(core.PhaseAudit) || flake[0].Module != signalcenter.ModuleAudit {
		t.Fatalf("the Center holds ONE stamped AUDIT_CIPARITY_TIER_FLAKE_ABSORBED: %+v", flake)
	}
	if NewDefault(&fakeBridge{}, fakePromptsFS("body")).SignalsWired() {
		t.Error("NewDefault (the registry root) is the declared Null Object")
	}
	if New(Config{}).SignalsWired() {
		t.Error("New(Config{}) is the Null Object")
	}
	if New(Config{Signals: func() *signalcenter.Center { return rec }}).SignalsWired() != true {
		t.Error("Config.Signals is stored by New")
	}
}

// passReport is the auditor's artifact for a whole Run through the production
// constructor.
const passReport = "# Audit Report\n\n## Verdict\n**PASS**\n"

// productionRunFixture builds what a whole audit Run through the production
// constructor needs: a go-module worktree with a build handoff touching one
// Go package (so the scope decision says run) and a workspace holding a clean
// ACS verdict; the request names them all.
func productionRunFixture(t *testing.T, cycle int) core.PhaseRequest {
	t.Helper()
	root, _ := goWorktree(t)
	ws := filepath.Join(t.TempDir(), "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeACSVerdict(t, ws, 0)
	buildRun := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(buildRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildRun, "handoff-build.json"), []byte(`{"thrusts":[{"files_modified":["go/internal/p/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.PhaseRequest{Cycle: cycle, ProjectRoot: root, Worktree: root, Workspace: ws, WorktreeVerified: true}
}

// Test 38 (review fold — architecture MEDIUM 1) — an Option over the full
// Config is honoured in FULL: a hook an Option sets through the production
// constructor is the hook the phase runs (never post-clobbered by the
// CI-parity wiring, which fills only the hooks no Option set), while the
// hooks it left nil are still the wired gates (go vet runs through the runner).
func TestNewDefaultWithStageCompactSpec_AnOptionSettingAHookIsHonoured(t *testing.T) {
	req := productionRunFixture(t, 8)
	vetRuns, tierRuns := 0, 0
	withFakeRunner(t, func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		switch {
		case args[0] == "vet":
			vetRuns++
		case len(args) > 1 && args[0] == "test" && args[1] == "-race":
			tierRuns++
		}
		return 0, nil
	})
	hookCalls := 0
	fakeTier := func(core.PhaseRequest) ([]string, error) { hookCalls++; return nil, errors.New("fake tier hook ran") }
	phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: passReport}, fakePromptsFS("body"),
		config.StageOff, false, nil, func(cfg *Config) { cfg.CheckIntegrationTier = fakeTier })
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hookCalls != 1 || tierRuns != 0 || !hasDiagContaining(resp.Diagnostics, "fake tier hook ran") {
		t.Fatalf("the Option's hook must be the one the phase runs: hookCalls=%d tierRuns=%d diagnostics=%+v", hookCalls, tierRuns, resp.Diagnostics)
	}
	if vetRuns != 1 {
		t.Fatalf("the hooks the Option left nil are still the wired gates: go vet ran %d times", vetRuns)
	}
}

// Test 35 — the facades read the package-var seams PER CALL: a seam that has
// already run a gate (so a lazily cached construction would be frozen) still
// observes a runner and a budget swapped afterwards.
func TestCIParityFacades_ReadThePackageVarSeamsPerCall(t *testing.T) {
	req := tierFixture(t)
	ci := ciParity{}
	withFakeRunner(t, fakeRunFunc(0, "", "", nil))
	if off, err := ci.goVet(req); off != nil || err != nil {
		t.Fatalf("first call: (%v, %v)", off, err)
	}
	var deadlines []time.Time
	calls := 0
	withFakeRunner(t, killedAtDeadline(func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		calls++
		d, _ := ctx.Deadline()
		deadlines = append(deadlines, d)
		if calls == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestSlowRed (0.00s)\n")
			return 1, nil
		}
		_, _ = io.WriteString(so, "partial toolchain chatter, no verdict lines\nsignal: killed\n")
		return -1, nil
	}))
	oldBudget := integrationTierTimeout
	integrationTierTimeout = time.Nanosecond
	t.Cleanup(func() { integrationTierTimeout = oldBudget })
	off, err := ci.integrationTier(req)
	if calls != 2 || off != nil || err == nil || !strings.Contains(err.Error(), "exceeded its 1ns budget") {
		t.Fatalf("the swapped runner and budget must be seen by a seam built earlier: calls=%d (%v, %v)", calls, off, err)
	}
	for i, d := range deadlines {
		if d.IsZero() || d.After(time.Now()) {
			t.Errorf("attempt %d ran under the shrunk 1 ns budget, deadline %v", i+1, d)
		}
	}
}

// Test 36 (the JSON half of audit/ciparity_unit_test.go:162-182, kept in the
// host) — the locator's handoff shape and its two file names.
func TestChangedPackagesForAudit_HandoffShape(t *testing.T) {
	if pkgs, derivable := changedPackagesForAudit("", 1); pkgs != nil || derivable {
		t.Errorf("no root: (%v, %v)", pkgs, derivable)
	}
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		root, _ := goWorktree(t)
		runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(`{"thrusts":[{"files_modified":["go/internal/p/x.go"]}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		pkgs, derivable := changedPackagesForAudit(root, 1)
		if !derivable || len(pkgs) != 1 || pkgs[0] != "./internal/p/..." {
			t.Errorf("%s: (%v, %v), want ([./internal/p/...], true)", name, pkgs, derivable)
		}
	}
}

// Test 37 — through the FACADES, a mixed env-exclusive scope and a deferred
// graduation write ZERO bytes to os.Stderr (the two declared replacements,
// measured against the pre-move stderr golden).
func TestCIParity_WritesNothingToStderr(t *testing.T) {
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "testonly"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "testonly", "only_test.go"), []byte("package testonly\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/testonly/only_test.go","go/internal/bridge/b.go","go/internal/prompts/p.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	withFakeRunner(t, fakeRunFunc(0, "", "", nil))
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	req := core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1}
	off, gerr := apicoverNewPackageGraduationDefault(req)
	toff, terr := integrationTierCheckDefault(req)
	os.Stderr = orig
	_ = w.Close()
	got, _ := io.ReadAll(r)
	if gerr != nil || len(off) != 0 || terr != nil || toff != nil {
		t.Fatalf("fixtures: graduation (%v, %v), tier (%v, %v)", off, gerr, toff, terr)
	}
	if want, _ := os.ReadFile(filepath.Join("ciparitygate", "testdata", "stderr.golden.txt")); len(got) != 0 || len(want) == 0 {
		t.Fatalf("the two stderr lines (golden %d bytes) are replaced by codes; stderr now %q", len(want), got)
	}
}

// Test 41 (review fold — go MINOR 2, the Q13 consumer pin) — the host's
// applyCIGate keys off len(offenders), not nilness (gates.go:66): the
// graduation gate's empty-but-non-nil list for an all-deferred cycle is
// PASS at the consumer — no override, no diagnostic — while one offender FAILs.
func TestApplyCIGate_EmptyNonNilOffendersIsPass(t *testing.T) {
	for name, tc := range map[string]struct {
		offenders []string
		overrides int
	}{"empty non-nil (Q13)": {[]string{}, 0}, "nil": {nil, 0}, "one offender": {[]string{"x"}, 1}} {
		a := &auditClassification{req: core.PhaseRequest{Cycle: 1}, verdict: core.VerdictPASS}
		a.applyCIGate(func(core.PhaseRequest) ([]string, error) { return tc.offenders, nil }, "apicover new-package graduation gate", "%d ungraduated: %s")
		if len(a.overrodeBy) != tc.overrides || len(a.diagnostics) != tc.overrides {
			t.Errorf("%s: overrodeBy=%v diagnostics=%+v, want %d override(s)", name, a.overrodeBy, a.diagnostics, tc.overrides)
		}
	}
}

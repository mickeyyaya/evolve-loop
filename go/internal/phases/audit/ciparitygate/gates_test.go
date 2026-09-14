package ciparitygate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 10 — the ten codes are registered under module audit with docs, the
// five budgets and the twelve allowlist keys are the pre-extraction values.
func TestCodes_TenRegisteredUnderModuleAuditWithDocs(t *testing.T) {
	codes := []signalcenter.Code{CodeChangeSetUnderivable, CodeGateStepFailed, CodeGateFailed, CodeTierEnvExclusiveSkipped,
		CodeTierFlakeAbsorbed, CodeTierDeadlineNoVerdict, CodeTierRetakeExecFailed, CodeTierLockUnavailable, CodeTierLogWriteFailed, CodeGraduationDeferred}
	for _, c := range codes {
		owner, ok := signalcenter.IsRegistered(c)
		if !ok || owner != signalcenter.ModuleAudit || !c.BelongsTo(signalcenter.ModuleAudit) {
			t.Errorf("%s: registered=%v owner=%q", c, ok, owner)
		}
	}
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("registry conflicts: %+v", conflicts)
	}
	want := Timeouts{GoVet: 4 * time.Minute, ACSDurable: 8 * time.Minute, Apicover: 8 * time.Minute, TierAttempt: 15 * time.Minute, TierLockWait: 5 * time.Minute}
	if DefaultTimeouts() != want {
		t.Errorf("DefaultTimeouts() = %+v, want %+v", DefaultTimeouts(), want)
	}
	if got := strings.Join(tierEnvAllowlist, " "); got != golden(t, "argv.golden.txt")["tier.env_allowlist"] {
		t.Errorf("allowlist drifted: %q", got)
	}
}

// Test 11 — Request has exactly the four declared fields: a positional
// literal is a build break when a field is added, so the host projection is
// revisited instead of silently passing a zero value. The two root rules are
// distinct and named.
func TestRequest_HasExactlyTheDeclaredFields(t *testing.T) {
	r := Request{7, "/project", "/worktree", "/workspace"}
	if r.root() != "/worktree" || r.lockRoot() != "/project" {
		t.Fatalf("root()=%q lockRoot()=%q — worktree-first for the module, project-first for the lock", r.root(), r.lockRoot())
	}
	if (Request{1, "/p", "", ""}).root() != "/p" || (Request{1, "", "/w", ""}).lockRoot() != "/w" {
		t.Fatal("each root falls back to the other when its preferred field is empty")
	}
	if moduleDir("") != "" || moduleDir(t.TempDir()) != "" {
		t.Fatal("no root / no go.mod → no module dir")
	}
	root, goDir := goWorktree(t)
	if moduleDir(root) != goDir {
		t.Fatalf("moduleDir(%s) = %q, want the go/ module", root, moduleDir(root))
	}
}

// enforcedFixture is a go module with ./internal/p enforced and holding src.
func enforcedFixture(t *testing.T, src string) (root, goDir string) {
	t.Helper()
	root, goDir = goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, goDir
}

// pipelineRunner fakes the toolchain forks: `go list` answers with the
// fixture's package dir; every other command exits with code and out.
func pipelineRunner(goDir string, code int, out string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if args[0] == "list" {
			_, _ = io.WriteString(so, filepath.Join(goDir, "internal", "p")+"\n")
			return 0, nil
		}
		_, _ = io.WriteString(so, out)
		return code, nil
	}
}

// Test 31 — (a) a clean five-gate run emits NOTHING and writes nothing to
// stderr; (b) every FAIL emits exactly one GATE_FAILED with gate, cause,
// offenders == len and first == offenders[0].
func TestGates_CleanRunEmitsNothing_AndEveryFailEmitsOneGateFailed(t *testing.T) {
	root, goDir := enforcedFixture(t, "package p\n\nfunc helper() {}\n")
	g, events := observed(t, pipelineRunner(goDir, 0, ""), fixedSet("./internal/p/..."))
	req := tierRequest(root, t.TempDir())
	gates := map[string]func(Request) ([]string, error){"vet": g.GoVet, "acs": g.ACSDurable, "tier": g.IntegrationTier, "enforce": g.ApicoverEnforce, "graduation": g.ApicoverGraduation}
	out := captureStderr(t, func() {
		for name, gate := range gates {
			if off, err := gate(req); off != nil || err != nil {
				t.Errorf("%s clean = (%v, %v)", name, off, err)
			}
		}
	})
	if len(*events) != 0 || out != "" {
		t.Fatalf("a clean run must emit nothing: events=%v stderr=%q", codesOf(*events), out)
	}

	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n\nfunc Exported() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "brandnew", "x.go"), []byte("package brandnew\n\nfunc Exported() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// vet, acs-durable and the tier are red; apicover's pre-steps stay green
	// so the in-process measurement is what finds the uncovered export.
	red := func(ctx context.Context, name, dir string, args, env []string, in io.Reader, so, se io.Writer) (int, error) {
		if args[0] == "vet" || (args[0] == "test" && args[1] != "-tags") {
			_, _ = io.WriteString(so, "--- FAIL: TestGenuine (0.01s)\nFAIL\tpkg\t2.0s\n")
			return 1, nil
		}
		return pipelineRunner(goDir, 0, "")(ctx, name, dir, args, env, in, so, se)
	}
	g, events = observed(t, red, fixedSet("./internal/p/...", "./internal/brandnew/..."))
	for name, tc := range map[string]struct {
		gate  func(Request) ([]string, error)
		cause string
	}{"vet": {g.GoVet, "exit"}, "acs": {g.ACSDurable, "exit"}, "tier": {g.IntegrationTier, "retake_red"}, "enforce": {g.ApicoverEnforce, "apicover"}, "graduation": {g.ApicoverGraduation, "ungraduated"}} {
		*events = nil
		off, err := tc.gate(req)
		if err != nil || len(off) == 0 {
			t.Fatalf("%s must FAIL: (%v, %v)", name, off, err)
		}
		e := only(t, *events, CodeGateFailed)
		if e.Fields["cause"] != tc.cause || e.Fields["offenders"] != strconv.Itoa(len(off)) || e.Fields["first"] != log.SanitizeField(off[0]) || e.Fields["gate"] == "" || e.Reason != log.SanitizeField(strings.Join(off, "; ")) {
			t.Errorf("%s GATE_FAILED %+v for offenders %v", name, e, off)
		}
	}
}

// Test 32 — the Null Object and the live accessor: no option, or an accessor
// returning nil, → SignalsWired false and the gate runs to its golden
// verdict; a recording Center on a red-then-green tier under a held lock
// (timed out on the injected clock) with an unwritable Workspace yields the
// ordered stream golden G5 — the attempt-1 write PRECEDES the lock wait — and
// every event carries module, kind, severity, origin, phase and cycle. On a
// writable Workspace the sleep hook sees attempt 1 on disk during the wait.
func TestSignals_NullObjectLiveAccessorAndStreamGolden(t *testing.T) {
	root, _ := goWorktree(t)
	script := []step{{1, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n"}, {0, "ok\n"}}
	fn, _, _ := seqRunFunc(t, script)
	for name, g := range map[string]*Gates{
		"no option":    New(fn, fixedSet("./internal/widget/...")),
		"nil accessor": New(fn, fixedSet("./internal/widget/..."), WithSignals(func() *signalcenter.Center { return nil })),
	} {
		if g.SignalsWired() {
			t.Errorf("%s: SignalsWired must be false", name)
		}
	}
	if _, err := New(fn, fixedSet("./internal/widget/...")).IntegrationTier(tierRequest(root, "")); err == nil || err.Error() != golden(t, "messages.golden.txt")["tier.flake.no_log"] {
		t.Fatalf("the Null Object runs the gate to its golden verdict: %v", err)
	}

	release, held, err := flock.TryLock(filepath.Join(root, ".evolve", "locks", "integration-tier.lock"))
	if err != nil || held {
		t.Fatal("the test must hold the retake lock")
	}
	defer release()
	clock := func() (func() time.Time, func(time.Duration)) {
		tick := 0
		base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
		return func() time.Time { tick++; return base.Add(time.Duration(tick-1) * 3 * time.Minute) }, func(time.Duration) {} // deadline now+5m; one 3 m step polls once, the next times out
	}
	wsFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fn, _, _ = seqRunFunc(t, script)
	now, sleep := clock()
	g, events := observed(t, fn, fixedSet("./internal/widget/..."), WithClock(now, sleep))
	if _, err := g.IntegrationTier(Request{5, root, root, wsFile}); err == nil || !strings.Contains(err.Error(), "integration-tier.log unavailable") {
		t.Fatalf("flake absorbed with no log: %v", err)
	}
	var got []string
	for _, e := range *events {
		if e.Module != signalcenter.ModuleAudit || e.Kind != signalcenter.KindAuditWarning || e.Severity != signalcenter.SeverityWarn ||
			e.Origin != "Gates.IntegrationTier" || e.Phase != string(cyclestate.PhaseAudit) || e.Cycle != 5 || e.Fields["gate"] != "integration_tier" {
			t.Errorf("event stamp: %+v", e)
		}
		got = append(got, streamLine(e))
	}
	want, err := os.ReadFile(filepath.Join("testdata", "signals.golden.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n")+"\n" != string(want) {
		t.Errorf("stream drifted from the golden:\n got %q\nwant %q", strings.Join(got, "\n")+"\n", want)
	}

	ws := t.TempDir()
	logDuringWait := ""
	now, _ = clock()
	fn, _, _ = seqRunFunc(t, script)
	g = New(fn, fixedSet("./internal/widget/..."), WithClock(now, func(time.Duration) {
		b, _ := os.ReadFile(filepath.Join(ws, "integration-tier.log"))
		logDuringWait = string(b)
	}))
	if _, err := g.IntegrationTier(Request{5, root, root, ws}); err == nil {
		t.Fatal("red-then-green under a held lock still absorbs the flake")
	}
	if !strings.Contains(logDuringWait, "# attempt 1 (lane env, contended)") || strings.Contains(logDuringWait, "# attempt 2") {
		t.Errorf("attempt 1 must be on disk BEFORE the lock is waited for; the wait saw %q", logDuringWait)
	}
}

// Test 39 (review fold — architecture LOW 4) — warn's kv are key/value PAIRS,
// a programming contract like the nil runner: an odd trailing key panics at
// the producer (before any event is emitted) instead of silently dropping a
// field a triage would then never see.
func TestWarn_OddTrailingKeyPanicsBeforeEmitting(t *testing.T) {
	g, events := observed(t, nil, nil)
	defer func() {
		r, _ := recover().(string)
		if !strings.Contains(r, "key/value pairs") || len(*events) != 0 {
			t.Fatalf("an odd kv must panic with the producer's own message before emitting: recovered=%q events=%v", r, codesOf(*events))
		}
	}()
	g.warn(gateGoVet, Request{}, CodeGateStepFailed, "reason", "step", "exec", "orphan")
}

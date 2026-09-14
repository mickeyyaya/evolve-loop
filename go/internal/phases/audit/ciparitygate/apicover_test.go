package ciparitygate

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

// Test 26 (moved intent: audit/ciparity_unit_test.go:237-245 and the pre-move
// pins TestApicoverGates_MissingEnforceListWinsOverUnderivable /
// TestChangedSetUnderivable_SeverityAsymmetryBytes) — the shared prologue's
// order and the cycle-581 severity asymmetry: no module → (nil, nil); no
// .apicover-enforce + underivable → (nil, nil) (the read precedes the
// derivable check); enforce file + underivable → exactly the golden D1 / D2
// offender, ONE GATE_FAILED{cause=underivable} and NO CHANGESET_UNDERIVABLE;
// touched∩enforced empty → (nil, nil).
func TestApicoverEnforce_InputsOrderAndSeverity(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	g, events := observed(t, fakeRunFunc(0, "", "", nil), underivableSet())
	noModule := Request{1, t.TempDir(), "", ""}
	if off, err := g.ApicoverEnforce(noModule); off != nil || err != nil {
		t.Errorf("no module: (%v, %v)", off, err)
	}
	if off, err := g.ApicoverGraduation(noModule); off != nil || err != nil {
		t.Errorf("no module: (%v, %v)", off, err)
	}
	root, goDir := goWorktree(t)
	req := tierRequest(root, "")
	if off, err := g.ApicoverEnforce(req); off != nil || err != nil {
		t.Errorf("no enforce list + underivable: (%v, %v), want (nil, nil)", off, err)
	}
	if off, err := g.ApicoverGraduation(req); off != nil || err != nil {
		t.Errorf("no enforce list + underivable: (%v, %v), want (nil, nil)", off, err)
	}
	if len(*events) != 0 {
		t.Fatalf("no event before the enforce list exists: %v", codesOf(*events))
	}
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		gate func(Request) ([]string, error)
		want string
	}{"enforce": {g.ApicoverEnforce, g1["apicover.underivable"]}, "graduation": {g.ApicoverGraduation, g1["graduation.underivable"]}} {
		*events = nil
		off, err := tc.gate(req)
		if err != nil || len(off) != 1 || off[0] != tc.want {
			t.Errorf("%s underivable: (%v, %v), want exactly the golden hard-FAIL offender", name, off, err)
		}
		e := only(t, *events, CodeGateFailed)
		if e.Fields["cause"] != "underivable" || e.Fields["offenders"] != "1" || len(*events) != 1 {
			t.Errorf("%s: %+v (events %v)", name, e, codesOf(*events))
		}
	}
	g, events = observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/other/..."))
	if off, err := g.ApicoverEnforce(req); off != nil || err != nil || len(*events) != 0 {
		t.Errorf("touched∩enforced empty: (%v, %v) events=%v", off, err, codesOf(*events))
	}
}

// Test 27 — each pre-verdict step failure is fail-open with the golden text
// and ONE GATE_STEP_FAILED naming the step; an empty `go list` is a silent no-op.
func TestApicoverEnforce_PreStepFailuresCodedByStep(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, goDir := enforcedFixture(t, "package p\n\nfunc Exported() {}\n")
	req := tierRequest(root, "")

	if err := os.RemoveAll(filepath.Join(goDir, "bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, events := observed(t, pipelineRunner(goDir, 0, ""), fixedSet("./internal/p/..."))
	off, err := g.ApicoverEnforce(req)
	if off != nil || err == nil || err.Error() != fill(g1["apicover.bin_dir"], root, "") {
		t.Fatalf("bin dir is a file: (%v, %v)", off, err)
	}
	if e := only(t, *events, CodeGateStepFailed); e.Fields["step"] != "bin_dir" || e.Reason != err.Error() {
		t.Errorf("bin_dir event: %+v", e)
	}
	if err := os.Remove(filepath.Join(goDir, "bin")); err != nil {
		t.Fatal(err)
	}

	for i, tc := range []struct{ step, key string }{{"cover_run", "apicover.cover_run"}, {"cover_func", "apicover.cover_func"}, {"pkg_dirs", "apicover.pkg_dirs"}} {
		failAt, n := i+1, 0
		g, events = observed(t, func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, _, se io.Writer) (int, error) {
			n++
			if n == failAt {
				_, _ = io.WriteString(se, "boom")
				return 1, nil
			}
			return 0, nil
		}, fixedSet("./internal/p/..."))
		off, err := g.ApicoverEnforce(req)
		if off != nil || err == nil || err.Error() != fill(g1[tc.key], root, "") {
			t.Errorf("%s: (%v, %v), want the golden text", tc.step, off, err)
		}
		if e := only(t, *events, CodeGateStepFailed); e.Fields["step"] != tc.step || !strings.HasPrefix(e.Fields["cmd"], "go ") || e.Reason != err.Error() {
			t.Errorf("%s event: %+v", tc.step, e)
		}
	}

	if err := os.MkdirAll(filepath.Join(goDir, "bin", "ciparity-cover.txt.func.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, events = observed(t, pipelineRunner(goDir, 0, ""), fixedSet("./internal/p/..."))
	off, err = g.ApicoverEnforce(req)
	if off != nil || err == nil || err.Error() != fill(g1["apicover.write_func_cover"], root, "") {
		t.Fatalf("func cover path is a directory: (%v, %v)", off, err)
	}
	if e := only(t, *events, CodeGateStepFailed); e.Fields["step"] != "write_func_cover" {
		t.Errorf("write_func_cover event: %+v", e)
	}
	if err := os.RemoveAll(filepath.Join(goDir, "bin", "ciparity-cover.txt.func.txt")); err != nil {
		t.Fatal(err)
	}

	g, events = observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/p/...")) // go list prints nothing
	if off, err := g.ApicoverEnforce(req); off != nil || err != nil || len(*events) != 0 {
		t.Errorf("empty go list: (%v, %v) events=%v, want a silent no-op", off, err, codesOf(*events))
	}
	if entries, _ := os.ReadDir(filepath.Join(goDir, "bin")); len(entries) != 0 {
		t.Errorf("scratch files must not accumulate: %v", entries)
	}
}

// Test 28 (moved: audit/ciparity_apicover_ctx_test.go) — enforceVerdict is a
// pure table; a ctx interruption of the in-process measurement fails OPEN with
// the context error in the chain and ONE GATE_STEP_FAILED{step=measure}.
func TestEnforceVerdict_PureTableAndMeasureInterruption(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	if off, err := enforceVerdict(0, nil, ""); off != nil || err != nil {
		t.Errorf("clean: (%v, %v)", off, err)
	}
	if off, err := enforceVerdict(1, nil, "x.go:1: UNCOVERED (no test names it): Exported\nsummary\n"); err != nil || len(off) != 1 || !strings.Contains(off[0], "UNCOVERED") {
		t.Errorf("offenders: (%v, %v)", off, err)
	}
	off, err := enforceVerdict(2, errors.New("parse boom"), "partial")
	if err != nil || off[len(off)-1] != "apicover -enforce measurement error: parse boom" {
		t.Errorf("measurement error: (%v, %v)", off, err)
	}
	for _, ctxErr := range []error{context.DeadlineExceeded, context.Canceled} {
		off, err := enforceVerdict(2, ctxErr, "")
		if off != nil || !errors.Is(err, ctxErr) || err.Error() != "apicover gate: measurement interrupted: "+ctxErr.Error() {
			t.Errorf("%v: (%v, %v)", ctxErr, off, err)
		}
	}

	root, goDir := enforcedFixture(t, "package p\n\nfunc Exported() {}\n")
	timeouts := DefaultTimeouts()
	timeouts.Apicover = -time.Nanosecond // ctx born expired: the fake pre-steps ignore it; apicover.Run must not
	g, events := observed(t, pipelineRunner(goDir, 0, ""), fixedSet("./internal/p/..."), WithTimeouts(timeouts))
	off, err = g.ApicoverEnforce(tierRequest(root, ""))
	if off != nil || err == nil || !errors.Is(err, context.DeadlineExceeded) || err.Error() != g1["apicover.measure_interrupted"] {
		t.Fatalf("interrupted measurement must fail OPEN: (%v, %v)", off, err)
	}
	if e := only(t, *events, CodeGateStepFailed); e.Fields["step"] != "measure" || e.Reason != err.Error() {
		t.Errorf("measure event: %+v", e)
	}
}

// Test 29 (moved intent: audit/ciparity_unit_test.go:250-363) — apicover runs
// IN-PROCESS: an uncovered export → the golden offenders + ONE
// GATE_FAILED{cause=apicover}; no executable file is created anywhere under
// the worktree and the scratch profile files are removed; an unparseable
// package → a measurement-error FAIL (offenders, nil), never a WARN.
func TestApicoverEnforce_InProcessLeavesNoBinaryOrScratchAndCatchesAnUncoveredExport(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, goDir := enforcedFixture(t, "package p\n\n// Exported is public but no test names it → uncovered.\nfunc Exported() {}\n")
	var seen []string
	rec := func(ctx context.Context, name, dir string, args, env []string, in io.Reader, so, se io.Writer) (int, error) {
		seen = append(seen, name+" "+strings.Join(args, " "))
		return pipelineRunner(goDir, 0, "")(ctx, name, dir, args, env, in, so, se)
	}
	g, events := observed(t, rec, fixedSet("./internal/p/..."))
	before := executableFiles(t, root)
	off, err := g.ApicoverEnforce(tierRequest(root, ""))
	if err != nil || strings.Join(off, "\n") != fill(g1["apicover.uncovered"], root, "") {
		t.Fatalf("uncovered export: (%v, %v), want the golden offenders", off, err)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["cause"] != "apicover" || e.Fields["first"] != log.SanitizeField(off[0]) {
		t.Errorf("GATE_FAILED: %+v", e)
	}
	for p := range executableFiles(t, root) {
		if !before[p] {
			t.Errorf("the gate created an executable %q — the one-binary invariant", p)
		}
	}
	for _, c := range seen {
		if strings.Contains(c, "build") {
			t.Errorf("no apicover build is forked: %q", c)
		}
	}
	for _, scratch := range []string{"ciparity-cover.txt", "ciparity-cover.txt.func.txt", "apicover"} {
		if _, err := os.Stat(filepath.Join(goDir, "bin", scratch)); !os.IsNotExist(err) {
			t.Errorf("bin/%s must be gone after the gate (err=%v)", scratch, err)
		}
	}

	broot, bgoDir := enforcedFixture(t, "package p\n\nfunc (\n")
	g, events = observed(t, pipelineRunner(bgoDir, 0, ""), fixedSet("./internal/p/..."))
	off, err = g.ApicoverEnforce(tierRequest(broot, ""))
	if err != nil || strings.Join(off, "\n") != fill(g1["apicover.measurement_error"], broot, "") {
		t.Fatalf("measurement error must FAIL with the golden line: (%v, %v)", off, err)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["cause"] != "apicover" {
		t.Errorf("GATE_FAILED: %+v", e)
	}
}

// executableFiles returns the regular files under root with any exec bit set.
func executableFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			out[path] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Test 30 (moved: audit/ciparity_newpkg_test.go:196-221) — a test-only
// ungraduated package is not an offender: ONE GRADUATION_DEFERRED{pkg, dir},
// zero bytes on stderr, an empty (non-nil, as before) offender list; a
// production package → exactly the golden prescriptive offender and ONE
// GATE_FAILED{cause=ungraduated}; go/cmd/... is never flagged; the pure split
// returns both halves.
func TestApicoverGraduation_DeferredIsCodedNotPrinted(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, goDir := enforcedFixture(t, "package p\n")
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "testonly"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "testonly", "only_test.go"), []byte("package testonly\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := tierRequest(root, "")
	g, events := observed(t, nil, fixedSet("./internal/testonly/..."))
	var off []string
	var err error
	out := captureStderr(t, func() { off, err = g.ApicoverGraduation(req) })
	if err != nil || off == nil || len(off) != 0 || out != "" {
		t.Fatalf("test-only package: (%v, %v) stderr=%q — want no offender (an empty non-nil list) and no stderr", off, err, out)
	}
	e := only(t, *events, CodeGraduationDeferred)
	if e.Fields["pkg"] != "./internal/testonly" || e.Fields["dir"] != goDir || !strings.HasPrefix(e.Reason, "graduation deferred: ./internal/testonly has no production .go surface") || len(*events) != 1 {
		t.Errorf("GRADUATION_DEFERRED: %+v", e)
	}

	if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "brandnew", "x.go"), []byte("package brandnew\n\nfunc Exported() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, events = observed(t, nil, fixedSet("./internal/brandnew/...", "./internal/testonly/..."))
	off, err = g.ApicoverGraduation(req)
	if err != nil || len(off) != 1 || off[0] != g1["graduation.offender"] {
		t.Fatalf("production package: (%v, %v), want exactly the golden prescriptive offender", off, err)
	}
	if !strings.Contains(off[0], ciparity.GraduationPrescription([]string{"./internal/brandnew"})) {
		t.Error("the offender carries the prescription verbatim")
	}
	if got := codesOf(*events); strings.Join(got, ",") != string(CodeGraduationDeferred)+","+string(CodeGateFailed) {
		t.Errorf("events: %v", got)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["cause"] != "ungraduated" || e.Fields["offenders"] != "1" {
		t.Errorf("GATE_FAILED: %+v", e)
	}
	offenders, deferred := graduationOffenders(goDir, []string{"./internal/brandnew", "./internal/testonly", "./internal/absent"})
	if len(offenders) != 1 || strings.Join(deferred, " ") != "./internal/testonly ./internal/absent" {
		t.Errorf("pure split: offenders=%v deferred=%v", offenders, deferred)
	}

	g, events = observed(t, nil, fixedSet("./cmd/evolve/..."))
	if off, err := g.ApicoverGraduation(req); err != nil || len(off) != 0 || len(*events) != 0 {
		t.Errorf("go/cmd/... only change: (%v, %v) events=%v — cmd/ is out of apicover's scope", off, err, codesOf(*events))
	}
}

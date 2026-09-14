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
)

// Test 13 (moved from audit/ciparity_unit_test.go TestOffenderLines) — real
// markers are kept, no marker falls back to the last 6 non-empty lines, the
// cap is the last 12.
func TestOffenderLines_MarkersFallbackAndCap(t *testing.T) {
	if got := offenderLines("noise\nbad.go:1: import cycle not allowed\nmore\n--- FAIL: X"); len(got) != 2 {
		t.Errorf("marker extraction: %v, want 2", got)
	}
	if got := offenderLines("a\nb\nc"); len(got) != 3 {
		t.Errorf("no-marker fallback: %v, want the 3 lines", got)
	}
	if got := offenderLines("1\n2\n3\n4\n5\n6\n7\n8"); len(got) != 6 || got[0] != "3" {
		t.Errorf("the fallback is the LAST 6 non-empty lines: %v", got)
	}
	long := strings.Repeat("FAIL line\n", 40)
	if got := offenderLines(long); len(got) != 12 {
		t.Errorf("cap: %d lines, want exactly 12", len(got))
	}
	for _, marker := range []string{"apicover -enforce measurement error: x", "x: UNCOVERED (no test names it)", "a.go:1:2: undefined", "panic: boom", "# pkg/x", "import cycle not allowed", "FAIL\tpkg", "--- FAIL: T"} {
		if !hasOffenderMarker("chatter\n" + marker + "\n") {
			t.Errorf("%q must be recognised as a failure marker", marker)
		}
	}
	if hasOffenderMarker("some output mentioning FAIL mid-line\nerror: x") {
		t.Error("hasOffenderMarker must NOT inherit the fallback: a marker-free truncation is not judged")
	}
}

// Moved verbatim (audit/ciparity_unit_test.go:103-123) — the cycle-930/931/932
// false-FAIL diagnostic corruption: only line-anchored failure markers survive.
func TestOffenderLines_DropsPassingTestChatter(t *testing.T) {
	out := strings.Join([]string{
		"[orchestrator] WARN phase scout attempt 1/2 hit a transient bridge error or timeout; relaunching (self-heal)", // chatter: mid-line "error"
		"    --check               warn if changes introduce conflict markers or whitespace errors",                    // git usage dump chatter
		"    highlight whitespace errors in the 'context', 'old' or 'new' lines in the diff",                           // git usage dump chatter
		"audit verdict=FAIL: something quoted by a passing test",                                                       // chatter: mid-line "FAIL"
		"--- FAIL: TestRealThing (0.03s)",                                 // real: test failure header
		"panic: runtime error: index out of range",                        // real: panic
		"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/internal/core\t55.2s", // real: package summary
		"apicover -enforce measurement error: go.mod not found above /x",  // real: apicover infra line (go-review LOW)
	}, "\n")
	got := offenderLines(out)
	if len(got) != 4 {
		t.Fatalf("got %d offender lines %v, want exactly the 4 real failure markers", len(got), got)
	}
	for _, ln := range got {
		if strings.Contains(ln, "orchestrator") || strings.Contains(ln, "whitespace") || strings.HasPrefix(ln, "audit verdict") {
			t.Errorf("chatter survived into the verdict diagnostic: %q", ln)
		}
	}
}

// Test 14 (moved from audit/ciparity_unit_test.go:49-81) — the exit-code
// mapping: exit 0 → clean, no event; exit 1 + output → offenders + ONE
// GATE_FAILED{cause=exit}; exit 2 no output → the synthesized golden line; a
// start error → the golden could-not-run text + ONE GATE_STEP_FAILED{step=exec};
// no go module → (nil, nil) and the runner is never called.
func TestRunGate_ExitCodeMappingAndNoModule(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, _ := goWorktree(t)
	req := tierRequest(root, "")

	g, events := observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/p/..."))
	if off, err := g.GoVet(req); off != nil || err != nil || len(*events) != 0 {
		t.Errorf("exit 0: (%v,%v) events=%v, want (nil,nil) and no event", off, err, codesOf(*events))
	}

	g, events = observed(t, fakeRunFunc(1, "", "bad.go:5: import cycle not allowed", nil), fixedSet("./internal/p/..."))
	off, err := g.GoVet(req)
	if err != nil || len(off) != 1 || off[0] != "bad.go:5: import cycle not allowed" {
		t.Errorf("exit 1: (%v,%v), want the offender", off, err)
	}
	e := only(t, *events, CodeGateFailed)
	if e.Fields["gate"] != "go_vet" || e.Fields["cause"] != "exit" || e.Fields["exit"] != "1" || e.Fields["offenders"] != "1" || e.Fields["first"] != off[0] || e.Origin != "Gates.GoVet" {
		t.Errorf("GATE_FAILED fields: %+v", e)
	}

	g, events = observed(t, fakeRunFunc(2, "", "", nil), fixedSet("./internal/p/..."))
	if off, err := g.GoVet(req); err != nil || len(off) != 1 || off[0] != g1["vet.no_output"] {
		t.Errorf("exit 2 no-output: (%v,%v), want the synthesized golden line", off, err)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["exit"] != "2" {
		t.Errorf("exit field: %+v", e.Fields)
	}

	g, events = observed(t, fakeRunFunc(-1, "", "", errors.New("executable file not found")), fixedSet("./internal/p/..."))
	if off, err := g.GoVet(req); off != nil || err == nil || err.Error() != g1["vet.exec_failed"] {
		t.Errorf("start error: (%v,%v), want (nil, the golden could-not-run text)", off, err)
	}
	e = only(t, *events, CodeGateStepFailed)
	if e.Fields["step"] != "exec" || e.Fields["cmd"] != "go vet ./..." || e.Fields["err"] != "executable file not found" || e.Reason != g1["vet.exec_failed"] {
		t.Errorf("GATE_STEP_FAILED fields: %+v", e)
	}
	if _, err := g.ACSDurable(req); err == nil || err.Error() != g1["acs.exec_failed"] {
		t.Errorf("acs-durable start error: %v", err)
	}

	called := false
	g, events = observed(t, func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		called = true
		return 1, nil
	}, fixedSet("./internal/p/..."))
	for _, r := range []Request{{1, t.TempDir(), "", ""}, {1, "", "", ""}} {
		if off, err := g.GoVet(r); off != nil || err != nil || called || len(*events) != 0 {
			t.Errorf("no module %+v: (%v,%v) called=%v", r, off, err, called)
		}
	}
	if _, err := g.runGate(gateGoVet, Request{1, t.TempDir(), "", ""}, "x", time.Second, "go", "vet"); err != nil || called {
		t.Error("runGate itself guards the module dir")
	}
}

// Test 16 — every subprocess vector the five gates fork equals the golden
// captured on 8e8f080f, line for line. Absorbs the three moved scope tests
// (audit/ciparity_scope_test.go): the scoped tier runs ONLY the touched
// package with -race -count=1 -p 4 -parallel 4 -tags integration and never
// shells out to `go list`; a module-root change falls back to `go list ./...`
// and tests every non-acs package; apicover's three forks carry the scoped
// cover profile.
func TestWholeRepoGates_ArgVectorsMatchTheGolden(t *testing.T) {
	g2 := golden(t, "argv.golden.txt")
	recorder := func(root string, seen *[]string, list string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
			*seen = append(*seen, name+" "+strings.Join(args, " "))
			if args[0] == "list" {
				_, _ = io.WriteString(so, list)
			}
			return 0, nil
		}
	}
	root, goDir := goWorktree(t)
	req := tierRequest(root, "")
	for _, tc := range []struct {
		gate string
		set  ChangedSetFunc
		list string
		keys []string
	}{
		{"vet", fixedSet("./internal/widget/..."), "", []string{"vet.1"}},
		{"acs", fixedSet("./internal/widget/..."), "", []string{"acs.1"}},
		{"tier", fixedSet("./internal/widget/..."), "", []string{"tier.scoped.1"}},
		{"tier", fixedSet("./..."), "ciparitytest/cmd/foo\nciparitytest/acs/regression\nciparitytest/internal/other\nciparitytest/internal/bridge\n", []string{"tier.whole.1", "tier.whole.2"}},
	} {
		var seen []string
		g := New(recorder(root, &seen, tc.list), tc.set)
		var err error
		switch tc.gate {
		case "vet":
			_, err = g.GoVet(req)
		case "acs":
			_, err = g.ACSDurable(req)
		default:
			_, err = g.IntegrationTier(req)
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.gate, err)
		}
		want := make([]string, 0, len(tc.keys))
		for _, k := range tc.keys {
			want = append(want, fill(g2[k], root, ""))
		}
		if strings.Join(seen, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s argv drifted:\n got %q\nwant %q", tc.gate, seen, want)
		}
	}
	// apicover: the three forks, then the in-process measurement.
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte("./internal/p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "p", "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen []string
	g := New(recorder(root, &seen, filepath.Join(goDir, "internal", "p")+"\n"), fixedSet("./internal/p/..."))
	if off, err := g.ApicoverEnforce(req); off != nil || err != nil {
		t.Fatalf("apicover clean: (%v, %v)", off, err)
	}
	want := []string{fill(g2["apicover.1"], root, ""), fill(g2["apicover.2"], root, ""), fill(g2["apicover.3"], root, "")}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("apicover argv drifted:\n got %q\nwant %q", seen, want)
	}
}

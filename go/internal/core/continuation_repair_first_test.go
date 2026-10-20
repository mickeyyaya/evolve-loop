package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func adoptWithPriorVerdict(t *testing.T, cycle int, verdict string) PhaseRequest {
	t.Helper()
	root, wt := initContinuationRepo(t, cycle)
	m := stampedContinuation(t, root, wt, cycle)
	predicate := filepath.Join(root, "go", "acs", "regression", "cache_key_test.go")
	if err := os.MkdirAll(filepath.Dir(predicate), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(predicate, []byte("package regression\n\nfunc TestKeepsTheCacheKey(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "go")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "land a regression predicate")
	if err := os.WriteFile(m.FindingsPath, []byte(`{"schema_version":1,"phase":"audit","reasons":["EGPS red_count=1"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		if err := os.WriteFile(filepath.Join(filepath.Dir(m.FindingsPath), "acs-verdict.json"), []byte(verdict), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seedStampedInboxItem(t, root, cycle, "task-a")

	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	runners[PhaseTriage] = &claimingTriageRunner{fakeRunner: runners[PhaseTriage].(*fakeRunner), root: root, taskID: "task-a"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithContinuationResolver(productionResolver(t)))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if buildR.calls == 0 {
		t.Fatal("build must have dispatched")
	}
	return buildR.requests[0]
}

func TestRunCycle_AContinuationWithAnInheritedRedLeadsWithTheRepairDirective(t *testing.T) {
	t.Parallel()
	req := adoptWithPriorVerdict(t, 86, `{"results":[{"ac_id":"regression/TestKeepsTheCacheKey","result":"red"}]}`)

	got := req.Context[CtxKeyContinuationRepairFirst]
	if !strings.HasPrefix(got, "FIRST ACTION:") || !strings.Contains(got, "- go/acs/regression/cache_key_test.go: TestKeepsTheCacheKey") {
		t.Errorf("the build context must carry the repair directive naming the file; got %q", got)
	}
	if !strings.Contains(req.Context["continuation_findings"], "EGPS red_count=1") {
		t.Errorf("the descriptive findings must still be served; got %q", req.Context["continuation_findings"])
	}
}

func TestRunCycle_AContinuationWithoutARedCarriesNoRepairDirective(t *testing.T) {
	t.Parallel()
	req := adoptWithPriorVerdict(t, 87, `{"results":[{"ac_id":"regression/TestKeepsTheCacheKey","result":"green"}]}`)

	if got, ok := req.Context[CtxKeyContinuationRepairFirst]; ok {
		t.Errorf("no red artifact ⇒ no directive key; got %q", got)
	}
	if !strings.Contains(req.Context["continuation_findings"], "EGPS red_count=1") {
		t.Errorf("the descriptive findings must be unchanged; got %q", req.Context["continuation_findings"])
	}
}

func TestContinuationRepairFirst_WarnsOnAnUnreadableVerdictAndOnlyReadsABoundWorkspace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	c := &continuation.Continuation{FindingsPath: filepath.Join(ws, "audit-fail-reason.json")}
	if got := continuationRepairFirst(c, t.TempDir(), &warn); got != "" {
		t.Errorf("an unreadable verdict yields no directive, got %q", got)
	}
	if !strings.Contains(warn.String(), "WARN continuation: no repair-first directive") {
		t.Errorf("an unreadable verdict must warn, got %q", warn.String())
	}
	warn.Reset()
	if got := continuationRepairFirst(&continuation.Continuation{}, t.TempDir(), &warn); got != "" || warn.Len() != 0 {
		t.Errorf("a binding without findings reads nothing; got %q, warn %q", got, warn.String())
	}
}

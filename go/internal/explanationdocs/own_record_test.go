package explanationdocs

import (
	"context"
	"strings"
	"testing"
)

func (f fixture) prepareTestOnlyChange(t *testing.T) {
	t.Helper()
	f.write(t, "pkg/app_test.go", "package pkg\n")
}

func (f fixture) testOnlyDocument() string {
	return strings.Replace(f.validDocument(),
		"- `config/app.yaml` — flips the existing runtime setting while preserving its schema.",
		"- `pkg/app_test.go` — pins the enabled and the disabled behavior of the existing setting.", 1)
}

func (f fixture) sealAndVerify(t *testing.T) {
	t.Helper()
	view, err := Load(f.workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	if verified, active, err := Verify(context.Background(), f.binding()); err != nil || !active || !SameView(verified, view) {
		t.Fatalf("Verify=(%+v,%v,%v)", verified, active, err)
	}
}

func TestNonMaterialDiff_ADeclaredExplanationIsHonored(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareTestOnlyChange(t)
	f.write(t, cycleDocumentPath(f.cycle, f.runID), f.testOnlyDocument())
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: REQUIRED\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("a declared explanation of a test-only change was refused: %v", failures)
	}
	view, err := Load(f.workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Status != statusRequired || view.DocumentPath != cycleDocumentPath(f.cycle, f.runID) || view.DocumentSHA256 == "" || len(view.MaterialPaths) != 0 {
		t.Fatalf("derived view=%+v", view)
	}
	f.sealAndVerify(t)
}

func TestNonMaterialDiff_TheCyclesOwnRecordIsNotHistory(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareTestOnlyChange(t)
	f.write(t, cycleDocumentPath(f.cycle, f.runID), "a draft the report never declared\n")
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the base-bound Build diff changes tests only\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("the cycle's own record was refused as history: %v", failures)
	}
	view, err := Load(f.workspace)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if view.Status != statusNA || view.DocumentPath != "" {
		t.Fatalf("derived view=%+v", view)
	}
	f.sealAndVerify(t)
}

func TestNonMaterialDiff_ARecordAddedAfterTheCheckDoesNotVerify(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareTestOnlyChange(t)
	f.writeWorkspace(t, "build-report.md", "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the base-bound Build diff changes tests only\n")
	if failures := f.check(t); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
	f.write(t, "docs/explain/builds/cycle-41-prior-run.md", "rewritten after the check\n")
	if _, _, err := Verify(context.Background(), f.binding()); err == nil || !strings.Contains(err.Error(), "diff SHA256 does not match") {
		t.Fatalf("Verify accepted a record added after the check: err=%v", err)
	}
}

func TestNonMaterialDiff_AForeignRecordIsStillImmutable(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareTestOnlyChange(t)
	f.write(t, "docs/explain/builds/cycle-41-prior-run.md", "rewritten prior explanation\n")
	f.write(t, cycleDocumentPath(f.cycle, f.runID), f.testOnlyDocument())
	f.writeWorkspace(t, "build-report.md", "# Build Report\n\n## Explanation Documentation\n- Status: REQUIRED\n- Document: "+cycleDocumentPath(f.cycle, f.runID)+"\n")
	if failures := f.check(t); !containsFailure(failures, "published cycle records are immutable") {
		t.Fatalf("a declared explanation let a prior cycle-document rewrite through: %v", failures)
	}
}

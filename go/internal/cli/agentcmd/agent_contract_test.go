package agentcmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

const wrongHeadingReport = "## Result\nNothing to change.\n## Checked\nThe diff.\nVerdict: PASS\n"

const simplifierReport = "## Simplifications\nNone.\nVerdict: PASS\n"

func simplifierProject(t *testing.T) string {
	t.Helper()
	root := project(t)
	repo := filepath.Join("..", "..", "..", "..")
	for _, rel := range []string{".evolve/phases/code-simplifier/phase.json", ".evolve/profiles/diff-code-simplifier.json", "agents/evolve-diff-code-simplifier.md"} {
		raw, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func simplifierArgs(root string) []string {
	return []string{"run", "code-simplifier", "--project-root", root, "--json"}
}

func runSimplifier(t *testing.T, root, report string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := New(depsWith(&fakeBridge{artifact: report}))(simplifierArgs(root), strings.NewReader(diff), &stdout, &stderr)
	return code, stderr.String()
}

func onlyRecord(t *testing.T, root string) launchrecord.Record {
	t.Helper()
	recs, err := launchrecord.List(filepath.Join(root, ".evolve"))
	if err != nil || len(recs) != 1 {
		t.Fatalf("launch records = %+v, %v; want one", recs, err)
	}
	return recs[0]
}

func TestAgentRun_AReportWithoutTheRequiredHeadingRecordsTheMissingSectionAndKeepsTheFile(t *testing.T) {
	root := simplifierProject(t)

	code, stderr := runSimplifier(t, root, wrongHeadingReport)

	if code != exitFail {
		t.Fatalf("exit %d, want %d; stderr=%s", code, exitFail, stderr)
	}
	rec := onlyRecord(t, root)
	var missing string
	for _, v := range rec.Violations {
		if v.Code == "missing_section" {
			missing = v.Message
		}
	}
	if !strings.Contains(missing, "Simplifications") {
		t.Fatalf("record violations = %+v, want missing_section naming Simplifications", rec.Violations)
	}
	kept, err := os.ReadFile(filepath.Join(root, ".evolve", launchrecord.Subdir, rec.RunID+".deliverable.md"))
	if err != nil || string(kept) != wrongHeadingReport {
		t.Fatalf("kept deliverable = %q, %v; want the failing report", kept, err)
	}
}

func TestAgentRun_AVerifiedReportKeepsNoDeliverableAndRecordsNoViolation(t *testing.T) {
	root := simplifierProject(t)

	code, stderr := runSimplifier(t, root, simplifierReport)

	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%s", code, stderr)
	}
	rec := onlyRecord(t, root)
	if len(rec.Violations) != 0 {
		t.Errorf("violations = %+v, want none", rec.Violations)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", launchrecord.Subdir, rec.RunID+".deliverable.md")); !os.IsNotExist(err) {
		t.Errorf("a verified run kept a deliverable: %v", err)
	}
}

func TestAgentRun_AFailWithNoReportKeepsNoEmptyDeliverable(t *testing.T) {
	root := simplifierProject(t)

	code, stderr := runSimplifier(t, root, "")

	if code == 0 {
		t.Fatalf("exit 0 for a run with no report; stderr=%s", stderr)
	}
	rec := onlyRecord(t, root)
	if rec.Verdict != "FAIL" {
		t.Fatalf("verdict = %q, want FAIL", rec.Verdict)
	}
	kept, _ := filepath.Glob(filepath.Join(root, ".evolve", launchrecord.Subdir, "*.deliverable.md"))
	if len(kept) != 0 {
		t.Errorf("a FAIL with no report kept %v, want no deliverable", kept)
	}
}

func TestAgentRun_FailingDeliverablesAreBoundedAndRecordsAreNeverPruned(t *testing.T) {
	root := simplifierProject(t)
	dir := filepath.Join(root, ".evolve", launchrecord.Subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	for i := 0; i < keptDeliverables+3; i++ {
		for _, name := range []string{fmt.Sprintf("old-%02d.deliverable.md", i), fmt.Sprintf("old-%02d.json", i)} {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
			at := base.Add(time.Duration(i) * time.Minute)
			if err := os.Chtimes(path, at, at); err != nil {
				t.Fatal(err)
			}
		}
	}

	runSimplifier(t, root, wrongHeadingReport)

	kept, _ := filepath.Glob(filepath.Join(dir, "*.deliverable.md"))
	records, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(kept) != keptDeliverables || len(records) != keptDeliverables+4 {
		t.Fatalf("kept %d deliverables and %d records, want %d and %d", len(kept), len(records), keptDeliverables, keptDeliverables+4)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-00.deliverable.md")); !os.IsNotExist(err) {
		t.Errorf("the oldest deliverable survived: %v", err)
	}
}

func TestKeepFailedDeliverable_ABlockedDirectoryWarnsAndNamesTheLabel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", launchrecord.Subdir), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	keepFailedDeliverable(standaloneRun{root: root, label: "run-9"}, "body", &stderr)

	if !strings.Contains(stderr.String(), "run-9") {
		t.Errorf("stderr = %q, want a warning naming the run", stderr.String())
	}
}

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func writeFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFirstNonEmpty_AllEmptyYieldsEmpty(t *testing.T) {
	t.Parallel()
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty of empties = %q", got)
	}
	if got := firstNonEmpty("", "b", "c"); got != "b" {
		t.Fatalf("firstNonEmpty = %q, want b", got)
	}
}

func TestResolvedFingerprints_FailureModes(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	t.Run("load: unreadable path", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, resolvedFingerprintsFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadResolvedFingerprints(dir); err == nil {
			t.Fatal("want error reading a directory as the file")
		}
	})
	t.Run("load: malformed json", func(t *testing.T) {
		dir := t.TempDir()
		writeFileAt(t, filepath.Join(dir, resolvedFingerprintsFile), "{not json")
		if _, err := LoadResolvedFingerprints(dir); err == nil {
			t.Fatal("want error on malformed json")
		}
	})
	t.Run("append: empty fingerprint", func(t *testing.T) {
		if err := AppendResolvedFingerprint(t.TempDir(), "  ", "me", now); err == nil {
			t.Fatal("want error for blank fingerprint")
		}
	})
	t.Run("append: evolve dir is a file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		writeFileAt(t, f, "x")
		if err := AppendResolvedFingerprint(f, "a|b|c", "me", now); err == nil {
			t.Fatal("want mkdir error")
		}
	})
	t.Run("append: malformed existing file", func(t *testing.T) {
		dir := t.TempDir()
		writeFileAt(t, filepath.Join(dir, resolvedFingerprintsFile), "{not json")
		if err := AppendResolvedFingerprint(dir, "a|b|c", "me", now); err == nil {
			t.Fatal("want unmarshal error")
		}
	})
	t.Run("append: existing path is a directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, resolvedFingerprintsFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := AppendResolvedFingerprint(dir, "a|b|c", "me", now); err == nil {
			t.Fatal("want read error")
		}
	})
	t.Run("append: tmp path blocked", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, resolvedFingerprintsFile+".tmp"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := AppendResolvedFingerprint(dir, "a|b|c", "me", now); err == nil {
			t.Fatal("want write error")
		}
	})
	t.Run("consume: append failure propagates", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		writeFileAt(t, f, "x")
		if _, err := ConsumePipelineDefectFingerprint(f, "fingerprint=a|b|c", "", "me", now); err == nil {
			t.Fatal("want append error")
		}
	})
}

func TestShipSHAMismatch_UnreadableBinary(t *testing.T) {
	t.Parallel()
	if _, _, err := ShipSHAMismatch(filepath.Join(t.TempDir(), "absent"), "x"); err == nil {
		t.Fatal("want read error")
	}
}

func TestCompactHandoffError_TruncatesAndCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word  ", 200)
	got := compactHandoffError(&testErr{long})
	if len(got) != 500 || strings.Contains(got, "  ") {
		t.Fatalf("len=%d double-space=%v", len(got), strings.Contains(got, "  "))
	}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

func brokenActivationRoot(t *testing.T, cycle int) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".evolve", "build-explanation-contracts", "cycle-"+itoa(cycle)+".json")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func TestProjectBuildExplanation_ActivationErrorIsInvalid(t *testing.T) {
	t.Parallel()
	root, ws := brokenActivationRoot(t, 9)
	h := projectBuildExplanation(root, CycleState{CycleID: 9, WorkspacePath: ws, ExplanationDocumentationVersion: 1})
	if h.State != BuildExplanationInvalid || h.Error == "" {
		t.Fatalf("handoff = %+v", h)
	}
}

func TestRequireResumeExplanationIdentity_Branches(t *testing.T) {
	t.Parallel()
	t.Run("activation error", func(t *testing.T) {
		root, ws := brokenActivationRoot(t, 9)
		if err := requireResumeExplanationIdentity(root, ws, 9, CycleState{}, 0); err == nil {
			t.Fatal("want activation error")
		}
	})
	t.Run("inactive with versioned cycle state", func(t *testing.T) {
		root := t.TempDir()
		ws := filepath.Join(root, "ws")
		err := requireResumeExplanationIdentity(root, ws, 9, CycleState{ExplanationDocumentationVersion: 1}, 0)
		if err == nil || !strings.Contains(err.Error(), "no host activation matches") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("identity mismatch", func(t *testing.T) {
		root := t.TempDir()
		ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := explanationdocs.Activate(explanationdocs.CycleBinding{
			ProjectRoot: root, Workspace: ws, Cycle: 9, RunID: "run-9",
			ContractVersion: explanationdocs.CurrentContractVersion,
		}); err != nil {
			t.Fatal(err)
		}
		cs := CycleState{CycleID: 9, RunID: "other", WorkspacePath: ws, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion}
		err := requireResumeExplanationIdentity(root, ws, 9, cs, 0)
		if err == nil || !strings.Contains(err.Error(), "does not match host activation") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExplanationLifecycleReviewer_NonBuildPhaseApproves(t *testing.T) {
	t.Parallel()
	res := explanationLifecycleReviewer{}.Review(context.Background(), ReviewInput{
		ExplanationDocumentationVersion: 1, Phase: string(PhaseAudit),
	})
	if !res.Approve {
		t.Fatalf("result = %+v", res)
	}
}

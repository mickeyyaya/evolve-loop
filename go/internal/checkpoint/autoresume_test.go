package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func knownResetFixture(t *testing.T) (root, path string, cs core.CycleState) {
	t.Helper()
	root, cs = quotaCheckpointFixture(t)
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "quota-reset-hint.txt"), []byte("resets 4:10pm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, ".evolve", "cycle-state.json"), cs
}

func readBlock(t *testing.T, path string) Checkpoint {
	t.Helper()
	cp, err := readBlockAt(path)
	if err != nil {
		t.Fatal(err)
	}
	return cp
}

func TestRecordResume_ThreeAutoResumesCountToTheCapTheFourthIsRefusedAndAnOperatorResumeResets(t *testing.T) {
	root, path, cs := knownResetFixture(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.Local)

	for attempt := 1; attempt <= DefaultAutoResumeAttempts; attempt++ {
		if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
			t.Fatal(err)
		}
		if pause := readBlock(t, path); pause.AutoResumeAttempts != attempt-1 || pause.OperatorAction != "" {
			t.Fatalf("pause %d = %+v, want attempts=%d and no operator action", attempt, pause, attempt-1)
		}
		cp, admitted, err := RecordResume(path, true)
		if err != nil || !admitted || cp.AutoResumeAttempts != attempt {
			t.Fatalf("auto-resume %d = (%+v, %v, %v), want admitted with attempts=%d", attempt, cp, admitted, err, attempt)
		}
		if err := core.ResumeBoundaryCheckpointer(cs, root, now); err != nil {
			t.Fatal(err)
		}
		if got := readBlock(t, path).AutoResumeAttempts; got != attempt {
			t.Fatalf("the resume boundary write after auto-resume %d left attempts=%d, want %d carried", attempt, got, attempt)
		}
	}

	if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatal(err)
	}
	pause := readBlock(t, path)
	if pause.AutoResumeAttempts != DefaultAutoResumeAttempts || pause.OperatorAction != autoResumeSpentAction {
		t.Fatalf("pause at the cap = %+v, want attempts=%d and the spent-budget operator action", pause, DefaultAutoResumeAttempts)
	}

	cp, admitted, err := RecordResume(path, true)
	if err != nil || admitted || cp.AutoResumeAttempts != DefaultAutoResumeAttempts || cp.OperatorAction != autoResumeSpentAction {
		t.Fatalf("the fourth auto-resume = (%+v, %v, %v), want refused at the cap with the operator action", cp, admitted, err)
	}
	if !strings.Contains(autoResumeSpentAction, "evolve loop --resume") {
		t.Errorf("operator action %q, want it to name the manual resume", autoResumeSpentAction)
	}

	cp, admitted, err = RecordResume(path, false)
	if err != nil || !admitted || cp.AutoResumeAttempts != 0 {
		t.Fatalf("operator resume = (%+v, %v, %v), want admitted with the counter reset to 0", cp, admitted, err)
	}
	if got := readBlock(t, path).AutoResumeAttempts; got != 0 {
		t.Fatalf("on disk attempts=%d after the operator resume, want 0", got)
	}
}

func TestRecordResume_AnUnknownResetIsNeverAutoResumedButAnOperatorResumeRuns(t *testing.T) {
	withUsageReset(t, nil)
	root, cs := quotaCheckpointFixture(t)
	path := filepath.Join(root, ".evolve", "cycle-state.json")
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)); err != nil {
		t.Fatal(err)
	}

	if cp, admitted, err := RecordResume(path, true); err != nil || admitted || cp.OperatorAction != unknownResetAction {
		t.Fatalf("auto-resume of an unknown reset = (%+v, %v, %v), want refused with the unknown-reset action kept", cp, admitted, err)
	}
	if _, admitted, err := RecordResume(path, false); err != nil || !admitted {
		t.Fatalf("operator resume = (%v, %v), want admitted", admitted, err)
	}
}

func TestRecordResume_AStateFileWithNoReadableCheckpointIsAnError(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"no checkpoint block": `{"cycle_id":7}`,
		"malformed state":     `{`,
	} {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, admitted, err := RecordResume(path, false); err == nil || admitted {
			t.Errorf("%s: admitted=%v err=%v, want an error", name, admitted, err)
		}
	}
	if _, _, err := RecordResume(filepath.Join(dir, "missing", "cycle-state.json"), false); err == nil {
		t.Error("a missing state file: want an error")
	}
}

func TestPhaseBoundaryCheckpointer_AMalformedStateFileIsAnErrorNotAClobber(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".evolve", "cycle-state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := core.PhaseBoundaryCheckpointer(core.CycleState{CycleID: 7, Phase: "build"}, root, time.Now())

	if err == nil || !strings.Contains(err.Error(), "parse state") {
		t.Fatalf("err=%v, want the parse error: the escalation guard cannot read the state", err)
	}
	if b, _ := os.ReadFile(path); string(b) != `{` {
		t.Fatalf("the state file changed to %q, want it untouched", b)
	}
}

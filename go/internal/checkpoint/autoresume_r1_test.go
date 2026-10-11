package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const unreadableBudgetState = `{"cycle_id":656,"checkpoint":{"enabled":true,"reason":"quota-likely","autoResumeAttempts":"two","autoResumeMaxAttempts":3}}`

func writeState(t *testing.T, root, body string) string {
	t.Helper()
	path := filepath.Join(root, ".evolve", "cycle-state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCarriedAttempts_OnlyAnAbsentBlockIsZeroAndAnUnreadableBlockIsAnError(t *testing.T) {
	root := t.TempDir()
	if n, err := carriedAttempts(filepath.Join(root, "missing.json")); n != 0 || err != nil {
		t.Errorf("missing file = (%d, %v), want (0, nil)", n, err)
	}
	if n, err := carriedAttempts(writeState(t, root, `{"cycle_id":656}`)); n != 0 || err != nil {
		t.Errorf("no checkpoint block = (%d, %v), want (0, nil)", n, err)
	}
	if _, err := carriedAttempts(writeState(t, root, unreadableBudgetState)); err == nil {
		t.Error("an unreadable budget = nil error, want an error: zero would wipe the budget")
	}
	if _, err := carriedAttempts(root); err == nil || !strings.Contains(err.Error(), "read state") {
		t.Errorf("a state path that is a directory = %v, want the read error", err)
	}
	if n, err := carriedAttempts(writeState(t, root, `{"checkpoint":{"autoResumeAttempts":2}}`)); n != 2 || err != nil {
		t.Errorf("a readable block = (%d, %v), want (2, nil)", n, err)
	}
}

func TestQuotaBoundaryCheckpointer_AnUnreadableBudgetSpendsItLoudly(t *testing.T) {
	root, _, cs := knownResetFixture(t)
	path := writeState(t, root, unreadableBudgetState)

	err := core.QuotaBoundaryCheckpointer(cs, root, time.Date(2026, 7, 30, 12, 0, 0, 0, time.Local))

	if err == nil || !strings.Contains(err.Error(), "parse state") {
		t.Fatalf("err=%v, want the wrapped parse error", err)
	}
	pause := readBlock(t, path)
	if pause.AutoResumeAttempts != pause.AutoResumeMaxAttempts || pause.OperatorAction != autoResumeSpentAction {
		t.Fatalf("pause=%+v, want the budget spent with the operator action", pause)
	}
	if _, admitted, err := RecordResume(path, true); err != nil || admitted {
		t.Fatalf("the next auto-resume = (admitted %v, err %v), want refused", admitted, err)
	}
}

func TestApplyQuotaPause_AMissingStateFileIsAnError(t *testing.T) {
	if err := applyQuotaPause(filepath.Join(t.TempDir(), "cycle-state.json"), Checkpoint{AutoResumeMaxAttempts: 3}); err == nil {
		t.Fatal("want the read error")
	}
}

func TestResumeBoundaryCheckpointer_CarriesTheCountAndRefusesAnUnreadableBudget(t *testing.T) {
	root := t.TempDir()
	path := writeState(t, root, `{"cycle_id":656,"checkpoint":{"enabled":true,"reason":"quota-likely","autoResumeAttempts":2,"autoResumeMaxAttempts":3}}`)
	cs := core.CycleState{CycleID: 656, Phase: "build"}

	if err := core.ResumeBoundaryCheckpointer(cs, root, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := readBlock(t, path).AutoResumeAttempts; got != 2 {
		t.Fatalf("attempts=%d after the real resume boundary write, want 2 carried", got)
	}

	writeState(t, root, unreadableBudgetState)
	if err := core.ResumeBoundaryCheckpointer(cs, root, time.Now()); err == nil {
		t.Fatal("an unreadable budget at the resume boundary = nil error, want an error")
	}
	if b, _ := os.ReadFile(path); string(b) != unreadableBudgetState {
		t.Fatalf("the state changed to %s, want it untouched", b)
	}
}

func TestNextResume_TheCapIsInclusiveAndAnOldBlockWithNoCapFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name             string
		attempts, max    int
		wantAdmitted     bool
		wantAttemptsNext int
	}{
		{"one under the cap", 2, 3, true, 3},
		{"exactly at the cap", 3, 3, false, 3},
		{"an old block with no cap", 0, 0, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp, admitted := nextResume(Checkpoint{AutoResumeAttempts: tc.attempts, AutoResumeMaxAttempts: tc.max}, true)
			if admitted != tc.wantAdmitted || cp.AutoResumeAttempts != tc.wantAttemptsNext {
				t.Fatalf("= (%+v, %v), want admitted=%v attempts=%d", cp, admitted, tc.wantAdmitted, tc.wantAttemptsNext)
			}
			if !admitted && cp.OperatorAction == "" {
				t.Fatal("a refusal records no operator action")
			}
		})
	}
	root := t.TempDir()
	path := writeState(t, root, `{"cycle_id":656,"checkpoint":{"enabled":true,"reason":"quota-likely"}}`)
	if _, admitted, err := RecordResume(path, true); err != nil || admitted {
		t.Fatalf("an old block = (admitted %v, err %v), want refused", admitted, err)
	}
}

func TestRecordResume_AnOperatorResumeClearsTheOperatorAction(t *testing.T) {
	path := writeState(t, t.TempDir(), `{"checkpoint":{"enabled":true,"reason":"quota-likely","autoResumeAttempts":3,"autoResumeMaxAttempts":3,"operatorAction":"run it by hand"}}`)

	cp, admitted, err := RecordResume(path, false)

	if err != nil || !admitted || cp.OperatorAction != "" || readBlock(t, path).OperatorAction != "" {
		t.Fatalf("= (%+v, %v, %v), want admitted with the operator action cleared on disk", cp, admitted, err)
	}
}

package heldaudit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func writeCalls(t *testing.T, ws string, recs ...llmcalls.Record) {
	t.Helper()
	for _, r := range recs {
		if err := llmcalls.AppendWorkspace(ws, r); err != nil {
			t.Fatal(err)
		}
	}
}

func writeAuditFix(t *testing.T, ws string, rec auditfix.Record) {
	t.Helper()
	raw, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(ws, auditfix.RecordFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuilderFamilies_AGeminiBuildPatchedByAClaudeAuditFixNamesBothFamilies(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"}, llmcalls.Record{Phase: "scout", Cycle: 7, CLI: "codex-tmux", TS: "t"})
	writeAuditFix(t, ws, auditfix.Record{Auditor: auditfix.Seat{Family: "claude"}, Patch: auditfix.Patch{Diff: "x"}})

	got, err := BuilderFamilies(ws, 7, modelFamily)

	if err != nil || len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Errorf("families = %v, %v, want claude and gemini, and no scout family", got, err)
	}
}

func TestBuilderFamilies_AnAuditFixThatKeptNoPatchAddsNoFamily(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"})
	writeAuditFix(t, ws, auditfix.Record{Auditor: auditfix.Seat{Family: "claude"}})

	got, err := BuilderFamilies(ws, 7, modelFamily)

	if err != nil || len(got) != 1 || got[0] != "gemini" {
		t.Errorf("families = %v, %v, want gemini only", got, err)
	}
}

func TestBuilderFamilies_ADebuggerRecordCountsAsAWriter(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"}, llmcalls.Record{Phase: "debugger", Cycle: 7, CLI: "claude-tmux", TS: "t"})

	got, err := BuilderFamilies(ws, 7, modelFamily)

	if err != nil || len(got) != 2 {
		t.Errorf("families = %v, %v, want both writer families", got, err)
	}
}

func TestBuilderFamilies_FailsClosedOnAnUnknownEmptyOrUnreadableSource(t *testing.T) {
	t.Parallel()
	unknown := t.TempDir()
	writeCalls(t, unknown, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "mystery-tmux", TS: "t"})
	if _, err := BuilderFamilies(unknown, 7, modelFamily); !errors.Is(err, heldproposal.ErrAuditUnattributed) {
		t.Errorf("unknown cli err = %v, want ErrAuditUnattributed", err)
	}
	if _, err := BuilderFamilies(t.TempDir(), 7, modelFamily); !errors.Is(err, heldproposal.ErrAuditUnattributed) {
		t.Errorf("no records err = %v, want ErrAuditUnattributed", err)
	}
	unknownFix := t.TempDir()
	writeCalls(t, unknownFix, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"})
	writeAuditFix(t, unknownFix, auditfix.Record{Auditor: auditfix.Seat{Family: "mystery"}, Patch: auditfix.Patch{Diff: "x"}})
	if _, err := BuilderFamilies(unknownFix, 7, modelFamily); !errors.Is(err, heldproposal.ErrAuditUnattributed) {
		t.Errorf("unknown audit-fix family err = %v, want ErrAuditUnattributed", err)
	}
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, auditfix.RecordFile), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCalls(t, bad, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"})
	if _, err := BuilderFamilies(bad, 7, modelFamily); !errors.Is(err, heldproposal.ErrAuditUnattributed) {
		t.Errorf("corrupt audit-fix err = %v, want ErrAuditUnattributed", err)
	}
	corruptCalls := t.TempDir()
	if err := os.WriteFile(llmcalls.Path(corruptCalls), []byte("{bad\n"), 0o644); err == nil {
		if _, err := BuilderFamilies(corruptCalls, 7, modelFamily); err == nil {
			t.Error("an unreadable dispatch record must fail closed")
		}
	}
}

func TestBuilderFamilies_AnAuditFixPatchWithOnlyChangesStillNamesItsFamily(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeCalls(t, ws, llmcalls.Record{Phase: "build", Cycle: 7, CLI: "agy-tmux", TS: "t"})
	writeAuditFix(t, ws, auditfix.Record{Auditor: auditfix.Seat{Family: "claude"}, Patch: auditfix.Patch{Changes: []auditfix.Change{{Path: "go/x.go"}}}})

	got, err := BuilderFamilies(ws, 7, modelFamily)

	if err != nil || len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Errorf("families = %v, %v, want claude and gemini: a patch with changes and no diff text still has an author", got, err)
	}
}

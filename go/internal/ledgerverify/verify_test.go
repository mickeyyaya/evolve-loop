package ledgerverify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// fakeLedger lets a test drive VerifyCycle from a slice of entries
// without touching the real FileLedger format. Useful for the error
// branch (iter returns an error) which a healthy FileLedger never hits.
type fakeLedger struct {
	entries []core.LedgerEntry
	iterErr error
	nextErr error
}

func (f *fakeLedger) Append(context.Context, core.LedgerEntry) error { return nil }
func (f *fakeLedger) Verify(context.Context) error                   { return nil }
func (f *fakeLedger) Iter(context.Context) (core.LedgerIterator, error) {
	if f.iterErr != nil {
		return nil, f.iterErr
	}
	return &fakeIter{entries: f.entries, nextErr: f.nextErr}, nil
}

type fakeIter struct {
	entries []core.LedgerEntry
	i       int
	nextErr error
}

func (it *fakeIter) Next() (core.LedgerEntry, bool, error) {
	if it.nextErr != nil {
		return core.LedgerEntry{}, false, it.nextErr
	}
	if it.i >= len(it.entries) {
		return core.LedgerEntry{}, false, nil
	}
	e := it.entries[it.i]
	it.i++
	return e, true, nil
}
func (it *fakeIter) Close() error { return nil }

// entry is a terse helper for table-driven tests.
func entry(cycle int, role, kind string, rc int) core.LedgerEntry {
	return core.LedgerEntry{Cycle: cycle, Role: role, Kind: kind, ExitCode: rc}
}

func TestVerifyCycle_CompletePipeline(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "scout", "agent_subprocess", 0),
		entry(1, "builder", "agent_subprocess", 0),
		entry(1, "auditor", "agent_subprocess", 0),
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("expected OK; missing=%v", r.Missing)
	}
	if r.Scout != 1 || r.Builder != 1 || r.Auditor != 1 {
		t.Fatalf("counts: scout=%d builder=%d auditor=%d", r.Scout, r.Builder, r.Auditor)
	}
	if got, want := len(r.Required), 3; got != want {
		t.Fatalf("required len=%d want %d", got, want)
	}
}

func TestVerifyCycle_MissingScout(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "builder", "agent_subprocess", 0),
		entry(1, "auditor", "agent_subprocess", 0),
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if r.OK {
		t.Fatalf("expected not OK")
	}
	if len(r.Missing) != 1 || r.Missing[0] != "scout" {
		t.Fatalf("missing=%v", r.Missing)
	}
}

func TestVerifyCycle_NonZeroExitDoesNotCount(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "scout", "agent_subprocess", 0),
		entry(1, "builder", "agent_subprocess", 124), // timeout — does not count
		entry(1, "auditor", "agent_subprocess", 0),
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if r.OK {
		t.Fatalf("builder with rc=124 should not count toward verify")
	}
	if r.Builder != 0 {
		t.Fatalf("builder count=%d want 0", r.Builder)
	}
}

func TestVerifyCycle_WrongCycleIgnored(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		// Wrong cycle — must not satisfy cycle 1's requirement.
		entry(2, "scout", "agent_subprocess", 0),
		entry(2, "builder", "agent_subprocess", 0),
		entry(2, "auditor", "agent_subprocess", 0),
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if r.OK {
		t.Fatalf("cycle 1 has no entries, should be NOT OK")
	}
	if len(r.Missing) != 3 {
		t.Fatalf("missing=%v want all 3", r.Missing)
	}
}

// Bookkeeping kinds (agent_fanout, cycle_terminal, routing_decision, …)
// must NOT satisfy a required role — only agent_subprocess and phase do.
func TestVerifyCycle_BookkeepingKindIgnored(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "scout", "routing_decision", 0), // bookkeeping kind
		entry(1, "builder", "agent_subprocess", 0),
		entry(1, "auditor", "agent_subprocess", 0),
	}}
	r, _ := VerifyCycle(context.Background(), l, 1, Options{})
	if r.OK {
		t.Fatalf("scout via bookkeeping kind should not count")
	}
	if r.Scout != 0 {
		t.Fatalf("scout count=%d want 0 (routing_decision kind ignored)", r.Scout)
	}
}

// cycle-137 regression: the Go-native orchestrator records kind="phase"
// with PHASE-name roles (scout, build, audit) — never agent_subprocess.
// VerifyCycle must accept this vocabulary or it false-negatives every
// native cycle as "missing [scout builder auditor]".
func TestVerifyCycle_GoNativePhaseVocabulary(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "scout", "phase", 0),
		entry(1, "triage", "phase", 0),        // not required; ignored
		entry(1, "tdd", "phase", 0),           // not required; ignored
		entry(1, "build-planner", "phase", 0), // not required; ignored
		entry(1, "build", "phase", 0),         // canonicalizes to builder
		entry(1, "audit", "phase", 0),         // canonicalizes to auditor
		entry(1, "retro", "phase", 0),         // not required; ignored
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("Go-native phase vocabulary should verify OK; missing=%v", r.Missing)
	}
	if r.Scout != 1 || r.Builder != 1 || r.Auditor != 1 {
		t.Fatalf("counts: scout=%d builder=%d auditor=%d (want 1/1/1)", r.Scout, r.Builder, r.Auditor)
	}
}

// Mixed vocabularies (a cycle with both bash agent_subprocess and Go
// phase entries) must not double-fault: either spelling satisfies a role.
func TestVerifyCycle_MixedVocabularies(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "scout", "agent_subprocess", 0),   // bash spelling
		entry(1, "build", "phase", 0),              // Go spelling → builder
		entry(1, "auditor", "agent_subprocess", 0), // bash spelling
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("mixed vocabularies should verify OK; missing=%v", r.Missing)
	}
}

// The Go-native PASS path: intent + memo also arrive as kind="phase".
func TestVerifyCycle_GoNativeIntentAndMemoPhases(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{entries: []core.LedgerEntry{
		entry(1, "intent", "phase", 0),
		entry(1, "scout", "phase", 0),
		entry(1, "build", "phase", 0),
		entry(1, "audit", "phase", 0),
		entry(1, "memo", "phase", 0),
	}}
	r, err := VerifyCycle(context.Background(), l, 1, Options{IntentRequired: true, CycleVerdict: "PASS"})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("intent+memo as phase entries should verify OK; missing=%v", r.Missing)
	}
	if r.Intent != 1 || r.Memo != 1 {
		t.Fatalf("intent=%d memo=%d (want 1/1)", r.Intent, r.Memo)
	}
}

func TestVerifyCycle_IntentRequired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		intent   bool
		entries  []core.LedgerEntry
		wantOK   bool
		wantMiss []string
	}{
		{
			name:   "intent_required true + intent present → OK",
			intent: true,
			entries: []core.LedgerEntry{
				entry(1, "intent", "agent_subprocess", 0),
				entry(1, "scout", "agent_subprocess", 0),
				entry(1, "builder", "agent_subprocess", 0),
				entry(1, "auditor", "agent_subprocess", 0),
			},
			wantOK: true,
		},
		{
			name:   "intent_required true + intent missing → NOT OK",
			intent: true,
			entries: []core.LedgerEntry{
				entry(1, "scout", "agent_subprocess", 0),
				entry(1, "builder", "agent_subprocess", 0),
				entry(1, "auditor", "agent_subprocess", 0),
			},
			wantOK:   false,
			wantMiss: []string{"intent"},
		},
		{
			name:   "intent_required false + intent missing → OK",
			intent: false,
			entries: []core.LedgerEntry{
				entry(1, "scout", "agent_subprocess", 0),
				entry(1, "builder", "agent_subprocess", 0),
				entry(1, "auditor", "agent_subprocess", 0),
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := &fakeLedger{entries: tc.entries}
			r, err := VerifyCycle(context.Background(), l, 1, Options{IntentRequired: tc.intent})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if r.OK != tc.wantOK {
				t.Fatalf("ok=%v want %v missing=%v", r.OK, tc.wantOK, r.Missing)
			}
			if tc.wantMiss != nil {
				if len(r.Missing) != len(tc.wantMiss) || r.Missing[0] != tc.wantMiss[0] {
					t.Fatalf("missing=%v want %v", r.Missing, tc.wantMiss)
				}
			}
		})
	}
}

func TestVerifyCycle_MemoOnPASS(t *testing.T) {
	t.Parallel()
	base := []core.LedgerEntry{
		entry(1, "scout", "agent_subprocess", 0),
		entry(1, "builder", "agent_subprocess", 0),
		entry(1, "auditor", "agent_subprocess", 0),
	}
	tests := []struct {
		name     string
		verdict  string
		withMemo bool
		wantOK   bool
	}{
		{"PASS + memo present → OK", "PASS", true, true},
		{"PASS + memo missing → NOT OK", "PASS", false, false},
		{"FAIL + memo missing → OK (no memo requirement)", "FAIL", false, true},
		{"WARN + memo missing → OK (only PASS triggers memo gate)", "WARN", false, true},
		{"empty verdict + memo missing → OK", "", false, true},
		{"pass (lowercase) treated as PASS via EqualFold → memo required", "pass", false, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries := append([]core.LedgerEntry{}, base...)
			if tc.withMemo {
				entries = append(entries, entry(1, "memo", "agent_subprocess", 0))
			}
			r, err := VerifyCycle(context.Background(), &fakeLedger{entries: entries}, 1, Options{CycleVerdict: tc.verdict})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if r.OK != tc.wantOK {
				t.Fatalf("ok=%v want %v missing=%v", r.OK, tc.wantOK, r.Missing)
			}
		})
	}
}

func TestVerifyCycle_LedgerIterError(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{iterErr: errors.New("boom")}
	if _, err := VerifyCycle(context.Background(), l, 1, Options{}); err == nil {
		t.Fatalf("expected error from iter")
	}
}

func TestVerifyCycle_LedgerNextError(t *testing.T) {
	t.Parallel()
	l := &fakeLedger{nextErr: errors.New("read err")}
	if _, err := VerifyCycle(context.Background(), l, 1, Options{}); err == nil {
		t.Fatalf("expected error from next")
	}
}

func TestVerifyCycle_AgainstRealFileLedger(t *testing.T) {
	t.Parallel()
	// Integration smoke: write entries via the real ledger adapter,
	// then verify reads them back correctly. Catches any
	// LedgerEntry-json-shape drift between writer and counter.
	dir := t.TempDir()
	l := ledger.New(dir)
	ctx := context.Background()
	for _, e := range []core.LedgerEntry{
		entry(1, "scout", "agent_subprocess", 0),
		entry(1, "builder", "agent_subprocess", 0),
		entry(1, "auditor", "agent_subprocess", 0),
		entry(1, "memo", "agent_subprocess", 0),
	} {
		if err := l.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	r, err := VerifyCycle(ctx, l, 1, Options{CycleVerdict: "PASS"})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !r.OK {
		t.Fatalf("expected OK against real ledger; missing=%v", r.Missing)
	}
	if r.Memo != 1 {
		t.Fatalf("memo=%d want 1", r.Memo)
	}
}

func TestLoadVerifyContext_PerCyclePriority(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	ev := t.TempDir()
	// Per-cycle says true, global says false — per-cycle must win.
	if err := os.WriteFile(filepath.Join(ws, "cycle-state.json"), []byte(`{"intent_required":true}`), 0o644); err != nil {
		t.Fatalf("write per-cycle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ev, "state.json"), []byte(`{"intent_required":false}`), 0o644); err != nil {
		t.Fatalf("write global: %v", err)
	}
	vc := LoadVerifyContext(ws, ev)
	if !vc.IntentRequired {
		t.Fatalf("per-cycle should win; got intent_required=false")
	}
}

func TestLoadVerifyContext_FallbackToGlobal(t *testing.T) {
	t.Parallel()
	ws := t.TempDir() // no cycle-state.json
	ev := t.TempDir()
	if err := os.WriteFile(filepath.Join(ev, "state.json"), []byte(`{"intent_required":true}`), 0o644); err != nil {
		t.Fatalf("write global: %v", err)
	}
	vc := LoadVerifyContext(ws, ev)
	if !vc.IntentRequired {
		t.Fatalf("should fall back to global state.json; got false")
	}
}

func TestLoadVerifyContext_VerdictFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, ".cycle-verdict"), []byte(" PASS \n"), 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
	vc := LoadVerifyContext(ws, "")
	if vc.CycleVerdict != "PASS" {
		t.Fatalf("verdict=%q want PASS (trimmed)", vc.CycleVerdict)
	}
}

func TestLoadVerifyContext_AllMissing(t *testing.T) {
	t.Parallel()
	vc := LoadVerifyContext(t.TempDir(), t.TempDir())
	if vc.IntentRequired || vc.CycleVerdict != "" {
		t.Fatalf("expected zero values; got %+v", vc)
	}
}

func TestLoadVerifyContext_BadJSON(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "cycle-state.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	vc := LoadVerifyContext(ws, "")
	if vc.IntentRequired {
		t.Fatalf("bad JSON must default to false")
	}
}

func TestLoadVerifyContext_NonBoolField(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// intent_required as a string ("true") — must NOT coerce to bool.
	if err := os.WriteFile(filepath.Join(ws, "cycle-state.json"), []byte(`{"intent_required":"true"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	vc := LoadVerifyContext(ws, "")
	if vc.IntentRequired {
		t.Fatalf("string field must not coerce; got intent_required=true")
	}
}

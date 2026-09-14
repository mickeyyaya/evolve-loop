package defectledger

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 36 — the collaborators are REQUIRED: New(nil, resolver) panics at the
// first arm, New(reader, nil) at the first FIXED claim; WithSignals(nil) and a
// nil-returning accessor are the Null Object (every path runs, nothing is
// reported); a recording Center is reached live and stamps Module audit, Kind
// audit.warning, Phase audit, the cycle and the exported method as origin.
func TestNew_RequiredDepsPanicAndSignalsNullObject(t *testing.T) {
	f := newFixture(t)
	f.removeManifest(t)
	mustPanic(t, "nil lane-scope reader", func() { New(nil, resolveNever).Reconcile(f.req) })
	f = newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
	mustPanic(t, "nil resolver", func() { New(scopeOf(), nil).Reconcile(f.req) })

	noop := Option(func(*Ledger) {})
	for name, l := range map[string]*Ledger{
		"no option":        New(scopeOf(), resolveUnder(f.root)),
		"a no-op Option":   New(scopeOf(), resolveUnder(f.root), noop),
		"WithSignals(nil)": New(scopeOf(), resolveUnder(f.root), WithSignals(nil)),
		"nil-returning":    New(scopeOf(), resolveUnder(f.root), WithSignals(func() *signalcenter.Center { return nil })),
	} {
		if l.SignalsWired() {
			t.Fatalf("%s: the Null Object", name)
		}
		f.claims(t, `{"dispositions":[]}`)
		if v := l.Reconcile(f.req); !v.Blocked {
			t.Fatalf("%s: every path runs unwired: %+v", name, v)
		}
		if block := l.PromptBlock(f.req); !strings.Contains(block, "d1") {
			t.Fatalf("%s: the prompt renders unwired: %q", name, block)
		}
	}

	acc, got := recording()
	var live *signalcenter.Center
	l := New(scopeOf(), resolveUnder(f.root), WithSignals(func() *signalcenter.Center { return live }))
	if l.SignalsWired() {
		t.Fatal("the accessor is read live: nil now")
	}
	live = acc()
	if !l.SignalsWired() {
		t.Fatal("the accessor is read live: wired now")
	}
	l.Reconcile(f.req)
	if len(*got) == 0 {
		t.Fatal("events flow to the late Center")
	}
	for _, e := range *got {
		if e.Module != signalcenter.ModuleAudit || e.Kind != signalcenter.KindAuditWarning || e.Phase != "audit" || e.Cycle != 1270 || !strings.HasPrefix(e.Origin, "Ledger.") || !signalcenter.ValidOrigin(e.Origin) || e.Fields["drift"] != "" {
			t.Fatalf("stamped for triage, no drift: %+v", e)
		}
	}
}

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: a forgotten collaborator must panic at first use, never disarm silently", name)
		}
	}()
	fn()
}

// Test 37 — all thirteen codes are registered under module audit with a
// doc, carry the family prefix, and have a fixed severity: INFO for the
// prompt degrade, WARN for the twelve faults.
func TestCodes_RegisteredUnderModuleAudit_WithFixedSeverity(t *testing.T) {
	all := []signalcenter.Code{CodeEmitFailed, CodeOverflow, CodeManifestUnreadable, CodeManifestMissing, CodeLineageDisagrees,
		CodeLedgerUnreadable, CodeAncestorEmpty, CodeWritebackFailed, CodeDefectsUnaccounted, CodeDispositionsMissing,
		CodeDispositionsIncomplete, CodeDispositionsUnreadable, CodePromptDegraded}
	docs := map[signalcenter.Code]string{}
	for _, cd := range signalcenter.RegisteredCodes()[signalcenter.ModuleAudit] {
		docs[cd.Code] = cd.Doc
	}
	if len(all) != 13 || len(codeSeverity) != 13 {
		t.Fatalf("thirteen codes, thirteen severity rows: %d %d", len(all), len(codeSeverity))
	}
	for _, c := range all {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleAudit || !c.BelongsTo(signalcenter.ModuleAudit) || !strings.HasPrefix(string(c), "AUDIT_LEDGER_") {
			t.Fatalf("%s: registered under audit with the family prefix", c)
		}
		if strings.TrimSpace(docs[c]) == "" {
			t.Fatalf("%s: a doc for signal-codes.md", c)
		}
		want := signalcenter.SeverityWarn
		if c == CodePromptDegraded {
			want = signalcenter.SeverityInfo
		}
		if codeSeverity[c] != want {
			t.Fatalf("%s: severity %s, want %s", c, codeSeverity[c], want)
		}
	}
	if len(docs) != 13 {
		t.Fatalf("module audit registers exactly the thirteen: %d", len(docs))
	}
}

// The vocabulary's spellings — the ONE home audit, carryover and the seeder
// project — and the producer's id-list bound.
func TestVocabulary_IsTheProducersSpelling(t *testing.T) {
	if LedgerFile != "defect-ledger.json" || DispositionsFile != "defect-dispositions.json" || StatusOpen != "OPEN" || StatusFixed != "FIXED" || StatusDeferred != "DEFERRED" ||
		PrescriptionPrefix != "PRESCRIPTION: " || MaxEntries != 64 || TextMaxRunes != 2000 ||
		PreflightMissingMarker != "disposition-preflight: MISSING" || PreflightIncompleteMarker != "disposition-preflight: INCOMPLETE" {
		t.Fatal("the vocabulary drifted")
	}
	if !strings.Contains(DispositionsSchemaExample, `"evidence": "go/internal/phases/audit/defect_ledger.go:267-356"`) || !strings.HasPrefix(DispositionsSchemaExample, `{"dispositions": [`) {
		t.Fatalf("the schema example is byte-verbatim (three homes): %s", DispositionsSchemaExample)
	}
	if head, truncated := boundedIDs([]string{"a"}); head != "a" || truncated != "false" {
		t.Fatalf("boundedIDs: %q %s", head, truncated)
	}
	if head, truncated := boundedIDs([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}); head != "0, 1, 2, 3, 4, 5, 6, 7" || truncated != "true" {
		t.Fatalf("bounded to eight: %q %s", head, truncated)
	}
}

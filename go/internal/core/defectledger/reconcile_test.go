package defectledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// G4 replay — every diagnostic scenario captured on 8e8f080f
// (diagnostics.golden.json) rebuilt on the same fixture and graded through the
// leaf: the messages, severities, order, block decision and lineage are
// byte-identical.
func TestReconcile_DiagnosticsAreByteIdenticalToTheGoldens(t *testing.T) {
	cases := map[string]func(t *testing.T, f fixture) LaneScopeReader{
		"corrupt_manifest": func(t *testing.T, f fixture) LaneScopeReader {
			f.write(t, ".evolve/runs/cycle-1270/continuation-manifest.json", "{")
			return scopeOf()
		},
		"registry_binds_manifest_missing": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			f.bindRegistry(t, 1255)
			f.removeManifest(t)
			return scopeOf("lane-scope-id")
		},
		"manifest_registry_disagree": func(t *testing.T, f fixture) LaneScopeReader {
			f.bindRegistry(t, 1200)
			return scopeOf("lane-scope-id")
		},
		"ancestor_unreadable_parse": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, "garbage")
			return scopeOf()
		},
		"ancestor_unreadable_read": func(t *testing.T, f fixture) LaneScopeReader {
			if err := os.MkdirAll(filepath.Join(f.ancestorWS, LedgerFile), 0o755); err != nil {
				t.Fatal(err)
			}
			return scopeOf()
		},
		"ancestor_absent": func(t *testing.T, f fixture) LaneScopeReader { return scopeOf() },
		"ancestor_empty": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, `{"origin_cycle":1255,"entries":[]}`)
			return scopeOf()
		},
		"own_unreadable": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			f.ownLedger(t, "garbage")
			return scopeOf()
		},
		"dispositions_missing": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			return scopeOf()
		},
		"dispositions_missing_nothing_owed": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, `{"origin_cycle":1255,"entries":[{"id":"d1","text":"first defect","status":"FIXED","evidence":"docs/x.md"}]}`)
			return scopeOf()
		},
		"dispositions_read_error": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			if err := os.MkdirAll(filepath.Join(f.ws, DispositionsFile), 0o755); err != nil {
				t.Fatal(err)
			}
			return scopeOf()
		},
		"dispositions_unparseable": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":{"a":1}}]}`)
			return scopeOf()
		},
		"incomplete_unaccounted": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorTwoOpen)
			f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
			return scopeOf()
		},
		"status_table": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, `{"origin_cycle":1255,"entries":[{"id":"d1","text":"a","status":"OPEN"},{"id":"d2","text":"b","status":"OPEN"},{"id":"d3","text":"c","status":"OPEN"},{"id":"d4","text":"d","status":"OPEN"}]}`)
			f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"nowhere/none.go"},{"id":"d2","status":"DEFERRED","reason":"  "},{"id":"d3","status":"WONTFIX"},{"id":"d4","status":"OPEN","text":"d","reason":""}]}`)
			return scopeOf()
		},
		"reconciled_writeback": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, reconciledAncestor)
			f.ownLedger(t, reconciledCurrent)
			f.claims(t, reconciledClaims)
			return scopeOf()
		},
		"graded_clean": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, `{"origin_cycle":1250,"entries":[{"id":"d1","text":"a","status":"OPEN"},{"id":"d2","text":"b","status":"OPEN"}]}`)
			f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go:1-2"},{"id":"d2","status":"DEFERRED","reason":"later"}]}`)
			return scopeOf()
		},
		"writeback_failed": func(t *testing.T, f fixture) LaneScopeReader {
			f.ancestorLedger(t, ancestorOneOpen)
			f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
			if err := os.Chmod(f.ws, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(f.ws, 0o755) })
			return scopeOf()
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			scope := arrange(t, f)
			l, _ := observed(scope, resolveUnder(f.root))
			assertVerdict(t, name, l.Reconcile(f.req), goldenScenario(t, name, f))
		})
	}
}

func fieldsOf(t *testing.T, e signalcenter.Event, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if e.Fields[k] != v {
			t.Fatalf("%s: fields[%s] = %q, want %q (%v)", e.Code, k, e.Fields[k], v, e.Fields)
		}
	}
	if e.Module != signalcenter.ModuleAudit || e.Kind != signalcenter.KindAuditWarning || e.Phase != "audit" {
		t.Fatalf("%s: module/kind/phase: %+v", e.Code, e)
	}
}

// Test 15 — no manifest and a nil lane-scope pin ⇒ the zero Verdict, no event.
func TestReconcile_NotAContinuation_IsANoOp(t *testing.T) {
	f := newFixture(t)
	f.removeManifest(t)
	l, got := observed(scopeOf(), resolveNever)
	if v := l.Reconcile(f.req); v.Blocked || v.Diagnostics != nil || v.LineageCycles != nil || len(*got) != 0 {
		t.Fatalf("no-op: %+v %v", v, codesOf(*got))
	}
	if v := l.Reconcile(Request{Cycle: 1, Workspace: "", ProjectRoot: f.root}); v.Blocked || v.Diagnostics != nil {
		t.Fatalf("the guard: %+v", v)
	}
}

// Test 16 — a corrupt manifest blocks with the golden text and reports
// AUDIT_LEDGER_MANIFEST_UNREADABLE {step=arm, blocked=true, workspace}.
func TestReconcile_CorruptManifest_BlocksVerbatim(t *testing.T) {
	f := newFixture(t)
	f.write(t, ".evolve/runs/cycle-1270/continuation-manifest.json", "{")
	f.bindRegistry(t, 1255)
	l, got := observed(scopeOf("lane-scope-id"), resolveNever)
	v := l.Reconcile(f.req)
	assertVerdict(t, "corrupt_manifest", v, goldenScenario(t, "corrupt_manifest", f))
	e := only(t, *got, CodeManifestUnreadable)
	fieldsOf(t, e, map[string]string{"step": "arm", "blocked": "true", "workspace": f.ws})
	if e.Origin != "Ledger.Reconcile" || e.Cycle != 1270 || e.Severity != signalcenter.SeverityWarn || len(*got) != 1 {
		t.Fatalf("one WARN from the arm: %+v", *got)
	}
}

// Test 17 — the registry binds the lane, the manifest is gone: the missing-
// manifest finding is PREPENDED to the graded diagnostics; blocked;
// AUDIT_LEDGER_MANIFEST_MISSING {registry_path, ancestor_cycle}.
func TestReconcile_RegistryBindsManifestMissing_PrependsThenGrades(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.bindRegistry(t, 1255)
	f.removeManifest(t)
	l, got := observed(scopeOf("lane-scope-id"), resolveUnder(f.root))
	v := l.Reconcile(f.req)
	assertVerdict(t, "registry_binds_manifest_missing", v, goldenScenario(t, "registry_binds_manifest_missing", f))
	e := only(t, *got, CodeManifestMissing)
	fieldsOf(t, e, map[string]string{"step": "arm", "blocked": "true", "registry_path": filepath.Join(f.root, ".evolve", "continuation-registry.json"), "ancestor_cycle": "1255"})
	if want := []signalcenter.Code{CodeManifestMissing, CodeDispositionsMissing, CodeDefectsUnaccounted}; strings.Join(codeStrings(*got), ",") != strings.Join(codeStrings(want), ",") {
		t.Fatalf("the arm's finding precedes the grade's: %v", codesOf(*got))
	}
}

func codeStrings(in any) []string {
	var out []string
	switch v := in.(type) {
	case []signalcenter.Event:
		for _, e := range v {
			out = append(out, string(e.Code))
		}
	case []signalcenter.Code:
		for _, c := range v {
			out = append(out, string(c))
		}
	}
	return out
}

// Test 18 — manifest and registry naming different ancestors block;
// AUDIT_LEDGER_LINEAGE_DISAGREES {manifest_cycle, registry_cycle}.
func TestReconcile_ManifestAndRegistryDisagree_Blocks(t *testing.T) {
	f := newFixture(t)
	f.bindRegistry(t, 1200)
	l, got := observed(scopeOf("lane-scope-id"), resolveNever)
	assertVerdict(t, "manifest_registry_disagree", l.Reconcile(f.req), goldenScenario(t, "manifest_registry_disagree", f))
	fieldsOf(t, only(t, *got, CodeLineageDisagrees), map[string]string{"step": "arm", "blocked": "true", "manifest_cycle": "1255", "registry_cycle": "1200"})
}

// Test 19 — a nil lane-scope pin never arms from the registry, however it binds.
func TestReconcile_NilLaneScope_NeverArmsFromTheRegistry(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.bindRegistry(t, 1255)
	f.removeManifest(t)
	l, got := observed(scopeOf(), resolveNever)
	if v := l.Reconcile(f.req); v.Blocked || len(v.Diagnostics) != 0 || len(*got) != 0 {
		t.Fatalf("the recorded ceiling: %+v %v", v, codesOf(*got))
	}
}

// Test 20 — a garbage registry is a miss (fail-closed is unavailable): not armed, no signal.
func TestReconcile_RegistryReadErrorIsAMiss(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.write(t, ".evolve/continuation-registry.json", "{garbage")
	f.removeManifest(t)
	l, got := observed(scopeOf("lane-scope-id"), resolveNever)
	if v := l.Reconcile(f.req); v.Blocked || len(v.Diagnostics) != 0 || len(*got) != 0 {
		t.Fatalf("a miss is a miss: %+v %v", v, codesOf(*got))
	}
}

// Test 21 — an unreadable ancestor ledger blocks: AUDIT_LEDGER_UNREADABLE
// {which=ancestor, op=parse|read, path, ancestor_cycle}.
func TestReconcile_AncestorLedgerUnreadable_Blocks(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, "garbage")
	l, got := observed(scopeOf(), resolveNever)
	assertVerdict(t, "ancestor_unreadable_parse", l.Reconcile(f.req), goldenScenario(t, "ancestor_unreadable_parse", f))
	fieldsOf(t, only(t, *got, CodeLedgerUnreadable), map[string]string{"step": "grade", "blocked": "true", "which": "ancestor", "op": "parse", "path": filepath.Join(f.ancestorWS, LedgerFile), "ancestor_cycle": "1255"})

	f = newFixture(t)
	if err := os.MkdirAll(filepath.Join(f.ancestorWS, LedgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	l, got = observed(scopeOf(), resolveNever)
	assertVerdict(t, "ancestor_unreadable_read", l.Reconcile(f.req), goldenScenario(t, "ancestor_unreadable_read", f))
	fieldsOf(t, only(t, *got, CodeLedgerUnreadable), map[string]string{"which": "ancestor", "op": "read"})
}

// Test 22 — an absent or empty ancestor ledger warns without blocking or
// vouching: AUDIT_LEDGER_ANCESTOR_EMPTY {blocked=false, ancestor_cycle, path}.
func TestReconcile_AncestorEmptyOrAbsent_WarnsWithoutBlockingOrVouching(t *testing.T) {
	for name, body := range map[string]string{"ancestor_absent": "", "ancestor_empty": `{"origin_cycle":1255,"entries":[]}`} {
		f := newFixture(t)
		if body != "" {
			f.ancestorLedger(t, body)
		}
		l, got := observed(scopeOf(), resolveNever)
		v := l.Reconcile(f.req)
		assertVerdict(t, name, v, goldenScenario(t, name, f))
		if v.Diagnostics[0].Severity != "warning" || v.LineageCycles != nil {
			t.Fatalf("%s: a warning, no vouch: %+v", name, v)
		}
		fieldsOf(t, only(t, *got, CodeAncestorEmpty), map[string]string{"step": "grade", "blocked": "false", "ancestor_cycle": "1255", "path": filepath.Join(f.ancestorWS, LedgerFile)})
	}
}

// Test 23 — this cycle's own ledger unreadable blocks: {which=own}.
func TestReconcile_OwnLedgerUnreadable_Blocks(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.ownLedger(t, "garbage")
	l, got := observed(scopeOf(), resolveNever)
	assertVerdict(t, "own_unreadable", l.Reconcile(f.req), goldenScenario(t, "own_unreadable", f))
	fieldsOf(t, only(t, *got, CodeLedgerUnreadable), map[string]string{"step": "grade", "blocked": "true", "which": "own", "op": "parse", "path": filepath.Join(f.ws, LedgerFile)})
}

// Test 26 — an absent dispositions file with defects owed yields TWO
// diagnostics (the warning, then MISSING) and ONE signal per code; with
// nothing owed the warning stands alone and nothing is signalled.
func TestReconcile_AbsentDispositionsFile_TwoDiagnosticsOneSignal(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	l, got := observed(scopeOf(), resolveNever)
	assertVerdict(t, "dispositions_missing", l.Reconcile(f.req), goldenScenario(t, "dispositions_missing", f))
	if want := "AUDIT_LEDGER_DISPOSITIONS_MISSING,AUDIT_LEDGER_DEFECTS_UNACCOUNTED"; strings.Join(codeStrings(*got), ",") != want {
		t.Fatalf("one signal per code, in order: %v", codesOf(*got))
	}
	fieldsOf(t, only(t, *got, CodeDispositionsMissing), map[string]string{"step": "preflight", "blocked": "true", "ancestor_cycle": "1255", "open": "1", "path": filepath.Join(f.ws, DispositionsFile)})
	if e := only(t, *got, CodeDispositionsMissing); e.Origin != "Ledger.Preflight" {
		t.Fatalf("origin: %s", e.Origin)
	}

	f = newFixture(t)
	f.ancestorLedger(t, `{"origin_cycle":1255,"entries":[{"id":"d1","text":"first defect","status":"FIXED","evidence":"docs/x.md"}]}`)
	l, got = observed(scopeOf(), resolveNever)
	assertVerdict(t, "dispositions_missing_nothing_owed", l.Reconcile(f.req), goldenScenario(t, "dispositions_missing_nothing_owed", f))
	if len(*got) != 0 {
		t.Fatalf("nothing owed, nothing signalled: %v", codesOf(*got))
	}
}

// Test 27 — the merge: the first current row wins the index on a duplicated
// id; an inherited id whose current text differs is shadowed (the 120-rune
// cut of the planted text) and reset to the ancestor's text, OPEN.
func TestMergeInherited_FirstRowWinsAndShadowResetsToAncestorText(t *testing.T) {
	current := []Entry{{ID: "d1", Text: "A", Status: StatusOpen}, {ID: "d1", Text: "B", Status: StatusOpen}}
	ancestor := []Entry{{ID: "d1", Text: "A", Status: StatusOpen}}
	merged, missing := mergeInherited(current, ancestor, map[string]Entry{"d1": {ID: "d1", Status: StatusDeferred, Reason: "r"}}, nil)
	if len(missing) != 0 || len(merged) != 2 || merged[0].Status != StatusDeferred || merged[1].Text != "B" || merged[1].Status != StatusOpen {
		t.Fatalf("first row wins, the duplicate untouched: %+v %v", merged, missing)
	}
	planted := strings.Repeat("p", 130)
	current = []Entry{{ID: "d1", Text: planted, Status: StatusFixed, Evidence: "x"}}
	merged, missing = mergeInherited(current, ancestor, map[string]Entry{"d1": {ID: "d1", Status: StatusDeferred, Reason: "r"}}, nil)
	want := "d1 (id shadowed: this cycle's ledger holds different text " + strconv.Quote(strings.Repeat("p", 120)+"…[truncated]") + " for the same id)"
	if len(missing) != 1 || missing[0].String() != want || missing[0].id != "d1" {
		t.Fatalf("shadow named with the cut text: %v", missing)
	}
	if merged[0] != (Entry{ID: "d1", Text: "A", Status: StatusOpen}) {
		t.Fatalf("reset to the ancestor's text, OPEN: %+v", merged[0])
	}
}

// Test 28 — the status table over a stub resolver: the four reason strings
// verbatim; a rejected FIXED row carries no evidence or reason.
func TestGradeClaim_StatusTable(t *testing.T) {
	a := Entry{ID: "d1", Text: "t", Status: StatusOpen}
	open := Entry{ID: "d1", Text: "t", Status: StatusOpen}
	yes := func(string) (bool, string) { return true, "" }
	no := func(string) (bool, string) { return false, "evidence \"x\" resolves to no file under the project root" }
	cases := []struct {
		name    string
		claim   Entry
		has     bool
		resolve func(string) (bool, string)
		want    Entry
		why     string
	}{
		{"no claim", Entry{}, false, yes, open, "no disposition"},
		{"FIXED resolving", Entry{Status: StatusFixed, Evidence: "go/x.go:1", Reason: "done"}, true, yes, Entry{ID: "d1", Text: "t", Status: StatusFixed, Evidence: "go/x.go:1", Reason: "done"}, ""},
		{"FIXED unresolvable", Entry{Status: StatusFixed, Evidence: "x", Reason: "done"}, true, no, open, "FIXED but evidence \"x\" resolves to no file under the project root"},
		{"DEFERRED with reason", Entry{Status: StatusDeferred, Reason: "later"}, true, yes, Entry{ID: "d1", Text: "t", Status: StatusDeferred, Reason: "later"}, ""},
		{"DEFERRED blank", Entry{Status: StatusDeferred, Reason: "  "}, true, yes, open, "DEFERRED without reason"},
		{"OPEN (the seeded row)", Entry{Status: StatusOpen}, true, yes, open, "status \"OPEN\" is not FIXED or DEFERRED"},
		{"WONTFIX", Entry{Status: "WONTFIX"}, true, yes, open, "status \"WONTFIX\" is not FIXED or DEFERRED"},
	}
	for _, tc := range cases {
		got, why := gradeClaim(a, tc.claim, tc.has, tc.resolve)
		if got != tc.want || why != tc.why {
			t.Fatalf("%s: got %+v %q, want %+v %q", tc.name, got, why, tc.want, tc.why)
		}
	}
}

// Test 29 — an ancestor row dispositioned upstream is carried verbatim and
// never graded: the counting resolver sees zero calls.
func TestMergeInherited_UpstreamDispositionedRowCarriedVerbatim(t *testing.T) {
	calls := 0
	counting := func(string) (bool, string) { calls++; return true, "" }
	upstream := Entry{ID: "d5", Text: "closed upstream", Status: StatusFixed, Evidence: "docs/x.md", Reason: "done"}
	merged, missing := mergeInherited(nil, []Entry{upstream}, map[string]Entry{"d5": {Status: StatusFixed, Evidence: "other"}}, counting)
	if calls != 0 || len(missing) != 0 || len(merged) != 1 || merged[0] != upstream {
		t.Fatalf("carried verbatim, ungraded: %+v %v calls=%d", merged, missing, calls)
	}
	current := []Entry{{ID: "d5", Text: "closed upstream", Status: StatusOpen}}
	merged, _ = mergeInherited(current, []Entry{upstream}, nil, counting)
	if calls != 0 || merged[0] != upstream {
		t.Fatalf("the carried row replaces the workspace row: %+v", merged)
	}
}

// Test 31 — the merged ledger is written back BEFORE grading: the G3 bytes
// land even though the grade blocks; AUDIT_LEDGER_DEFECTS_UNACCOUNTED carries
// the count and a bounded id list; a 64-id unaccounted set renders under the
// line cap with nothing truncated.
func TestReconcile_WritesBackBeforeGrading_GoldenBytes(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, reconciledAncestor)
	f.ownLedger(t, reconciledCurrent)
	f.claims(t, reconciledClaims)
	l, got := observed(scopeOf(), resolveUnder(f.root))
	v := l.Reconcile(f.req)
	assertVerdict(t, "reconciled_writeback", v, goldenScenario(t, "reconciled_writeback", f))
	if body := readFile(t, filepath.Join(f.ws, LedgerFile)); body != string(goldenBytes(t, "reconciled_ledger.golden.json")) {
		t.Fatalf("G3 bytes on disk despite the block:\n%s", body)
	}
	e := only(t, *got, CodeDefectsUnaccounted)
	fieldsOf(t, e, map[string]string{"step": "grade", "blocked": "true", "ancestor_cycle": "1255", "count": "2", "ids": "d3, d4", "ids_truncated": "false", "path": filepath.Join(f.ws, DispositionsFile)})
	if e.Origin != "Ledger.Reconcile" || !strings.HasPrefix(e.Reason, "defect ledger: 2 defect(s) inherited from cycle-1255 are unaccounted for") || strings.Contains(e.Reason, "d3") {
		t.Fatalf("the reason is the bounded head sentence: %+v", e)
	}

	f = newFixture(t)
	var entries []Entry
	for i := 0; i < 64; i++ {
		entries = append(entries, Entry{ID: ID("defect " + strconv.Itoa(i)), Text: "defect " + strconv.Itoa(i), Status: StatusOpen})
	}
	body, _ := json.Marshal(Doc{OriginCycle: 1255, Entries: entries})
	f.ancestorLedger(t, string(body))
	f.claims(t, `{"dispositions":[]}`)
	l, got = observed(scopeOf(), resolveNever)
	v = l.Reconcile(f.req)
	if !v.Blocked || len(v.Diagnostics) != 2 {
		t.Fatalf("64 unaccounted: %+v", v)
	}
	e = only(t, *got, CodeDefectsUnaccounted)
	line, _ := json.Marshal(e)
	if len(line) >= signalcenter.MaxLineBytes || e.Fields["truncated"] != "" || strings.HasSuffix(e.Reason, "…") || e.Fields["ids_truncated"] != "true" || strings.Count(e.Fields["ids"], ",") != 7 || e.Fields["count"] != "64" {
		t.Fatalf("bounded by the producer, never by Normalize: %d bytes %+v", len(line), e)
	}
	inc := only(t, *got, CodeDispositionsIncomplete)
	if inc.Fields["uncovered_truncated"] != "true" || strings.Count(inc.Fields["uncovered"], ",") != 7 || inc.Fields["open"] != "64" || inc.Fields["covered"] != "0" {
		t.Fatalf("the preflight list is bounded too: %+v", inc)
	}
}

// Test 32 — a write-back failure blocks and reports AUDIT_LEDGER_WRITEBACK_FAILED {path}.
func TestReconcile_WritebackFailure_BlocksAndSignals(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, ancestorOneOpen)
	f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
	if err := os.Chmod(f.ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.ws, 0o755) })
	l, got := observed(scopeOf(), resolveUnder(f.root))
	v := l.Reconcile(f.req)
	assertVerdict(t, "writeback_failed", v, goldenScenario(t, "writeback_failed", f))
	if _, err := os.Stat(filepath.Join(f.ws, LedgerFile)); err == nil {
		t.Fatal("the write must actually have failed (never skip under root)")
	}
	fieldsOf(t, only(t, *got, CodeWritebackFailed), map[string]string{"step": "grade", "blocked": "true", "path": filepath.Join(f.ws, LedgerFile)})
}

// Test 33 — the vouched lineage: [ancestor] when origin_cycle equals it or
// is 0; [ancestor, origin] when it differs; nil when blocked.
func TestReconcile_LineageCycles(t *testing.T) {
	for origin, want := range map[string]string{`"origin_cycle":1255,`: "1255", ``: "1255", `"origin_cycle":1250,`: "1255,1250"} {
		f := newFixture(t)
		f.ancestorLedger(t, `{`+origin+`"entries":[{"id":"d1","text":"a","status":"OPEN"}]}`)
		f.claims(t, `{"dispositions":[{"id":"d1","status":"DEFERRED","reason":"later"}]}`)
		l, _ := observed(scopeOf(), resolveNever)
		if v := l.Reconcile(f.req); v.Blocked || strings.Join(ints(v.LineageCycles), ",") != want {
			t.Fatalf("%s: %+v want %s", origin, v, want)
		}
		f.claims(t, `{"dispositions":[]}`)
		if v := l.Reconcile(f.req); !v.Blocked || v.LineageCycles != nil {
			t.Fatalf("blocked vouches nothing: %+v", v)
		}
	}
}

// Test 34 — vouching reads the ancestor Doc the grade already loaded: a
// resolver that deletes the ancestor ledger mid-grade still sees the vouch.
func TestReconcile_VouchesFromTheGradesOwnAncestorDoc_NoSecondRead(t *testing.T) {
	f := newFixture(t)
	f.ancestorLedger(t, `{"origin_cycle":1250,"entries":[{"id":"d1","text":"a","status":"OPEN"}]}`)
	f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
	deleting := func(string, Request) (bool, string) {
		if err := os.Remove(filepath.Join(f.ancestorWS, LedgerFile)); err != nil {
			t.Fatal(err)
		}
		return true, ""
	}
	l, _ := observed(scopeOf(), deleting)
	if v := l.Reconcile(f.req); v.Blocked || strings.Join(ints(v.LineageCycles), ",") != "1255,1250" {
		t.Fatalf("vouched from the loaded Doc, no second read: %+v", v)
	}
}

package defectledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 24 — ReadDispositions: the string and array evidence shapes are read,
// `[]` joins to "" (no evidence, not a pass); object, number and bool shapes
// block with the schema example inlined and the evidence error verbatim; a
// missing file is the warning + an empty map + not blocked + NO event; a
// directory blocks with op=read; the seeded {id,status,text,reason} row parses.
func TestReadDispositions_ShapesMissingUnparseable_AndSeededTextKeyTolerated(t *testing.T) {
	f := newFixture(t)
	l, got := observed(scopeOf(), resolveNever)
	f.claims(t, `{"dispositions":[{"id":"a","status":"FIXED","evidence":"go/x.go:1"},{"id":"b","status":"FIXED","evidence":["go/x.go:1","go/y.go"]},{"id":"c","status":"FIXED","evidence":[]},{"id":"d","status":"OPEN","text":"seeded","reason":""}]}`)
	claims, diags, blocked := l.ReadDispositions(f.req, 1255)
	if blocked || diags != nil || len(claims) != 4 || claims["a"].Evidence != "go/x.go:1" || claims["b"].Evidence != "go/x.go:1; go/y.go" || claims["c"].Evidence != "" || claims["d"].Status != StatusOpen {
		t.Fatalf("shapes: %+v %v %v", claims, diags, blocked)
	}
	if claims["a"].ID != "a" || claims["a"].Status != StatusFixed {
		t.Fatalf("id and status carried: %+v", claims["a"])
	}
	for _, bad := range []string{`{"a":1}`, `42`, `true`} {
		f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":`+bad+`}]}`)
		claims, diags, blocked = l.ReadDispositions(f.req, 1255)
		if !blocked || claims != nil || len(diags) != 1 || diags[0].Severity != "error" ||
			!strings.Contains(diags[0].Message, "`evidence` must be a citation string or an array of citation strings, got "+bad) ||
			!strings.Contains(diags[0].Message, DispositionsSchemaExample) {
			t.Fatalf("%s blocks with the schema example: %v", bad, diags)
		}
	}
	assertVerdictDiag := goldenScenario(t, "dispositions_unparseable", f).Diagnostics[0]
	f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":{"a":1}}]}`)
	_, diags, _ = l.ReadDispositions(f.req, 1255)
	if diags[0] != assertVerdictDiag {
		t.Fatalf("golden text:\n got %s\nwant %s", diags[0].Message, assertVerdictDiag.Message)
	}
	if n := len(*got); n != 4 {
		t.Fatalf("one AUDIT_LEDGER_DISPOSITIONS_UNREADABLE per parse fault: %d %v", n, codesOf(*got))
	}
	e := (*got)[3]
	fieldsOf(t, e, map[string]string{"step": "read", "blocked": "true", "op": "parse", "path": filepath.Join(f.ws, DispositionsFile)})
	if e.Code != CodeDispositionsUnreadable || e.Origin != "Ledger.ReadDispositions" || strings.Contains(e.Reason, "Expected schema") {
		t.Fatalf("the signal carries no schema example: %+v", e)
	}

	if err := os.Remove(filepath.Join(f.ws, DispositionsFile)); err != nil {
		t.Fatal(err)
	}
	claims, diags, blocked = l.ReadDispositions(f.req, 1255)
	want := goldenScenario(t, "dispositions_missing", f).Diagnostics[0]
	if blocked || claims == nil || len(claims) != 0 || len(diags) != 1 || diags[0] != want || len(*got) != 4 {
		t.Fatalf("missing: warning, empty map, not blocked, no event: %+v %v %v", claims, diags, blocked)
	}

	if err := os.MkdirAll(filepath.Join(f.ws, DispositionsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	claims, diags, blocked = l.ReadDispositions(f.req, 1255)
	want = goldenScenario(t, "dispositions_read_error", f).Diagnostics[0]
	if !blocked || claims != nil || len(diags) != 1 || diags[0] != want {
		t.Fatalf("a directory blocks with the read error: %v %v", diags, blocked)
	}
	fieldsOf(t, (*got)[4], map[string]string{"op": "read", "step": "read", "blocked": "true"})
}

// Test 25 — Preflight: MISSING and INCOMPLETE with their counts; nil when
// no OPEN row or every one is covered; the two codes with open/covered/uncovered.
func TestPreflight_MissingAndIncompleteMarkers_SilentWhenCoveredOrNothingOwed(t *testing.T) {
	f := newFixture(t)
	l, got := observed(scopeOf(), resolveNever)
	ancestor := []Entry{{ID: "d1", Status: StatusOpen}, {ID: "d2", Status: StatusOpen}, {ID: "d3", Status: StatusFixed}}
	if d := l.Preflight(f.req, 1255, ancestor, map[string]Entry{"d1": {}, "d2": {}}); d != nil {
		t.Fatalf("all covered: %v", d)
	}
	if d := l.Preflight(f.req, 1255, []Entry{{ID: "d3", Status: StatusFixed}}, nil); d != nil {
		t.Fatalf("nothing owed: %v", d)
	}
	if len(*got) != 0 {
		t.Fatalf("silent: %v", codesOf(*got))
	}
	diags := l.Preflight(f.req, 1255, []Entry{{ID: "d1", Status: StatusOpen}}, nil)
	want := goldenScenario(t, "dispositions_missing", f).Diagnostics[1]
	if len(diags) != 1 || diags[0] != want {
		t.Fatalf("MISSING:\n got %v\nwant %v", diags, want)
	}
	fieldsOf(t, only(t, *got, CodeDispositionsMissing), map[string]string{"step": "preflight", "blocked": "true", "ancestor_cycle": "1255", "open": "1", "path": filepath.Join(f.ws, DispositionsFile)})

	f.claims(t, `{"dispositions":[{"id":"d1","status":"FIXED","evidence":"go/x.go"}]}`)
	ancestor = []Entry{{ID: "d1", Status: StatusOpen}, {ID: "d2", Status: StatusOpen}}
	diags = l.Preflight(f.req, 1255, ancestor, map[string]Entry{"d1": {}})
	want = goldenScenario(t, "incomplete_unaccounted", f).Diagnostics[0]
	if len(diags) != 1 || diags[0] != want {
		t.Fatalf("INCOMPLETE:\n got %v\nwant %v", diags, want)
	}
	e := only(t, *got, CodeDispositionsIncomplete)
	fieldsOf(t, e, map[string]string{"step": "preflight", "blocked": "true", "ancestor_cycle": "1255", "open": "2", "covered": "1", "uncovered": "d2", "uncovered_truncated": "false", "path": filepath.Join(f.ws, DispositionsFile)})
	if e.Origin != "Ledger.Preflight" || !strings.HasPrefix(e.Reason, "defect ledger: "+PreflightIncompleteMarker) {
		t.Fatalf("origin/reason: %+v", e)
	}
}

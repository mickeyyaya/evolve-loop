package defectledger

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

func TestEmit_ARowTakesTheSeverityOfTheAuditFindingThatMintedIt(t *testing.T) {
	ws := t.TempDir()
	l, _ := observed(scopeOf(), resolveNever)
	r := Rejection{
		Defects:       []string{"H1 go/x.go:3 the floor tests fail", "M2 docs drift from the code", "**C1** the ship gate is open", "an unlabelled defect", "X9 an unknown id", "C2-foo.go: a path, not an id"},
		Prescriptions: []string{"add a regression test"},
		Findings:      []reportdoc.Finding{{ID: "M2", Severity: "HIGH", Title: "docs drift"}},
	}
	mustEmit(t, l, Request{Cycle: 1572, Workspace: ws}, r)
	doc, _, _ := Read(ws)
	want := map[string]string{
		"H1 go/x.go:3 the floor tests fail": "HIGH", "M2 docs drift from the code": "HIGH", "**C1** the ship gate is open": "CRITICAL",
		"an unlabelled defect": "", "X9 an unknown id": "", "C2-foo.go: a path, not an id": "", PrescriptionPrefix + "add a regression test": "",
	}
	if len(doc.Entries) != len(want) {
		t.Fatalf("entries = %+v", doc.Entries)
	}
	for _, e := range doc.Entries {
		if sev, ok := want[e.Text]; !ok || e.Severity != sev {
			t.Errorf("%q has severity %q, want %q (the finding of the report wins over the id prefix)", e.Text, e.Severity, sev)
		}
	}
}

func TestBlockerSeverity_CriticalAndHighBlockAndTheRestDoNot(t *testing.T) {
	for sev, want := range map[string]bool{"CRITICAL": true, "HIGH": true, "MEDIUM": false, "LOW": false, "": false, "high": false} {
		if got := isBlocker(sev); got != want {
			t.Errorf("isBlocker(%q) = %v, want %v", sev, got, want)
		}
	}
}

func TestGradeClaim_ABlockerCannotBeDeferredButANonBlockerCan(t *testing.T) {
	yes := func(string) (bool, string) { return true, "" }
	deferred := Entry{Status: StatusDeferred, Reason: "out of this lane's scope"}
	for _, sev := range []string{"CRITICAL", "HIGH"} {
		a := Entry{ID: "d1", Text: "t", Status: StatusOpen, Severity: sev}
		got, why := gradeClaim(a, deferred, true, yes)
		if got.Status != StatusOpen || !strings.Contains(why, "DEFERRED but it is a "+sev+" blocker") {
			t.Errorf("%s DEFERRED: got %+v %q, want it reopened and refused", sev, got, why)
		}
		fixed, why := gradeClaim(a, Entry{Status: StatusFixed, Evidence: "go/x.go:1"}, true, yes)
		if fixed.Status != StatusFixed || why != "" {
			t.Errorf("%s FIXED: got %+v %q, want it accepted", sev, fixed, why)
		}
	}
	standIn := overflowRow(3, 1572)
	standIn.Severity = "HIGH"
	if got, why := gradeClaim(standIn, deferred, true, yes); got.Status != StatusDeferred || why != "" {
		t.Errorf("a HIGH overflow stand-in names no defect to repair, so it stays deferrable: got %+v %q", got, why)
	}
	for _, sev := range []string{"MEDIUM", "LOW", ""} {
		a := Entry{ID: "d1", Text: "t", Status: StatusOpen, Severity: sev}
		if got, why := gradeClaim(a, deferred, true, yes); got.Status != StatusDeferred || why != "" {
			t.Errorf("%q DEFERRED with a reason: got %+v %q, want it accepted", sev, got, why)
		}
	}
}

func TestReconcile_AContinuationThatDefersAnInheritedBlockerIsBlocked(t *testing.T) {
	ancestor := `{"origin_cycle":1255,"entries":[{"id":"d1","text":"H1 first defect","status":"OPEN","severity":"HIGH"},{"id":"d2","text":"M1 second defect","status":"OPEN","severity":"MEDIUM"}]}`
	claims := func(first string) string {
		return `{"dispositions":[` + first + `,{"id":"d2","status":"DEFERRED","reason":"queued as a follow-up"}]}`
	}
	f := newFixture(t)
	f.ancestorLedger(t, ancestor)
	f.claims(t, claims(`{"id":"d1","status":"DEFERRED","reason":"out of scope"}`))
	l, _ := observed(scopeOf(), resolveUnder(f.root))
	v := l.Reconcile(f.req)
	if !v.Blocked || !strings.Contains(diagsText(v.Diagnostics), "d1 (DEFERRED but it is a HIGH blocker") || strings.Contains(diagsText(v.Diagnostics), "d2 (") {
		t.Fatalf("a deferred blocker must block, and only the blocker is named: %+v", v)
	}

	twin := newFixture(t)
	twin.ancestorLedger(t, ancestor)
	twin.claims(t, claims(`{"id":"d1","status":"FIXED","evidence":"go/x.go"}`))
	l, _ = observed(scopeOf(), resolveUnder(twin.root))
	if v := l.Reconcile(twin.req); v.Blocked {
		t.Fatalf("a repaired blocker and a deferred non-blocker pass: %+v", v)
	}
}

func TestPromptRows_MarkABlockerAsFixedOnly(t *testing.T) {
	rows := promptRows([]Entry{{ID: "d1", Text: "H1 broke", Severity: "HIGH"}, {ID: "d2", Text: "M1 drift", Severity: "MEDIUM"}})
	want := "- d1 (HIGH blocker: FIXED only): H1 broke\n- d2: M1 drift\n"
	if rows != want {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
}

func TestSeverityOf_ALeadingSeverityTagCountsLikeAFindingID(t *testing.T) {
	for text, want := range map[string]string{
		"[HIGH] go/x.go leak":              "HIGH",
		"[CRITICAL] the ship gate is open": "CRITICAL",
		"HIGH: go/x.go leak":               "HIGH",
		"**[MEDIUM]** docs drift":          "MEDIUM",
		"[LOW] a nit":                      "LOW",
		"HIGHLY unusual text":              "",
		"[high] lower case is no tag":      "",
		"[HIGH half a bracket":             "",
		"[HIGH]x no separator":             "",
		"HIGH loose word":                  "",
	} {
		if got := severityOf(text, nil); got != want {
			t.Errorf("severityOf(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestReconcile_AnInheritedHighCodeReviewRowCannotBeDeferredButAMediumOneCan(t *testing.T) {
	row := func(sev string) string {
		return `{"origin_cycle":1255,"entries":[{"id":"d1","text":"[` + sev + `] correctness go/x.go:1 — t","status":"OPEN","source":"code-review","round":1,"severity":"` + sev + `"}]}`
	}
	for sev, blocked := range map[string]bool{"HIGH": true, "MEDIUM": false} {
		f := newFixture(t)
		f.ancestorLedger(t, row(sev))
		f.claims(t, `{"dispositions":[{"id":"d1","status":"DEFERRED","reason":"later"}]}`)
		l, _ := observed(scopeOf(), resolveNever)
		v := l.Reconcile(f.req)
		if v.Blocked != blocked || (blocked && !strings.Contains(diagsText(v.Diagnostics), "DEFERRED but it is a HIGH blocker")) {
			t.Errorf("%s code-review row DEFERRED: blocked=%v %+v, want blocked=%v", sev, v.Blocked, v.Diagnostics, blocked)
		}
	}
}

func TestSeverityOf_ALeadingTagWinsOverTheFindingOfAnID(t *testing.T) {
	if got := severityOf("[LOW] H1 a nit beside a high finding", []reportdoc.Finding{{ID: "H1", Severity: "HIGH"}}); got != "LOW" {
		t.Errorf("severityOf = %q, want the tag LOW", got)
	}
}

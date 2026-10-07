package defectledger

import "testing"

func reviewAncestor(status string) Entry {
	return Entry{ID: "d9", Text: "[HIGH] concurrency go/x.go:3 — leak", Status: status, Source: "code-review", Round: 2, Severity: "HIGH", Dimension: "concurrency"}
}

func TestGradeClaim_KeepsTheRowsProvenanceWhateverTheStatus(t *testing.T) {
	yes := func(string) (bool, string) { return true, "" }
	no := func(string) (bool, string) { return false, "unresolvable" }
	for name, tc := range map[string]struct {
		claim   Entry
		has     bool
		resolve func(string) (bool, string)
		status  string
	}{
		"no claim":          {Entry{}, false, yes, StatusOpen},
		"FIXED resolving":   {Entry{Status: StatusFixed, Evidence: "go/x.go:1"}, true, yes, StatusFixed},
		"FIXED unresolved":  {Entry{Status: StatusFixed, Evidence: "x"}, true, no, StatusOpen},
		"DEFERRED reasoned": {Entry{Status: StatusDeferred, Reason: "later"}, true, yes, StatusDeferred},
		"DISPUTED claimed":  {Entry{Status: "DISPUTED"}, true, yes, StatusOpen},
	} {
		got, _ := gradeClaim(reviewAncestor(StatusOpen), tc.claim, tc.has, tc.resolve)
		if got.Status != tc.status || got.Source != "code-review" || got.Round != 2 || got.Severity != "HIGH" || got.Dimension != "concurrency" {
			t.Errorf("%s: graded %+v, want status %s with source, round, severity and dimension carried through", name, got, tc.status)
		}
	}
}

func TestMergeInherited_AShadowedIdResetsToTheAncestorRowWithItsProvenance(t *testing.T) {
	ancestor := reviewAncestor(StatusOpen)
	current := []Entry{{ID: "d9", Text: "planted text", Status: StatusFixed, Evidence: "x"}}
	merged, missing := mergeInherited(current, []Entry{ancestor}, nil, nil)
	if len(missing) != 1 || merged[0] != ancestor {
		t.Errorf("merged %+v (missing %v), want the ancestor's OPEN row back, provenance included", merged, missing)
	}
}

func TestAppend_TheStandInTakesTheHighestCutSeverity(t *testing.T) {
	doc := Doc{Entries: held("code-review", MaxEntries)}
	low := reviewRow("cut low", "LOW")
	critical := reviewRow("cut critical", "CRITICAL")
	medium := reviewRow("cut medium", "MEDIUM")
	got, _, overflow := Append(doc, []Entry{low, critical, medium}, 7)
	tail := got.Entries[MaxEntries]
	if overflow != 3 || tail.Severity != "CRITICAL" || tail.Dimension != "" {
		t.Errorf("stand-in = %+v (overflow %d), want the highest cut severity, CRITICAL, and no single dimension", tail, overflow)
	}
}

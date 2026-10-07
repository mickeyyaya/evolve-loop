package defectledger

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func reviewRow(text, severity string) Entry {
	return Entry{Text: text, Status: StatusDeferred, Reason: "shadow stage", Source: "code-review", Round: 1, Severity: severity}
}

func held(source string, n int) []Entry {
	rows := make([]Entry, n)
	for i := range rows {
		rows[i] = Entry{Text: source + " held " + strconv.Itoa(i), Status: StatusOpen, Source: source}
	}
	return rows
}

func auditOnly(rows []Entry) []Entry {
	var out []Entry
	for _, e := range rows {
		if e.Source == "" {
			out = append(out, e)
		}
	}
	return out
}

func TestAppend_AddsRowsDedupedPerSourceWithSourceQualifiedIDsAndKeepsTheirProvenance(t *testing.T) {
	existing := Doc{OriginCycle: 7, Entries: []Entry{{ID: ID("audit defect"), Text: "audit defect", Status: StatusOpen}}}
	long := strings.Repeat("y", TextMaxRunes+5)
	rows := []Entry{reviewRow("new finding", "HIGH"), reviewRow("new finding", "LOW"), reviewRow(long, "MEDIUM")}

	got, added, overflow := Append(existing, rows, 9)

	if !added || overflow != 0 {
		t.Fatalf("added=%v overflow=%d, want added and no overflow", added, overflow)
	}
	if got.OriginCycle != 7 || len(got.Entries) != 3 {
		t.Fatalf("got %+v, want origin 7 and the existing row plus two new ones (the duplicate texts merge)", got)
	}
	fresh := got.Entries[1]
	if fresh.ID != ID("code-review"+sourceSeparator+"new finding") || fresh.Status != StatusDeferred || fresh.Reason != "shadow stage" || fresh.Source != "code-review" || fresh.Round != 1 || fresh.Severity != "HIGH" {
		t.Errorf("row = %+v, want the first occurrence's provenance and the id of its source-qualified text", fresh)
	}
	if capped := got.Entries[2]; capped.Text != Truncate(long, TextMaxRunes) || capped.ID != ID("code-review"+sourceSeparator+capped.Text) {
		t.Errorf("an over-long text must be capped before its id is derived, got id %s for %d runes", capped.ID, len([]rune(capped.Text)))
	}
	if len(existing.Entries) != 1 {
		t.Errorf("Append mutated its input: %+v", existing.Entries)
	}
}

func TestAppend_NothingNewAddsNothing(t *testing.T) {
	doc := Doc{OriginCycle: 3, Entries: []Entry{{ID: ID("x"), Text: "x", Status: StatusOpen}, reviewRow("x", "HIGH")}}
	got, added, overflow := Append(doc, []Entry{reviewRow("x", "LOW"), {Text: "x", Status: StatusOpen}}, 3)
	if added || overflow != 0 || !reflect.DeepEqual(got, doc) {
		t.Errorf("Append(dup) = %+v added=%v overflow=%d, want the doc unchanged", got, added, overflow)
	}
}

func TestAppend_AnAuditDefectWhoseTextAShadowRowHoldsIsStillOwed(t *testing.T) {
	text := "[HIGH] concurrency go/x.go:3 — leak | scenario: s | evidence: e | fix: f"
	shadow := reviewRow(text, "HIGH")
	shadow.ID = ID("code-review" + sourceSeparator + text)

	got, added, overflow := Append(Doc{OriginCycle: 7, Entries: []Entry{shadow}}, openRows([]string{text}), 7)

	want := Entry{ID: ID(text), Text: text, Status: StatusOpen}
	if !added || overflow != 0 || len(got.Entries) != 2 || got.Entries[1] != want {
		t.Errorf("Append = %+v (added %v), want the audit's OPEN row %+v beside the shadow row: a shadow row must never launder an audit defect", got.Entries, added, want)
	}
}

func TestAppend_EachSourceIsCappedOnlyByItsOwnRows(t *testing.T) {
	for name, tc := range map[string]struct {
		holder, incoming      string
		overflow, wantEntries int
	}{
		"shadow rows leave the audit's cap":  {"code-review", "", 0, MaxEntries + 2},
		"audit rows leave the shadow's cap":  {"", "code-review", 0, MaxEntries + 2},
		"the audit's own rows fill its cap":  {"", "", 2, MaxEntries + 1},
		"the shadow's own rows fill its cap": {"code-review", "code-review", 2, MaxEntries + 1},
	} {
		rows := []Entry{{Text: "a", Status: StatusOpen, Source: tc.incoming}, {Text: "b", Status: StatusOpen, Source: tc.incoming}, {Text: "c", Status: StatusOpen, Source: tc.incoming}}

		got, _, overflow := Append(Doc{Entries: held(tc.holder, MaxEntries-1)}, rows, 7)

		if overflow != tc.overflow || len(got.Entries) != tc.wantEntries {
			t.Errorf("%s: overflow=%d entries=%d, want overflow %d and %d entries: a source's cap counts only that source's rows", name, overflow, len(got.Entries), tc.overflow, tc.wantEntries)
		}
	}
}

func TestAppend_OverflowRowStandsForTheTruncatedRowsWithTheirDispositionAndSource(t *testing.T) {
	rows := []Entry{reviewRow("fits", "HIGH"), reviewRow("cut one", "CRITICAL"), reviewRow("cut two", "LOW")}

	got, added, overflow := Append(Doc{Entries: held("code-review", MaxEntries-1)}, rows, 42)

	if !added || overflow != 2 || len(got.Entries) != MaxEntries+1 {
		t.Fatalf("added=%v overflow=%d entries=%d, want 2 overflowed into one synthetic row past the cap", added, overflow, len(got.Entries))
	}
	tail := got.Entries[MaxEntries]
	wantText := "code-review: " + overflowRow(2, 42).Text
	if tail.Text != wantText || tail.ID != ID("code-review"+sourceSeparator+wantText) {
		t.Errorf("tail = %+v, want the source-qualified overflow text %q under its source-qualified id", tail, wantText)
	}
	if tail.Status != StatusDeferred || tail.Reason != "shadow stage" || tail.Source != "code-review" || tail.Round != 1 {
		t.Errorf("tail = %+v, want the overflowed rows' status, reason, source and round: a shadow cut must never mint an OPEN row", tail)
	}
}

func TestAppend_TheAuditsStandInIsNeverSwallowedByAShadowStandIn(t *testing.T) {
	doc := Doc{Entries: append(held("", MaxEntries), held("code-review", MaxEntries)...)}
	doc, _, _ = Append(doc, []Entry{reviewRow("cut r1", "HIGH"), reviewRow("cut r2", "HIGH")}, 7)

	got, added, overflow := Append(doc, openRows([]string{"audit cut 1", "audit cut 2"}), 7)

	if want := overflowRow(2, 7); !added || overflow != 2 || got.Entries[len(got.Entries)-1] != want {
		t.Errorf("audit overflow=%d added=%v tail=%+v, want the audit's own OPEN stand-in %+v: a shadow stand-in must never swallow it", overflow, added, got.Entries[2*MaxEntries:], want)
	}
}

func TestAppend_EachSourcesCutGetsItsOwnStandIn(t *testing.T) {
	doc := Doc{Entries: append(held("", MaxEntries), held("code-review", MaxEntries)...)}
	rows := []Entry{reviewRow("cut r1", "LOW"), {Text: "audit cut", Status: StatusOpen}, reviewRow("cut r2", "HIGH")}

	got, _, overflow := Append(doc, rows, 7)

	tails := got.Entries[2*MaxEntries:]
	if overflow != 3 || len(tails) != 2 {
		t.Fatalf("overflow=%d tails=%+v, want 3 cut into one stand-in per source", overflow, tails)
	}
	if tails[0].Source != "code-review" || tails[0].Status != StatusDeferred || tails[0].Severity != "HIGH" || !strings.HasPrefix(tails[0].Text, "code-review: 2 further") {
		t.Errorf("shadow stand-in = %+v, want the two shadow cuts, DEFERRED at HIGH", tails[0])
	}
	if tails[1] != overflowRow(1, 7) {
		t.Errorf("audit stand-in = %+v, want %+v: the audit's cut is owed under its own OPEN row", tails[1], overflowRow(1, 7))
	}
}

func TestAppend_TheAuditsRowsAreTheSameWithOrWithoutShadowRows(t *testing.T) {
	defects := make([]string, MaxEntries+2)
	shadow := make([]Entry, MaxEntries)
	for i := range defects {
		defects[i] = "defect " + strconv.Itoa(i)
	}
	for i := range shadow {
		shadow[i] = reviewRow(defects[i], "HIGH")
	}

	alone, aloneAdded, aloneOverflow := Append(Doc{OriginCycle: 7}, openRows(defects), 7)
	beside, besideAdded, besideOverflow := Append(Doc{OriginCycle: 7, Entries: shadow}, openRows(defects), 7)

	if aloneAdded != besideAdded || aloneOverflow != besideOverflow || !reflect.DeepEqual(alone.Entries, auditOnly(beside.Entries)) {
		t.Errorf("the audit's rows beside shadow rows differ from the audit's rows alone (overflow %d vs %d): the shadow stage must leave the audit byte-identical", besideOverflow, aloneOverflow)
	}
}

func TestEntry_LegacyLedgerRoundTripsByteIdentical(t *testing.T) {
	ws := t.TempDir()
	golden := goldenBytes(t, "emit_ledger.golden.json")
	var doc Doc
	if err := json.Unmarshal(golden, &doc); err != nil {
		t.Fatal(err)
	}
	if err := Write(ws, doc); err != nil {
		t.Fatal(err)
	}
	if body := readFile(t, filepath.Join(ws, LedgerFile)); body != string(golden) {
		t.Fatalf("a ledger written before the provenance fields must re-serialize byte-identical:\n%s", body)
	}
	for _, e := range doc.Entries {
		if e.Source != "" || e.Round != 0 || e.Severity != "" {
			t.Errorf("legacy row %s decoded provenance %q/%d/%q, want none", e.ID, e.Source, e.Round, e.Severity)
		}
	}
}

func TestAppend_ARepeatedShadowCutIsOneStandIn(t *testing.T) {
	rows := []Entry{reviewRow("cut a", "HIGH"), reviewRow("cut b", "LOW")}
	once, _, _ := Append(Doc{Entries: held("code-review", MaxEntries)}, rows, 7)

	twice, added, overflow := Append(once, rows, 7)

	if added || overflow != 2 || len(twice.Entries) != len(once.Entries) {
		t.Errorf("re-cut: added=%v overflow=%d entries %d→%d, want the same stand-in recognised: one per source and cut", added, overflow, len(once.Entries), len(twice.Entries))
	}
}

func TestAppend_AReReportedRowNeverSpendsTheCap(t *testing.T) {
	doc := Doc{Entries: held("", MaxEntries-1)}

	got, added, overflow := Append(doc, []Entry{{Text: doc.Entries[0].Text, Status: StatusOpen}, {Text: "fresh", Status: StatusOpen}}, 7)

	if !added || overflow != 0 || got.Entries[len(got.Entries)-1].Text != "fresh" {
		t.Errorf("added=%v overflow=%d tail=%+v, want the fresh defect recorded: a duplicate must not consume the last slot", added, overflow, got.Entries[len(got.Entries)-1])
	}
}

func TestAppend_AnAuditDefectSpellingAShadowRowsPreimageKeepsItsOwnID(t *testing.T) {
	shadowText := "[HIGH] correctness go/x.go:1 — t | scenario: s | evidence: e | fix: f"
	var quoting []string
	for _, raw := range []string{`"code-review: ` + shadowText + `"`, `"code-review\u0000` + shadowText + `"`} {
		var text string
		if err := json.Unmarshal([]byte(raw), &text); err != nil {
			t.Fatal(err)
		}
		quoting = append(quoting, text)
	}
	doc, _, _ := Append(Doc{OriginCycle: 1255}, []Entry{reviewRow(shadowText, "HIGH")}, 1255)

	got, _, _ := Append(doc, openRows(quoting), 1255)

	ids := map[string]bool{}
	for _, e := range got.Entries {
		ids[e.ID] = true
	}
	if len(ids) != len(got.Entries) {
		t.Errorf("entries %+v share an id: a non-audit row's id preimage must be a string no JSON-decoded defect text can spell", got.Entries)
	}
}

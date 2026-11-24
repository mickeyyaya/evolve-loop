package ledger

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var realLedgerCopy = flag.String("real-ledger-copy", "", "path to a COPY of a real ledger for the equivalence test")

func legacyLatestCompositionVerdict(ledgerPath, method, laneAuditRef string) (CompositionVerdict, bool, error) {
	body, err := os.ReadFile(ledgerPath)
	if os.IsNotExist(err) {
		return CompositionVerdict{}, false, nil
	}
	if err != nil {
		return CompositionVerdict{}, false, err
	}
	var latest CompositionVerdict
	found := false
	for _, line := range bytes.Split(body, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec compositionRecord
		if json.Unmarshal(line, &rec) != nil || rec.Kind != CompositionVerdictKind || rec.Method != method || rec.LaneAuditRef != laneAuditRef {
			continue
		}
		latest = CompositionVerdict{Cycle: rec.Cycle, Method: rec.Method, LaneAuditRef: rec.LaneAuditRef, PatchID: rec.PatchID,
			AuditedBase: rec.AuditedBase, GitHead: rec.GitHead, TreeStateSHA: rec.TreeStateSHA, AuditedTreeSHA: rec.AuditedTreeSHA,
			GateResults: rec.GateResults, AuditedDiffPath: rec.AuditedDiffPath, ComposedDiffPath: rec.ComposedDiffPath}
		found = true
	}
	return latest, found, nil
}

func blobSize(i int) int {
	if i%1500 == 5 {
		return 70000
	}
	return 40
}

type scanShape struct {
	rows            int
	trailingNewline bool
	truncatedLast   bool
	malformedMiddle bool
}

func buildCompositionFixture(shape scanShape) string {
	var sb strings.Builder
	methods := []string{IdenticalRebaseMethod, TrivialRebaseMethod, ScopedReviewMethod}
	for i := 0; i < shape.rows; i++ {
		switch i % 6 {
		case 0:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"kind":"composition-verdict","method":%q,"lane_audit_ref":"ref-%d","patch_id":"p%d","audited_tree_sha":"tree-%d","gate_results":{"g":"pass"}}`+"\n", i, i, methods[i%3], i%11, i, i)
		case 1:
			fmt.Fprintf(&sb, `{"ts":"t%d","cycle":%d,"role":"orchestrator","kind":"routing_decision","note":%q}`+"\n", i, i, strings.Repeat("n", 150+i%400))
		case 2:
			fmt.Fprintf(&sb, `{"ts":"t%d","kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-%d","cycle":%d}`+"\n", i, i%11, i)
		case 3:
			fmt.Fprintf(&sb, " {\"ts\":\"t%d\",\"kind\":\"composition-verdict\",\"method\":\"identical-rebase\",\"lane_audit_ref\":\"ref-%d\",\"cycle\":%d}\t\r\n", i, i%11, i)
		case 4:
			fmt.Fprintf(&sb, `{"ts":"t%d","kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-%d","cycle":%d,"audited_tree_sha":"esc"}`+"\n", i, i%11, i)
		default:
			fmt.Fprintf(&sb, `{"ts":"t%d","kind":"phase","blob":%q}`+"\n", i, strings.Repeat("B", blobSize(i)))
		}
		if shape.malformedMiddle && i == shape.rows/2 {
			sb.WriteString(`{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1","cyc` + "\n")
		}
	}
	out := sb.String()
	if shape.truncatedLast {
		out += `{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1","cy`
	}
	if !shape.trailingNewline && !shape.truncatedLast {
		out = strings.TrimSuffix(out, "\n")
	}
	return out
}

func assertCompositionEquivalent(t *testing.T, path string) {
	t.Helper()
	for _, method := range []string{IdenticalRebaseMethod, TrivialRebaseMethod, ScopedReviewMethod, "absent-method", ""} {
		for i := 0; i < 13; i++ {
			ref := fmt.Sprintf("ref-%d", i)
			if i == 12 {
				ref = ""
			}
			oldV, oldFound, oldErr := legacyLatestCompositionVerdict(path, method, ref)
			newV, newFound, newErr := LatestCompositionVerdict(path, method, ref)
			if fmt.Sprint(oldErr) != fmt.Sprint(newErr) || oldFound != newFound || !reflect.DeepEqual(oldV, newV) {
				t.Fatalf("(%q,%q) differs: old %+v %v %v, new %+v %v %v", method, ref, oldV, oldFound, oldErr, newV, newFound, newErr)
			}
		}
	}
}

func TestLatestCompositionVerdictMatchesTheLegacyReaderOnARealShapedLedger(t *testing.T) {
	for name, shape := range map[string]scanShape{
		"clean, trailing newline": {rows: 9000, trailingNewline: true},
		"no trailing newline":     {rows: 9000},
		"truncated last line":     {rows: 9000, truncatedLast: true},
		"malformed middle line":   {rows: 9000, trailingNewline: true, malformedMiddle: true},
	} {
		t.Run(name, func(t *testing.T) {
			body := buildCompositionFixture(shape)
			if len(body) < 1<<20 {
				t.Fatalf("fixture is %d bytes; it must span many 64 KiB chunks", len(body))
			}
			p := filepath.Join(t.TempDir(), "ledger.jsonl")
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			assertCompositionEquivalent(t, p)
		})
	}
}

func TestLatestCompositionVerdictMatchesTheLegacyReaderOnEdgeLedgers(t *testing.T) {
	row := `{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1","cycle":5}`
	for name, body := range map[string]string{
		"empty":           "",
		"blank only":      "\n \n",
		"one row no eol":  row,
		"nbsp padded":     " " + row + " \n",
		"upper-case keys": `{"KIND":"composition-verdict","Method":"identical-rebase","LANE_AUDIT_REF":"ref-1","cycle":6}` + "\n",
		"duplicate keys":  `{"kind":"x","kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1","cycle":7}` + "\n",
		"escaped values":  `{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1","cycle":8}` + "\n",
		"later non-match": row + "\n" + `{"kind":"composition-verdict","method":"trivial-rebase","lane_audit_ref":"ref-1","cycle":9}` + "\n",
		"later malformed": row + "\n{bad\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "ledger.jsonl")
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			assertCompositionEquivalent(t, p)
		})
	}
}

func TestLatestCompositionVerdictMatchesTheLegacyReaderOnAMissingAndAnUnreadableLedger(t *testing.T) {
	assertCompositionEquivalent(t, filepath.Join(t.TempDir(), "none.jsonl"))
	dir := t.TempDir()
	_, _, oldErr := legacyLatestCompositionVerdict(dir, IdenticalRebaseMethod, "r")
	_, _, newErr := LatestCompositionVerdict(dir, IdenticalRebaseMethod, "r")
	if oldErr == nil || newErr == nil {
		t.Fatalf("a directory is not a ledger: old=%v new=%v", oldErr, newErr)
	}
}

func TestLatestCompositionVerdictMatchesTheLegacyReaderOnARealLedgerCopy(t *testing.T) {
	if *realLedgerCopy == "" {
		t.Skip("pass -real-ledger-copy=<path to a copy> to run")
	}
	assertCompositionEquivalent(t, *realLedgerCopy)
}

func TestCompositionFilterPassesEveryRowTheCompositionPredicateAccepts(t *testing.T) {
	spec := compositionSpec{method: IdenticalRebaseMethod, laneAuditRef: "ref-1"}
	for _, line := range []string{
		`{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1"}`,
		`{"KIND":"composition-verdict","METHOD":"identical-rebase","Lane_Audit_Ref":"ref-1"}`,
		`{"kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1"}`,
		`{"kind":"x","kind":"composition-verdict","method":"identical-rebase","lane_audit_ref":"ref-1"}`,
	} {
		var rec compositionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil || !spec.match(rec) {
			t.Fatalf("premise: %s is not accepted (%v)", line, err)
		}
		if !spec.filter().Pass([]byte(line)) {
			t.Errorf("the prefilter drops an accepted row: %s", line)
		}
	}
	if spec.filter().Pass([]byte(`{"kind":"composition-verdict","method":"trivial-rebase","lane_audit_ref":"ref-1"}`)) {
		t.Error("the prefilter must drop a row of another method")
	}
}

func TestCompositionFilterNeedlesComeFromThePredicateValues(t *testing.T) {
	got := compositionSpec{method: "m1", laneAuditRef: "r1"}.filter().Needles
	if len(got) != 3 || string(got[0]) != CompositionVerdictKind || string(got[1]) != "m1" || string(got[2]) != "r1" {
		t.Fatalf("needles = %q", got)
	}
	if n := (compositionSpec{}).filter().Needles; len(n) != 1 || string(n[0]) != CompositionVerdictKind {
		t.Fatalf("an empty method and ref must add no needle, got %q", n)
	}
}

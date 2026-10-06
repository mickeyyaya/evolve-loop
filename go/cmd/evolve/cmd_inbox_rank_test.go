package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

var rankClock = time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

func rankRoot(t *testing.T) string {
	return curationRoot(t, map[string]string{
		"2026-09-06T00-00-00Z-sec-fix.json": `{"id":"sec-fix","title":"close the token leak","kind":"bug","priority_class":"security",` +
			`"weight":0.4,"created_at":"2026-09-06","files":["go/internal/x/x.go"]}`,
		"2026-10-06T00-00-00Z-tidy-docs.json": `{"id":"tidy-docs","title":"tidy the docs","kind":"chore","priority_class":"hygiene",` +
			`"weight":0.6,"created_at":"2026-10-06"}`,
		"2026-08-07T00-00-00Z-console-only.json": `{"id":"console-only","title":"the console owns this","kind":"feature",` +
			`"priority_class":"stability","weight":0.3,"created_at":"2026-08-07","route":"console-manual"}`,
		"2026-10-06T00-00-00Z-after-sec.json": `{"id":"after-sec","title":"waits on the fix","kind":"feature","priority_class":"feature",` +
			`"weight":0.5,"created_at":"2026-10-06","deps":["sec-fix"]}`,
		"consumed/2026-09-01T00-00-00Z-gone.json": `{"id":"gone","kind":"feature","priority_class":"security","weight":1}`,
	})
}

func rankAt(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := rankInboxAt(args, rankClock, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestCmd_InboxRank_JSONIsPinned(t *testing.T) {
	rankRoot(t)
	want, err := os.ReadFile(filepath.Join("testdata", "inbox-rank-all.golden.json"))
	if err != nil {
		t.Fatal(err)
	}

	rc, stdout, stderr := rankAt(t, "--list", "all", "--json")

	if rc != 0 || stdout != string(want) {
		t.Fatalf("rc = %d stderr = %q\n got: %s\nwant: %s", rc, stderr, stdout, want)
	}
}

func TestCmd_InboxRank_IsAnInboxVerb(t *testing.T) {
	rankRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"rank", "--json"}, nil, &stdout, &stderr)

	var doc rankedInboxDoc
	if err := json.Unmarshal(stdout.Bytes(), &doc); rc != 0 || err != nil || len(doc.Lists) != 1 || doc.Lists[0].List != "ready" {
		t.Fatalf("rc = %d err = %v doc = %+v stderr = %q, want the ready list by default", rc, err, doc, stderr.String())
	}
	if !strings.Contains(inboxUsage("rank"), "--explain <id>") {
		t.Errorf("usage = %q", inboxUsage("rank"))
	}
}

func TestCmd_InboxRank_TableShowsRankScoreBreakdownIDAndTitle(t *testing.T) {
	rankRoot(t)

	rc, stdout, _ := rankAt(t)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if rc != 0 || len(lines) != 4 || lines[0] != "ready: 2 item(s)" {
		t.Fatalf("rc = %d stdout:\n%s", rc, stdout)
	}
	header := []string{"rank", "score"}
	for _, f := range inboxrank.Factors() {
		header = append(header, string(f))
	}
	if want := append(header, "id", "title"); !slices.Equal(strings.Fields(lines[1]), want) {
		t.Errorf("header = %q, want the columns %v: the factors in the order the rows print them", lines[1], want)
	}
	if fields := strings.Fields(lines[2]); len(fields) < 10 || fields[0] != "1" || fields[1] != "0.3200" || fields[2] != "0.2700" ||
		fields[3] != "0.0500" || fields[4] != "0.0000" || fields[6] != "0.0000" || fields[8] != "tidy-docs" || !strings.HasSuffix(lines[2], "tidy the docs") {
		t.Errorf("row 1 = %q, want rank, score, the six contributions, id and title", lines[2])
	}
	if fields := strings.Fields(lines[3]); len(fields) < 10 || fields[0] != "2" || fields[1] != "0.2800" || fields[3] != "0.0250" ||
		fields[4] != "0.0500" || fields[6] != "0.0250" || fields[8] != "sec-fix" {
		t.Errorf("row 2 = %q, want the security item second: its class is last in the order", lines[3])
	}
}

func TestCmd_InboxRank_EachListIsRankedWithinItselfAndTopCutsEach(t *testing.T) {
	rankRoot(t)

	rc, stdout, _ := rankAt(t, "--list", "all", "--top", "1")

	if rc != 0 || strings.Count(stdout, "\n     1  ") != 3 || strings.Contains(stdout, "sec-fix") ||
		!strings.Contains(stdout, "console: 1 item(s)") || !strings.Contains(stdout, "waiting: 1 item(s)") {
		t.Errorf("rc = %d stdout:\n%s", rc, stdout)
	}
	for _, list := range []string{"console", "waiting"} {
		rc, stdout, _ := rankAt(t, "--list", list)
		if rc != 0 || !strings.HasPrefix(stdout, list+": 1 item(s)") || strings.Contains(stdout, "sec-fix") {
			t.Errorf("--list %s: rc = %d stdout:\n%s", list, rc, stdout)
		}
	}
}

func TestCmd_InboxRank_ExplainContributionsAddUpToTheScore(t *testing.T) {
	rankRoot(t)

	rc, stdout, stderr := rankAt(t, "--explain", "sec-fix", "--json")

	var doc explainedItemDoc
	if err := json.Unmarshal([]byte(stdout), &doc); rc != 0 || err != nil {
		t.Fatalf("rc = %d err = %v stdout = %q stderr = %q", rc, err, stdout, stderr)
	}
	sum := 0.0
	for _, term := range doc.Terms {
		sum += term.Contribution
	}
	if len(doc.Terms) != 6 || sum != doc.Score || doc.List != "ready" || doc.Of != 2 || doc.Rank != 2 || doc.ID != "sec-fix" {
		t.Errorf("explain = %+v: the six contributions sum to %v", doc, sum)
	}
}

func TestCmd_InboxRank_ExplainPrintsEachFactorsValueWeightAndContribution(t *testing.T) {
	rankRoot(t)

	rc, stdout, _ := rankAt(t, "--explain", "console-only")

	want := []string{
		"console-only: rank 1 of 1 on the console list, score 0.3475",
		"  factor       value  weight  contribution  from",
		"  base        0.3000  0.4500        0.1350  weight 0.3",
		"  class       0.8750  0.2000        0.1750  stability, 2 of 8 in class_order",
		"  unblocks    0.0000  0.1500        0.0000  0 queued item(s) wait on it (cap 3)",
		"  recurrence  0.0000  0.1000        0.0000  0 recurrence(s) of its failure pattern (cap 5)",
		"  age         0.7500  0.0500        0.0375  60.0 day(s) since filing (half-life 30 days)",
		"  goal        0.0000  0.0500        0.0000  no campaign",
		"  score                             0.3475",
	}
	for _, line := range want {
		if rc != 0 || !strings.Contains(stdout, line) {
			t.Errorf("rc = %d: explain lacks %q:\n%s", rc, line, stdout)
		}
	}
}

func TestCmd_InboxRank_ExplainOfAnItemNotPendingExitsOne(t *testing.T) {
	rankRoot(t)

	for id, why := range map[string]string{"gone": "is not pending", "never-filed": "no inbox item"} {
		rc, _, stderr := rankAt(t, "--explain", id)
		if rc != 1 || !strings.Contains(stderr, why) {
			t.Errorf("--explain %s: rc = %d stderr = %q, want 1 naming %q", id, rc, stderr, why)
		}
	}
}

func TestCmd_InboxRank_AnUnknownClassIsWarnedLoudlyAndRanksLast(t *testing.T) {
	curationRoot(t, map[string]string{
		"a.json": `{"id":"a-urgent","title":"t","priority_class":"urgent","weight":0.9,"created_at":"2026-10-06"}`,
		"b.json": `{"id":"b-hygiene","title":"t","priority_class":"hygiene","weight":0.9,"created_at":"2026-10-06"}`,
	})

	rc, stdout, stderr := rankAt(t, "--explain", "a-urgent")

	if rc != 0 || !strings.Contains(stderr, `inbox rank: WARN a-urgent: unknown priority_class "urgent"`) || !strings.Contains(stderr, "below every class") {
		t.Errorf("rc = %d stderr = %q, want a loud warning naming the item and its class", rc, stderr)
	}
	if !strings.Contains(stdout, "rank 2 of 2") || !strings.Contains(stdout, `"urgent" is not in class_order`) {
		t.Errorf("explain = %q, want it ranked last and the reason shown", stdout)
	}
}

func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		tree[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestCmd_InboxRank_ARecurrenceCountMovesTheRankWithoutWritingAnyFile(t *testing.T) {
	root := curationRoot(t, map[string]string{
		"a.json": `{"id":"a-quiet","title":"t","priority_class":"stability","weight":0.5,"created_at":"2026-10-06"}`,
		"b.json": `{"id":"b-recurring","title":"t","priority_class":"stability","weight":0.5,"created_at":"2026-10-06"}`,
	})
	ledger := `{"entries":{"verify-gate-flake":{"pattern":"verify-gate-flake","cycles":[1,2,3],"count":3,"fix_item_id":"b-recurring"}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "recurrence-ledger.json"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeBytes(t, root)

	rc, stdout, stderr := rankAt(t, "--json")

	var doc rankedInboxDoc
	if err := json.Unmarshal([]byte(stdout), &doc); rc != 0 || err != nil || len(doc.Lists[0].Items) != 2 {
		t.Fatalf("rc = %d err = %v stderr = %q", rc, err, stderr)
	}
	if first := doc.Lists[0].Items[0]; first.ID != "b-recurring" || first.Facts.Recurrence != 3 || first.Weight != 0.5 {
		t.Errorf("first = %+v, want b-recurring ranked first by its recurrence, its weight untouched", first)
	}
	if after := treeBytes(t, root); len(after) != len(before) {
		t.Errorf("rank wrote files: %d before, %d after", len(before), len(after))
	} else {
		for path, body := range before {
			if after[path] != body {
				t.Errorf("rank rewrote %s", path)
			}
		}
	}
}

func TestCmd_InboxRank_AnUnreadableRecurrenceLedgerIsWarnedAndCountsNothing(t *testing.T) {
	root := rankRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "recurrence-ledger.json"), []byte(`{"entries":`), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, stdout, stderr := rankAt(t)

	if rc != 0 || !strings.Contains(stderr, "WARN recurrence ledger") || !strings.Contains(stdout, "sec-fix") {
		t.Errorf("rc = %d stderr = %q stdout = %q, want a warning and the ranking", rc, stderr, stdout)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "recurrence-ledger.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, stderr := rankAt(t); rc != 0 || strings.Contains(stderr, "WARN recurrence ledger") {
		t.Errorf("an empty ledger is an empty ledger: rc = %d stderr = %q", rc, stderr)
	}
}

func TestCmd_InboxRank_ReadsTheClassOrderFromThePolicy(t *testing.T) {
	root := rankRoot(t)
	if rc, stdout, _ := rankAt(t, "--top", "1"); rc != 0 || !strings.Contains(stdout, "tidy-docs") || strings.Contains(stdout, "sec-fix") {
		t.Fatalf("rc = %d stdout:\n%s, want the default order to put the hygiene item above the security one", rc, stdout)
	}
	writePolicy(t, root, `{"inbox_priority": {"class_order": ["security", "hygiene"]}}`)

	rc, stdout, _ := rankAt(t, "--top", "1")

	if rc != 0 || !strings.Contains(stdout, "sec-fix") || strings.Contains(stdout, "tidy-docs") {
		t.Errorf("rc = %d stdout:\n%s, want the policy's own order to put the security item first", rc, stdout)
	}
}

func TestCmd_InboxRank_AMalformedPolicyOrAnUnreadableInboxIsAFault(t *testing.T) {
	root := rankRoot(t)
	writePolicy(t, root, `{"inbox_priority": {"factors": {"base": 1}}}`)

	if rc, _, stderr := rankAt(t); rc != 2 || !strings.Contains(stderr, "inbox_priority") {
		t.Errorf("a malformed policy: rc = %d stderr = %q, want 2", rc, stderr)
	}
	notADir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(notADir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notADir, ".evolve", "inbox"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", notADir)
	if rc, _, stderr := rankAt(t); rc != 2 || !strings.Contains(stderr, "inbox rank:") {
		t.Errorf("an unreadable inbox: rc = %d stderr = %q, want 2", rc, stderr)
	}
}

func TestCmd_InboxRank_RefusesABadRequest(t *testing.T) {
	rankRoot(t)
	for _, args := range [][]string{
		{"--list", "claimed"}, {"--top", "0"}, {"--top", "x"}, {"--top"}, {"--explain"}, {"--explain", " "}, {"extra"}, {"--bogus"},
	} {
		if rc, _, _ := rankAt(t, args...); rc != 10 {
			t.Errorf("%v: rc = %d, want 10", args, rc)
		}
	}
}

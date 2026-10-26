package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const poolItem = `{"id":"rolling-pool","kind":"feature","priority_class":"performance","weight":0.6,` +
	`"title":"A free lane takes the next ready task at once","files":["go/internal/fleet/pool.go"],` +
	`"summary":"The pool waits.","fix":"Backfill the lane.",` +
	`"acceptance":["TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce passes"]}`

func addItem(t *testing.T, body string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runInbox(append([]string{"add"}, args...), strings.NewReader(body), &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func filedItem(t *testing.T, root, id string) map[string]any {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-"+id+".json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want one filed %s, got %v (%v)", id, matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCmd_InboxAdd_StampsTheOriginConsoleByDefaultAndTheNamedOrigin(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	if rc, _, stderr := addItem(t, poolItem); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	if rc, _, stderr := addItem(t, strings.Replace(cliInboxItem, `"id":"cli-inbox-show"`, `"id":"op-item"`, 1), "--origin", "operator"); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	if got := filedItem(t, root, "rolling-pool")["origin"]; got != "console" {
		t.Errorf("origin = %v, want console by default", got)
	}
	if got := filedItem(t, root, "op-item")["origin"]; got != "operator" {
		t.Errorf("origin = %v, want operator from --origin", got)
	}
}

func TestCmd_InboxAdd_RefusesAnItemTheIntakeRefuses(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	for name, tc := range map[string]struct {
		body string
		args []string
		why  string
	}{
		"an unknown origin flag":  {poolItem, []string{"--origin", "lane"}, "origin"},
		"an unknown kind":         {strings.Replace(poolItem, `"kind":"feature"`, `"kind":"wish"`, 1), nil, `kind "wish"`},
		"no fix surface":          {strings.Replace(poolItem, `"files":["go/internal/fleet/pool.go"]`, `"files":[]`, 1), nil, "fix surface"},
		"a console route, no why": {strings.Replace(poolItem, `"kind":"feature"`, `"kind":"feature","route":"console-manual"`, 1), nil, "--reason"},
	} {
		rc, _, stderr := addItem(t, tc.body, tc.args...)
		if rc != 1 || !strings.Contains(stderr, tc.why) {
			t.Errorf("%s: rc = %d stderr = %q, want 1 naming %q", name, rc, stderr, tc.why)
		}
	}
}

func TestCmd_InboxAdd_AnAuthoredOriginThatDiffersFromTheFlagIsAUsageError(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	body := strings.Replace(poolItem, `"kind":"feature"`, `"kind":"feature","origin":"retro"`, 1)

	if rc, _, stderr := addItem(t, body, "--origin", "operator"); rc != 10 || !strings.Contains(stderr, "origin") {
		t.Errorf("rc = %d stderr = %q, want 10 naming the two origins", rc, stderr)
	}
	if rc, _, stderr := addItem(t, body); rc != 0 {
		t.Errorf("an authored origin with no flag files: rc = %d stderr = %q", rc, stderr)
	}
}

func TestCmd_InboxAdd_AConsoleRouteWithAReasonFilesAndRecordsTheReason(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	body := strings.Replace(poolItem, `"kind":"feature"`, `"kind":"feature","route":"console-manual"`, 1)

	if rc, _, stderr := addItem(t, body, "--reason", "needs an operator decision on the pool width"); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	override, _ := filedItem(t, root, "rolling-pool")["intake_override"].(map[string]any)
	if override["reason"] != "needs an operator decision on the pool width" || override["console_route"] != true {
		t.Errorf("intake_override = %v, want the reason and console_route", override)
	}
}

func TestCmd_InboxAdd_ADuplicateIsRefusedNamingTheOverlappingID(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	if rc, _, stderr := addItem(t, poolItem); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	dup := strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-backfill"`, 1)

	rc, _, stderr := addItem(t, dup)

	if rc != 3 || !strings.Contains(stderr, "rolling-pool") || !strings.Contains(stderr, "--allow-overlap") {
		t.Errorf("rc = %d stderr = %q, want 3 naming rolling-pool and the override", rc, stderr)
	}
}

func TestCmd_InboxAdd_AnAllowedOverlapNeedsAReasonAndRecordsIt(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if rc, _, stderr := addItem(t, poolItem); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	dup := strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-backfill"`, 1)

	if rc, _, stderr := addItem(t, dup, "--allow-overlap"); rc != 10 || !strings.Contains(stderr, "--reason") {
		t.Errorf("--allow-overlap alone: rc = %d stderr = %q, want 10", rc, stderr)
	}
	if rc, _, stderr := addItem(t, dup, "--allow-overlap", "--reason", "a second pool, a different queue"); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	override, _ := filedItem(t, root, "pool-backfill")["intake_override"].(map[string]any)
	if overlaps, _ := override["overlaps"].([]any); len(overlaps) != 1 || overlaps[0] != "rolling-pool" || override["reason"] != "a second pool, a different queue" {
		t.Errorf("intake_override = %v, want the overlapping id and the reason", override)
	}
}

func TestCmd_InboxAdd_ANearOverlapIsWarnedAndFiled(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	if rc, _, stderr := addItem(t, poolItem); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	near := strings.NewReplacer(`"id":"rolling-pool"`, `"id":"pool-metrics"`,
		`"title":"A free lane takes the next ready task at once"`, `"title":"A free lane takes the pool metric"`,
		`"acceptance":["TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce passes"]`, `"acceptance":["TestPoolMetric passes"]`).Replace(poolItem)

	rc, _, stderr := addItem(t, near)

	if rc != 0 || !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, "rolling-pool") {
		t.Errorf("rc = %d stderr = %q, want a filed item with a WARN naming rolling-pool", rc, stderr)
	}
}

func TestCmd_InboxAdd_PrintsWhereTheItemRanks(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	if rc, _, stderr := addItem(t, cliInboxItem); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	rc, stdout, stderr := addItem(t, poolItem)

	if rc != 0 || !strings.Contains(stdout, "rank 1 of 2 in the ready list") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want the rank of the new item", rc, stdout, stderr)
	}
}

func TestCmd_InboxAdd_AnItemThatIsNotJSONIsRefusedByTheFiler(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))

	if rc, _, stderr := addItem(t, "not json"); rc != 1 || !strings.Contains(stderr, "one JSON object") {
		t.Errorf("rc = %d stderr = %q, want 1 from the filer", rc, stderr)
	}
}

func TestCmd_InboxAdd_AnUnreadableInboxFailsTheOverlapCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	if rc, _, stderr := addItem(t, poolItem); rc != 2 || !strings.Contains(stderr, "overlap check") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the overlap check", rc, stderr)
	}
}

func TestCmd_InboxAdd_AFieldOfTheWrongTypeIsRefusedBeforeTheIntake(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))

	rc, _, stderr := addItem(t, strings.Replace(poolItem, `"files":["go/internal/fleet/pool.go"]`, `"files":"go/internal/fleet/pool.go"`, 1))

	if rc != 1 || !strings.Contains(stderr, "does not decode") {
		t.Errorf("rc = %d stderr = %q, want 1 naming the decode error", rc, stderr)
	}
}

func TestCmd_InboxAdd_KeepsHTMLCharactersInTheFiledText(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	if rc, _, stderr := addItem(t, strings.Replace(poolItem, "at once", "at once <& fast>", 1)); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}

	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-rolling-pool.json"))
	raw, err := os.ReadFile(matches[0])
	if err != nil || !strings.Contains(string(raw), "at once <& fast>") {
		t.Errorf("the filed text = %s (%v), want the title unescaped", raw, err)
	}
}

func TestParseInboxAddArgs_NamesTheBadFlagOnce(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	if _, ok := parseInboxAddArgs([]string{"--bogus"}, &stderr); ok || strings.Count(stderr.String(), "bogus") != 1 || strings.Contains(stderr.String(), "Usage") {
		t.Errorf("stderr = %q, want one line naming the bad flag and no flag-package usage", stderr.String())
	}
}

func TestPrintFiledRank_WarnsWhenTheRankIsNotKnown(t *testing.T) {
	saved := filedRankSource
	t.Cleanup(func() { filedRankSource = saved })
	for name, tc := range map[string]struct {
		source func(time.Time, io.Writer) (rankedInbox, error)
		want   string
	}{
		"the rank fails":     {func(time.Time, io.Writer) (rankedInbox, error) { return rankedInbox{}, errors.New("no policy") }, "no policy"},
		"no row is the item": {func(time.Time, io.Writer) (rankedInbox, error) { return rankedInbox{}, nil }, "holds no row"},
	} {
		filedRankSource = tc.source
		var stdout, stderr bytes.Buffer

		printFiledRank("/x/.evolve/inbox/a.json", &stdout, &stderr)

		if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("%s: stdout = %q stderr = %q, want a WARN naming %q", name, stdout.String(), stderr.String(), tc.want)
		}
	}
}

func TestCmd_InboxAdd_AClaimedItemIsAnOpenItemForTheOverlapCheck(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	claimed := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-4")
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimed, "2026-10-01T00-00-00Z-rolling-pool.json"), []byte(poolItem), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "torn.json"), []byte(`{"id":`), 0o644); err != nil {
		t.Fatal(err)
	}

	rc, _, stderr := addItem(t, strings.Replace(poolItem, `"id":"rolling-pool"`, `"id":"pool-again"`, 1))

	if rc != 3 || !strings.Contains(stderr, "rolling-pool") || !strings.Contains(stderr, "skips a file") {
		t.Errorf("rc = %d stderr = %q, want 3 naming the claimed rolling-pool and a WARN for the torn file", rc, stderr)
	}
}

func TestCmd_InboxAdd_ALoadNoticeOfAReadableItemIsNotASkippedFile(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	for _, name := range []string{"a-twin.json", "b-twin.json"} {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", name), []byte(`{"id":"twin","title":"t"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rc, _, stderr := addItem(t, poolItem)

	if rc != 0 || strings.Contains(stderr, "skips a file") {
		t.Errorf("rc = %d stderr = %q, want a filing with no skipped-file WARN for a duplicate-id notice", rc, stderr)
	}
}

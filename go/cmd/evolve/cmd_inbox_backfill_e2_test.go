package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type errReaderE2 struct{}

func (errReaderE2) Read([]byte) (int, error) { return 0, errors.New("stdin torn") }

func e2InboxDirAsFile(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeE2File(t, filepath.Join(root, ".evolve", "inbox"), "not a dir")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

func TestInboxUsage_NamesTheVerbsSynopsisOrTheGenericLine(t *testing.T) {
	t.Parallel()
	for _, v := range inboxVerbs() {
		if got := inboxUsage(v.name); got != "usage: evolve inbox "+v.synopsis {
			t.Errorf("inboxUsage(%q) = %q", v.name, got)
		}
	}
	for _, unknown := range []string{"", "nope", "ADD", "add "} {
		if got := inboxUsage(unknown); got != "usage: evolve inbox <verb> ..." {
			t.Errorf("inboxUsage(%q) = %q, want the generic usage", unknown, got)
		}
	}
}

func TestParseInboxBatchesArgs_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantOK   bool
		wantJSON bool
		wantMax  int
		wantErr  string
	}{
		{"no args", nil, true, false, 0, ""},
		{"json", []string{"--json"}, true, true, 0, ""},
		{"max", []string{"--max", "3"}, true, false, 3, ""},
		{"max zero", []string{"--max", "0"}, true, false, 0, ""},
		{"max negative", []string{"--max", "-2"}, true, false, -2, ""},
		{"json and max in either order", []string{"--max", "7", "--json"}, true, true, 7, ""},
		{"the last max wins", []string{"--max", "1", "--max", "9"}, true, false, 9, ""},
		{"max without a value", []string{"--max"}, false, false, 0, "inbox batches: --max needs a value\n"},
		{"max after json without a value", []string{"--json", "--max"}, false, false, 0, "inbox batches: --max needs a value\n"},
		{"a non-numeric max", []string{"--max", "lots"}, false, false, 0, "inbox batches: bad --max \"lots\"\n"},
		{"a float max", []string{"--max", "1.5"}, false, false, 0, "inbox batches: bad --max \"1.5\"\n"},
		{"an unknown arg", []string{"--weird"}, false, false, 0, "inbox batches: unknown arg \"--weird\"\n"},
		{"a positional", []string{"x"}, false, false, 0, "inbox batches: unknown arg \"x\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			asJSON, cfg, ok := parseInboxBatchesArgs(tc.args, &stderr)

			if ok != tc.wantOK || stderr.String() != tc.wantErr {
				t.Fatalf("ok=%v stderr=%q, want %v %q", ok, stderr.String(), tc.wantOK, tc.wantErr)
			}
			if ok && (asJSON != tc.wantJSON || cfg.MaxItems != tc.wantMax) {
				t.Fatalf("asJSON=%v max=%d, want %v %d", asJSON, cfg.MaxItems, tc.wantJSON, tc.wantMax)
			}
			if !ok && asJSON {
				t.Fatal("a refused parse must not report json")
			}
		})
	}
}

func FuzzParseInboxBatchesArgs(f *testing.F) {
	for _, seed := range []string{"", "--json", "--max 3", "--max", "--max x", "--json --max 2", "a b c", "--max -1 --json"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		args := strings.Fields(line)
		var stderr bytes.Buffer

		_, cfg, ok := parseInboxBatchesArgs(args, &stderr)

		if ok == (stderr.Len() > 0) {
			t.Fatalf("args %q: ok=%v but stderr=%q; exactly one must hold", args, ok, stderr.String())
		}
		if ok && cfg.MaxItems != 0 && !strings.Contains(line, "--max") {
			t.Fatalf("args %q set MaxItems=%d without --max", args, cfg.MaxItems)
		}
	})
}

func TestRunInboxBatches_RefusesBadArgsAndAnUnreadableInbox(t *testing.T) {
	t.Run("bad args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := runInboxBatches(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{"--max"}, &stdout, &stderr); code != 10 || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q", code, stdout.String())
		}
	})
	t.Run("an unreadable inbox", func(t *testing.T) {
		e2InboxDirAsFile(t)
		var stdout, stderr bytes.Buffer
		code := runInboxBatches(envOrCwd("EVOLVE_PROJECT_ROOT"), nil, &stdout, &stderr)
		if code != 1 || !strings.HasPrefix(stderr.String(), "inbox batches: inboxbatch: read dir:") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
}

func TestEncodeInboxBatches_WritesIndentedJSONOrFailsOnABrokenWriter(t *testing.T) {
	t.Parallel()
	doc := inboxBatchesDoc{Batches: []inboxbatch.Batch{}}
	var stdout, stderr bytes.Buffer

	if code := encodeInboxBatches(&stdout, &stderr, doc); code != 0 || stdout.String() != "{\n \"batches\": []\n}\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}

	stderr.Reset()
	code := encodeInboxBatches(&failingWriter{}, &stderr, doc)

	if code != 1 || stderr.String() != "inbox batches: encode: broken pipe\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestWarnMonotonicBinaryTargets_ASkippedLintIsAWarning(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"{not json", `{"acceptance":5}`, ""} {
		var stderr bytes.Buffer

		warnMonotonicBinaryTargets([]byte(raw), &stderr)

		if !strings.HasPrefix(stderr.String(), "inbox add: WARN: acceptance lint skipped: ") || strings.Count(stderr.String(), "\n") != 1 {
			t.Fatalf("raw %q: stderr=%q", raw, stderr.String())
		}
	}
}

func TestWarnMonotonicBinaryTargets_ACleanItemIsSilent(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	warnMonotonicBinaryTargets([]byte(`{"id":"x","class":"feature"}`), &stderr)

	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q, want silence", stderr.String())
	}
}

func TestReadInboxAddItem_SourcesAndFailures(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "item.json")
	writeE2File(t, file, `{"id":"from-file"}`)
	cases := []struct {
		name     string
		args     []string
		stdin    *strings.Reader
		useNil   bool
		failRead bool
		want     string
		wantCode int
		wantErr  string
	}{
		{name: "stdin", args: []string{"--file", "-"}, stdin: strings.NewReader(`{"id":"from-stdin"}`), want: `{"id":"from-stdin"}`},
		{name: "empty stdin", args: []string{"--file", "-"}, stdin: strings.NewReader(""), want: ""},
		{name: "stdin without --file -", stdin: strings.NewReader(`{"id":"from-stdin"}`), wantCode: 10, wantErr: inboxUsage("add")},
		{name: "--file - with no stdin", args: []string{"--file", "-"}, useNil: true, wantCode: 10, wantErr: inboxUsage("add")},
		{name: "a file", args: []string{"--file", file}, useNil: true, want: `{"id":"from-file"}`},
		{name: "a missing file", args: []string{"--file", file + ".absent"}, useNil: true, wantCode: 2, wantErr: "inbox add: open "},
		{name: "no stdin and no file", useNil: true, wantCode: 10, wantErr: inboxUsage("add")},
		{name: "a bare --file", args: []string{"--file"}, useNil: true, wantCode: 10, wantErr: inboxUsage("add")},
		{name: "a wrong flag", args: []string{"--path", file}, useNil: true, wantCode: 10, wantErr: inboxUsage("add")},
		{name: "a torn stdin", args: []string{"--file", "-"}, failRead: true, wantCode: 2, wantErr: "inbox add: read the item: stdin torn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			var raw []byte
			var code int
			switch {
			case tc.failRead:
				raw, code = readInboxAddItem(tc.args, errReaderE2{}, &stderr)
			case tc.useNil:
				raw, code = readInboxAddItem(tc.args, nil, &stderr)
			default:
				raw, code = readInboxAddItem(tc.args, tc.stdin, &stderr)
			}

			if code != tc.wantCode || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("code=%d stderr=%q, want %d containing %q", code, stderr.String(), tc.wantCode, tc.wantErr)
			}
			if code == 0 && string(raw) != tc.want {
				t.Fatalf("raw=%q, want %q", raw, tc.want)
			}
			if code != 0 && raw != nil {
				t.Fatalf("a failure returned bytes %q", raw)
			}
		})
	}
}

func e2ConsumeRoot(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	withProjectRoot(t, root)
	return root, filepath.Join(root, ".evolve")
}

func TestRunInboxConsume_AFailedConsumedDirLeavesTheItemPending(t *testing.T) {
	_, evolveDir := e2ConsumeRoot(t)
	item := filepath.Join(t.TempDir(), "elsewhere.json")
	writeE2File(t, item, `{"id":"e"}`)
	writeE2File(t, filepath.Join(evolveDir, "inbox"), "a file blocks the consumed dir")
	var stdout, stderr bytes.Buffer

	code := runInboxConsume(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{item}, &stdout, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "inbox consume: "+item+": mkdir ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(item); err != nil {
		t.Fatalf("the item must stay: %v", err)
	}
}

func TestRunInboxConsume_AFailedMoveLeavesTheItemPending(t *testing.T) {
	_, evolveDir := e2ConsumeRoot(t)
	name := "2026-10-01T00-00-00Z-blocked.json"
	item := writePendingItem(t, evolveDir, name, `{"id":"blocked"}`)
	writeE2File(t, filepath.Join(evolveDir, "inbox", "consumed", name, "occupant"), "x")
	var stdout, stderr bytes.Buffer

	code := runInboxConsume(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{item}, &stdout, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "inbox consume: "+item+": rename ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(item); err != nil {
		t.Fatalf("the item must stay pending: %v", err)
	}
}

func TestRunInboxConsume_AFailedStampIsReportedAfterTheMove(t *testing.T) {
	_, evolveDir := e2ConsumeRoot(t)
	name := strings.Repeat("s", 245-len(".json")) + ".json"
	item := writePendingItem(t, evolveDir, name, `{"id":"long-name"}`)
	var stdout, stderr bytes.Buffer

	code := runInboxConsume(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{item}, &stdout, &stderr)

	dest := filepath.Join(evolveDir, "inbox", "consumed", name)
	if code != 1 || !strings.HasPrefix(stderr.String(), "inbox consume: "+dest+" consumed, but the consumed stamp write failed: ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("the move landed first, so the item must be in consumed/: %v", err)
	}
}

func TestRunInboxConsume_AFailedAckIsReportedAfterTheMove(t *testing.T) {
	_, evolveDir := e2ConsumeRoot(t)
	writeE2File(t, filepath.Join(evolveDir, "resolved-fingerprints.json"), "{corrupt")
	name := "2026-10-01T00-00-00Z-acked.json"
	item := writePendingItem(t, evolveDir, name, `{"id":"acked","consumed_by":"`+realConsumedByNarrative+`"}`)
	var stdout, stderr bytes.Buffer

	code := runInboxConsume(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{item}, &stdout, &stderr)

	dest := filepath.Join(evolveDir, "inbox", "consumed", name)
	if code != 1 || !strings.HasPrefix(stderr.String(), "inbox consume: "+dest+" consumed, but the fingerprint ack failed: resolved-fingerprints.json:") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil || doc[inboxbatch.ConsumedField] == nil {
		t.Fatalf("the consumed item must carry its stamp: %s", raw)
	}
}

func TestRunInboxConsume_ADirectoryItemIsRefusedInPlace(t *testing.T) {
	e2ConsumeRoot(t)
	dir := filepath.Join(t.TempDir(), "item.json")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runInboxConsume(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{dir}, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "(item left in place)") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestParseInboxConsumeArgs_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		args      []string
		wantOK    bool
		wantPath  string
		wantCycle string
		wantRes   string
	}{
		{"path only", []string{"a.json"}, true, "a.json", "console", ""},
		{"flags before", []string{"--cycle", "12", "--resolution", " fixed ", "a.json"}, true, "a.json", "12", "fixed"},
		{"flags after", []string{"a.json", "--cycle", " 0 "}, true, "a.json", "0", ""},
		{"a unicode resolution", []string{"a.json", "--resolution", "修好了"}, true, "a.json", "console", "修好了"},
		{"no path", nil, false, "", "", ""},
		{"an empty path", []string{""}, false, "", "", ""},
		{"an unknown flag before", []string{"--bogus", "a.json"}, false, "", "", ""},
		{"an unknown flag after", []string{"a.json", "--bogus"}, false, "", "", ""},
		{"two paths", []string{"a.json", "b.json"}, false, "", "", ""},
		{"a negative cycle", []string{"a.json", "--cycle", "-1"}, false, "", "", ""},
		{"a word cycle", []string{"a.json", "--cycle", "later"}, false, "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			path, stamp, ok := parseInboxConsumeArgs(tc.args, &stderr)

			if ok != tc.wantOK {
				t.Fatalf("ok=%v stderr=%q, want %v", ok, stderr.String(), tc.wantOK)
			}
			if !ok {
				if path != "" || stamp != (inboxConsumedStamp{}) || !strings.Contains(stderr.String(), inboxUsage("consume")) {
					t.Fatalf("a refusal returned path=%q stamp=%+v stderr=%q", path, stamp, stderr.String())
				}
				return
			}
			want := inboxConsumedStamp{Via: "console-manual", Cycle: tc.wantCycle, Resolution: tc.wantRes}
			if path != tc.wantPath || stamp != want {
				t.Fatalf("path=%q stamp=%+v, want %q %+v", path, stamp, tc.wantPath, want)
			}
		})
	}
}

func TestReadInboxItemObject_Table(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name    string
		body    *string
		want    map[string]json.RawMessage
		wantErr string
	}{
		{name: "absent", wantErr: "no such file or directory"},
		{name: "an object", body: e2Ptr(`{"id":"x","n":1}`), want: map[string]json.RawMessage{"id": json.RawMessage(`"x"`), "n": json.RawMessage(`1`)}},
		{name: "an empty object", body: e2Ptr(`{}`), want: map[string]json.RawMessage{}},
		{name: "null", body: e2Ptr(`null`), wantErr: "not a JSON object: null"},
		{name: "an array", body: e2Ptr(`[1]`), wantErr: "not a JSON object: json: cannot unmarshal array"},
		{name: "garbage", body: e2Ptr(`{`), wantErr: "not a JSON object: unexpected end of JSON input"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, fmt.Sprintf("item-%d.json", i))
			if tc.body != nil {
				writeE2File(t, path, *tc.body)
			}

			got, err := readInboxItemObject(path)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || got != nil {
					t.Fatalf("got %v err=%v, want %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v err=%v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestWriteConsumedStamp_AddsTheStampAndKeepsEveryField(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "item.json")
	doc := map[string]json.RawMessage{"id": json.RawMessage(`"x"`), "weight": json.RawMessage(`0.5`)}
	stamp := inboxConsumedStamp{At: "2026-10-12T00:00:00Z", Via: "console-manual", Cycle: "7", Resolution: "done"}

	if err := writeConsumedStamp(path, doc, stamp); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ID       string             `json:"id"`
		Weight   float64            `json:"weight"`
		Consumed inboxConsumedStamp `json:"consumed"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "x" || got.Weight != 0.5 || got.Consumed != stamp {
		t.Fatalf("got %+v from %s", got, raw)
	}
	if len(doc) != 2 {
		t.Fatalf("the caller's doc was mutated: %v", doc)
	}
}

func TestWriteConsumedStamp_FailuresAreReturned(t *testing.T) {
	t.Run("an unencodable stamp", func(t *testing.T) {
		previous := consumedStampMarshal
		consumedStampMarshal = func(any) ([]byte, error) { return nil, errors.New("stamp refused") }
		t.Cleanup(func() { consumedStampMarshal = previous })
		path := filepath.Join(t.TempDir(), "item.json")
		err := writeConsumedStamp(path, map[string]json.RawMessage{}, inboxConsumedStamp{})
		if err == nil || err.Error() != "stamp refused" {
			t.Fatalf("err=%v", err)
		}
		if _, serr := os.Stat(path); !os.IsNotExist(serr) {
			t.Fatalf("a refused stamp wrote the item: %v", serr)
		}
	})
	t.Run("an invalid field", func(t *testing.T) {
		err := writeConsumedStamp(filepath.Join(t.TempDir(), "item.json"), map[string]json.RawMessage{"bad": json.RawMessage(`{`)}, inboxConsumedStamp{})
		if err == nil || !strings.Contains(err.Error(), "atomicwrite: marshal") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestRunInboxList_AnUnreadableInboxExits2(t *testing.T) {
	e2InboxDirAsFile(t)
	var stdout, stderr bytes.Buffer

	code := runInboxList(envOrCwd("EVOLVE_PROJECT_ROOT"), nil, &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "inbox list: inboxbatch: read dir:") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestClaimExitCode_MapsEveryClaimOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		err     error
		want    int
		wantErr string
	}{
		{"success", nil, 0, ""},
		{"bad args", inboxmover.ErrBadArgs, 1, ""},
		{"not found", fmt.Errorf("x: %w", inboxmover.ErrNotFound), 1, ""},
		{"move failed", fmt.Errorf("x: %w", inboxmover.ErrMvFailed), 2, ""},
		{"console routed", inboxmover.ErrConsoleRouted, 3, ""},
		{"waiting on a dependency", fmt.Errorf("item a: %w", inboxmover.ErrWaitingOnDependency), 4, ""},
		{"an unclassified error", errors.New("disk on fire"), 1, "[inbox-mover] ERROR: disk on fire\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			if got := claimExitCode(tc.err, &stderr); got != tc.want || stderr.String() != tc.wantErr {
				t.Fatalf("claimExitCode(%v) = %d stderr=%q, want %d %q", tc.err, got, stderr.String(), tc.want, tc.wantErr)
			}
		})
	}
}

func TestTermSource_DescribesEveryFactor(t *testing.T) {
	t.Parallel()
	cfg := policy.InboxPriorityConfig{ClassOrder: []string{"correctness", "stability"}, UnblocksCap: 3, RecurrenceCap: 5, AgeHalflifeDays: 7}
	doc := func(f inboxrank.Facts) explainedItemDoc {
		return explainedItemDoc{rankedRow: rankedRow{Weight: 0.25, Breakdown: inboxrank.Breakdown{Facts: f}}}
	}
	cases := []struct {
		factor inboxrank.Factor
		facts  inboxrank.Facts
		want   string
	}{
		{inboxrank.FactorBase, inboxrank.Facts{}, "weight 0.25"},
		{inboxrank.FactorClass, inboxrank.Facts{Class: "stability", ClassPosition: 1}, "stability, 2 of 2 in class_order"},
		{inboxrank.FactorClass, inboxrank.Facts{Class: "vibes", ClassPosition: -1}, `priority_class "vibes" is not in class_order; ranked below every class`},
		{inboxrank.FactorUnblocks, inboxrank.Facts{Unblocks: 2}, "2 queued item(s) wait on it (cap 3)"},
		{inboxrank.FactorRecurrence, inboxrank.Facts{Recurrence: 4}, "4 recurrence(s) of its failure pattern (cap 5)"},
		{inboxrank.FactorAge, inboxrank.Facts{}, "no filing date"},
		{inboxrank.FactorAge, inboxrank.Facts{Dated: true, AgeDays: 2.25}, "2.2 day(s) since filing (half-life 7 days)"},
		{inboxrank.FactorGoal, inboxrank.Facts{}, "no campaign"},
		{inboxrank.FactorGoal, inboxrank.Facts{Campaign: "c1", CampaignActive: true}, `campaign "c1" is active`},
		{inboxrank.FactorGoal, inboxrank.Facts{Campaign: "c2"}, `campaign "c2" is not active`},
		{inboxrank.Factor("future"), inboxrank.Facts{Campaign: "c3"}, `campaign "c3" is not active`},
	}
	for _, tc := range cases {
		if got := termSource(tc.factor, doc(tc.facts), cfg); got != tc.want {
			t.Errorf("termSource(%s, %+v) = %q, want %q", tc.factor, tc.facts, got, tc.want)
		}
	}
}

func TestGoalSource_ActiveOnlyCountsWithACampaign(t *testing.T) {
	t.Parallel()
	cases := []struct {
		facts inboxrank.Facts
		want  string
	}{
		{inboxrank.Facts{}, "no campaign"},
		{inboxrank.Facts{CampaignActive: true}, "no campaign"},
		{inboxrank.Facts{Campaign: "κ", CampaignActive: true}, `campaign "κ" is active`},
		{inboxrank.Facts{Campaign: "κ"}, `campaign "κ" is not active`},
	}
	for _, tc := range cases {
		if got := goalSource(tc.facts); got != tc.want {
			t.Errorf("goalSource(%+v) = %q, want %q", tc.facts, got, tc.want)
		}
	}
}

func TestRunInboxShow_AnUnreadableInboxExits2(t *testing.T) {
	e2InboxDirAsFile(t)
	var stdout, stderr bytes.Buffer

	code := runInboxShow(envOrCwd("EVOLVE_PROJECT_ROOT"), []string{"any"}, &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "inbox show: inboxbatch: read dir:") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func e2ShownInbox(t *testing.T) pendingInbox {
	t.Helper()
	dir := t.TempDir()
	return pendingInbox{opts: inboxmover.Options{InboxDir: dir}, isProtected: func(string) bool { return false }}
}

func TestPrintShownItem_AVanishedItemExits2(t *testing.T) {
	t.Parallel()
	inbox := e2ShownInbox(t)
	var stdout, stderr bytes.Buffer

	code := printShownItem(inbox, inboxbatch.Item{ID: "gone", Path: "gone.json"}, false, &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "inbox show: open ") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPrintShownItem_UnindentableJSONExits2InTextButPassesThroughAsJSONFails(t *testing.T) {
	t.Parallel()
	inbox := e2ShownInbox(t)
	writeE2File(t, filepath.Join(inbox.opts.InboxDir, "bad.json"), `{"id":"bad",`)
	var stdout, stderr bytes.Buffer

	code := printShownItem(inbox, inboxbatch.Item{ID: "bad", Path: "bad.json"}, false, &stdout, &stderr)

	if code != 2 || !strings.HasPrefix(stderr.String(), "inbox show: .evolve/inbox/bad.json: ") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPrintShownItem_TextShowsStatusPathAndThePrettyItem(t *testing.T) {
	t.Parallel()
	inbox := e2ShownInbox(t)
	writeE2File(t, filepath.Join(inbox.opts.InboxDir, "ok.json"), `{"id":"ok","title":"t"}`)
	var stdout, stderr bytes.Buffer

	code := printShownItem(inbox, inboxbatch.Item{ID: "ok", Path: "ok.json"}, false, &stdout, &stderr)

	if code != 0 || !strings.HasPrefix(stdout.String(), "ok: ") || !strings.Contains(stdout.String(), "path: .evolve/inbox/ok.json\n{\n  \"id\": \"ok\",\n  \"title\": \"t\"\n}\n") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestEncodeInboxJSON_KeepsHTMLAndFailsOnABrokenWriter(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	if code := encodeInboxJSON("show", map[string]string{"a": "<b>&"}, &stdout, &stderr); code != 0 || stdout.String() != "{\n \"a\": \"<b>&\"\n}\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}

	code := encodeInboxJSON("list", map[string]string{}, &failingWriter{}, &stderr)

	if code != 2 || stderr.String() != "inbox list: encode: broken pipe\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

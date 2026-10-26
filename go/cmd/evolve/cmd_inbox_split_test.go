package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxsplit"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const splitProtectedFile = "go/internal/inboxrank/weightsort_ban_test.go"
const splitLaneFile = "go/internal/inboxrank/inputs.go"

func splitItem(id string, files ...string) string {
	return splitItemWith(id, []string{"the grader judges this"}, files...)
}

func splitItemWith(id string, acceptance []string, files ...string) string {
	list, _ := json.Marshal(files)
	criteria, _ := json.Marshal(acceptance)
	return `{"id":"` + id + `","kind":"feature","priority_class":"debuggability","weight":0.5,` +
		`"title":"Mixed work","files":` + string(list) + `,` +
		`"summary":"Two surfaces.","fix":"Change both.","acceptance":` + string(criteria) + `}`
}

func splitRoot(t *testing.T, item string) string {
	t.Helper()
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-"}, strings.NewReader(item), &stdout, &stderr); rc != 0 {
		t.Fatalf("add: rc = %d stderr = %q", rc, stderr.String())
	}
	return root
}

func inboxFile(t *testing.T, root, sub, id string) string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", sub, "*"+id+".json"))
	if err != nil || len(m) != 1 {
		t.Fatalf("%s/%s: %v %v", sub, id, m, err)
	}
	return m[0]
}

func TestCmd_InboxSplit_FilesTheTwoPartsAndKeepsTheOriginalInConsumed(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	original, err := os.ReadFile(inboxFile(t, root, "", "mixed"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	console := itemFields(t, inboxFile(t, root, "", "mixed-console"))
	lane := itemFields(t, inboxFile(t, root, "", "mixed-lane"))
	if got := console["files"].([]any); len(got) != 1 || got[0] != splitProtectedFile {
		t.Errorf("console files = %v", got)
	}
	if got := lane["files"].([]any); len(got) != 1 || got[0] != splitLaneFile {
		t.Errorf("lane files = %v", got)
	}
	if lane["route"] != "lane" || console["split_from"] != "mixed" || lane["split_from"] != "mixed" {
		t.Errorf("lane route = %v, split_from = %v / %v", lane["route"], console["split_from"], lane["split_from"])
	}
	retired := itemFields(t, inboxFile(t, root, "consumed", "mixed"))
	var originalFields map[string]any
	if err := json.Unmarshal(original, &originalFields); err != nil {
		t.Fatal(err)
	}
	for k, v := range originalFields {
		if got, _ := json.Marshal(retired[k]); string(got) != mustJSON(t, v) {
			t.Errorf("the retired original lost %s: %s != %s", k, got, mustJSON(t, v))
		}
	}
	if !strings.Contains(mustJSON(t, retired["consumed"]), "mixed-console") {
		t.Errorf("consumed stamp = %v, want the parts named", retired["consumed"])
	}
	if _, err := os.Stat(inboxFile(t, root, "", "mixed-console")); err != nil {
		t.Error(err)
	}
	if !strings.Contains(stdout.String(), "mixed-console") || !strings.Contains(stdout.String(), "mixed-lane") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCmd_InboxSplit_JSONNamesTheThreeIDs(t *testing.T) {
	splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed", "--json", "--lane-after-console"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	var res map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("%v: %q", err, stdout.String())
	}
	if res["original"] != "mixed" || res["console"] != "mixed-console" || res["lane"] != "mixed-lane" {
		t.Errorf("result = %v", res)
	}
}

func TestCmd_InboxSplit_TheLanePartWaitsForTheConsolePartWhenAsked(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"split", "--lane-after-console", "mixed"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if deps := itemFields(t, inboxFile(t, root, "", "mixed-lane"))["deps"]; mustJSON(t, deps) != `["mixed-console"]` {
		t.Errorf("deps = %v", deps)
	}
}

func TestCmd_InboxSplit_AcceptingTwiceIsRefusedAndChangesNothing(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("first: rc = %d", rc)
	}
	before := listInboxTree(t, root)
	stderr.Reset()

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "mixed") {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the tree changed:\n%v\n%v", before, after)
	}
}

func listInboxTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(filepath.Join(root, ".evolve", "inbox"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCmd_InboxSplit_RefusesItemsThatNeedNoSplit(t *testing.T) {
	for name, files := range map[string][]string{
		"only protected": {splitProtectedFile},
		"only lane safe": {splitLaneFile},
	} {
		root := splitRoot(t, splitItem("mixed", files...))
		before := listInboxTree(t, root)
		var stdout, stderr bytes.Buffer

		rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

		if rc != 1 {
			t.Errorf("%s: rc = %d stderr = %q", name, rc, stderr.String())
		}
		if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
			t.Errorf("%s: the tree changed", name)
		}
	}
}

func TestCmd_InboxSplit_UsageAndUnknownIDs(t *testing.T) {
	splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	for name, args := range map[string][]string{
		"no id":      {"split"},
		"two ids":    {"split", "a", "b"},
		"bad flag":   {"split", "mixed", "--nope"},
		"blank id":   {"split", " "},
		"flag first": {"split", "--json"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "usage: evolve inbox split") {
			t.Errorf("%s: rc = %d stderr = %q", name, rc, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"split", "no-such-item"}, nil, &stdout, &stderr); rc != 1 {
		t.Errorf("unknown id: rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestCmd_InboxSplit_TakesTheProjectRootFlag(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed", "--project-root", root}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	inboxFile(t, root, "", "mixed-lane")
}

func TestCmd_InboxSplit_RefusesWhileALaneIsLive(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	writeLiveRunLease(t, root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "a loop lane is live") {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestCmd_InboxSplit_RefusesWhileAPendingItemDependsOnTheOriginal(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	dependent := strings.Replace(splitItem("waiter", splitLaneFile), `"weight"`, `"deps":["mixed"],"weight"`, 1)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-"}, strings.NewReader(dependent), &stdout, &stderr); rc != 0 {
		t.Fatalf("add: rc = %d stderr = %q", rc, stderr.String())
	}
	before := listInboxTree(t, root)

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "waiter") {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("the tree changed")
	}
}

func TestCmd_InboxSplit_SplitExitCodeMapsEachRefusal(t *testing.T) {
	t.Parallel()
	for err, want := range map[error]int{
		inboxsplit.ErrNotPending:        1,
		inboxsplit.ErrAllProtected:      1,
		inboxsplit.ErrFixNamesProtected: 1,
		inboxsplit.ErrHasDependents:     1,
		inboxmover.ErrInvalidItem:       1,
		inboxmover.ErrConsoleRouted:     1,
		inboxsplit.ErrRetireFailed:      2,
		errors.New("disk"):              2,
	} {
		if got := splitExitCode(err); got != want {
			t.Errorf("%v: %d, want %d", err, got, want)
		}
	}
}

func writeLiveRunLease(t *testing.T, root string) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_InboxSplit_AnAgentFiledPipelineItemRollsBackBothPartsWhenTheLaneRouteIsRefused(t *testing.T) {
	item := strings.NewReplacer(`"feature"`, `"pipeline-repair"`, `"weight"`, `"injected_by":"agent","weight"`).Replace(splitItem("mixed", splitProtectedFile, splitLaneFile))
	root := splitRoot(t, item)
	before := listInboxTree(t, root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("a part stayed filed:\n%v\n%v", before, after)
	}
}

func TestCmd_InboxSplit_AnUnreadablePolicyIsAnIOFailure(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestSplitInbox_DependentsFailsWhenTheInboxCannotBeScanned(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (splitInbox{dir: file}).Dependents("x"); err == nil {
		t.Error("no error")
	}
}

func TestSplitInbox_WithdrawRemovesAFiledItem(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	in := newSplitInbox(inboxmover.Options{ProjectRoot: root})
	if err := in.Withdraw("mixed", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Item("mixed"); !errors.Is(err, inboxsplit.ErrNotPending) {
		t.Errorf("err = %v", err)
	}
}

func TestSplitInbox_RetireMovesTheItemIntactAndReportsEachFailure(t *testing.T) {
	t.Run("missing item", func(t *testing.T) {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
		if err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Retire("ghost", "r"); err == nil {
			t.Error("no error")
		}
	})
	t.Run("item cannot be read", func(t *testing.T) {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
		prev := readSplitItem
		readSplitItem = func(string) (map[string]json.RawMessage, error) { return nil, errors.New("read") }
		defer func() { readSplitItem = prev }()
		if err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Retire("mixed", "r"); err == nil {
			t.Error("no error")
		}
	})
	t.Run("consumed is a file", func(t *testing.T) {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
		if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "consumed"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Retire("mixed", "r"); err == nil {
			t.Error("no error")
		}
	})
	t.Run("destination is a directory", func(t *testing.T) {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
		src := inboxFile(t, root, "", "mixed")
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "consumed", filepath.Base(src), "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Retire("mixed", "r"); err == nil {
			t.Error("no error")
		}
	})
	t.Run("stamp cannot be written", func(t *testing.T) {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
		prev := consumedStampMarshal
		consumedStampMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal") }
		defer func() { consumedStampMarshal = prev }()
		if err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Retire("mixed", "r"); err == nil {
			t.Error("no error")
		}
	})
}

func TestCmd_InboxSplit_RefusesAFixThatNamesAProtectedPathAndFilesNothing(t *testing.T) {
	item := strings.Replace(splitItem("mixed", splitProtectedFile, splitLaneFile), "Change both.", "Edit "+splitProtectedFile+" first.", 1)
	root := splitRoot(t, item)
	before := listInboxTree(t, root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), splitProtectedFile) {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("the tree changed")
	}
}

func TestSplitInbox_ItemReadFailureIsReported(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, splitLaneFile))
	prev := readSplitItem
	readSplitItem = func(string) (map[string]json.RawMessage, error) { return nil, errors.New("read") }
	defer func() { readSplitItem = prev }()
	if _, err := newSplitInbox(inboxmover.Options{ProjectRoot: root}).Item("mixed"); err == nil {
		t.Error("no error")
	}
}

func stringsOf(t *testing.T, fields map[string]any, key string) []string {
	t.Helper()
	var out []string
	for _, v := range fields[key].([]any) {
		out = append(out, v.(string))
	}
	return out
}

func TestCmd_InboxSplit_ADirectoryThatHoldsProtectedSurfaceStaysOnTheConsolePart(t *testing.T) {
	for _, dir := range []string{"go/internal/core", "go/internal/core/", "go/internal/core/ (the orchestrator)", "go/cmd/evolve"} {
		root := splitRoot(t, splitItem("mixed", splitProtectedFile, dir, splitLaneFile))
		var stdout, stderr bytes.Buffer

		rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

		if rc != 0 {
			t.Fatalf("%q: rc = %d stderr = %q", dir, rc, stderr.String())
		}
		console := stringsOf(t, itemFields(t, inboxFile(t, root, "", "mixed-console")), "files")
		lane := stringsOf(t, itemFields(t, inboxFile(t, root, "", "mixed-lane")), "files")
		if !slices.Equal(console, []string{splitProtectedFile, dir}) || !slices.Equal(lane, []string{splitLaneFile}) {
			t.Errorf("%q: console files = %v, lane files = %v, want the directory on the console part", dir, console, lane)
		}
	}
}

func TestCmd_InboxSplit_TheLanePartKeepsTheCriteriaThatNameOnlyLaneFiles(t *testing.T) {
	item := splitItemWith("mixed", []string{
		splitLaneFile + " returns the sorted list",
		splitProtectedFile + " rejects an empty id",
		"the suite stays green",
	}, splitProtectedFile, splitLaneFile)
	root := splitRoot(t, item)
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}

	lane := stringsOf(t, itemFields(t, inboxFile(t, root, "", "mixed-lane")), "acceptance")
	console := stringsOf(t, itemFields(t, inboxFile(t, root, "", "mixed-console")), "acceptance")
	if want := []string{splitLaneFile + " returns the sorted list", "the suite stays green"}; !slices.Equal(lane, want) {
		t.Errorf("lane acceptance = %v, want %v", lane, want)
	}
	if want := []string{splitProtectedFile + " rejects an empty id", "the suite stays green"}; !slices.Equal(console, want) {
		t.Errorf("console acceptance = %v, want %v", console, want)
	}
}

func TestCmd_InboxSplit_RefusesAnItemWhoseLanePartWouldHaveNoCriterionAndFilesNothing(t *testing.T) {
	root := splitRoot(t, splitItemWith("mixed", []string{splitProtectedFile + " rejects an empty id"}, splitProtectedFile, splitLaneFile))
	before := listInboxTree(t, root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "lane part") {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("the tree changed")
	}
}

func TestCmd_InboxSplit_ALanePartThatOnlyRegistersARatchetWaitsForTheConsolePartAndSaysSo(t *testing.T) {
	root := splitRoot(t, splitItem("mixed", splitProtectedFile, "go/.cover-strict"))
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if deps := itemFields(t, inboxFile(t, root, "", "mixed-lane"))["deps"]; mustJSON(t, deps) != `["mixed-console"]` {
		t.Errorf("deps = %v", deps)
	}
	if !strings.Contains(stdout.String(), "waits for mixed-console") {
		t.Errorf("stdout = %q, want the wait named", stdout.String())
	}
}

func TestCmd_InboxSplit_RefusesAnItemWhoseTextNamesAConsoleFileByItsBasename(t *testing.T) {
	item := strings.Replace(splitItem("mixed", splitProtectedFile, splitLaneFile), "Change both.", "Tighten weightsort_ban_test.go first.", 1)
	root := splitRoot(t, item)
	before := listInboxTree(t, root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"split", "mixed"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "weightsort_ban_test.go") {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	if after := listInboxTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("the tree changed")
	}
}

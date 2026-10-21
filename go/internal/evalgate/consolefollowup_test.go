package evalgate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

var followupClasses = []string{"correctness", "stability", "performance"}

func inboxFiler(root string, raw []byte) error {
	_, err := inboxmover.File(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard, PriorityClasses: followupClasses}, raw)
	return err
}

func followupCycle(t *testing.T, laneItem string) (core.ReviewInput, string) {
	t.Helper()
	in := tddReport(t, cycle1788Table("console-followup"))
	in.Cycle, in.ProjectRoot = 42, t.TempDir()
	inbox := filepath.Join(in.ProjectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if laneItem == "" {
		return in, inbox
	}
	writeJSON(t, filepath.Join(in.Workspace, "lane-scope.json"), map[string]any{"todo_ids": []string{laneItem}})
	writeJSON(t, filepath.Join(inbox, "processing", "cycle-42", laneItem+".json"), map[string]any{
		"id": laneItem, "kind": "pipeline-repair", "priority_class": "performance", "weight": 0.7,
	})
	return in, inbox
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

type filedItem struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	PriorityClass string   `json:"priority_class"`
	Weight        float64  `json:"weight"`
	Route         string   `json:"route"`
	Files         []string `json:"files"`
	Acceptance    []string `json:"acceptance"`
	Summary       string   `json:"summary"`
	Fix           string   `json:"fix"`
}

func pendingItems(t *testing.T, inbox string) map[string]filedItem {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(inbox, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]filedItem{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var it filedItem
		if err := json.Unmarshal(raw, &it); err != nil {
			t.Fatal(err)
		}
		out[it.ID] = it
	}
	return out
}

func TestWithConsoleFollowupFiler_EachConsoleFollowupRowFilesOneConsoleManualCompanionOnce(t *testing.T) {
	in, inbox := followupCycle(t, "fix-the-lint")
	rev := NewReviewer(config.StageEnforce, WithConsoleFollowupFiler(inboxFiler))

	for run := 1; run <= 2; run++ {
		if res := rev.Review(context.Background(), in); !res.Approve {
			t.Fatalf("run %d: a legal console-followup table must pass, got %q", run, res.Reason)
		}
	}

	if reason, block := (protectedACGate{file: inboxFiler}).check(in); reason != "" || block {
		t.Fatalf("a third check = (%q, %v); a companion that is already filed is done, not a fault", reason, block)
	}
	items := pendingItems(t, inbox)
	if got := slices.Sorted(maps.Keys(items)); !slices.Equal(got, []string{"fix-the-lint-ac-7-console", "fix-the-lint-ac-8-console"}) {
		t.Fatalf("filed ids = %v; want one companion per console-followup row, filed once over two reviews", got)
	}
	ac7 := items["fix-the-lint-ac-7-console"]
	if ac7.Route != "console-manual" || ac7.PriorityClass != "performance" || ac7.Weight != 0.7 || ac7.Kind != "pipeline-repair" {
		t.Errorf("companion = %+v; want route console-manual and the lane item's kind, class and weight", ac7)
	}
	if !slices.Equal(ac7.Files, []string{"go/internal/cli/guardcmd/eval.go"}) {
		t.Errorf("files = %v; want the protected path the row needs changed", ac7.Files)
	}
	if len(ac7.Acceptance) != 1 || !strings.Contains(ac7.Acceptance[0], "`evolve eval quality-check` runs the lint") {
		t.Errorf("acceptance = %v; want the row's criterion", ac7.Acceptance)
	}
	if !strings.Contains(ac7.Summary, "fix-the-lint") || !strings.Contains(ac7.Summary, "cycle 42") {
		t.Errorf("summary %q must name the lane item and the cycle", ac7.Summary)
	}
}

func TestWithConsoleFollowupFiler_AConsumedCompanionIsNotFiledAgain(t *testing.T) {
	in, inbox := followupCycle(t, "fix-the-lint")
	rev := NewReviewer(config.StageEnforce, WithConsoleFollowupFiler(inboxFiler))
	rev.Review(context.Background(), in)
	consumed := filepath.Join(inbox, "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	paths, _ := filepath.Glob(filepath.Join(inbox, "*.json"))
	for _, p := range paths {
		if err := os.Rename(p, filepath.Join(consumed, filepath.Base(p))); err != nil {
			t.Fatal(err)
		}
	}

	if res := rev.Review(context.Background(), in); !res.Approve {
		t.Fatalf("review = %q", res.Reason)
	}
	if items := pendingItems(t, inbox); len(items) != 0 {
		t.Fatalf("pending = %v; a consumed companion is already filed", items)
	}
}

func TestWithConsoleFollowupFiler_WithNoLaneItemTheCycleNamesTheCompanion(t *testing.T) {
	in, inbox := followupCycle(t, "")
	NewReviewer(config.StageEnforce, WithConsoleFollowupFiler(inboxFiler)).Review(context.Background(), in)

	it, ok := pendingItems(t, inbox)["cycle-42-ac-7-console"]
	if !ok || it.PriorityClass != "correctness" || it.Weight != 0.5 || it.Kind != "feature" {
		t.Fatalf("items = %v; with no committed item the id comes from the cycle and the defaults apply", pendingItems(t, inbox))
	}
}

func TestWithConsoleFollowupFiler_AFilingFaultIsReportedAndDoesNotBlock(t *testing.T) {
	in, _ := followupCycle(t, "fix-the-lint")
	var logged []string
	rev := NewReviewer(config.StageEnforce, WithConsoleFollowupFiler(func(string, []byte) error { return errors.New("disk full") })).(*reviewer)
	rev.logf = func(f string, a ...any) {
		if line := fmt.Sprintf(f, a...); strings.Contains(line, "protected-ac-predicate") {
			logged = append(logged, line)
		}
	}

	if res := rev.Review(context.Background(), in); !res.Approve {
		t.Fatalf("a filing fault blocked the TDD phase: %q", res.Reason)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "disk full") {
		t.Fatalf("logged %q; want the filing fault logged once", logged)
	}
	reason, block := protectedACGate{file: func(string, []byte) error { return errors.New("disk full") }}.check(in)
	if block || !strings.Contains(reason, "fix-the-lint-ac-7-console") || !strings.Contains(reason, "disk full") {
		t.Fatalf("check = (%q, %v); want a non-blocking reason naming the id and the fault", reason, block)
	}
}

func TestWithConsoleFollowupFiler_FilesNothingWhenTheGateBlocksOrHasNoInbox(t *testing.T) {
	calls := 0
	count := func(string, []byte) error { calls++; return nil }

	blocked := tddReport(t, cycle1788Table("predicate"))
	blocked.ProjectRoot = t.TempDir()
	protectedACGate{file: count}.check(blocked)

	rootless, _ := followupCycle(t, "fix-the-lint")
	rootless.ProjectRoot = ""
	protectedACGate{file: count}.check(rootless)

	if calls != 0 {
		t.Fatalf("filer called %d times; a blocked table or a missing project root files nothing", calls)
	}
}

func TestSourceOf_TheLaneItemLendsItsRankFieldsOnlyWhenItIsFiledWhole(t *testing.T) {
	for _, tc := range []struct {
		name  string
		filed map[string]any
		want  followupSource
	}{
		{"filed whole", map[string]any{"id": "x", "kind": "pipeline-repair", "priority_class": "stability", "weight": 0.9}, followupSource{ID: "x", Kind: "pipeline-repair", PriorityClass: "stability", Weight: 0.9}},
		{"filed without a class", map[string]any{"id": "x", "kind": "pipeline-repair", "weight": 0.9}, followupSource{ID: "x", Kind: "feature", PriorityClass: "correctness", Weight: 0.5}},
		{"filed with a mistyped weight", map[string]any{"id": "x", "kind": "pipeline-repair", "priority_class": "stability", "weight": "heavy"}, followupSource{ID: "x", Kind: "feature", PriorityClass: "correctness", Weight: 0.5}},
		{"not in the inbox", nil, followupSource{ID: "x", Kind: "feature", PriorityClass: "correctness", Weight: 0.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := core.ReviewInput{Cycle: 9, Workspace: t.TempDir(), ProjectRoot: t.TempDir()}
			writeJSON(t, filepath.Join(in.Workspace, "lane-scope.json"), map[string]any{"todo_ids": []string{"x"}})
			if tc.filed != nil {
				writeJSON(t, filepath.Join(in.ProjectRoot, ".evolve", "inbox", "x.json"), tc.filed)
			}
			if got := sourceOf(in); got != tc.want {
				t.Fatalf("sourceOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCompanion_ARowWithNoCriterionColumnOrProtectedPathStillMakesAValidItem(t *testing.T) {
	table := acMaterialization{disposition: 1, needs: 2, predicates: -1, criterion: -1}
	id, raw := table.companion(followupSource{ID: "x", Kind: "feature", PriorityClass: "correctness", Weight: 0.5}, 3, []string{"AC-2", "console-followup", "docs/a.md"})

	var it filedItem
	if err := json.Unmarshal(raw, &it); err != nil {
		t.Fatal(err)
	}
	if id != "x-ac-2-console" || it.ID != id {
		t.Fatalf("id = %q (item %q); an AC id that already starts with ac is not doubled", id, it.ID)
	}
	if len(it.Acceptance) != 1 || it.Acceptance[0] != "AC-2 | console-followup | docs/a.md" {
		t.Errorf("acceptance = %v; with no criterion column the whole row is the criterion", it.Acceptance)
	}
	if it.Files != nil {
		t.Errorf("files = %v; a row with no protected path names no files", it.Files)
	}
}

func TestWithConsoleFollowupFiler_AFilerThatReportsAlreadyFiledIsNotAFault(t *testing.T) {
	in, _ := followupCycle(t, "fix-the-lint")
	var calls []string
	var filer ConsoleFollowupFiler = func(root string, _ []byte) error {
		calls = append(calls, root)
		return fmt.Errorf("file: %w", inboxmover.ErrAlreadyFiled)
	}
	var opt Option = WithConsoleFollowupFiler(filer)
	rev := NewReviewer(config.StageEnforce, opt).(*reviewer)
	var logged []string
	rev.logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }

	res := rev.Review(context.Background(), in)

	if !res.Approve || len(calls) != 2 || calls[0] != in.ProjectRoot {
		t.Fatalf("review = %+v, filer calls = %v; want approval and one call per console-followup row at the project root", res, calls)
	}
	for _, line := range logged {
		if strings.Contains(line, "could not be filed") {
			t.Errorf("an already-filed companion is done, not a fault: %q", line)
		}
	}
}

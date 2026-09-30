package inboxstamps

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const (
	untouchedItem = ".evolve/inbox/untouched.json"
	retiredItem   = ".evolve/inbox/retired.json"
	curatedItem   = ".evolve/inbox/curated.json"
	contestedItem = ".evolve/inbox/contested.json"
	sourceFile    = "go/x.go"
)

func write(t *testing.T, repo *gittest.Repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fields(t *testing.T, repo *gittest.Repo, rel string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo.Dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return got
}

func routeStamp(id string) string {
	return `{"id":"` + id + `","weight":0.5,"route":"console-manual","routed_reason":"protected surface","failure_count":2}`
}

type world struct {
	plane, console *gittest.Repo
}

func newWorld(t *testing.T) world {
	t.Helper()
	origin := gittest.Bare(t)
	seed := gittest.Clone(t, origin.Dir)
	for _, id := range []string{"untouched", "retired", "curated", "contested"} {
		write(t, seed, ".evolve/inbox/"+id+".json", `{"id":"`+id+`","weight":0.5,"failure_count":1}`)
	}
	write(t, seed, sourceFile, "package x\n")
	seed.Git("add", "-A")
	seed.Git("commit", "-q", "-m", "seed")
	seed.Git("push", "-q", "origin", "main")
	return world{plane: gittest.Clone(t, origin.Dir), console: gittest.Clone(t, origin.Dir)}
}

func (w world) consoleLands(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(w.console.Dir, ".evolve", "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.console.Git("mv", retiredItem, ".evolve/inbox/consumed/retired.json")
	write(t, w.console, curatedItem, `{"id":"curated","weight":0.9,"failure_count":1}`)
	write(t, w.console, contestedItem, `{"id":"contested","weight":0.5,"failure_count":7}`)
	w.console.Git("commit", "-q", "-am", "console curates")
	w.console.Git("push", "-q", "origin", "main")
}

func (w world) planeStamps(t *testing.T) {
	t.Helper()
	for _, id := range []string{"untouched", "retired", "curated", "contested"} {
		write(t, w.plane, ".evolve/inbox/"+id+".json", routeStamp(id))
	}
}

func git(repo *gittest.Repo) gitexec.Git {
	return gitexec.Default(repo.Dir)
}

func TestClassify_AStampChangesOnlyFieldsTheMoverWrites(t *testing.T) {
	w := newWorld(t)
	w.planeStamps(t)
	write(t, w.plane, curatedItem, `{"id":"curated","weight":0.7,"failure_count":1}`)
	write(t, w.plane, sourceFile, "package x\n\nvar Dirty = 1\n")

	partition, err := Classify(context.Background(), git(w.plane))

	stampOf := func(path string) Stamp {
		return Stamp{Path: path, Set: map[string]json.RawMessage{
			"route": json.RawMessage(`"console-manual"`), "routed_reason": json.RawMessage(`"protected surface"`), "failure_count": json.RawMessage(`2`),
		}}
	}
	want := Partition{Stamps: []Stamp{stampOf(contestedItem), stampOf(retiredItem), stampOf(untouchedItem)}, Other: []string{curatedItem, sourceFile}}
	if err != nil || !reflect.DeepEqual(partition, want) {
		t.Errorf("Classify = (%+v, %v), want %+v: a stamp changes route, routed_* and failure_count only; the operator's weight edit and the source file are other dirt", partition, err, want)
	}
}

func TestClassify_ADeletedOrAddedInboxFileIsNotAStamp(t *testing.T) {
	w := newWorld(t)
	w.planeStamps(t)
	w.plane.Git("rm", "-q", "-f", retiredItem)
	write(t, w.plane, ".evolve/inbox/new.json", `{"id":"new","route":"console-manual"}`)
	w.plane.Git("add", ".evolve/inbox/new.json")
	write(t, w.plane, ".evolve/inbox/new.json", `{"id":"new","route":"console-manual","failure_count":1}`)

	partition, err := Classify(context.Background(), git(w.plane))

	if err != nil || !slices.Contains(partition.Other, retiredItem) || !slices.Contains(partition.Other, ".evolve/inbox/new.json") {
		t.Errorf("Classify = (%+v, %v), want the deletion and the added-then-edited file as other dirt", partition, err)
	}
}

func TestPlanAgainst_SortsEachStampByWhatOriginDidToItsItem(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.planeStamps(t)
	w.plane.Git("fetch", "-q", "origin")
	partition, err := Classify(context.Background(), git(w.plane))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	if err != nil {
		t.Fatal(err)
	}
	if got := Paths(plan.Kept); !slices.Equal(got, []string{untouchedItem}) {
		t.Errorf("kept = %v, want the item origin never touched", got)
	}
	if got := Paths(plan.Retired); !slices.Equal(got, []string{retiredItem}) {
		t.Errorf("retired = %v, want the item origin moved to consumed/", got)
	}
	if got := Paths(plan.Replay); !slices.Equal(got, []string{contestedItem, curatedItem}) {
		t.Errorf("replay = %v, want the items origin edited", got)
	}
	for _, stamp := range plan.Replay {
		if _, overwritesOrigin := stamp.Set["failure_count"]; stamp.Path == contestedItem && overwritesOrigin {
			t.Errorf("the replay of %s carries failure_count, which origin also changed: origin must win", contestedItem)
		}
	}
}

func TestPlanAgainst_AnAheadPlaneKeepsANewStampOnAnItemOriginNeverTouched(t *testing.T) {
	w := newWorld(t)
	write(t, w.plane, untouchedItem, `{"id":"untouched","weight":0.5,"failure_count":2}`)
	w.plane.Git("commit", "-q", "-am", keptMessage)
	write(t, w.plane, untouchedItem, `{"id":"untouched","weight":0.5,"failure_count":3}`)
	w.plane.Git("fetch", "-q", "origin")
	partition, err := Classify(context.Background(), git(w.plane))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	if err != nil || !slices.Equal(Paths(plan.Kept), []string{untouchedItem}) || len(plan.Retired)+len(plan.Replay) != 0 {
		t.Errorf("PlanAgainst = (%+v, %v), want the second bump kept: origin never touched the item since the merge base", plan, err)
	}
}

func TestPlan_LandsEveryStampAndOriginsEditsTogether(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.planeStamps(t)
	w.plane.Git("fetch", "-q", "origin")
	ctx, g := context.Background(), git(w.plane)
	partition, _ := Classify(ctx, g)
	plan, err := PlanAgainst(ctx, g, partition.Stamps, "origin/main")
	if err != nil {
		t.Fatal(err)
	}

	for _, step := range []func() error{
		func() error { return plan.Prepare(ctx, g) },
		func() error { return plan.CommitKept(ctx, g) },
		func() error { _, _, _, err := g.Capture(ctx, "merge", "-q", "--no-edit", "origin/main"); return err },
		func() error { return plan.ApplyReplay(w.plane.Dir) },
		func() error { return plan.CommitReplay(ctx, g) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}

	if got := w.plane.Git("status", "--porcelain", "--untracked-files=no"); got != "" {
		t.Errorf("status after = %q, want clean", got)
	}
	if got := fields(t, w.plane, curatedItem); got["weight"] != 0.9 || got["route"] != "console-manual" || got["failure_count"] != 2.0 {
		t.Errorf("%s = %v, want origin's weight 0.9 with the loop's route and failure_count replayed", curatedItem, got)
	}
	if got := fields(t, w.plane, contestedItem); got["failure_count"] != 7.0 || got["route"] != "console-manual" {
		t.Errorf("%s = %v, want origin's failure_count 7 (origin wins the field both changed) and the loop's route", contestedItem, got)
	}
	if got := fields(t, w.plane, untouchedItem); got["route"] != "console-manual" {
		t.Errorf("%s = %v, want the loop's stamp committed", untouchedItem, got)
	}
	if _, err := os.Stat(filepath.Join(w.plane.Dir, retiredItem)); !os.IsNotExist(err) {
		t.Errorf("%s still in the root (%v), want origin's retirement applied and the stamp dropped", retiredItem, err)
	}
}

func TestPlan_PrepareRestoresAStagedStampFromTheIndexToo(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.planeStamps(t)
	w.plane.Git("add", retiredItem)
	w.plane.Git("fetch", "-q", "origin")
	ctx, g := context.Background(), git(w.plane)
	partition, _ := Classify(ctx, g)
	plan, _ := PlanAgainst(ctx, g, partition.Stamps, "origin/main")

	if err := plan.Prepare(ctx, g); err != nil {
		t.Fatal(err)
	}

	if got := w.plane.Git("diff", "--cached", "--name-only"); strings.Contains(got, "retired.json") {
		t.Errorf("staged after Prepare = %q, want the retired item's stamp gone from the index too", got)
	}
}

func TestChangedOnRemote_ListsWhatOriginChangedSinceTheMergeBase(t *testing.T) {
	w := newWorld(t)
	write(t, w.plane, untouchedItem, `{"id":"untouched","failure_count":2}`)
	w.plane.Git("commit", "-q", "-am", keptMessage)
	w.consoleLands(t)
	w.plane.Git("fetch", "-q", "origin")

	changed, err := ChangedOnRemote(context.Background(), git(w.plane), "origin/main")

	if err != nil || changed[untouchedItem] || !changed[curatedItem] || !changed[retiredItem] {
		t.Errorf("ChangedOnRemote = (%v, %v), want origin's changes only, never the plane's own commit", changed, err)
	}
	for path := range changed {
		if !strings.HasPrefix(path, Pathspec) {
			t.Errorf("%s lies outside %s", path, Pathspec)
		}
	}
}

type scriptedGit struct {
	status  string
	failsOn string
}

func (g scriptedGit) Capture(_ context.Context, args ...string) (string, string, int, error) {
	switch {
	case args[0] == g.failsOn:
		return "", "boom", 1, nil
	case args[0] == "status":
		return g.status, "", 0, nil
	}
	return "", "", 0, nil
}

func TestClassifyAndPlan_NameTheGitCommandThatFailed(t *testing.T) {
	ctx := context.Background()
	stamp := Stamp{Path: untouchedItem, Set: map[string]json.RawMessage{"route": json.RawMessage(`"x"`)}}
	for failing, call := range map[string]func(Git) error{
		"status":     func(g Git) error { _, err := Classify(ctx, g); return err },
		"merge-base": func(g Git) error { _, err := PlanAgainst(ctx, g, []Stamp{stamp}, "origin/main"); return err },
		"restore":    func(g Git) error { return Plan{Retired: []Stamp{stamp}, prepared: []Stamp{stamp}}.Prepare(ctx, g) },
		"commit":     func(g Git) error { return Plan{Kept: []Stamp{stamp}}.CommitKept(ctx, g) },
		"diff":       func(g Git) error { _, err := ChangedOnRemote(ctx, g, "origin/main"); return err },
	} {
		err := call(scriptedGit{failsOn: failing})

		if err == nil || !strings.Contains(err.Error(), "git "+failing) || !strings.Contains(err.Error(), "boom") {
			t.Errorf("git %s failing: err = %v, want it named with its stderr", failing, err)
		}
	}
}

func TestClassify_ACaptureErrorIsReported(t *testing.T) {
	if _, err := Classify(context.Background(), erroringGit{}); err == nil || !strings.Contains(err.Error(), "git status") {
		t.Errorf("Classify = %v, want the status failure reported", err)
	}
}

type erroringGit struct{}

func (erroringGit) Capture(context.Context, ...string) (string, string, int, error) {
	return "", "", 0, errors.New("no git")
}

func TestPlan_RestorePutsBackEveryStampPrepareRemoved(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.planeStamps(t)
	w.plane.Git("fetch", "-q", "origin")
	ctx, g := context.Background(), git(w.plane)
	partition, _ := Classify(ctx, g)
	plan, _ := PlanAgainst(ctx, g, partition.Stamps, "origin/main")
	if err := plan.Prepare(ctx, g); err != nil {
		t.Fatal(err)
	}

	if err := plan.Restore(w.plane.Dir); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{retiredItem, curatedItem, contestedItem} {
		if got := fields(t, w.plane, rel); got["route"] != "console-manual" || got["failure_count"] != 2.0 {
			t.Errorf("%s = %v, want the loop's whole stamp back after a failed sync", rel, got)
		}
	}
}

func TestPlanAgainst_KeepsAStampOnAnItemAddedSinceTheMergeBase(t *testing.T) {
	w := newWorld(t)
	added := ".evolve/inbox/added.json"
	write(t, w.plane, added, `{"id":"added"}`)
	w.plane.Git("add", added)
	w.plane.Git("commit", "-q", "-m", "a plane-local item")
	write(t, w.plane, added, `{"id":"added","route":"console-manual"}`)
	w.plane.Git("fetch", "-q", "origin")
	partition, err := Classify(context.Background(), git(w.plane))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	if err != nil || !slices.Equal(Paths(plan.Kept), []string{added}) || len(plan.Retired) != 0 {
		t.Errorf("PlanAgainst = (%+v, %v), want the stamp kept: origin never had the item, so it cannot have retired it", plan, err)
	}
}

func TestClassify_TheMoversOwnStampOnAnItemWithHTMLCharactersIsAStamp(t *testing.T) {
	w := newWorld(t)
	raw := ".evolve/inbox/html.json"
	write(t, w.plane, raw, `{"id":"html","summary":"a <b> & c"}`)
	w.plane.Git("add", raw)
	w.plane.Git("commit", "-q", "-m", "an authored item")
	if err := inboxmover.UpdateItemJSON(filepath.Join(w.plane.Dir, raw), func(item map[string]json.RawMessage) {
		item["route"] = json.RawMessage(`"console-manual"`)
	}); err != nil {
		t.Fatal(err)
	}

	partition, err := Classify(context.Background(), git(w.plane))

	if err != nil || !slices.Equal(Paths(partition.Stamps), []string{raw}) || len(partition.Other) != 0 {
		t.Errorf("Classify = (%+v, %v), want the mover's route write a stamp although json.Marshal escaped the summary's <, > and &", partition, err)
	}
}

func TestPlanAgainst_AStampWhoseEveryFieldOriginChangedIsSuperseded(t *testing.T) {
	cases := []struct{ name, originItem string }{
		{name: "origin landed the same stamp by hand", originItem: routeStamp("untouched")},
		{name: "origin set its own values", originItem: `{"id":"untouched","weight":0.5,"route":"lane","routed_reason":"operator","failure_count":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			write(t, w.console, untouchedItem, tc.originItem)
			w.console.Git("commit", "-q", "-am", "origin writes every field the stamp sets")
			w.console.Git("push", "-q", "origin", "main")
			write(t, w.plane, untouchedItem, routeStamp("untouched"))
			w.plane.Git("fetch", "-q", "origin")
			partition, _ := Classify(context.Background(), git(w.plane))

			plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

			if err != nil || !slices.Equal(Paths(plan.Superseded), []string{untouchedItem}) || len(plan.Replay)+len(plan.Kept) != 0 {
				t.Errorf("PlanAgainst = (%+v, %v), want the stamp superseded: origin's values stand for every field it sets", plan, err)
			}
		})
	}
}

func TestPlanAgainst_ReplaysAStampOnAnItemOriginOnlyReformatted(t *testing.T) {
	w := newWorld(t)
	write(t, w.console, untouchedItem, "{\n  \"id\": \"untouched\",\n  \"weight\": 0.5,\n  \"failure_count\": 1\n}\n")
	w.console.Git("commit", "-q", "-am", "origin reformats the item")
	w.console.Git("push", "-q", "origin", "main")
	write(t, w.plane, untouchedItem, routeStamp("untouched"))
	w.plane.Git("fetch", "-q", "origin")
	partition, _ := Classify(context.Background(), git(w.plane))

	plan, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	if err != nil || !slices.Equal(Paths(plan.Replay), []string{untouchedItem}) || len(plan.Kept) != 0 {
		t.Errorf("PlanAgainst = (%+v, %v), want a replay: origin's bytes changed, so committing the stamp would conflict with the merge", plan, err)
	}
}

func TestPlan_AReplayRemovesAKeyTheStampRemovedUnlessOriginChangedIt(t *testing.T) {
	cases := []struct {
		name, originEdit string
		want             map[string]any
	}{
		{
			name:       "origin changed another field",
			originEdit: `{"id":"curated","weight":0.9,"failure_count":1,"routed_cycle":1785}`,
			want:       map[string]any{"id": "curated", "weight": 0.9, "failure_count": 1.0, "route": "lane"},
		},
		{
			name:       "origin changed the key the stamp removed",
			originEdit: `{"id":"curated","weight":0.5,"failure_count":1,"routed_cycle":1790}`,
			want:       map[string]any{"id": "curated", "weight": 0.5, "failure_count": 1.0, "route": "lane", "routed_cycle": 1790.0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := replayedOnto(t, tc.originEdit)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s = %v, want %v: the stamp's route lands, its removal of routed_cycle lands only where origin left routed_cycle alone", curatedItem, got, tc.want)
			}
		})
	}
}

func replayedOnto(t *testing.T, originEdit string) map[string]any {
	t.Helper()
	w := newWorld(t)
	write(t, w.plane, curatedItem, `{"id":"curated","weight":0.5,"failure_count":1,"routed_cycle":1785}`)
	w.plane.Git("commit", "-q", "-am", "a routed item")
	w.plane.Git("push", "-q", "origin", "main")
	w.console.Git("pull", "-q", "origin", "main")
	write(t, w.console, curatedItem, originEdit)
	w.console.Git("commit", "-q", "-am", "console curates")
	w.console.Git("push", "-q", "origin", "main")
	write(t, w.plane, curatedItem, `{"id":"curated","weight":0.5,"failure_count":1,"route":"lane"}`)
	w.plane.Git("fetch", "-q", "origin")
	ctx, g := context.Background(), git(w.plane)
	partition, _ := Classify(ctx, g)
	plan, err := PlanAgainst(ctx, g, partition.Stamps, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Prepare(ctx, g); err != nil {
		t.Fatal(err)
	}
	w.plane.Git("merge", "-q", "--no-edit", "origin/main")
	if err := plan.ApplyReplay(w.plane.Dir); err != nil {
		t.Fatal(err)
	}
	return fields(t, w.plane, curatedItem)
}

func TestPlan_RestoreAppliesEveryStampAndNamesEachItCouldNot(t *testing.T) {
	w := newWorld(t)
	w.consoleLands(t)
	w.planeStamps(t)
	w.plane.Git("fetch", "-q", "origin")
	ctx, g := context.Background(), git(w.plane)
	partition, _ := Classify(ctx, g)
	plan, _ := PlanAgainst(ctx, g, partition.Stamps, "origin/main")
	if err := plan.Prepare(ctx, g); err != nil {
		t.Fatal(err)
	}
	write(t, w.plane, contestedItem, `{not json`)

	err := plan.Restore(w.plane.Dir)

	if err == nil || !strings.Contains(err.Error(), contestedItem) {
		t.Errorf("Restore = %v, want the stamp it could not apply named", err)
	}
	for _, rel := range []string{curatedItem, retiredItem} {
		if got := fields(t, w.plane, rel); got["route"] != "console-manual" {
			t.Errorf("%s = %v, want its stamp restored although the stamp before it could not be", rel, got)
		}
	}
}

func TestPlanAgainst_AMalformedOriginCopyIsAnError(t *testing.T) {
	w := newWorld(t)
	write(t, w.console, untouchedItem, `{not json`)
	w.console.Git("commit", "-q", "-am", "a broken item")
	w.console.Git("push", "-q", "origin", "main")
	write(t, w.plane, untouchedItem, routeStamp("untouched"))
	w.plane.Git("fetch", "-q", "origin")
	partition, _ := Classify(context.Background(), git(w.plane))

	_, err := PlanAgainst(context.Background(), git(w.plane), partition.Stamps, "origin/main")

	if err == nil || !strings.Contains(err.Error(), "not a JSON object") {
		t.Errorf("PlanAgainst = %v, want a malformed origin copy reported, never read as a retirement", err)
	}
}

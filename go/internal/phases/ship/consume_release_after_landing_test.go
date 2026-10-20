package ship

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var landedBinding = continuation.Continuation{
	Worktree: "/tmp/evolve/worktrees/cycle-1484", Branch: "cycle-1484",
	SnapshotSHA: "9813bc621fe4aa0d55e1c0d3f0e1a2b3c4d5e6f7", BaseSHA: "d3c69cd2aa11bb22cc33dd44ee55ff6600112233", Cycle: 1484,
}

func releasedCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Released []continuation.Continuation `json:"released_continuations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return len(doc.Released)
}

func TestConsumeCommittedItems_KeepsTheBindingUntilTheLandingAndAReShipRecordsThePointerAgain(t *testing.T) {
	t.Parallel()
	const id = "continued-todo"
	root, workspace, itemBase := consumeFixture(t, id)
	if err := continuation.WriteRegistryEntry(root, id, landedBinding); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(root, ".evolve", "inbox", itemBase)
	original, err := os.ReadFile(item)
	if err != nil {
		t.Fatal(err)
	}
	opts := &Options{Class: ClassCycle, ProjectRoot: root, WorkspacePath: workspace, Runner: stubRunner}

	consumeCommittedItems(context.Background(), opts, &RunResult{}, "")

	if _, ok, err := continuation.ReadRegistryEntry(root, id); err != nil || !ok {
		t.Fatalf("consumption released the binding before the landing (ok=%v err=%v): a failed integrate loses the only copy outside ship's commit", ok, err)
	}
	consumed := filepath.Join(root, ".evolve", "inbox", "consumed", itemBase)
	if n := releasedCount(t, consumed); n != 1 {
		t.Fatalf("consumed item holds %d released_continuations, want the pointer once", n)
	}

	if err := os.Remove(consumed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(item, original, 0o644); err != nil {
		t.Fatal(err)
	}
	consumeCommittedItems(context.Background(), opts, &RunResult{}, "")

	if n := releasedCount(t, consumed); n != 1 {
		t.Errorf("the re-ship after a failed landing wrote %d released_continuations, want the pointer again", n)
	}
}

func landedRepo(t *testing.T, items map[string]string) (*gittest.Repo, *Options) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("README", "x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	for name, body := range items {
		repo.Write(".evolve/inbox/consumed/"+name, body)
	}
	repo.Write("go/x.go", "package x\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "ship")
	return repo, &Options{Class: ClassCycle, ProjectRoot: repo.Dir}
}

func consumedBody(t *testing.T, id string, released ...continuation.Continuation) string {
	t.Helper()
	doc := map[string]any{"id": id, "kind": "bug"}
	for _, c := range released {
		doc["released_continuations"] = continuation.AppendReleased(doc, c, "ship-consume-1507")
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw) + "\n"
}

func TestReleaseLandedBindings_ReleasesEachBindingThatTheLandedCommitRecords(t *testing.T) {
	t.Parallel()
	rebound := landedBinding
	rebound.Cycle = 1600
	repo, opts := landedRepo(t, map[string]string{
		"a-released.json": consumedBody(t, "released-todo", landedBinding),
		"b-rebound.json":  consumedBody(t, "rebound-todo", landedBinding),
		"c-plain.json":    consumedBody(t, "plain-todo"),
	})
	for scope, c := range map[string]continuation.Continuation{"released-todo": landedBinding, "rebound-todo": rebound, "live-sibling": landedBinding} {
		if err := continuation.WriteRegistryEntry(repo.Dir, scope, c); err != nil {
			t.Fatal(err)
		}
	}
	res := &RunResult{}

	releaseLandedBindings(context.Background(), opts, res, "HEAD")

	logs := strings.Join(res.Logs, "\n")
	if _, ok, _ := continuation.ReadRegistryEntry(repo.Dir, "released-todo"); ok {
		t.Errorf("the landed item's binding is still in the registry:\n%s", logs)
	}
	for _, scope := range []string{"rebound-todo", "live-sibling"} {
		if _, ok, _ := continuation.ReadRegistryEntry(repo.Dir, scope); !ok {
			t.Errorf("binding %q was released, but the landed commit does not own it:\n%s", scope, logs)
		}
	}
	if !strings.Contains(logs, `released continuation binding for "released-todo"`) || !strings.Contains(logs, `"rebound-todo": continuation binding was rebound`) {
		t.Errorf("logs do not name the release and the rebound binding:\n%s", logs)
	}

	again := &RunResult{}
	releaseLandedBindings(context.Background(), opts, again, "HEAD")
	if logs := strings.Join(again.Logs, "\n"); !strings.Contains(logs, `binding for "released-todo" is already released`) || strings.Contains(logs, `"released-todo": continuation binding was rebound`) {
		t.Errorf("a replayed settle must say the binding is already released, not rebound:\n%s", logs)
	}
}

func TestReleaseLandedBindings_AnUnreadableCommitOrItemReleasesNothingAndSaysSo(t *testing.T) {
	t.Parallel()
	repo, opts := landedRepo(t, map[string]string{"a-broken.json": "{not json\n"})
	if err := continuation.WriteRegistryEntry(repo.Dir, "a-broken", landedBinding); err != nil {
		t.Fatal(err)
	}
	res := &RunResult{}
	releaseLandedBindings(context.Background(), opts, res, "HEAD")
	releaseLandedBindings(context.Background(), opts, res, "no-such-commit")

	logs := strings.Join(res.Logs, "\n")
	if _, ok, _ := continuation.ReadRegistryEntry(repo.Dir, "a-broken"); !ok {
		t.Errorf("an unparseable landed item released a binding:\n%s", logs)
	}
	if !strings.Contains(logs, "a-broken.json") || !strings.Contains(logs, "no-such-commit") || strings.Count(logs, "binding NOT released") != 2 {
		t.Errorf("logs must name both faults and say the bindings stay:\n%s", logs)
	}
}

func scriptLandedConsumption(t *testing.T, r *argvRecorder, commit, id string) {
	t.Helper()
	const path = ".evolve/inbox/consumed/2026-09-26T10-17-00Z-landed.json"
	r.on("-c core.quotePath=false diff-tree -r --no-commit-id --name-only --diff-filter=A "+commit+"^ "+commit+" -- .evolve/inbox/consumed", scriptedCall{stdout: path + "\n"})
	r.on("show "+commit+":"+path, scriptedCall{stdout: consumedBody(t, id, landedBinding)})
}

func TestShipSites_EachSuccessfulLandingReleasesTheBindingItsCommitRecords(t *testing.T) {
	t.Parallel()
	sites := append([]pushSiteDriver{}, pushSites...)
	sites = append(sites, pushSiteDriver{name: "worktree", drive: func(t *testing.T, opts *Options, r *argvRecorder, res *RunResult) error {
		wt := filepath.Join(opts.ProjectRoot, "wt")
		scriptGreenLanding(r, wt)
		s := newWorktreeShip(context.Background(), opts, res, pinBranch, wt)
		s.cycleBranch = pinCycleBranch
		return s.land("")
	}})
	commits := map[string]string{"direct": "HEAD", "pushonly": pinHead, "worktree": pinHead}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			opts, _, _ := pinOptions(t, ClassCycle)
			r := newArgvRecorder(opts)
			scriptGreenPost(r)
			scriptLandedConsumption(t, r, commits[site.name], "landed-todo")
			if err := continuation.WriteRegistryEntry(opts.ProjectRoot, "landed-todo", landedBinding); err != nil {
				t.Fatal(err)
			}
			res := &RunResult{}
			if err := site.drive(t, opts, r, res); err != nil {
				t.Fatalf("drive: %v\n%s", err, strings.Join(res.Logs, "\n"))
			}
			if _, ok, _ := continuation.ReadRegistryEntry(opts.ProjectRoot, "landed-todo"); ok {
				t.Errorf("the landing left the binding in the registry:\n%s", strings.Join(res.Logs, "\n"))
			}
		})
	}
}

func TestShipSites_APushThatFailsKeepsTheBinding(t *testing.T) {
	t.Parallel()
	opts, _, _ := pinOptions(t, ClassManual)
	r := newArgvRecorder(opts)
	r.on("diff --cached --quiet", scriptedCall{exit: 1})
	r.set("push origin "+pinBranch, scriptedCall{exit: 1}, scriptedCall{exit: 1})
	r.on("rev-parse origin/"+pinBranch, scriptedCall{stdout: pinOriginRef + "\n"})
	r.on("merge-base --is-ancestor "+pinOriginRef+" HEAD", scriptedCall{exit: 1})
	scriptLandedConsumption(t, r, "HEAD", "landed-todo")
	if err := continuation.WriteRegistryEntry(opts.ProjectRoot, "landed-todo", landedBinding); err != nil {
		t.Fatal(err)
	}
	if err := shipDirect(context.Background(), opts, &RunResult{}, pinBranch); err == nil {
		t.Fatal("a diverged push must fail")
	}
	if _, ok, _ := continuation.ReadRegistryEntry(opts.ProjectRoot, "landed-todo"); !ok {
		t.Error("a ship whose push failed released the binding: the re-ship has nothing to re-read")
	}
}

func TestReleaseLandedBindings_AnUnreadableRegistryIsLoggedAndReleasesNothing(t *testing.T) {
	t.Parallel()
	repo, opts := landedRepo(t, map[string]string{"a.json": consumedBody(t, "landed-todo", landedBinding)})
	registry := continuation.RegistryPath(repo.Dir)
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &RunResult{}
	releaseLandedBindings(context.Background(), opts, res, "HEAD")
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, `"landed-todo": continuation binding release failed`) {
		t.Errorf("logs do not report the failed release:\n%s", logs)
	}
}

func TestReleaseLandedBindings_AnItemThatGitCannotShowReleasesNothing(t *testing.T) {
	t.Parallel()
	opts, _, _ := pinOptions(t, ClassCycle)
	r := newArgvRecorder(opts)
	const path = ".evolve/inbox/consumed/gone.json"
	r.on("-c core.quotePath=false diff-tree -r --no-commit-id --name-only --diff-filter=A "+pinHead+"^ "+pinHead+" -- .evolve/inbox/consumed", scriptedCall{stdout: path + "\n"})
	r.on("show "+pinHead+":"+path, scriptedCall{exit: 128})
	if err := continuation.WriteRegistryEntry(opts.ProjectRoot, "gone", landedBinding); err != nil {
		t.Fatal(err)
	}
	res := &RunResult{}
	releaseLandedBindings(context.Background(), opts, res, pinHead)
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "landed item "+path+": unreadable (rc=128") {
		t.Errorf("logs do not report the unreadable item:\n%s", logs)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(opts.ProjectRoot, "gone"); !ok {
		t.Error("an unreadable landed item released a binding")
	}
}

package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/landedclaim"
)

type landedFixture struct {
	root, ws, onMain, offMain, inbox string
	repo                             *gittest.Repo
}

func newLandedFixture(t *testing.T, files []string, decision func(onMain, offMain string) string) landedFixture {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("seed.txt", "seed\n")
	r.Git("add", "seed.txt")
	r.Git("commit", "-q", "-m", "seed")
	r.Write("go/pkg/a.go", "package pkg\n")
	r.Git("add", "go/pkg/a.go")
	r.Git("commit", "-q", "-m", "the delivery")
	f := landedFixture{repo: r, root: r.Dir, onMain: strings.TrimSpace(r.Git("rev-parse", "HEAD"))}
	r.Git("checkout", "-q", "-b", "lane")
	r.Write("go/pkg/b.go", "package pkg\n")
	r.Git("add", "go/pkg/b.go")
	r.Git("commit", "-q", "-m", "lane only")
	f.offMain = strings.TrimSpace(r.Git("rev-parse", "HEAD"))
	r.Git("checkout", "-q", "main")
	f.inbox = filepath.Join(f.root, ".evolve", "inbox")
	f.ws = filepath.Join(f.root, ".evolve", "runs", "cycle-7")
	for _, dir := range []string{f.inbox, f.ws} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item, _ := json.Marshal(map[string]any{"id": "item", "title": "fixture", "kind": "bug", "files": files})
	if err := os.WriteFile(filepath.Join(f.inbox, "2026-09-26T00-00-00Z-item.json"), item, 0o644); err != nil {
		t.Fatal(err)
	}
	pin, _ := json.Marshal(map[string]any{"todo_ids": []string{"item"}, "goal_hash": "g"})
	if err := os.WriteFile(filepath.Join(f.ws, "lane-scope.json"), pin, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.ws, "triage-decision.json"), []byte(decision(f.onMain, f.offMain)), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f landedFixture) apply(t *testing.T, check landedclaim.Check) NoWorkResult {
	t.Helper()
	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: f.root, Workspace: f.ws, Cycle: 7, Stderr: io.Discard, LandedCheck: check})
	if err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	return res
}

func shippedAt(sha func(onMain, offMain string) string) func(string, string) string {
	return func(onMain, offMain string) string {
		return `{"top_n":[],"skip_shipped":[{"task_id":"item","git_sha":"` + sha(onMain, offMain) + `"}]}`
	}
}

func onMainSHA(onMain, _ string) string   { return onMain }
func offMainSHA(_, offMain string) string { return offMain }

func TestApplyNoWork_ALandedClaimOnTheItemSurfaceStampsTheShaAndRoutesNothing(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))

	res := f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

	rec := itemRecord(t, f.inbox, "item")
	if rec["landed_sha"] != f.onMain || rec["route"] != nil || rec["routed_reason"] != nil {
		t.Errorf("a checked claim stamps landed_sha in place and routes nothing: %v", rec)
	}
	if len(res.Routed) != 0 || len(res.Retired) != 0 {
		t.Errorf("result = %+v, want nothing routed and nothing retired", res)
	}
	if _, err := os.Stat(filepath.Join(f.inbox, "consumed")); !os.IsNotExist(err) {
		t.Errorf("ApplyNoWork moved an inbox file to consumed/ (stat err=%v)", err)
	}
}

func TestApplyNoWork_ACommitNotOnMainRoutesTheItem(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/b.go"}, shippedAt(offMainSHA))

	res := f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

	rec := itemRecord(t, f.inbox, "item")
	if rec["route"] != "console-manual" || rec["landed_sha"] != nil || !slices.Equal(res.Routed, []string{"item"}) {
		t.Errorf("a commit off main routes the item, unstamped: %v routed=%v", rec, res.Routed)
	}
}

func TestApplyNoWork_ACommitOffTheItemSurfaceRoutesTheItem(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/other.go"}, shippedAt(onMainSHA))

	res := f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

	if rec := itemRecord(t, f.inbox, "item"); rec["route"] != "console-manual" || rec["landed_sha"] != nil || len(res.Routed) != 1 {
		t.Errorf("a commit that misses every declared path routes the item: %v", rec)
	}
}

func TestApplyNoWork_AGitErrorRoutesTheItem(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))

	res := f.apply(t, landedclaim.OnMain(gitexec.Default(t.TempDir())))

	if rec := itemRecord(t, f.inbox, "item"); rec["route"] != "console-manual" || rec["landed_sha"] != nil || len(res.Routed) != 1 {
		t.Errorf("a git error fails closed and routes the item: %v", rec)
	}
}

func TestApplyNoWork_WithNoCheckWiredTheClaimRoutesTheItem(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))

	res := f.apply(t, nil)

	if rec := itemRecord(t, f.inbox, "item"); rec["route"] != "console-manual" || rec["landed_sha"] != nil || len(res.Routed) != 1 {
		t.Errorf("no wired check means no claim is trusted: %v", rec)
	}
}

func TestApplyNoWork_ALandedClaimIsNotAHandoff(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))

	f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

	if committedset.HandedOff(f.ws) {
		t.Error("a checked landed claim counted as a no-work hand-off")
	}
}

func TestApplyNoWork_AnAlreadyShippedDropNamesItsSha(t *testing.T) {
	for name, reason := range map[string]string{
		"the sha follows the tag":    "already-shipped: ",
		"extra words follow the sha": "Already-Shipped:  ",
	} {
		t.Run(name, func(t *testing.T) {
			f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(onMain, _ string) string {
				return `{"top_n":[],"dropped":[{"id":"item","reason":"` + reason + onMain + ` (the delivery)"}]}`
			})

			res := f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

			if rec := itemRecord(t, f.inbox, "item"); rec["landed_sha"] != f.onMain || rec["route"] != nil || len(res.Routed) != 0 {
				t.Errorf("a dropped already-shipped answer names its sha: %v", rec)
			}
		})
	}
}

func TestApplyNoWork_AnAlreadyShippedDropWithoutAShaRoutesTheItem(t *testing.T) {
	for name, reason := range map[string]string{"no colon": "already-shipped", "empty token": "already-shipped:  ", "another tag": "stale: " + strings.Repeat("a", 40)} {
		t.Run(name, func(t *testing.T) {
			f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(string, string) string {
				return `{"top_n":[],"dropped":[{"id":"item","reason":"` + reason + `"}]}`
			})

			res := f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

			if rec := itemRecord(t, f.inbox, "item"); rec["route"] != "console-manual" || rec["landed_sha"] != nil || len(res.Routed) != 1 {
				t.Errorf("a drop with no sha token claims nothing: %v", rec)
			}
		})
	}
}

func TestApplyNoWork_TheNoWorkRouteIsStampedAsTheLoops(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(offMainSHA))

	f.apply(t, landedclaim.OnMain(gitexec.Default(f.root)))

	rec := itemRecord(t, f.inbox, "item")
	if rec["routed_by"] != inboxbatch.RoutedByLoop || !strings.HasPrefix(rec["routed_reason"].(string), inboxbatch.NoWorkRoutePrefix) {
		t.Errorf("the no-work route carries routed_by loop and the shared reason prefix: %v", rec)
	}
}

func protectedOnly(allowed ...string) func(string) bool {
	return func(p string) bool { return slices.Contains(allowed, p) }
}

func (f landedFixture) applyProtected(t *testing.T, pred func(string) bool) {
	t.Helper()
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: f.root, Workspace: f.ws, Cycle: 7, Stderr: io.Discard, Protected: pred}); err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
}

func TestApplyNoWork_AProtectedDropStampsTheConfirmedPath(t *testing.T) {
	const path = "go/internal/guards/integrity_surface.go"
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(string, string) string {
		return `{"top_n":[],"dropped":[{"id":"item","reason":"protected-surface: ` + path + ` is inside the fence"}]}`
	})

	f.applyProtected(t, protectedOnly(path))

	rec := itemRecord(t, f.inbox, "item")
	if got, _ := rec["protected_files"].([]any); len(got) != 1 || got[0] != path {
		t.Fatalf("protected_files = %v, want the confirmed path", rec["protected_files"])
	}
	if files, _ := rec["files"].([]any); len(files) != 1 || files[0] != "go/pkg/a.go" {
		t.Errorf("files = %v: the curator-owned field must stay as it was", rec["files"])
	}
	body, _ := json.Marshal(rec)
	var it inboxbatch.Item
	if err := json.Unmarshal(body, &it); err != nil {
		t.Fatal(err)
	}
	it.Route = ""
	if routed, why := inboxbatch.ConsoleRouted(it, protectedOnly(path)); !routed || !strings.HasPrefix(why, "protected fix surface: "+path) {
		t.Errorf("ConsoleRouted = %v, %q, want the protected fix surface fact", routed, why)
	}
}

func TestApplyNoWork_AnEscalatedProtectedCardStampsThePath(t *testing.T) {
	const path = "go/internal/guards/integrity_surface.go"
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(string, string) string {
		return `{"top_n":[],"escalate_block":[{"task_id":"item","reason":"protected-surface: ` + path + `"}]}`
	})

	f.applyProtected(t, protectedOnly(path))

	if got, _ := itemRecord(t, f.inbox, "item")["protected_files"].([]any); len(got) != 1 || got[0] != path {
		t.Errorf("protected_files = %v, want the escalated path", got)
	}
}

func TestApplyNoWork_APathThePredicateDeniesIsNotStamped(t *testing.T) {
	for name, tc := range map[string]struct {
		reason string
		pred   func(string) bool
	}{
		"the predicate denies the path": {"protected-surface: go/pkg/free.go", protectedOnly()},
		"no predicate is wired":         {"protected-surface: go/internal/guards/x.go", nil},
		"no path follows the tag":       {"protected-surface:", protectedOnly("")},
		"another tag":                   {"out-of-scope: go/internal/guards/x.go", protectedOnly("go/internal/guards/x.go")},
	} {
		t.Run(name, func(t *testing.T) {
			f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(string, string) string {
				return `{"top_n":[],"dropped":[{"id":"item","reason":"` + tc.reason + `"}]}`
			})

			f.applyProtected(t, tc.pred)

			rec := itemRecord(t, f.inbox, "item")
			if rec["protected_files"] != nil || rec["route"] != "console-manual" {
				t.Errorf("an unconfirmed path is not stamped and the item still routes: %v", rec)
			}
		})
	}
}

func TestStampLanded_AnItemThatCannotBeReadOrParsedIsNotStamped(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))
	always := func(string, []string) bool { return true }
	in := NoWorkInputs{ProjectRoot: f.root, LandedCheck: always}
	path := filepath.Join(f.inbox, "2026-09-26T00-00-00Z-item.json")

	if stampLanded(in, "absent", f.onMain) {
		t.Error("an id with no inbox item was stamped")
	}
	if stampLanded(NoWorkInputs{}, "item", f.onMain) || stampLanded(in, "item", "") {
		t.Error("no check or no sha was stamped")
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stampLanded(in, "item", f.onMain) {
		t.Error("an unparsable item was stamped")
	}
}

func TestLandedClaimsOf_ARunWithNoDecisionClaimsNothing(t *testing.T) {
	if got := landedClaimsOf(t.TempDir()); got != nil {
		t.Errorf("claims = %v, want none", got)
	}
}

func TestApplyNoWork_AProtectedStampThatFailsIsReported(t *testing.T) {
	const path = "go/internal/guards/x.go"
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, func(string, string) string {
		return `{"top_n":[],"dropped":[{"id":"item","reason":"protected-surface: ` + path + `"}]}`
	})
	if err := os.Chmod(f.inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(f.inbox, 0o755)

	_, err := ApplyNoWork(NoWorkInputs{ProjectRoot: f.root, Workspace: f.ws, Cycle: 7, Stderr: io.Discard, Protected: protectedOnly(path)})

	if err == nil && os.Geteuid() != 0 {
		t.Error("a failed protected stamp was swallowed")
	}
}

func TestApplyNoWork_TheLandedCheckSeesTheProtectedFilesOfTheItem(t *testing.T) {
	f := newLandedFixture(t, []string{"go/pkg/a.go"}, shippedAt(onMainSHA))
	path := filepath.Join(f.inbox, "2026-09-26T00-00-00Z-item.json")
	item, _ := json.Marshal(map[string]any{"id": "item", "title": "fixture", "kind": "bug", "files": []string{"go/pkg/a.go"}, "protected_files": []string{"go/internal/guards/role.go"}})
	if err := os.WriteFile(path, item, 0o644); err != nil {
		t.Fatal(err)
	}
	var asked []string
	f.apply(t, func(sha string, paths []string) bool {
		asked = append(asked, strings.Join(paths, ","))
		return true
	})

	if !slices.Equal(asked, []string{"go/pkg/a.go,go/internal/guards/role.go"}) {
		t.Errorf("the landed check saw %v, want the files and the protected files of the item", asked)
	}
}

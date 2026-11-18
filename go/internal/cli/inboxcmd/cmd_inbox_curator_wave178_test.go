package inboxcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const wave178Testdata = "../../inboxcurator/testdata/wave178"

const builderProfile = "../../../../.evolve/profiles/builder.json"

const shippedStampSHA = "c6462ce03"

const shippedFixPath = "go/internal/bridge/driver_tmux_submitverify.go"

var treeAtWave179 = []string{
	"go/internal/core/boot_preflight.go", "go/internal/repostate/doc.go",
	"go/internal/modelquery/picker.go", "go/internal/modelquery/recipe.go", "go/internal/modelquery/effort.go",
	"go/internal/bridge/doc.go", "go/internal/bridge/manifests/doc.go", "go/internal/bridge/driver_common.go",
	"agents/evolve-router.md", "go/internal/bridgechain/bridgechain.go",
	"go/internal/inboxmover/dispatchstate.go", "go/internal/inboxmover/holder.go",
	"go/cmd/evolve/cmd_inbox_claims.go", "go/cmd/evolve/cmd_gc.go",
	"docs/operations/runtime-reference.md", "go/internal/core/resume.go", "go/internal/cli/agentcmd/verdict.go",
	"go/internal/phases/ship/doc.go", "go/internal/events/doc.go", "docs/architecture/event-channels.md",
	"docs/conventions/code-comments.md", "skills/commit/SKILL.md", "go/internal/ciparity/gatesteps.go",
	"docs/architecture/packages/internal-phases-ship.md", "docs/architecture/packages/internal-ciparity.md",
	"go/internal/core/doc.go", "docs/architecture/checkpoint-resume.md",
}

func wave178Records(t *testing.T, set string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(wave178Testdata, set, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("fixture %s: %v (%d files)", set, err, len(files))
	}
	out := map[string]string{}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = string(body)
	}
	return out
}

func decodeReport(t *testing.T, stdout string) curateReport {
	t.Helper()
	var report curateReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("report: %v\n%s", err, stdout)
	}
	return report
}

func laneReroutes(t *testing.T, stdout string) []string {
	t.Helper()
	report := decodeReport(t, stdout)
	var ids []string
	for _, p := range report.Proposals {
		if p.Kind == "reroute" && p.Target == "lane" {
			ids = append(ids, p.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func proposalKinds(t *testing.T, stdout, id string) []string {
	t.Helper()
	var kinds []string
	for _, p := range decodeReport(t, stdout).Proposals {
		if p.ID == id {
			kinds = append(kinds, string(p.Kind))
		}
	}
	return kinds
}

func mirrorWave179Tree(t *testing.T, f curatorFixture) string {
	t.Helper()
	profile, err := os.ReadFile(builderProfile)
	if err != nil {
		t.Fatalf("the builder profile that the production lane fence reads: %v", err)
	}
	f.repo.Write(".evolve/profiles/builder.json", string(profile))
	for _, p := range treeAtWave179 {
		f.repo.Write(p, "x\n")
	}
	f.repo.Git("add", ".")
	f.repo.Git("commit", "-q", "-m", "base tree")
	f.repo.Write(shippedFixPath, "package bridge\n\nconst submitVerifyMinEchoRunes = 8\n")
	f.repo.Git("add", "go")
	f.repo.Git("commit", "-q", "-m", "fix: floor the submit verify at eight runes")
	return f.repo.Git("rev-parse", "--short=9", "HEAD")
}

func fileWave178(t *testing.T, f curatorFixture, shippedSHA string) []string {
	t.Helper()
	var ids []string
	for _, set := range []string{"kept", "bounced"} {
		for id, body := range wave178Records(t, set) {
			f.repo.Write(".evolve/inbox/2026-11-01T00-00-00Z-"+id+".json", strings.ReplaceAll(body, shippedStampSHA, shippedSHA))
			ids = append(ids, id)
		}
	}
	f.repo.Git("add", ".")
	f.repo.Git("commit", "-q", "-m", "file the wave 178 records")
	return ids
}

func TestCmd_InboxCurate_TheWave178BoundaryThroughTheProductionCuratorProposesTheNineKeptAndNoneOfTheSeven(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "shadow")
	sha := mirrorWave179Tree(t, f)
	fileWave178(t, f, sha)

	rc, stdout, stderr := f.curate(t, "", "--apply", "--json")

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr)
	}
	var kept []string
	for id := range wave178Records(t, "kept") {
		kept = append(kept, id)
	}
	slices.Sort(kept)
	got := laneReroutes(t, stdout)
	if len(kept) != 9 {
		t.Fatalf("fixture = %d kept items, want 9", len(kept))
	}
	var lane []string
	for _, id := range got {
		if id != "rolling-pool" {
			lane = append(lane, id)
		}
	}
	if !slices.Equal(lane, kept) {
		t.Errorf("lane reroutes = %v\nwant the 9 that lane triage kept = %v\nthe production curator, with no injected hook, must derive the freshness and the fix surface of the other 7 itself", lane, kept)
	}
}

func TestCmd_InboxCurate_AStaleItemOfTheWave178BoundaryGetsAConsumeOrQuarantineProposalNotAReroute(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "shadow")
	sha := mirrorWave179Tree(t, f)
	fileWave178(t, f, sha)

	_, stdout, _ := f.curate(t, "", "--apply", "--json")

	for _, id := range []string{"tmux-submit-verify-eight-rune-floor", "gate-child-env-allowlist", "main-red-stop-provable-means-a-lane-can-prove-it"} {
		kinds := proposalKinds(t, stdout, id)
		if slices.Contains(kinds, "reroute") {
			t.Errorf("%s: proposals = %v, want no reroute for an item whose fix is on main or whose premise is absent", id, kinds)
		}
		if !slices.Contains(kinds, "consume") && !slices.Contains(kinds, "quarantine") {
			t.Errorf("%s: proposals = %v, want a consume or quarantine proposal that carries the freshness verdict", id, kinds)
		}
	}
}

const verdictItem = `{"id":"%s","kind":"bug","origin":"console","priority_class":"correctness","weight":0.5,"title":"t",` +
	`"route":"console-manual","routed_by":"loop","routed_reason":%q,"files":[%q],"summary":"s","fix":"f","acceptance":["a"]}`

func curateVerdictItem(t *testing.T, id, file, reason string, tree ...string) []string {
	t.Helper()
	f := newCuratorFixture(t, "shadow")
	for _, p := range tree {
		f.repo.Write(p, "x\n")
	}
	f.repo.Write(".evolve/inbox/2026-11-01T00-00-00Z-"+id+".json", fmt.Sprintf(verdictItem, id, reason, file))
	f.repo.Git("add", ".")
	f.repo.Git("commit", "-q", "-m", "file")
	_, stdout, _ := f.curate(t, "", "--apply", "--json")
	return proposalKinds(t, stdout, id)
}

func TestCmd_InboxCurate_ANamedPremiseThatIsStillAbsentOnTheTreeStopsTheReroute(t *testing.T) {
	t.Parallel()
	kinds := curateVerdictItem(t, "premise-absent", "go/internal/newpkg/",
		"lane triage (cycle 9) dropped: stale: go/internal/newpkg/ absent at HEAD; package only on unmerged commits")

	if slices.Contains(kinds, "reroute") {
		t.Errorf("proposals = %v, want no reroute while the premise stays absent at HEAD", kinds)
	}
}

func TestCmd_InboxCurate_ANamedPremiseThatLandedOnTheTreeSinceTheVerdictIsRerouted(t *testing.T) {
	t.Parallel()
	kinds := curateVerdictItem(t, "premise-landed", "go/internal/newpkg/",
		"lane triage (cycle 9) dropped: stale: go/internal/newpkg/ absent at HEAD; package only on unmerged commits",
		"go/internal/newpkg/doc.go")

	if !slices.Contains(kinds, "reroute") {
		t.Errorf("proposals = %v, want the lane reroute back: the premise is on the tree now, so the old stale verdict no longer holds", kinds)
	}
}

func TestCmd_InboxCurate_AnAlreadyShippedVerdictWhoseCommitIsNotOnMainIsNotTrusted(t *testing.T) {
	t.Parallel()
	kinds := curateVerdictItem(t, "shipped-unproven", "go/internal/fleet/pool.go",
		"lane triage (cycle 9) dropped: already-shipped: deadbeef1")

	if !slices.Contains(kinds, "reroute") {
		t.Errorf("proposals = %v, want the lane reroute: a commit that main does not hold proves nothing", kinds)
	}
}

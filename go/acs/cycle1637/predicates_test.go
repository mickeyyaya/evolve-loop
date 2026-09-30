//go:build acs

package cycle1637

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const thisCycle = 1637

func threeItems() []inboxbatch.Item {
	return []inboxbatch.Item{
		{ID: "alpha", Title: "alpha", Weight: 0.8, Files: []string{"go/internal/alpha/a.go"}, Acceptance: []string{"alpha keeps its own AC"}},
		{ID: "beta", Title: "beta", Weight: 0.7, Files: []string{"go/internal/beta/b.go"}, Acceptance: []string{"beta keeps its own AC"}},
		{ID: "gamma", Title: "gamma", Weight: 0.6, Files: []string{"go/internal/gamma/c.go"}, Acceptance: []string{"gamma keeps its own AC"}},
	}
}

func completeCommitment(items []inboxbatch.Item) inboxbatch.UnifiedCommitment {
	members := make([]inboxbatch.UnifiedMember, 0, len(items))
	for _, it := range items {
		members = append(members, inboxbatch.UnifiedMember{
			ID:       it.ID,
			Evidence: it.ID + " re-implements the retry loop in " + it.Files[0] + " — one of N copies of the same bug",
		})
	}
	return inboxbatch.UnifiedCommitment{
		RootCauseHypothesis: "N callers each hand-roll the same retry/backoff loop; every member is one copy of that disease",
		SharedSeam:          "go/internal/retry (single Retrier with policy injection)",
		DesignRequirements:  []string{"single source with projection", "Strategy over flags", "immutable policy values"},
		Members:             members,
	}
}

func decisionDoc(t *testing.T, topN []string, extra map[string]any) string {
	t.Helper()
	top := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		top = append(top, map[string]string{"id": id})
	}
	doc := map[string]any{"cycle": thisCycle, "top_n": top}
	for k, v := range extra {
		doc[k] = v
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func reportMD(topN []string) string {
	var b strings.Builder
	b.WriteString("cycle_size_estimate: medium\ndeliverable_kind: code\n\n## top_n\n")
	for _, id := range topN {
		fmt.Fprintf(&b, "- %s: close %s, files=go/internal/%s/x.go\n", id, id, id)
	}
	b.WriteString("\n## deferred\n- none\n\n## dropped\n- none\n")
	return b.String()
}

type fakeBridge struct{ report, decision string }

func (b *fakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755); err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(req.ArtifactPath, []byte(b.report), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	dec := filepath.Join(filepath.Dir(req.ArtifactPath), "triage-decision.json")
	if err := os.WriteFile(dec, []byte(b.decision), 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{Stdout: b.report}, nil
}

func (b *fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type placement map[string]string

const inboxRootDir = ""

func writeInbox(t *testing.T, items []inboxbatch.Item, where placement) string {
	t.Helper()
	proj := t.TempDir()
	for _, it := range items {
		dir := filepath.Join(proj, ".evolve", "inbox", where[it.ID])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(it)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, it.ID+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return proj
}

func runTriageAt(t *testing.T, items []inboxbatch.Item, where placement, topN []string, extra map[string]any) (core.PhaseResponse, string, string) {
	t.Helper()
	proj := writeInbox(t, items, where)
	ws := t.TempDir()
	fb := &fakeBridge{report: reportMD(topN), decision: decisionDoc(t, topN, extra)}
	ph := triage.New(triage.Config{
		Bridge: fb,
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-triage.md": &fstest.MapFile{Data: []byte("---\nname: evolve-triage\n---\ntriage body")},
		}),
	})
	resp, err := ph.Run(context.Background(), core.PhaseRequest{Cycle: thisCycle, ProjectRoot: proj, Workspace: ws})
	if err != nil {
		t.Fatalf("triage runner: %v", err)
	}
	return resp, proj, ws
}

func runTriage(t *testing.T, items []inboxbatch.Item, topN []string, c *inboxbatch.UnifiedCommitment) (core.PhaseResponse, string, string) {
	t.Helper()
	var extra map[string]any
	if c != nil {
		extra = map[string]any{"unified_commitment": c}
	}
	return runTriageAt(t, items, placement{}, topN, extra)
}

func digestTriage(t *testing.T, ws string) router.RoutingSignals {
	t.Helper()
	sig, err := router.Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("router.Digest: %v", err)
	}
	return sig
}

func hasDiagMentioning(diags []core.Diagnostic, needle string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func memberIDs(items []inboxbatch.Item) []string {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func assertRejectedFailOpen(t *testing.T, resp core.PhaseResponse, ws string, wantCommitted int, mustMention string) {
	t.Helper()
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("a rejected synthesis claim must fail OPEN to the independent top_n (verdict PASS); got %q diags=%v", resp.Verdict, resp.Diagnostics)
	}
	if !hasDiagMentioning(resp.Diagnostics, "unified_commitment") {
		t.Errorf("fail-open must be LOUD: expected a diagnostic mentioning unified_commitment; got %v", resp.Diagnostics)
	}
	if mustMention != "" && !hasDiagMentioning(resp.Diagnostics, mustMention) {
		t.Errorf("the rejection diagnostic must name %q; got %v", mustMention, resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "" || sig.Triage.UnifiedMemberCount != 0 {
		t.Errorf("router must see NO unified signal after a rejected claim; got UnifiedSize=%q UnifiedMemberCount=%d", sig.Triage.UnifiedSize, sig.Triage.UnifiedMemberCount)
	}
	if sig.Triage.CommittedCount != wantCommitted {
		t.Errorf("independent top_n must survive fail-open: CommittedCount=%d, want %d", sig.Triage.CommittedCount, wantCommitted)
	}
	if _, err := os.Stat(filepath.Join(ws, "campaign-plan.json")); err == nil {
		t.Error("a rejected claim must not leave a campaign-plan.json behind")
	}
}

func TestC1637_001_UnifiedCommitmentRejectsMixedCampaignMembership(t *testing.T) {
	cases := []struct {
		name      string
		campaigns []string
	}{
		{"unscoped first, campaign second", []string{"", "pipeline-integrity", ""}},
		{"campaign first, unscoped second", []string{"pipeline-integrity", "", ""}},
		{"one unscoped among two campaign members", []string{"retry-2026-08", "", "retry-2026-08"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := threeItems()
			for i := range items {
				items[i].Campaign = tc.campaigns[i]
			}
			c := completeCommitment(items)
			err := c.Validate(items)
			if err == nil {
				t.Fatalf("%s: a commitment mixing unscoped and campaign-scoped members must be rejected as heterogeneous (campaigns=%v)", tc.name, tc.campaigns)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "campaign") {
				t.Errorf("%s: the rejection must state the campaign-scope reason; got %v", tc.name, err)
			}
		})
	}
	t.Run("all unscoped is homogeneous", func(t *testing.T) {
		items := threeItems()
		if err := completeCommitment(items).Validate(items); err != nil {
			t.Fatalf("all-unscoped members must validate; got %v", err)
		}
	})
	t.Run("all one campaign is homogeneous", func(t *testing.T) {
		items := threeItems()
		for i := range items {
			items[i].Campaign = "retry-2026-08"
		}
		if err := completeCommitment(items).Validate(items); err != nil {
			t.Fatalf("members sharing one campaign must validate; got %v", err)
		}
	})
}

func TestC1637_002_TriagePhaseFailsOpenOnHeterogeneousBacklog(t *testing.T) {
	t.Run("unscoped member beside a campaign member", func(t *testing.T) {
		items := threeItems()
		items[1].Campaign = "pipeline-integrity"
		c := completeCommitment(items)
		resp, _, ws := runTriage(t, items, memberIDs(items), &c)
		assertRejectedFailOpen(t, resp, ws, 3, "campaign")
	})
	t.Run("code member beside a document member", func(t *testing.T) {
		items := threeItems()
		items[2].DeliverableKind = "document"
		c := completeCommitment(items)
		resp, _, ws := runTriage(t, items, memberIDs(items), &c)
		assertRejectedFailOpen(t, resp, ws, 3, "")
	})
}

func TestC1637_003_TriagePhaseValidatesMembersClaimedIntoThisCyclesProcessingDir(t *testing.T) {
	thisLane := filepath.Join("processing", fmt.Sprintf("cycle-%d", thisCycle))
	items := threeItems()
	ids := memberIDs(items)

	t.Run("all members claimed by this cycle", func(t *testing.T) {
		c := completeCommitment(items)
		resp, _, ws := runTriageAt(t, items, placement{"alpha": thisLane, "beta": thisLane, "gamma": thisLane}, ids,
			map[string]any{"unified_commitment": c})
		if resp.Verdict != core.VerdictPASS {
			t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
		}
		if hasDiagMentioning(resp.Diagnostics, "unified_commitment rejected") {
			t.Errorf("members claimed into this cycle's processing dir are KNOWN inbox items; the claim must not be rejected: %v", resp.Diagnostics)
		}
		sig := digestTriage(t, ws)
		if sig.Triage.UnifiedSize != "small" || sig.Triage.UnifiedMemberCount != 3 {
			t.Errorf("router.Digest must project the validated commitment over claimed members: UnifiedSize=%q UnifiedMemberCount=%d, want small/3", sig.Triage.UnifiedSize, sig.Triage.UnifiedMemberCount)
		}
		if sig.Triage.CommittedCount != 3 {
			t.Errorf("per-member acceptance stays separate: CommittedCount=%d, want 3", sig.Triage.CommittedCount)
		}
	})

	t.Run("root and claimed members mixed", func(t *testing.T) {
		c := completeCommitment(items)
		resp, _, ws := runTriageAt(t, items, placement{"alpha": inboxRootDir, "beta": thisLane, "gamma": thisLane}, ids,
			map[string]any{"unified_commitment": c})
		if resp.Verdict != core.VerdictPASS {
			t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
		}
		if sig := digestTriage(t, ws); sig.Triage.UnifiedSize != "small" {
			t.Errorf("a claim mixing pending and this-cycle-claimed members must project; UnifiedSize=%q diags=%v", sig.Triage.UnifiedSize, resp.Diagnostics)
		}
	})

	t.Run("member already landed in processed/ is rejected", func(t *testing.T) {
		c := completeCommitment(items)
		resp, _, ws := runTriageAt(t, items, placement{"alpha": thisLane, "beta": thisLane, "gamma": filepath.Join("processed", "cycle-1600")}, ids,
			map[string]any{"unified_commitment": c})
		assertRejectedFailOpen(t, resp, ws, 3, "gamma")
	})
	t.Run("member claimed by another lane is rejected", func(t *testing.T) {
		c := completeCommitment(items)
		resp, _, ws := runTriageAt(t, items, placement{"alpha": thisLane, "beta": filepath.Join("processing", "cycle-1500"), "gamma": thisLane}, ids,
			map[string]any{"unified_commitment": c})
		assertRejectedFailOpen(t, resp, ws, 3, "beta")
	})
}

func TestC1637_004_LandedDecisionClosesEveryMemberTransactionally(t *testing.T) {
	const landedSHA = "abc1234"
	items := threeItems()
	ids := memberIDs(items)
	c := completeCommitment(items)

	apply := func(t *testing.T, proj string, passed bool) inboxmover.OutcomeResult {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(proj, "decision.json"))
		if err != nil {
			t.Fatal(err)
		}
		committed := inboxmover.CommittedIDs(body)
		for _, id := range ids {
			if !contains(committed, id) {
				t.Fatalf("the runner-emitted decision must keep member %q in the worked set read by ship; CommittedIDs=%v", id, committed)
			}
		}
		res, err := inboxmover.ApplyCycleOutcome(inboxmover.Options{
			ProjectRoot: proj,
			Stderr:      io.Discard,
			IsLandedFn:  func(string) (bool, error) { return true, nil },
		}, inboxmover.CycleOutcome{Cycle: thisCycle, Passed: passed, CommittedIDs: committed, CommitSHA: landedSHA})
		if err != nil {
			t.Fatalf("ApplyCycleOutcome(passed=%v): %v", passed, err)
		}
		return res
	}
	stage := func(t *testing.T) string {
		t.Helper()
		resp, proj, ws := runTriage(t, items, ids, &c)
		if resp.Verdict != core.VerdictPASS {
			t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
		}
		if sig := digestTriage(t, ws); sig.Triage.UnifiedSize != "small" {
			t.Fatalf("precondition: valid claim must project; UnifiedSize=%q", sig.Triage.UnifiedSize)
		}
		emitted, err := os.ReadFile(filepath.Join(ws, "triage-decision.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(proj, "decision.json"), emitted, 0o644); err != nil {
			t.Fatal(err)
		}
		return proj
	}
	processedDir := func(proj string) string {
		return filepath.Join(proj, ".evolve", "inbox", "processed", fmt.Sprintf("cycle-%d", thisCycle))
	}

	t.Run("landed PASS promotes every member", func(t *testing.T) {
		proj := stage(t)
		res := apply(t, proj, true)
		for _, id := range ids {
			if !contains(res.Promoted, id) {
				t.Errorf("member %q must be promoted with the landing; Promoted=%v", id, res.Promoted)
			}
			if _, err := os.Stat(filepath.Join(processedDir(proj), landedSHA+"-"+id+".json")); err != nil {
				t.Errorf("member %q must be in processed/cycle-%d after landing: %v", id, thisCycle, err)
			}
		}
	})
	t.Run("FAIL promotes no member", func(t *testing.T) {
		proj := stage(t)
		res := apply(t, proj, false)
		if len(res.Promoted) != 0 {
			t.Errorf("nothing may be promoted on a FAIL (transactional with landing); Promoted=%v", res.Promoted)
		}
		if _, err := os.Stat(processedDir(proj)); err == nil {
			t.Errorf("processed/cycle-%d must not exist after a FAIL", thisCycle)
		}
	})
}

func TestC1637_005_RouterRoutesOnlyOnTheValidatedProjection(t *testing.T) {
	items := threeItems()
	ids := memberIDs(items)

	t.Run("forged projection without a commitment is cleared", func(t *testing.T) {
		resp, _, ws := runTriageAt(t, items, placement{}, ids,
			map[string]any{"unified_projection": map[string]any{"size": "small", "member_count": 3}})
		assertRejectedFailOpen(t, resp, ws, 3, "")
	})

	t.Run("forged projection beside a valid claim is recomputed", func(t *testing.T) {
		c := completeCommitment(items)
		resp, _, ws := runTriageAt(t, items, placement{}, ids, map[string]any{
			"unified_commitment": c,
			"unified_projection": map[string]any{"size": "large", "member_count": 99},
		})
		if resp.Verdict != core.VerdictPASS {
			t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
		}
		sig := digestTriage(t, ws)
		if sig.Triage.UnifiedSize != "small" || sig.Triage.UnifiedMemberCount != 3 {
			t.Errorf("the projection must be the validator's (small/3), not the agent's (large/99); got %q/%d", sig.Triage.UnifiedSize, sig.Triage.UnifiedMemberCount)
		}
		if _, err := os.Stat(filepath.Join(ws, "campaign-plan.json")); err == nil {
			t.Error("a forged 'large' must not route a 3-member claim to the campaign planner")
		}
	})

	t.Run("raw claim without the triage phase does not route", func(t *testing.T) {
		ws := t.TempDir()
		c := completeCommitment(items)
		if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(reportMD(ids)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decisionDoc(t, ids, map[string]any{"unified_commitment": c})), 0o644); err != nil {
			t.Fatal(err)
		}
		sig := digestTriage(t, ws)
		if sig.Triage.UnifiedSize != "" || sig.Triage.UnifiedMemberCount != 0 {
			t.Errorf("an unvalidated raw unified_commitment must not produce a routing signal; got %q/%d", sig.Triage.UnifiedSize, sig.Triage.UnifiedMemberCount)
		}
		if sig.Triage.CommittedCount != 3 {
			t.Errorf("the independent top_n is still digested: CommittedCount=%d, want 3", sig.Triage.CommittedCount)
		}
	})

	t.Run("projection is not read before triage completes", func(t *testing.T) {
		c := completeCommitment(items)
		_, _, ws := runTriage(t, items, ids, &c)
		sig, err := router.Digest(ws, []string{"scout"})
		if err != nil {
			t.Fatalf("router.Digest: %v", err)
		}
		if sig.Triage.UnifiedSize != "" {
			t.Errorf("no triage in completed ⇒ no unified signal; got %q", sig.Triage.UnifiedSize)
		}
	})
}

func TestC1637_006_SalvagedCycle1633ContractStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "acs", "cycle1633")
	if !acsassert.FileExists(t, filepath.Join(pkgDir, "predicates_test.go")) {
		t.Fatalf("the salvaged predicate package must stay in the tree: %s", pkgDir)
	}
	cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "./acs/cycle1633/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cycle-1633 contract regressed (the Builder's fixes must keep it GREEN):\n%s\n%v", out, err)
	}
	if !strings.Contains(string(out), "ok ") {
		t.Errorf("expected an `ok` line from the cycle1633 package; got:\n%s", out)
	}
}

func TestC1637_007_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1637", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}

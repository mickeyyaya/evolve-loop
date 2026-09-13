//go:build acs

// Package cycle1633 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane:
//
//   - triage-unified-solution-synthesis → a validated, evidence-cited UNIFIED
//     commitment at the inboxbatch/triage seam that fails OPEN to independent
//     top_n selection, and is projected ONLY when valid: a small commitment
//     pins plan-review + build-planner through the phase registry, a large one
//     emits an ADR-0054 campaign plan. Per-member acceptance stays separate
//     (members ⊆ top_n; every member keeps its own acceptance contract).
//
// The second lane-scoped id, overlay-family-name-transport-ambiguity, was
// dropped by triage as already shipped (797b8518 — verified an ancestor of
// this tree; its five llmroute tests pass here). R9.3 forbids predicates for
// non-top_n work, so nothing below binds to it.
//
// Provenance: cycle 1629 built this exact contract to 20/20 predicates GREEN
// and an auditor-narrative PASS; the deterministic explanation-documentation
// gate forced FAIL on a path:line citation gap (audit-fail-reason.json), not
// on the code. Its snapshot (b5b2b669, branch cycle-cd3ae73e-1629) is the
// salvage source. This package re-pins the SAME wire contract so the salvaged
// implementation satisfies it, with one deliberate narrowing: 015 asserts
// wave membership through the existing fleet.CycleSpec.Scope, so the fix
// needs NO edit under go/internal/fleet (the file the 1629 gate tripped on).
//
// The contract pinned here (every symbol below is RED — the package does not
// compile until Builder provides it):
//
//	inboxbatch.UnifiedMember{ID, Evidence string}            json: id, evidence
//	inboxbatch.UnifiedCommitment{RootCauseHypothesis string   json: root_cause_hypothesis
//	                             SharedSeam string            json: shared_seam
//	                             DesignRequirements []string  json: design_requirements
//	                             Members []UnifiedMember}     json: members
//	func (c UnifiedCommitment) Validate(items []Item) error   nil ⇔ credible
//	func (c UnifiedCommitment) Size() string                  "small" | "large"
//	                                                          (len(Members) <= DefaultMaxItems ⇒ small)
//	router.TriageSignals.UnifiedSize string                   "" when absent/invalid
//	router.TriageSignals.UnifiedMemberCount int
//	routing field "triage.unified_size" (registry conditional_mandatory)
//	campaign.PlanFromUnifiedCommitment(c, items) (*campaign.Plan, error)
//
// triage-decision.json carries the agent's claim under the top-level key
// "unified_commitment". The triage PHASE (hooks.Classify, reached through the
// real runner) is the production validator: an invalid claim keeps verdict
// PASS (fail-open to the independent top_n), surfaces a diagnostic, and leaves
// NO unified signal for the router; a valid claim is projected by
// router.Digest.
//
// Predicate strategy — every predicate exercises the system under test (a
// direct call on the typed seam, the REAL triage runner driven by a fake
// core.Bridge, router.Digest/Route over the REAL phase registry, or a real
// emitted campaign-plan.json), never a source grep (the cycle-85 ban):
//
//   - 001–007: the typed contract — accept the complete/evidence-cited case;
//     reject missing evidence, unknown member, duplicate member, incomplete
//     shape, heterogeneous members (distinct campaigns / mixed deliverable
//     kinds — the forced-unification failure mode); size boundary pinned to
//     inboxbatch.DefaultMaxItems.
//   - 008: regression — the deterministic batch rules are untouched (a
//     commitment is a triage-declared artifact, not an inferred grouping rule).
//   - 009–011: the triage seam through the production runner — fail-open on an
//     invalid claim, projection of a valid small claim, rejection of a member
//     outside top_n (per-member acceptance stays separate).
//   - 012–014: the routing projection over the REAL phase registry — a small
//     commitment pins plan-review and build-planner even against an advisor
//     plan that declined them (Plan != nil bypasses insert_when; only
//     conditional_mandatory survives — router.shouldRun), and build-planner's
//     own ShouldSkip agrees; the no-commitment baseline is unchanged.
//   - 015–016: the campaign route — a large commitment projects to a Verify()-
//     clean campaign.Plan honoring member deps AND each member's own
//     acceptance contract; an invalid one is refused; the triage runner EMITS
//     campaign-plan.json in the workspace for large claims only.
//   - 017: this package is git-tracked (cycle-93: untracked predicates are
//     dropped at ship; cycle-1623 audit M1 recurrence).
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002/003/004/
// 005/006/009/011 and the invalid-claim half of 015 reject; a no-op that
// accepts everything fails them. EDGE — blank strings, zero/one member, exactly
// DefaultMaxItems vs DefaultMaxItems+1, a member outside top_n. SEMANTIC —
// validation (001–007), classifier isolation (008), phase fail-open (009–011),
// routing (012–014) and campaign projection (015–016) are five distinct
// behaviors, not one restated.
//
// Flaky-shape hygiene: no whole-package `go test` subprocesses, no wall-clock
// bounds, no literal PIDs, no bare `git` (every git call is -C rooted), no
// load generators.
package cycle1633

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// threeItems is a homogeneous small backlog: three code items, no campaign,
// disjoint file areas (so no mechanical rule binds them — the synthesis seam is
// the ONLY thing that can unify them).
func threeItems() []inboxbatch.Item {
	return []inboxbatch.Item{
		{ID: "alpha", Title: "alpha", Weight: 0.8, Files: []string{"go/internal/alpha/a.go"}},
		{ID: "beta", Title: "beta", Weight: 0.7, Files: []string{"go/internal/beta/b.go"}},
		{ID: "gamma", Title: "gamma", Weight: 0.6, Files: []string{"go/internal/gamma/c.go"}},
	}
}

// chainItems returns n code items delta1..deltaN where delta<k> depends on
// delta<k-1> — a dependency chain the campaign waves must honor. Every item
// carries its OWN acceptance criterion so the campaign projection can be
// checked for per-member contract retention (no blended contract).
func chainItems(n int) []inboxbatch.Item {
	items := make([]inboxbatch.Item, 0, n)
	for i := 1; i <= n; i++ {
		it := inboxbatch.Item{
			ID:         fmt.Sprintf("delta%d", i),
			Title:      fmt.Sprintf("delta %d", i),
			Weight:     0.5,
			Files:      []string{fmt.Sprintf("go/internal/delta%d/d.go", i)},
			Acceptance: []string{fmt.Sprintf("delta%d acceptance: its own retry call site uses the shared Retrier", i)},
		}
		if i > 1 {
			it.Deps = []string{fmt.Sprintf("delta%d", i-1)}
		}
		items = append(items, it)
	}
	return items
}

// completeCommitment is the positive fixture: every field present, every
// member evidence-cited, every member a known item.
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

// decisionJSON renders a triage-decision.json body: the independent top_n over
// topN ids plus an optional unified_commitment object. json.Marshal on the typed
// struct pins the wire field names the Builder's JSON tags must produce.
func decisionJSON(t *testing.T, topN []string, c *inboxbatch.UnifiedCommitment) string {
	t.Helper()
	top := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		top = append(top, map[string]string{"id": id})
	}
	doc := map[string]any{"cycle": 1633, "top_n": top}
	if c != nil {
		doc["unified_commitment"] = c
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// reportMD renders a minimal but contract-complete triage-report.md whose
// ## top_n lists ids (cycle_size medium keeps tdd pinned, matching production).
func reportMD(topN []string) string {
	var b strings.Builder
	b.WriteString("cycle_size_estimate: medium\ndeliverable_kind: code\n\n## top_n\n")
	for _, id := range topN {
		fmt.Fprintf(&b, "- %s: close %s, files=go/internal/%s/x.go\n", id, id, id)
	}
	b.WriteString("\n## deferred\n- none\n\n## dropped\n- none\n")
	return b.String()
}

// fakeBridge is the minimal core.Bridge: it writes the scripted triage report
// and decision into the workspace, exactly where the real agent would.
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

// writeInbox materializes items as .evolve/inbox/<id>.json under a temp project
// root and returns that root — the inbox the triage phase validates against.
func writeInbox(t *testing.T, items []inboxbatch.Item) string {
	t.Helper()
	proj := t.TempDir()
	dir := filepath.Join(proj, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
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

// runTriage drives the REAL triage phase runner (triage.New → BaseRunner.Run →
// hooks.Classify) over a temp project whose inbox holds items, with the fake
// bridge writing report+decision. Returns the response and the workspace.
func runTriage(t *testing.T, items []inboxbatch.Item, topN []string, c *inboxbatch.UnifiedCommitment) (core.PhaseResponse, string) {
	t.Helper()
	proj := writeInbox(t, items)
	ws := t.TempDir()
	fb := &fakeBridge{report: reportMD(topN), decision: decisionJSON(t, topN, c)}
	ph := triage.New(triage.Config{
		Bridge: fb,
		Prompts: prompts.NewFromFS(fstest.MapFS{
			"agents/evolve-triage.md": &fstest.MapFile{Data: []byte("---\nname: evolve-triage\n---\ntriage body")},
		}),
	})
	resp, err := ph.Run(context.Background(), core.PhaseRequest{Cycle: 1633, ProjectRoot: proj, Workspace: ws})
	if err != nil {
		t.Fatalf("triage runner: %v", err)
	}
	return resp, ws
}

// digestTriage projects the workspace's triage artifacts through the production
// router digest — the ONE reader the routing kernel uses.
func digestTriage(t *testing.T, ws string) router.RoutingSignals {
	t.Helper()
	sig, err := router.Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("router.Digest: %v", err)
	}
	return sig
}

// realRegistryConfig loads the REAL phase registry (docs/architecture/
// phase-registry.json under the worktree) — the config-first wiring surface.
func realRegistryConfig(t *testing.T) config.RoutingConfig {
	t.Helper()
	root := acsassert.RepoRoot(t)
	cfg, _ := config.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"), map[string]string{})
	return cfg
}

// declinedPlan is an advisor plan that runs the spine but DECLINES plan-review
// and build-planner — the production shape a conditional pin must beat
// (Plan != nil bypasses insert_when; only conditional_mandatory survives).
func declinedPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "triage", Run: true},
		{Phase: "plan-review", Run: false}, {Phase: "tdd", Run: true},
		{Phase: "build-planner", Run: false}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

// walkPhases follows router.Route from `from` until it reaches "build" or
// "end" (or 16 steps), returning every phase it visits in order. Robust to
// extra inserts the Builder may add: the assertions check membership, not the
// exact next hop.
func walkPhases(cfg config.RoutingConfig, sig router.RoutingSignals, plan *router.PhasePlan, from string, completed []string) []string {
	var visited []string
	cur := from
	done := append([]string(nil), completed...)
	for i := 0; i < 16; i++ {
		d := router.Route(router.RouteInput{Current: cur, Verdict: "PASS", Signals: sig, Cfg: cfg, Completed: done, Plan: plan}, nil)
		if d.NextPhase == router.PhaseEnd {
			return visited
		}
		visited = append(visited, d.NextPhase)
		if d.NextPhase == "build" {
			return visited
		}
		done = append(done, d.NextPhase)
		cur = d.NextPhase
	}
	return visited
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hasDiagMentioning(diags []core.Diagnostic, needle string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, needle) {
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

// ---------------------------------------------------------------------------
// 001–007: the typed contract at the inboxbatch seam
// ---------------------------------------------------------------------------

func TestC1633_001_UnifiedCommitmentAcceptsCompleteEvidenceCitedMembers(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	if err := c.Validate(items); err != nil {
		t.Fatalf("complete, evidence-cited commitment over known items must validate; got %v", err)
	}
	if got := c.Size(); got != "small" {
		t.Errorf("3 members (<= DefaultMaxItems=%d) must be size %q; got %q", inboxbatch.DefaultMaxItems, "small", got)
	}
}

func TestC1633_002_UnifiedCommitmentRejectsMissingEvidence(t *testing.T) {
	items := threeItems()
	for _, blank := range []string{"", "   \t"} {
		c := completeCommitment(items)
		c.Members[1].Evidence = blank
		err := c.Validate(items)
		if err == nil {
			t.Fatalf("member %q with evidence %q must be rejected (evidence-cited, not vibes)", c.Members[1].ID, blank)
		}
		if !strings.Contains(err.Error(), "beta") {
			t.Errorf("rejection must name the offending member %q; got %v", "beta", err)
		}
	}
}

func TestC1633_003_UnifiedCommitmentRejectsUnknownMember(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members = append(c.Members, inboxbatch.UnifiedMember{ID: "phantom", Evidence: "cites the root cause"})
	err := c.Validate(items)
	if err == nil {
		t.Fatal("a member that is not a known inbox item must be rejected")
	}
	if !strings.Contains(err.Error(), "phantom") {
		t.Errorf("rejection must name the unknown member; got %v", err)
	}
}

func TestC1633_004_UnifiedCommitmentRejectsDuplicateMember(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members = append(c.Members, c.Members[0]) // alpha twice
	err := c.Validate(items)
	if err == nil {
		t.Fatal("a duplicated member id must be rejected (double-counting one item as two closures)")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Errorf("rejection must name the duplicated member; got %v", err)
	}
}

func TestC1633_005_UnifiedCommitmentRejectsIncompleteShape(t *testing.T) {
	items := threeItems()
	cases := []struct {
		name   string
		mutate func(c *inboxbatch.UnifiedCommitment)
	}{
		{"empty root cause", func(c *inboxbatch.UnifiedCommitment) { c.RootCauseHypothesis = "" }},
		{"blank root cause", func(c *inboxbatch.UnifiedCommitment) { c.RootCauseHypothesis = "  " }},
		{"empty shared seam", func(c *inboxbatch.UnifiedCommitment) { c.SharedSeam = "" }},
		{"no design requirements", func(c *inboxbatch.UnifiedCommitment) { c.DesignRequirements = nil }},
		{"blank design requirement", func(c *inboxbatch.UnifiedCommitment) { c.DesignRequirements = []string{" "} }},
		{"single member is not unified", func(c *inboxbatch.UnifiedCommitment) { c.Members = c.Members[:1] }},
		{"zero members", func(c *inboxbatch.UnifiedCommitment) { c.Members = nil }},
		{"empty member id", func(c *inboxbatch.UnifiedCommitment) { c.Members[0].ID = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := completeCommitment(items)
			tc.mutate(&c)
			if err := c.Validate(items); err == nil {
				t.Errorf("%s: incomplete commitment must be rejected (fail-open to independent top_n)", tc.name)
			}
		})
	}
	// The zero value is the degenerate incomplete shape.
	if err := (inboxbatch.UnifiedCommitment{}).Validate(items); err == nil {
		t.Error("zero-value commitment must be rejected")
	}
}

func TestC1633_006_UnifiedCommitmentRejectsHeterogeneousMembers(t *testing.T) {
	// Distinct non-empty campaigns are the operator's explicit "two
	// initiatives" partition — Classify never merges them on an inferred
	// signal, and a synthesis claim must not either (forced unification of
	// unrelated items is THE failure mode this task pins against).
	t.Run("distinct campaigns", func(t *testing.T) {
		items := threeItems()
		items[0].Campaign = "retry-2026-08"
		items[2].Campaign = "docs-2026-09"
		c := completeCommitment(items)
		if err := c.Validate(items); err == nil {
			t.Fatal("members spanning two distinct non-empty campaigns must be rejected as heterogeneous")
		}
	})
	// Same campaign (or none) on every member stays homogeneous — the guard
	// must not reject the legitimate case.
	t.Run("one campaign is homogeneous", func(t *testing.T) {
		items := threeItems()
		for i := range items {
			items[i].Campaign = "retry-2026-08"
		}
		c := completeCommitment(items)
		if err := c.Validate(items); err != nil {
			t.Fatalf("members sharing one campaign must validate; got %v", err)
		}
	})
	// A cycle has ONE authoritative deliverable kind (ADR-0099); a code item
	// and a document item cannot be one solution.
	t.Run("mixed deliverable kinds", func(t *testing.T) {
		items := threeItems()
		items[1].DeliverableKind = "document"
		c := completeCommitment(items)
		if err := c.Validate(items); err == nil {
			t.Fatal("members mixing code and document deliverable kinds must be rejected as heterogeneous")
		}
	})
}

func TestC1633_007_UnifiedCommitmentSizeBoundaryIsDefaultMaxItems(t *testing.T) {
	// The small/large boundary reuses the batch cap (one cycle's pipeline
	// carries at most DefaultMaxItems related items) — no second constant.
	small := chainItems(inboxbatch.DefaultMaxItems)
	cs := completeCommitment(small)
	if err := cs.Validate(small); err != nil {
		t.Fatalf("%d-member commitment must validate; got %v", len(small), err)
	}
	if got := cs.Size(); got != "small" {
		t.Errorf("exactly DefaultMaxItems members must be %q; got %q", "small", got)
	}
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	if err := cl.Validate(large); err != nil {
		t.Fatalf("%d-member commitment must validate; got %v", len(large), err)
	}
	if got := cl.Size(); got != "large" {
		t.Errorf("DefaultMaxItems+1 members must be %q; got %q", "large", got)
	}
}

// ---------------------------------------------------------------------------
// 008: regression — mechanical batching is untouched by synthesis
// ---------------------------------------------------------------------------

func TestC1633_008_DefaultBatchRulesUnchangedBySynthesis(t *testing.T) {
	// A unified commitment is a triage-declared artifact validated AFTER
	// classification; it must not become a grouping Rule. Three items with no
	// campaign, disjoint areas and no deps stay three batches under the default
	// rule set, and the rule set is still the three structural signals
	// (rules_rootcause_regression_test.go forbids a prose root-cause rule).
	if got := len(inboxbatch.DefaultRules()); got != 3 {
		t.Errorf("DefaultRules must remain the 3 structural signals (campaign, file-area, dep); got %d", got)
	}
	batches := inboxbatch.Classify(threeItems(), inboxbatch.Config{})
	if len(batches) != 3 {
		t.Errorf("unrelated items must stay independent batches: got %d, want 3", len(batches))
	}
}

// ---------------------------------------------------------------------------
// 009–011: the triage seam through the production runner
// ---------------------------------------------------------------------------

func TestC1633_009_TriagePhaseFailsOpenOnInvalidCommitment(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	c.Members[2] = inboxbatch.UnifiedMember{ID: "phantom", Evidence: "not an inbox item"}
	resp, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("an invalid synthesis claim must fail OPEN to the independent top_n (verdict PASS); got %q diags=%v", resp.Verdict, resp.Diagnostics)
	}
	if !hasDiagMentioning(resp.Diagnostics, "unified_commitment") {
		t.Errorf("fail-open must be LOUD: expected a diagnostic mentioning unified_commitment; got %v", resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "" {
		t.Errorf("router must see NO unified signal after an invalid claim; got UnifiedSize=%q", sig.Triage.UnifiedSize)
	}
	if sig.Triage.CommittedCount != 3 {
		t.Errorf("independent top_n must survive fail-open: CommittedCount=%d, want 3", sig.Triage.CommittedCount)
	}
}

func TestC1633_010_TriagePhaseProjectsValidSmallCommitment(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	resp, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("valid commitment: verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "small" {
		t.Errorf("router.Digest must project the validated commitment: UnifiedSize=%q, want %q", sig.Triage.UnifiedSize, "small")
	}
	if sig.Triage.UnifiedMemberCount != 3 {
		t.Errorf("UnifiedMemberCount=%d, want 3", sig.Triage.UnifiedMemberCount)
	}
	if sig.Triage.CommittedCount != 3 {
		t.Errorf("per-member acceptance stays separate: the independent top_n count must be preserved (got %d, want 3)", sig.Triage.CommittedCount)
	}
}

func TestC1633_011_TriagePhaseRejectsMemberOutsideTopN(t *testing.T) {
	// The Task Contract (ADR-0098) projects acceptance per top_n id. A member
	// the decision did not commit has no separate acceptance reference, so the
	// ONE solution could not be graded against it — reject, fail open.
	items := threeItems()
	c := completeCommitment(items)
	resp, ws := runTriage(t, items, []string{"alpha", "beta"}, &c) // gamma claimed, not committed
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS (fail-open); diags=%v", resp.Verdict, resp.Diagnostics)
	}
	if !hasDiagMentioning(resp.Diagnostics, "gamma") {
		t.Errorf("rejection must name the uncommitted member gamma; diags=%v", resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "" {
		t.Errorf("a claim whose members exceed top_n must not project: UnifiedSize=%q", sig.Triage.UnifiedSize)
	}
	if sig.Triage.CommittedCount != 2 {
		t.Errorf("the independent top_n must survive the rejection untouched: CommittedCount=%d, want 2", sig.Triage.CommittedCount)
	}
}

// ---------------------------------------------------------------------------
// 012–014: routing projection over the REAL phase registry
// ---------------------------------------------------------------------------

func TestC1633_012_SmallCommitmentPinsPlanReviewViaRegistry(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	sig := digestTriage(t, ws)
	cfg := realRegistryConfig(t)
	visited := walkPhases(cfg, sig, declinedPlan(), "triage", []string{"scout", "triage"})
	if !contains(visited, "plan-review") {
		t.Errorf("a validated small commitment must pin plan-review even against an advisor plan that declined it; walk visited %v", visited)
	}
	// Baseline: the same walk with NO commitment does not reach plan-review.
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	base := walkPhases(cfg, digestTriage(t, ws0), declinedPlan(), "triage", []string{"scout", "triage"})
	if contains(base, "plan-review") {
		t.Errorf("no commitment must leave routing unchanged (plan-review not pinned); baseline walk visited %v", base)
	}
	// Symmetry lock (cycle-1638 audit H1): the plan-review half must never fork
	// from the build-planner half on size. A LARGE commitment pins plan-review
	// for the same reason 013 now pins build-planner.
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	_, wsL := runTriage(t, large, memberIDs(large), &cl)
	if got := walkPhases(cfg, digestTriage(t, wsL), declinedPlan(), "triage", []string{"scout", "triage"}); !contains(got, "plan-review") {
		t.Errorf("a validated LARGE unified commitment must pin plan-review exactly as a small one does; walk visited %v", got)
	}
}

func TestC1633_013_SmallCommitmentPinsBuildPlannerViaRegistry(t *testing.T) {
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	sig := digestTriage(t, ws)
	cfg := realRegistryConfig(t)
	done := []string{"scout", "triage", "plan-review", "tdd"}
	visited := walkPhases(cfg, sig, declinedPlan(), "tdd", done)
	if !contains(visited, "build-planner") {
		t.Errorf("a validated small commitment must pin build-planner even against an advisor plan that declined it; walk from tdd visited %v", visited)
	}
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	base := walkPhases(cfg, digestTriage(t, ws0), declinedPlan(), "tdd", done)
	if contains(base, "build-planner") {
		t.Errorf("no commitment must leave build-planner opt-in (skipped); baseline walk visited %v", base)
	}
	// RECONCILED by TDD in cycle-1638 (audit round 1, H1). This block formerly
	// asserted the INVERSE — "only SMALL commitments route through
	// build-planner" — which projected how_to_apply step (3)'s EXECUTION split
	// ("small unified fix = one cycle; large = emit an ADR-0054 campaign plan")
	// onto ROUTING, a distinction the directive does not make there. Step (2) is
	// unqualified by size: "a unified commitment routes through
	// buildplanner+plan-review at deep tier"
	// (.evolve/inbox/consumed/2026-07-21T02-00-00Z-triage-unified-solution-synthesis.json:30).
	// The large commitment is the highest-blast-radius design the loop commits,
	// so it is the LAST one that may escape planning. The size split stays where
	// the directive puts it — in what execution EMITS — and 015/016 pin that
	// campaign-plan half. Both ACS packages are added by this same diff
	// (`git cat-file -e 4c58eb6d:go/acs/cycle1633/predicates_test.go` → absent),
	// so this is one author reconciling their own contract, not a superseded
	// inheritance.
	large := chainItems(inboxbatch.DefaultMaxItems + 1)
	cl := completeCommitment(large)
	_, wsL := runTriage(t, large, memberIDs(large), &cl)
	sigL := digestTriage(t, wsL)
	if sigL.Triage.UnifiedSize != "large" {
		t.Fatalf("precondition: large commitment must project UnifiedSize=large; got %q", sigL.Triage.UnifiedSize)
	}
	if got := walkPhases(cfg, sigL, declinedPlan(), "tdd", done); !contains(got, "build-planner") {
		t.Errorf("a validated LARGE unified commitment must ALSO pin build-planner — the campaign path is the one design that must not outrun planning (how_to_apply step 2 names both planning phases without a size qualifier); walk from tdd visited %v", got)
	}
	// The campaign route is the EXECUTION half and is unaffected: 015/016 pin
	// that a large commitment still projects a Verify()-clean campaign plan.
}

func TestC1633_014_BuildPlannerPhaseRunsForSmallCommitment(t *testing.T) {
	// The runner consults Skipper.ShouldSkip before dispatch: a router pin is
	// dead wiring if the phase then skips itself. Real repo root ⇒ the real
	// registry + policy; the workspace carries the validated commitment.
	root := acsassert.RepoRoot(t)
	items := threeItems()
	c := completeCommitment(items)
	_, ws := runTriage(t, items, []string{"alpha", "beta", "gamma"}, &c)
	bp := buildplanner.New(buildplanner.Config{})
	skipped, verdict, _, _ := bp.ShouldSkip(core.PhaseRequest{Cycle: 1633, ProjectRoot: root, Workspace: ws, Env: map[string]string{}})
	if skipped {
		t.Errorf("build-planner must RUN for a validated small commitment; ShouldSkip=true verdict=%q", verdict)
	}
	_, ws0 := runTriage(t, items, []string{"alpha", "beta", "gamma"}, nil)
	skipped0, _, _, _ := bp.ShouldSkip(core.PhaseRequest{Cycle: 1633, ProjectRoot: root, Workspace: ws0, Env: map[string]string{}})
	if !skipped0 {
		t.Error("build-planner must stay opt-in (skipped) when no commitment is present")
	}
}

// ---------------------------------------------------------------------------
// 015–016: the campaign route for large commitments
// ---------------------------------------------------------------------------

func TestC1633_015_LargeCommitmentProjectsToVerifiedCampaignPlan(t *testing.T) {
	items := chainItems(inboxbatch.DefaultMaxItems + 1)
	c := completeCommitment(items)
	plan, err := campaign.PlanFromUnifiedCommitment(c, items)
	if err != nil {
		t.Fatalf("PlanFromUnifiedCommitment: %v", err)
	}
	if err := plan.Verify(); err != nil {
		t.Fatalf("projected plan must pass the deterministic trust boundary: %v", err)
	}
	if strings.TrimSpace(plan.Goal) == "" {
		t.Error("plan goal must carry the commitment's root cause (non-empty)")
	}
	if len(plan.Cycles) != len(items) {
		t.Fatalf("one cycle per member: got %d, want %d", len(plan.Cycles), len(items))
	}
	// Per-member acceptance is RETAINED, not blended: every member's own
	// criterion must be the contract of its own cycle (the "ONE solution must
	// satisfy EVERY member item's ACs" half of the operator directive).
	byID := map[string]string{}
	for _, cy := range plan.Cycles {
		byID[cy.ID] = cy.OutputContract
	}
	for _, it := range items {
		contract, ok := byID[it.ID]
		if !ok {
			t.Errorf("member %q has no cycle in the projected plan", it.ID)
			continue
		}
		if !strings.Contains(contract, it.Acceptance[0]) {
			t.Errorf("cycle %q must carry its member's own acceptance %q; got contract %q", it.ID, it.Acceptance[0], contract)
		}
	}
	waves, err := plan.Waves()
	if err != nil {
		t.Fatalf("Waves: %v", err)
	}
	// delta<k> depends on delta<k-1>: a dependency chain must serialize into
	// one member per wave, in order — member deps are honored, not flattened.
	// Membership is read from the EXISTING fleet.CycleSpec.Scope (todo ids the
	// wave's cycle owns) — no new fleet field is required for this pin.
	if len(waves) != len(items) {
		t.Errorf("a %d-long dep chain must yield %d waves; got %d", len(items), len(items), len(waves))
	}
	for i, w := range waves {
		want := fmt.Sprintf("delta%d", i+1)
		if len(w) != 1 || len(w[0].Scope) != 1 || w[0].Scope[0] != want {
			t.Errorf("wave %d must hold exactly one cycle scoped to %q; got %+v", i+1, want, w)
		}
	}
	// The campaign route never launders an invalid claim.
	bad := completeCommitment(items)
	bad.Members[0].Evidence = ""
	if _, err := campaign.PlanFromUnifiedCommitment(bad, items); err == nil {
		t.Error("an invalid commitment must be refused by the campaign projection")
	}
}

func TestC1633_016_TriagePhaseEmitsCampaignPlanForLargeCommitment(t *testing.T) {
	items := chainItems(inboxbatch.DefaultMaxItems + 1)
	c := completeCommitment(items)
	ids := memberIDs(items)
	resp, ws := runTriage(t, items, ids, &c)
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS; diags=%v", resp.Verdict, resp.Diagnostics)
	}
	sig := digestTriage(t, ws)
	if sig.Triage.UnifiedSize != "large" {
		t.Errorf("UnifiedSize=%q, want %q", sig.Triage.UnifiedSize, "large")
	}
	planPath := filepath.Join(ws, "campaign-plan.json")
	if !acsassert.FileExists(t, planPath) {
		t.Fatalf("a validated LARGE commitment must emit %s for `evolve campaign run --plan`", planPath)
	}
	plan, err := campaign.LoadFile(planPath)
	if err != nil {
		t.Fatalf("emitted campaign plan must load: %v", err)
	}
	if err := plan.Verify(); err != nil {
		t.Fatalf("emitted campaign plan must verify: %v", err)
	}
	got := make([]string, 0, len(plan.Cycles))
	for _, cy := range plan.Cycles {
		got = append(got, cy.ID)
	}
	for _, id := range ids {
		if !contains(got, id) {
			t.Errorf("emitted plan must carry every member as a cycle; missing %q in %v", id, got)
		}
	}
	// Small commitments do NOT emit a campaign plan (one cycle is the route).
	small := threeItems()
	cs := completeCommitment(small)
	_, wsS := runTriage(t, small, []string{"alpha", "beta", "gamma"}, &cs)
	if _, err := os.Stat(filepath.Join(wsS, "campaign-plan.json")); err == nil {
		t.Error("a SMALL commitment must not emit campaign-plan.json")
	}
}

// ---------------------------------------------------------------------------
// 017: ship-tree tracking of this package (cycle-93 / cycle-1623 M1)
// ---------------------------------------------------------------------------

func TestC1633_017_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1633", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}

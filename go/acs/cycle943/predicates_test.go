//go:build acs

// Package cycle943 materializes the cycle-943 acceptance criteria for this
// fleet lane's sole inbox item, overlay-injection-dormant-wire-fable-deep,
// which the Scout re-scoped into three committed (## top_n) tasks after finding
// the phase-runner→prompt half already wired:
//
//	T1 skilloverlay-compact-md-first-materialization
//	T2 skill-overlay-dispatch-observability-log
//	T3 wire-overlay-resolution-non-phase-dispatch-sites
//
// EVERY behavioral predicate below EXERCISES THE SYSTEM UNDER TEST — it calls
// skilloverlay.Materialize on a real on-disk skill dir, or drives subagent.Run /
// retro.Phase.Run with a capturing fake core.Bridge and asserts on the
// core.BridgeRequest the dispatcher actually built. None assert solely on source
// text (the cycle-85 degenerate-predicate failure mode is avoided); the single
// doc-comment predicate carries the `// acs-predicate: config-check` waiver
// because a "this feature is still deferred" comment that has gone stale is an
// inherent text criterion, not a code path.
//
// RED today:
//   - T1 predicates fail on assertion: Materialize reads SKILL.md
//     unconditionally, so a dir with a distinct COMPACT.md yields the SKILL body.
//   - T2 predicates fail on COMPILE: runner.FormatSkillOverlayLog does not exist
//     yet. The Builder adds it (a pure formatter the dispatch closure calls).
//   - T3 predicates fail on assertion: subagent.Run and retro.Phase.Run build
//     their BridgeRequest with an empty Skills field today.
//
// SUT CONTRACT the Builder must satisfy WITHOUT modifying this file
// (full prose in test-report.md § Handoff to Builder):
//
//	skilloverlay.Materialize(skillsDir, names):
//	    prefer <skillsDir>/<name>/COMPACT.md when present+non-empty;
//	    fall back to SKILL.md when COMPACT.md is absent; unchanged otherwise.
//
//	runner.FormatSkillOverlayLog(phase string, skills []string, tier string) string:
//	    a pure formatter returning a line that contains "phase=<phase>",
//	    "skill-overlays=[<comma-joined skills>]", and "tier=<tier>". The
//	    per-attempt dispatch closure in runner.go emits it via log.Diag().Infof.
//
//	subagent.Run / retro.Phase.Run:
//	    resolve overlays for the launch's tier (Policy.ResolveOverlays on an
//	    OverlayDispatch built from the launch's phase/cli/model/tier) and set the
//	    resolved NAMES on core.BridgeRequest.Skills — matching runner.go. The
//	    tier→skill mapping stays SOLELY in policy.compiledDefaultOverlays (no new
//	    Go literal). A deep/top-tier launch attaches "fable"; a balanced launch
//	    attaches nothing.
package cycle943

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/skilloverlay"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// -----------------------------------------------------------------------------
// shared helpers
// -----------------------------------------------------------------------------

// writeSkillFile writes body into <dir>/<name>/<file>, creating the skill dir.
func writeSkillFile(t *testing.T, root, name, file, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

// contains reports whether want is one of the strings in got.
func contains(got []string, want string) bool {
	for _, s := range got {
		if s == want {
			return true
		}
	}
	return false
}

// capturingBridge records every BridgeRequest passed to Launch so a predicate
// can assert on the Skills the dispatcher resolved. Implements core.Bridge.
type capturingBridge struct {
	mu    sync.Mutex
	calls []core.BridgeRequest
	resp  core.BridgeResponse
	err   error
}

func (b *capturingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.calls = append(b.calls, req)
	b.mu.Unlock()
	return b.resp, b.err
}

func (b *capturingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func (b *capturingBridge) firstCall(t *testing.T) core.BridgeRequest {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.calls) == 0 {
		t.Fatalf("bridge.Launch was never called — dispatcher never reached the launch seam")
	}
	return b.calls[0]
}

// nopLedger is a no-op core.Ledger for subagent.Run wiring.
type nopLedger struct{}

func (nopLedger) Append(context.Context, core.LedgerEntry) error { return nil }
func (nopLedger) Verify(context.Context) error                   { return nil }
func (nopLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("nopLedger: Iter unused")
}

// fixedRand fills b with a constant byte so the challenge token is deterministic.
func fixedRand(b []byte) (int, error) {
	for i := range b {
		b[i] = 0xAB
	}
	return len(b), nil
}

// =============================================================================
// T1 — skilloverlay COMPACT.md-first materialization
// =============================================================================

// TestC943_001_MaterializePrefersCompactWhenPresent pins the TOKEN-ECONOMY
// requirement: when a skill dir carries BOTH COMPACT.md and SKILL.md,
// Materialize loads the COMPACT body and NOT the (larger) SKILL body. Exercises
// skilloverlay.Materialize directly and asserts on its returned prefix.
func TestC943_001_MaterializePrefersCompactWhenPresent(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "fable", "COMPACT.md", "COMPACT_BODY_marker rules ≤15")
	writeSkillFile(t, dir, "fable", "SKILL.md", "SKILL_BODY_marker full discipline")

	prefix, missing := skilloverlay.Materialize(dir, []string{"fable"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (both files readable)", missing)
	}
	if !strings.Contains(prefix, "COMPACT_BODY_marker") {
		t.Errorf("prefix did not include the COMPACT.md body; got:\n%s", prefix)
	}
	if strings.Contains(prefix, "SKILL_BODY_marker") {
		t.Errorf("prefix included the SKILL.md body — COMPACT.md must WIN when present; got:\n%s", prefix)
	}
}

// TestC943_002_MaterializeFallsBackToSkillWhenNoCompact is the fallback half of
// the contract: a dir with ONLY SKILL.md is byte-for-byte unchanged from
// pre-feature behavior. Anti-regression for the many existing SKILL.md-only
// overlays.
func TestC943_002_MaterializeFallsBackToSkillWhenNoCompact(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "fable", "SKILL.md", "SKILL_ONLY_marker discipline")

	prefix, missing := skilloverlay.Materialize(dir, []string{"fable"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none (SKILL.md readable)", missing)
	}
	if !strings.Contains(prefix, "SKILL_ONLY_marker") {
		t.Errorf("prefix did not fall back to SKILL.md when COMPACT.md absent; got:\n%s", prefix)
	}
}

// TestC943_003_MaterializeFailsOpenOnMissingBody is the NEGATIVE / fail-open
// (AC3) predicate: a named skill whose dir has NEITHER COMPACT.md nor SKILL.md
// is reported in `missing` and dropped from the prefix — Materialize returns
// (not panics, not errors), so the caller WARNs and proceeds rather than
// aborting the launch. A no-op that ignores missing bodies fails the `missing`
// assertion.
func TestC943_003_MaterializeFailsOpenOnMissingBody(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ghost"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prefix, missing := skilloverlay.Materialize(dir, []string{"ghost"})
	if !contains(missing, "ghost") {
		t.Errorf("missing = %v, want it to contain %q (no body on disk)", missing, "ghost")
	}
	if strings.Contains(prefix, "ghost") {
		t.Errorf("prefix must exclude a skill with no readable body; got:\n%s", prefix)
	}
}

// =============================================================================
// T2 — per-dispatch skill-overlay observability log line
// =============================================================================

// TestC943_004_FormatSkillOverlayLogEmitsResolvedSet exercises the pure
// formatter the dispatch closure emits, asserting it carries the phase, the
// resolved overlay set, and the tier — the operator-visible line
// `[runner] phase=audit skill-overlays=[fable] (tier=deep)`. Compiles only once
// the Builder adds runner.FormatSkillOverlayLog (RED = compile failure today).
func TestC943_004_FormatSkillOverlayLogEmitsResolvedSet(t *testing.T) {
	line := runner.FormatSkillOverlayLog("audit", []string{"fable"}, "deep")
	for _, want := range []string{"phase=audit", "skill-overlays=[fable]", "tier=deep"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q missing token %q", line, want)
		}
	}
}

// TestC943_005_FormatSkillOverlayLogEmptySet is the edge/negative case: a
// dispatch that resolves NO overlays still logs the empty set explicitly
// (`skill-overlays=[]`) so operators can tell "no overlay fired" apart from "the
// line never ran". Anti-no-op: a formatter that only handles the non-empty case
// fails here.
func TestC943_005_FormatSkillOverlayLogEmptySet(t *testing.T) {
	line := runner.FormatSkillOverlayLog("scout", nil, "balanced")
	if !strings.Contains(line, "skill-overlays=[]") {
		t.Errorf("empty-overlay log line %q must render skill-overlays=[]", line)
	}
	if !strings.Contains(line, "tier=balanced") {
		t.Errorf("empty-overlay log line %q must still carry tier=balanced", line)
	}
}

// =============================================================================
// T3 — wire overlay resolution into non-phase-runner dispatch sites
// =============================================================================

// subagentDeepProfileFS returns an in-memory profile the subagent runner loads.
func subagentDeepProfileFS() *profiles.Loader {
	return profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name":"builder","role":"builder","cli":"claude-p",
			"model_tier_default":"deep",
			"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
}

// runSubagent drives subagent.Run to the launch seam with the given model
// override and returns the capturing bridge. Run may return an error after
// Launch (the fake writes no artifact), but the BridgeRequest is already
// captured — this predicate asserts on what the dispatcher BUILT, not on the
// post-launch verification.
func runSubagent(t *testing.T, model string) *capturingBridge {
	t.Helper()
	br := &capturingBridge{}
	r, err := subagent.New(subagent.Config{
		Profiles: subagentDeepProfileFS(),
		Bridge:   br,
		Ledger:   nopLedger{},
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
		Rand:     fixedRand,
		GitState: func(context.Context, string) (string, string, error) {
			return "", "", errors.New("not a git repo")
		},
	})
	if err != nil {
		t.Fatalf("subagent.New: %v", err)
	}
	tmp := t.TempDir()
	_, _ = r.Run(context.Background(), subagent.Request{
		Agent:       "builder",
		Cycle:       943,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-943"),
		Prompt:      "do the work",
		Model:       model,
	})
	return br
}

// TestC943_007_SubagentDeepTierAttachesFableOverlay is the PRIMARY AC2 predicate
// for a non-phase dispatch site: subagent.Run must resolve the overlay set for a
// deep-tier launch and thread "fable" onto BridgeRequest.Skills, exactly as
// runner.go does. Today Skills is empty → RED.
func TestC943_007_SubagentDeepTierAttachesFableOverlay(t *testing.T) {
	br := runSubagent(t, "deep")
	got := br.firstCall(t).Skills
	if !contains(got, "fable") {
		t.Errorf("subagent deep-tier BridgeRequest.Skills = %v, want it to contain %q", got, "fable")
	}
}

// TestC943_009_SubagentBalancedTierAttachesNoOverlay is the NEGATIVE / selector
// predicate proving the wiring is TIER-GATED, not always-fable: a balanced
// (sonnet) launch resolves no overlay, so Skills stays empty. An implementation
// that unconditionally attaches "fable" fails here.
func TestC943_009_SubagentBalancedTierAttachesNoOverlay(t *testing.T) {
	br := runSubagent(t, "sonnet")
	got := br.firstCall(t).Skills
	if contains(got, "fable") {
		t.Errorf("subagent balanced-tier BridgeRequest.Skills = %v, want no fable overlay", got)
	}
}

// TestC943_008_RetroDeepTierAttachesFableOverlay covers the second non-phase
// site: retro.Phase.Run must attach the resolved overlay for its (deep) model
// tier onto the BridgeRequest it launches. Today retro builds BridgeRequest with
// no Skills field → RED.
func TestC943_008_RetroDeepTierAttachesFableOverlay(t *testing.T) {
	br := &capturingBridge{}
	phase := retro.New(retro.Config{
		Bridge:  br,
		Prompts: retroPromptsFS(),
		Model:   "deep",
		NowFn:   func() time.Time { return time.Unix(0, 0).UTC() },
	})
	// previous_verdict must be FAIL/WARN or retro SKIPs before dispatching.
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       943,
		ProjectRoot: t.TempDir(),
		Workspace:   t.TempDir(),
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	got := br.firstCall(t).Skills
	if !contains(got, "fable") {
		t.Errorf("retro deep-tier BridgeRequest.Skills = %v, want it to contain %q", got, "fable")
	}
}

// retroPromptsFS supplies the evolve-retrospective agent body retro.Run loads.
func retroPromptsFS() *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-retrospective.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-retrospective\n---\nretro body"),
		},
	})
}

// TestC943_010_OverlaysDocCommentNoLongerClaimsDeferred pins the stale-doc
// correction: overlays.go's header claimed the injection seam was "deferred to
// an out-of-cycle manual ship" while the phase-runner wiring already landed —
// left as-is a future Scout re-diagnoses this whole family as dormant. The
// Builder must correct it.
//
// acs-predicate: config-check — an "is this feature still deferred?" doc claim
// is an inherent text criterion, not a runnable code path; it has no behavioral
// SUT to exercise.
func TestC943_010_OverlaysDocCommentNoLongerClaimsDeferred(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go/internal/policy/overlays.go")
	acsassert.FileNotContains(t, path, "deferred to an out-of-cycle manual ship")
}

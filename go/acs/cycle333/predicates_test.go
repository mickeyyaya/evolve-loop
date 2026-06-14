//go:build acs

// Package cycle333 materializes the cycle-333 acceptance criteria for the one
// committed top_n task (scout-report.md "## Selected Tasks"):
//
//	hoist-subagent-json-extractors — replace the three per-call
//	    `regexp.MustCompile(fmt.Sprintf(...))` JSON field extractors in
//	    internal/subagent (extractProfileString in modeltier.go,
//	    extractInt in ctxadvisory.go, extractBoolField in dispatchparallel.go)
//	    with package-level static regexp vars + a thin matchField helper, so no
//	    dynamic regexp is compiled per call. Behaviour MUST be byte-identical;
//	    only the compilation timing (per-call → package init) changes.
//
// Predicate design (cycle-85 lesson — every gate EXERCISES the system under
// test, no load-bearing source grep):
//
//   - C333_001..003 are BEHAVIORAL + structural (the "Mixed" category): each
//     drives the real exported entry point that consumes one of the three
//     extractors (ResolveModelTier → extractProfileString, CheckCtxAdvisory →
//     extractInt, DispatchParallel → extractBoolField), asserting the extracted
//     value still steers the observable outcome (positive AND negative/missing-
//     field cases — the anti-no-op axis). The behavioral half passes today
//     (the refactor preserves behaviour); the auxiliary structural half
//     ("this file no longer compiles a regexp from fmt.Sprintf") is RED today
//     and is what fails until Builder hoists the pattern. So each predicate is
//     RED now for the RIGHT reason (refactor not done) yet pins behaviour so a
//     no-op or a behaviour-breaking edit cannot pass.
//
//   - C333_004 is the structural goal-completion gate (waived config/structure
//     check, see the inline waiver): across the three target files, ZERO
//     dynamic `regexp.MustCompile(fmt.Sprintf` calls remain AND the package
//     gained >= 5 static package-level regexp vars (the hoisted field
//     patterns). RED today (3 dynamic compiles present, only 3 static vars in
//     these files).
//
// Floor binding (R9.3): internal/subagent is the SOLE package the one committed
// top_n task targets — every predicate here binds committed work. No triage-
// DEFERRED item (guards/docdelete, quotareset) gets a predicate (cycle-280
// lesson: a deferred-task floor starves the committed task).
//
// AC map (1:1 with the task's Acceptance Criteria Summary):
//
//	AC-1 no dynamic regexp.MustCompile(fmt.Sprintf in prod subagent  → C333_004 (+ per-file in 001/002/003)
//	AC-2 >= 5 static package-level regexp vars added to subagent      → C333_004
//	AC-3 all subagent tests pass (behaviour preserved)               → C333_001..003 exercise the real APIs
//	AC-4 no regexp.MustCompile inside any surviving extractXxx body   → C333_004 (no dynamic compile anywhere)
//	AC-5 extractProfileString behaviour preserved                    → C333_001
//	AC-6 extractInt behaviour preserved                              → C333_002
//	AC-7 extractBoolField behaviour preserved                        → C333_003
package cycle333

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// the three production files that own the dynamic-compile extractors.
var subagentTargetFiles = []string{
	"go/internal/subagent/modeltier.go",
	"go/internal/subagent/ctxadvisory.go",
	"go/internal/subagent/dispatchparallel.go",
}

// prodPath joins a repo-relative path onto the git toplevel.
func prodPath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

// readProd returns the file content (Fatal on miss — a missing target file is a
// broken refactor, not a soft skip).
func readProd(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(prodPath(t, rel))
	if err != nil {
		t.Fatalf("RED: cannot read target file %s: %v", rel, err)
	}
	return string(b)
}

// assertNoDynamicCompile is the auxiliary structural half of the Mixed
// predicates: the named file must no longer compile a regexp from
// fmt.Sprintf (the per-call hot-path compile the task removes).
func assertNoDynamicCompile(t *testing.T, rel string) {
	t.Helper()
	if strings.Contains(readProd(t, rel), "regexp.MustCompile(fmt.Sprintf") {
		t.Errorf("RED: %s still calls regexp.MustCompile(fmt.Sprintf(...)) — hoist the pattern to a package-level var", rel)
	}
}

// ---------------------------------------------------------------------------
// C333_001 — extractProfileString behaviour, exercised via ResolveModelTier.
//
// ResolveModelTier reads role / name / model_tier_default OUT of the profile
// via extractProfileString. We pin every observable consequence of that
// extraction:
//   - role=="auditor" + streak<1            ⇒ "opus"   (role extracted)
//   - name=="auditor" fallback (no role)    ⇒ "opus"   (name fallback extracted)
//   - role=="builder"                       ⇒ model_tier_default ("haiku")
//   - model_tier_default absent             ⇒ error    (missing-field path)
// A broken/no-op extractor changes at least one of these, so the test is
// anti-no-op. Auxiliary structural: modeltier.go no longer fmt.Sprintf-compiles.
// ---------------------------------------------------------------------------
func TestC333_001_ResolveModelTierExtractsProfileStrings(t *testing.T) {
	resolve := func(profile string) (string, error) {
		return subagent.ResolveModelTier(
			subagent.ResolveModelTierRequest{ProfilePath: "in-memory"},
			subagent.ResolveModelTierOptions{
				ReadProfile: func(string) (string, error) { return profile, nil },
				// streak source absent ⇒ consecutiveSuccesses==0 (<1) ⇒ auditor opus rung.
				ReadState: func(string) (string, error) { return "", os.ErrNotExist },
			},
		)
	}

	// role extracted: auditor + streak<1 ⇒ opus (NOT the profile default sonnet).
	if tier, err := resolve(`{"role":"auditor","model_tier_default":"sonnet"}`); err != nil || tier != "opus" {
		t.Errorf("role extraction: got (%q, %v), want (\"opus\", nil) — auditor role not extracted", tier, err)
	}

	// name fallback extracted: no role, name=auditor ⇒ opus.
	if tier, err := resolve(`{"name":"auditor","model_tier_default":"sonnet"}`); err != nil || tier != "opus" {
		t.Errorf("name fallback: got (%q, %v), want (\"opus\", nil) — name not used when role absent", tier, err)
	}

	// model_tier_default extracted for a non-auditor.
	if tier, err := resolve(`{"role":"builder","model_tier_default":"haiku"}`); err != nil || tier != "haiku" {
		t.Errorf("model_tier_default extraction: got (%q, %v), want (\"haiku\", nil)", tier, err)
	}

	// negative: missing model_tier_default ⇒ error (empty-extraction path).
	if tier, err := resolve(`{"role":"builder"}`); err == nil {
		t.Errorf("missing model_tier_default: got (%q, nil), want error", tier)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/modeltier.go")
}

// ---------------------------------------------------------------------------
// C333_002 — extractInt behaviour, exercised via CheckCtxAdvisory.
//
// CheckCtxAdvisory pulls context_clear_trigger_tokens via extractInt and emits
// an advisory only when tokens exceed it. Pinned:
//   - threshold present, tokens>threshold   ⇒ Emit=true,  Threshold==value
//   - threshold present, tokens<=threshold  ⇒ Emit=false, Threshold==value
//   - threshold ABSENT                       ⇒ Emit=false (extractInt ok==false)
// CheckCtxAdvisory reads a real file, so we materialize fixtures on disk.
// Auxiliary structural: ctxadvisory.go no longer fmt.Sprintf-compiles.
// ---------------------------------------------------------------------------
func TestC333_002_CheckCtxAdvisoryExtractsIntThreshold(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return p
	}

	// positive: tokens over threshold ⇒ emit, threshold extracted as 1000.
	if res, err := subagent.CheckCtxAdvisory(write(`{"context_clear_trigger_tokens":1000}`), 2000); err != nil || !res.Emit || res.Threshold != 1000 {
		t.Errorf("over threshold: got %+v err=%v, want Emit=true Threshold=1000", res, err)
	}

	// boundary/negative: tokens under threshold ⇒ no emit, threshold still extracted.
	if res, err := subagent.CheckCtxAdvisory(write(`{"context_clear_trigger_tokens":1000}`), 500); err != nil || res.Emit || res.Threshold != 1000 {
		t.Errorf("under threshold: got %+v err=%v, want Emit=false Threshold=1000", res, err)
	}

	// missing-field: no trigger declared ⇒ no emit (extractInt ok==false path).
	if res, err := subagent.CheckCtxAdvisory(write(`{"role":"tester"}`), 2000); err != nil || res.Emit {
		t.Errorf("absent threshold: got %+v err=%v, want Emit=false", res, err)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/ctxadvisory.go")
}

// ---------------------------------------------------------------------------
// C333_003 — extractBoolField behaviour, exercised via DispatchParallel.
//
// DispatchParallel refuses to fan out unless profile.parallel_eligible is true
// (extractBoolField). Pinned by distinguishing the error message:
//   - parallel_eligible=false  ⇒ "not parallel_eligible" (false extracted)
//   - field ABSENT             ⇒ "not parallel_eligible" (default false)
//   - parallel_eligible=true   ⇒ passes the gate, fails later on
//                                "no parallel_subtasks" (true extracted)
// The true case proves the bool is genuinely read — a no-op returning false
// would never reach the subtasks error. Auxiliary structural: dispatchparallel.go
// no longer fmt.Sprintf-compiles.
// ---------------------------------------------------------------------------
func TestC333_003_DispatchParallelExtractsBoolField(t *testing.T) {
	dispatch := func(profile string) error {
		_, err := subagent.DispatchParallel(
			context.Background(),
			subagent.DispatchParallelRequest{
				Agent:         "scout",
				Cycle:         0,
				WorkspacePath: t.TempDir(),
				ProfilesDir:   t.TempDir(),
			},
			subagent.DispatchParallelOptions{
				ReadProfile: func(string) (string, error) { return profile, nil },
			},
		)
		return err
	}

	// false extracted ⇒ eligibility refusal.
	if err := dispatch(`{"parallel_eligible":false}`); err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("parallel_eligible=false: got err=%v, want a \"not parallel_eligible\" refusal", err)
	}

	// absent ⇒ default false ⇒ same refusal.
	if err := dispatch(`{"cli":"claude"}`); err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("parallel_eligible absent: got err=%v, want a \"not parallel_eligible\" refusal", err)
	}

	// true extracted ⇒ passes eligibility, fails LATER on missing subtasks.
	if err := dispatch(`{"parallel_eligible":true}`); err == nil || !strings.Contains(err.Error(), "no parallel_subtasks") {
		t.Errorf("parallel_eligible=true: got err=%v, want it to clear eligibility and fail on \"no parallel_subtasks\"", err)
	}

	assertNoDynamicCompile(t, "go/internal/subagent/dispatchparallel.go")
}

// ---------------------------------------------------------------------------
// C333_004 — structural goal-completion gate.
//
// acs-predicate: config-check — this gate inherently asserts a SOURCE-STRUCTURE
// outcome of a code-reduction goal (no dynamic compiles remain; the field
// patterns were hoisted to package-level vars). The behavioural exercise of the
// refactored code lives in C333_001..003; this gate pins that the per-call
// `regexp.MustCompile(fmt.Sprintf` hot path is gone across ALL three target
// files and that the hoisted static vars actually exist (so the patterns moved
// to package init rather than being deleted). Waiver per the Predicate Quality
// table (Waived grep).
// ---------------------------------------------------------------------------
func TestC333_004_SubagentRegexpHoisted(t *testing.T) {
	dynamicCompiles := 0
	staticVars := 0
	for _, rel := range subagentTargetFiles {
		body := readProd(t, rel)
		for _, line := range strings.Split(body, "\n") {
			if !strings.Contains(line, "regexp.MustCompile(") {
				continue
			}
			if strings.Contains(line, "fmt.Sprintf") {
				dynamicCompiles++
			} else {
				// a literal-pattern compile (package-level var or otherwise) —
				// these are the hoisted, compile-once forms.
				staticVars++
			}
		}
	}

	if dynamicCompiles != 0 {
		t.Errorf("RED: %d dynamic regexp.MustCompile(fmt.Sprintf(...)) calls remain across %v — want 0",
			dynamicCompiles, subagentTargetFiles)
	}

	// baseline static literal compiles in these 3 files today = 3
	// (consecutiveSuccessesRE, subtaskNameRE, subtaskTemplateRE); the 3 hoisted
	// field patterns + the matchField family must push this to >= 8.
	const wantStaticAtLeast = 8
	if staticVars < wantStaticAtLeast {
		t.Errorf("RED: only %d static literal regexp.MustCompile vars across %v — want >= %d (the hoisted field patterns)",
			staticVars, subagentTargetFiles, wantStaticAtLeast)
	}
}

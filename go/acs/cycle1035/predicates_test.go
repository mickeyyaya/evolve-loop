//go:build acs

// Package cycle1035 materialises the acceptance criteria for the single
// fleet-scoped task pinned to this lane: `guard-phase-hook-inert` (inbox
// weight 0.89), scoped as scout task `rewire-or-retire-guard-phase-hook`.
//
// THE DEFECT (verified live on this worktree). `go/internal/guards/phase.go`
// (`Phase.Decide`) only returns a non-Allow decision when `in.ToolName ==
// "Agent"`. But `.claude/settings.json` wires `evolve guard phase` EXCLUSIVELY
// under the `"Bash"` PreToolUse matcher, so the hook is only ever invoked with
// ToolName=="Bash", takes the `ToolName != "Agent"` early-return, and Allows
// unconditionally. The guard is a WIRED NO-OP: its one real branch can never
// fire. Meanwhile five doc surfaces assert that `evolve guard phase` enforces
// phase transitions / denies in-process Agent — enforcement that does not
// happen.
//
// DISPOSITION-AGNOSTIC BY DESIGN. The ticket names TWO valid fix shapes and
// the Builder chooses one; these predicates MUST green on EITHER and stay RED
// on the current wired-no-op state:
//
//   - REWIRE  → `.claude/settings.json` gains a matcher that covers the Agent
//     (subagent-dispatch) tool, so `Phase.Decide`'s Agent branch can
//     actually fire.
//   - RETIRE  → the guard source is removed and `evolve guard phase` is no
//     longer wired, with docs re-pointed at the state machine
//     (`go/internal/core`) as the real enforcement point.
//
// The load-bearing test is 001: it reads the REAL consumed config artifact
// (`.claude/settings.json`) plus the guard source presence and asserts the
// guard is no longer a wired no-op — a genuine end-to-end wiring assertion, not
// a source-grep of a magic fix-string (the cycle-85 degenerate-predicate ban).
// 003 couples DOC truth to that same wiring determination: a doc may claim
// `evolve guard phase` enforces phase order ONLY when the wiring actually makes
// it fire.
//
// Roots: settings.json / phase.go / docs / the ADR are all Builder deliverables
// in this cycle's WORKTREE, so they are read under acsassert.RepoRoot(t) (the
// worktree). Their absence / un-fixed state is a FAILURE this cycle, not a skip.
package cycle1035

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---- shared wiring model ------------------------------------------------

type settingsFile struct {
	Hooks struct {
		PreToolUse []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"PreToolUse"`
	} `json:"hooks"`
}

// matcherToken splits a PreToolUse matcher (e.g. "Edit|Write", "Bash",
// "WebSearch|WebFetch|Bash") into its individual tool tokens.
var matcherToken = regexp.MustCompile(`[A-Za-z]+`)

// guardPhaseWiring reads `.claude/settings.json` under the worktree root and
// answers two questions about the `evolve guard phase` hook:
//
//	wired          — is `guard phase` invoked by ANY PreToolUse matcher?
//	coversAgent    — does at least one matcher that invokes it cover the
//	                 in-process subagent-dispatch tool (Agent or Task)?
//
// It t.Fatalf's only when settings.json is unreadable (a mandatory artifact).
func guardPhaseWiring(t *testing.T) (wired, coversAgent bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf(".claude/settings.json not readable (mandatory config artifact): %v", err)
	}
	var sf settingsFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatalf(".claude/settings.json is not valid JSON: %v", err)
	}
	for _, entry := range sf.Hooks.PreToolUse {
		invokesGuardPhase := false
		for _, h := range entry.Hooks {
			if strings.Contains(h.Command, "guard phase") {
				invokesGuardPhase = true
				break
			}
		}
		if !invokesGuardPhase {
			continue
		}
		wired = true
		for _, tok := range matcherToken.FindAllString(entry.Matcher, -1) {
			if tok == "Agent" || tok == "Task" {
				coversAgent = true
			}
		}
	}
	return wired, coversAgent
}

// phaseGuardSourcePresent reports whether the guard source file still exists in
// the worktree (retire removes it).
func phaseGuardSourcePresent(t *testing.T) bool {
	t.Helper()
	root := acsassert.RepoRoot(t)
	_, err := os.Stat(filepath.Join(root, "go", "internal", "guards", "phase.go"))
	return err == nil
}

// ---- 001: the crux — guard is no longer a wired no-op -------------------

// TestC1035_001_PhaseGuardNotWiredNoop — AC1 (rewire-or-retire), the crux.
// The phase guard must no longer be a wired no-op. Disposition-agnostic:
//
//   - REWIRE branch: `guard phase` is wired AND at least one matcher invoking
//     it covers the Agent/Task tool, so its `ToolName=="Agent"` branch can
//     fire. (Current tree wires it under "Bash" only → coversAgent==false → RED.)
//   - RETIRE branch: `guard phase` is NOT wired anywhere AND `phase.go` is
//     removed, so no orphaned inert guard remains.
//
// The CURRENT wired-no-op state (wired, but not covering Agent) satisfies
// neither branch → RED now. Reads the real consumed config artifact, not a
// source magic-string.
func TestC1035_001_PhaseGuardNotWiredNoop(t *testing.T) {
	wired, coversAgent := guardPhaseWiring(t)
	sourcePresent := phaseGuardSourcePresent(t)

	switch {
	case wired && coversAgent:
		// REWIRE satisfied: the guard's Agent branch can now actually fire.
		return
	case !wired && !sourcePresent:
		// RETIRE satisfied: guard unwired and its source removed.
		return
	case wired && !coversAgent:
		t.Errorf("phase guard is a WIRED NO-OP: `evolve guard phase` is wired in " +
			".claude/settings.json but NO matcher covers the Agent/Task tool it branches on — " +
			"its one real branch can never fire (rewire must add an Agent matcher; retire must unwire it)")
	case !wired && sourcePresent:
		t.Errorf("`evolve guard phase` is no longer wired but go/internal/guards/phase.go " +
			"still exists — retire must remove the orphaned inert guard source (or rewire must re-wire it)")
	default:
		t.Errorf("unreachable wiring state: wired=%v coversAgent=%v sourcePresent=%v", wired, coversAgent, sourcePresent)
	}
}

// ---- 002: the fix is a real recorded decision, not a silent patch -------

// adrGlob matches the ADR the ticket requires (a real rewire-vs-retire
// decision, per the [[no_workaround_root_cause_redesign]] /
// [[ultrathink_architecture_strong_review]] standing rules).
func findGuardPhaseADR(t *testing.T) (string, bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "architecture", "adr", "*guard-phase*.md"))
	if err != nil {
		t.Fatalf("glob ADR dir: %v", err)
	}
	if len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// TestC1035_002_DecisionRecordedAsADR — AC1 (rewire-or-retire), decision half.
// The fix MUST be a real recorded architecture decision, not a silent patch:
// a `docs/architecture/adr/*guard-phase*.md` ADR that (a) explicitly weighs
// BOTH alternatives (mentions "rewire" AND "retire") and (b) names the state
// machine (`internal/core`) as the authoritative phase-order enforcement point
// the ticket's evidence identifies. Reads the emitted ADR artifact; RED now
// because no such ADR exists on this tree (only 0073 does).
func TestC1035_002_DecisionRecordedAsADR(t *testing.T) {
	path, ok := findGuardPhaseADR(t)
	if !ok {
		t.Fatalf("no docs/architecture/adr/*guard-phase*.md ADR exists — the rewire-or-retire fix " +
			"must be recorded as a real architecture decision, not a silent patch")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ADR %s: %v", filepath.Base(path), err)
	}
	body := strings.ToLower(string(raw))

	if !strings.Contains(body, "rewire") || !strings.Contains(body, "retire") {
		t.Errorf("ADR %s does not weigh BOTH alternatives (must mention 'rewire' AND 'retire' to be a real decision)", filepath.Base(path))
	}
	// The ticket's own evidence: the state machine (internal/core) is the real
	// enforcement point this guard is redundant with. The ADR must name it.
	if !strings.Contains(body, "internal/core") && !strings.Contains(body, "state machine") && !strings.Contains(body, "statemachine") {
		t.Errorf("ADR %s does not name the state machine (internal/core) as the authoritative phase-order enforcement point", filepath.Base(path))
	}
	// A decision must record a Status (accepted/superseded/etc.) — proves it is
	// a real ADR, not a scratch note.
	if !strings.Contains(body, "status") {
		t.Errorf("ADR %s has no Status section — not a well-formed ADR", filepath.Base(path))
	}
}

// ---- 003: doc truth is coupled to the actual wiring ---------------------

// docSurfaceClaim is a doc surface plus the concrete, currently-present clause
// that asserts `evolve guard phase` performs live phase enforcement. Each
// clause is matched on WHITESPACE-NORMALISED content so wording tweaks that
// keep the false assertion still trip, while legitimate rewrites remove it.
type docSurfaceClaim struct {
	rel     string
	needles []string // ALL must be present (normalised) for the claim to count as asserted
}

var wsRun = regexp.MustCompile(`\s+`)

func normWS(s string) string { return wsRun.ReplaceAllString(s, " ") }

// TestC1035_003_NoStaleEnforcementDocClaim — AC2 (docs corrected in lockstep).
// Doc truth must match the wiring: a doc surface may assert that `evolve guard
// phase` enforces phase transitions / denies in-process Agent ONLY when the
// guard is actually wired to fire on the Agent tool.
//
//   - If wired-to-Agent (REWIRE): the claims are true → allowed → PASS.
//   - Otherwise (RETIRE, or the current unfixed tree): every stale
//     false-enforcement clause across the code-adjacent doc surfaces must be
//     gone. Current tree = not-wired-to-Agent AND all clauses present → RED.
//
// The remaining two surfaces named by the ticket (the settings.json `_comment`
// and the ADR itself) are covered by 002 and the manual+checklist in the
// TDD report, keeping this predicate to precise, verified clause strings.
func TestC1035_003_NoStaleEnforcementDocClaim(t *testing.T) {
	_, coversAgent := guardPhaseWiring(t)
	if coversAgent {
		// Rewire path: the guard fires on Agent, so the enforcement claims are
		// accurate. No doc correction is required for this AC.
		return
	}

	surfaces := []docSurfaceClaim{
		{
			rel:     filepath.Join("docs", "operations", "runtime-reference.md"),
			needles: []string{"`evolve guard phase` precondition whenever `cycle-state.json` exists"},
		},
		{
			rel:     "CLAUDE.md",
			needles: []string{"Phase gate at every transition", "`evolve guard phase`"},
		},
		{
			rel:     filepath.Join("skills", "loop", "SKILL.md"),
			needles: []string{"Phase transitions are enforced by", "`evolve guard phase`"},
		},
	}

	root := acsassert.RepoRoot(t)
	for _, s := range surfaces {
		raw, err := os.ReadFile(filepath.Join(root, s.rel))
		if err != nil {
			// A surface the ticket names must exist to be corrected.
			t.Errorf("doc surface %s not readable: %v", s.rel, err)
			continue
		}
		content := normWS(string(raw))
		allPresent := true
		for _, n := range s.needles {
			if !strings.Contains(content, normWS(n)) {
				allPresent = false
				break
			}
		}
		if allPresent {
			t.Errorf("%s still asserts `evolve guard phase` enforces phase transitions, but the guard is "+
				"NOT wired to fire on the Agent tool — retire must correct this claim (needles: %v)", s.rel, s.needles)
		}
	}
}

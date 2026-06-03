// Package audit implements the EGPS gate phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// audit-specific variation points.
//
// Audit is the EGPS gate: PASS requires BOTH a parseable PASS verdict
// in audit-report.md AND red_count == 0 in acs-verdict.json.
// EVOLVE_STRICT_AUDIT=1 additionally promotes WARN to FAIL.
//
// Verdict mapping:
//   - empty artifact / no parseable verdict declaration → FAIL
//   - acs-verdict.json missing or unparseable → FAIL + error diag
//   - acs-verdict.json red_count > 0 → FAIL + EGPS diag
//   - WARN + EVOLVE_STRICT_AUDIT=1 → FAIL
//   - otherwise → whatever verdict the audit-report.md declares (PASS/WARN/FAIL/SKIPPED)
//
// The verdict declaration is recognized in several agent-produced shapes —
// canonical "## Verdict\n**PASS**" AND single-line variants like
// "**Verdict: PASS**" or "Verdict: PASS". Prose formatting varies by CLI, so
// the gate must not hinge on one exact shape: a genuine PASS written as
// "**Verdict: PASS**" with red_count==0 must not be mis-graded FAIL (the
// cycle-148 silent-no-ship bug). When the verdict is unparseable but the EGPS
// suite is green, a loud diagnostic is emitted (never a silent FAIL).
//
// Default model is "opus" for adversarial cross-family diversity from
// the build phase's Sonnet.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// verdictCanonicalRE matches the canonical two-line heading
// "## Verdict\n**PASS**" — bold optional, intervening blank lines tolerated.
var verdictCanonicalRE = regexp.MustCompile(`(?m)^##[^\S\n]*Verdict[^\S\n]*\n\s*\*{0,2}(PASS|FAIL|WARN|SKIPPED)\*{0,2}`)

// verdictInlineRE matches single-line variants agents emit when they don't
// follow the canonical heading: "**Verdict: PASS**", "**Verdict:** PASS",
// "## Verdict: PASS", "Verdict: PASS". Horizontal-whitespace classes keep the
// match on one line. Case-sensitive on "Verdict" (capital V) so it never
// matches the lowercase JSON key "verdict" in an embedded result blob. The
// colon is REQUIRED: every real inline form has one, and requiring it stops a
// prose line like "Verdict PASS is required before shipping." from being
// mis-read as a PASS declaration on the ship gate. The no-colon canonical
// "## Verdict\n**PASS**" shape is covered by verdictCanonicalRE above.
var verdictInlineRE = regexp.MustCompile(`(?m)^[^\S\n]*(?:##[^\S\n]*)?\*{0,2}Verdict\*{0,2}[^\S\n]*:[^\S\n]*\*{0,2}[^\S\n]*(PASS|FAIL|WARN|SKIPPED)\b`)

// hooks carries the audit phase's variation points. genVerdict is the
// seam that generates acs-verdict.json when it is absent (cycle-138/139
// fix): the autonomous loop never ran `evolve acs suite`, so the EGPS
// gate was forced to FAIL on the missing file every cycle. nil = no
// generation (a pre-staged file is then required, the legacy behavior).
type hooks struct {
	genVerdict func(req core.PhaseRequest) error
}

func (hooks) PhaseName() string                           { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string                     { return "evolve-auditor" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "audit-report.md" }
func (hooks) DefaultModel() string                        { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	return b.String()
}

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, verdictFound := extractAuditVerdict(artifact)
	if !verdictFound {
		verdict = core.VerdictFAIL
	}
	var diags []core.Diagnostic

	verdictPath := filepath.Join(req.Workspace, "acs-verdict.json")
	// Generate acs-verdict.json when absent and a generator is wired.
	// Pre-staged files (operator/CI) are honored untouched. If generation
	// writes nothing (zero predicates), the missing-file FAIL floor holds.
	if h.genVerdict != nil {
		if _, statErr := os.Stat(verdictPath); os.IsNotExist(statErr) {
			if genErr := h.genVerdict(req); genErr != nil {
				diags = append(diags, core.Diagnostic{
					Severity: "warning",
					Message:  fmt.Sprintf("acs-verdict generation failed: %s", genErr.Error()),
				})
			}
		}
	}

	redCount, acsErr := readRedCount(verdictPath)
	if acsErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("acs-verdict.json: %s", acsErr.Error()),
		})
		verdict = core.VerdictFAIL
	} else if redCount > 0 {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("EGPS: red_count=%d (cycle ships only when red_count==0)", redCount),
		})
		verdict = core.VerdictFAIL
	}

	if verdict == core.VerdictWARN && req.Env["EVOLVE_STRICT_AUDIT"] == "1" {
		verdict = core.VerdictFAIL
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "EVOLVE_STRICT_AUDIT=1 promoted WARN to FAIL",
		})
	}

	// A non-empty audit report whose verdict we could not parse, while the EGPS
	// predicate suite is itself green (red_count==0), is almost always a verdict
	// FORMAT miss — not a real defect (cycle-148: the agent wrote
	// "**Verdict: PASS**" but the parser required "## Verdict\n**PASS**", so a
	// genuine PASS was mis-graded FAIL and routed to retro, silently discarding
	// the cycle's work). FAIL loudly so the mis-grade is visible instead of
	// sinking the cycle without a trace.
	if !verdictFound && acsErr == nil && redCount == 0 && strings.TrimSpace(artifact) != "" {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "audit-report.md is non-empty with red_count=0 but declares no parseable verdict — treating as FAIL. Declare it as '## Verdict' + a bold verdict on the next line, or inline as '**Verdict: PASS**'.",
		})
	}
	return verdict, diags, string(core.PhaseShip)
}

// extractAuditVerdict returns the declared verdict word and whether a
// parseable verdict declaration was found. It tries the canonical
// "## Verdict\n**PASS**" heading first, then common single-line variants. The
// found bool lets the caller distinguish "no verdict declared" (a format miss
// worth a loud diagnostic) from an explicit "FAIL". A real FAIL/WARN/SKIPPED
// declaration is captured verbatim, so broadening the accepted FORMATS never
// turns a real non-PASS verdict into a PASS.
func extractAuditVerdict(content string) (string, bool) {
	// Layer-5 strangler: the machine-readable sentinel wins when present; the
	// legacy regex-on-prose remains the fallback for reports written against the
	// older templates.
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
	if m := verdictCanonicalRE.FindStringSubmatch(content); m != nil {
		return m[1], true
	}
	if m := verdictInlineRE.FindStringSubmatch(content); m != nil {
		return m[1], true
	}
	return "", false
}

func readRedCount(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	var v struct {
		RedCount int `json:"red_count"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	return v.RedCount, nil
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// GenerateVerdict, when set, produces <workspace>/acs-verdict.json from
	// the cycle's ACS predicates if the file is absent (cycle-138/139 fix).
	// nil = no generation (legacy: a pre-staged file is required to PASS).
	// The registry default wires generateACSVerdict (runs acssuite).
	GenerateVerdict func(req core.PhaseRequest) error
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{genVerdict: c.GenerateVerdict},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
		}),
	}
}

// NewDefault builds the audit phase with production defaults — notably
// GenerateVerdict wired to generateACSVerdict so the EGPS gate auto-generates
// acs-verdict.json when the auditor agent leaves it absent (cycle-138/139 fix).
// BOTH the registry init() and the loop's runner map (go/cmd/evolve/cmd_cycle.go)
// MUST construct audit via this single seam so the generator can never again be
// wired in one phase-construction path but dormant in the other — the
// dual-source divergence that left the loop force-FAILing on a missing verdict
// every cycle (cycle-147). New(Config) stays for tests that pin explicit
// (nil or fake) generators.
func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {
	return New(Config{Bridge: br, Prompts: prm, GenerateVerdict: generateACSVerdict})
}

// generateACSVerdict runs the ACS predicate suite for req.Cycle and writes
// <workspace>/acs-verdict.json. It runs the predicates discovered under the
// cycle's worktree (where this cycle's acs/cycle-N/*.sh live), falling back
// to the project root. When the suite discovers ZERO predicates it writes
// NOTHING — the audit's missing-file FAIL floor then holds, so a cycle with
// no predicates cannot auto-pass. EvolveDir is derived from the workspace
// (<evolveDir>/runs/cycle-N), matching where audit reads the verdict.
func generateACSVerdict(req core.PhaseRequest) error {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	// Discover predicate FILES from the worktree (Root), but resolve `.evolve/`
	// runtime data (history, baselines, current build-report) to the MAIN project
	// root via EVOLVE_PROJECT_ROOT — those live in main, not the worktree, so a
	// suite run from the worktree (issue #9 audit-cwd=worktree) would else false-RED
	// every regression predicate that reads .evolve/ (issue #12, cycle-177).
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return fmt.Errorf("acssuite run: %w", err)
	}
	if v.PredicateSuite.Total == 0 {
		// No predicates → leave the file absent so the EGPS floor fails the
		// cycle rather than auto-passing it on an empty suite.
		return nil
	}
	// evolveDir = parent of runs/, i.e. dirname(dirname(workspace)).
	evolveDir := filepath.Dir(filepath.Dir(req.Workspace))
	if _, err := acssuite.WriteVerdict(evolveDir, v); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

func init() {
	registry.Register(string(core.PhaseAudit), func(req core.PhaseRequest) core.PhaseRunner {
		return NewDefault(bridge.NewDefault(req.ProjectRoot), prompts.NewForProject(req.ProjectRoot))
	})
}

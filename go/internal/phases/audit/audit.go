// Package audit implements the EGPS gate phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// audit-specific variation points.
//
// Audit is the EGPS gate: PASS requires BOTH a parseable PASS verdict
// in audit-report.md AND red_count == 0 in acs-verdict.json.
// policy.json workflow.strict_audit additionally promotes WARN to FAIL.
//
// Verdict mapping:
//   - empty artifact / no parseable verdict declaration → FAIL
//   - acs-verdict.json missing or unparseable → FAIL + error diag
//   - acs-verdict.json red_count > 0 → FAIL + EGPS diag
//   - WARN + workflow.strict_audit → FAIL
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
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
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
	// gofmtCheck reports the worktree's .go files that are not gofmt -s clean.
	// It is the CI-parity gate that stops a cycle shipping a gofmt regression
	// to main (cycles 339-341 shipped CI-red because the cycle-scoped audit
	// never ran gofmt over the generated go/acs/cycle<N>/*.go files). nil = no
	// gofmt gate (legacy/tests). The registry default wires gofmtCheckDefault.
	gofmtCheck func(req core.PhaseRequest) ([]string, error)
	// skillsDriftCheck reports the worktree's SKILL.md files whose generated
	// phase-facts region has drifted from its SSOTs (profiles/registry/
	// phasecontract). A cycle that edits .evolve/profiles/*.json without
	// regenerating would FAIL the CI TestSkills_NoDrift gate (cycle 339), so the
	// drift must FAIL audit. nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault (in-process skillcheck.Check — no subprocess).
	skillsDriftCheck func(req core.PhaseRequest) ([]string, error)
	// phaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). At >= StageEnforce the evolve-verdict sentinel is mandatory —
	// the legacy prose/regex fallbacks are gated off. Zero value (StageOff) keeps
	// every path active, byte-identical.
	phaseIO config.Stage
}

func (hooks) PhaseName() string                           { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string                     { return "evolve-auditor" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "audit-report.md" }
func (hooks) DefaultModel() string                        { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	return b.String()
}

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, verdictFound := extractAuditVerdict(artifact, h.phaseIO)
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

	// gofmt CI-parity gate: a cycle whose worktree has any non-gofmt-s-clean
	// .go file would FAIL the CI `vet + fmt` step, so it must FAIL audit here —
	// never ship green-locally/red-in-CI (cycles 339-341). An infra error
	// (gofmt missing, unparseable source) fails OPEN with a loud warning: the
	// gate's own inability to run must not brick the cycle, but is never
	// silently treated as clean.
	if h.gofmtCheck != nil {
		dirty, gerr := h.gofmtCheck(req)
		switch {
		case gerr != nil:
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("gofmt gate skipped (could not run): %s", gerr.Error()),
			})
		case len(dirty) > 0:
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("gofmt: %d file(s) are not gofmt -s clean — CI `vet + fmt` would FAIL. Run `gofmt -w -s .` in go/. Offenders: %s", len(dirty), strings.Join(dirty, ", ")),
			})
			verdict = core.VerdictFAIL
		}
	}

	// SKILL.md phase-facts drift gate: a cycle that edits .evolve/profiles/*.json
	// (or any SSOT projected into SKILL.md) without regenerating would FAIL the
	// CI TestSkills_NoDrift gate — so it must FAIL audit here (cycle 339 shipped
	// this drift CI-red). Runs in-process (skillcheck.Check); an infra error
	// (e.g. the worktree can't load the registry) fails OPEN with a warning.
	if h.skillsDriftCheck != nil {
		drift, derr := h.skillsDriftCheck(req)
		switch {
		case derr != nil:
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("skills-drift gate skipped (could not run): %s", derr.Error()),
			})
		case len(drift) > 0:
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("SKILL.md drift: %d region(s) stale vs their SSOTs — CI TestSkills_NoDrift would FAIL. Run `evolve skills generate`. Drifted: %s", len(drift), strings.Join(drift, ", ")),
			})
			verdict = core.VerdictFAIL
		}
	}

	if verdict == core.VerdictWARN && policy.StrictAuditFor(req.ProjectRoot) {
		verdict = core.VerdictFAIL
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "policy.json workflow.strict_audit promoted WARN to FAIL",
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
func extractAuditVerdict(content string, stage config.Stage) (string, bool) {
	// Layer-5 strangler: the machine-readable sentinel wins when present; the
	// legacy regex-on-prose remains the fallback for reports written against the
	// older templates.
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
	// ADR-0050 §3.10 Slice 5: the regex-on-prose fallbacks serve reports written
	// against older templates; at enforce the sentinel above is mandatory, so gate
	// them off (>= StageEnforce). Below enforce they stay active — byte-identical.
	if stage < config.StageEnforce {
		if m := verdictCanonicalRE.FindStringSubmatch(content); m != nil {
			return m[1], true
		}
		if m := verdictInlineRE.FindStringSubmatch(content); m != nil {
			return m[1], true
		}
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
	// CheckGofmt, when set, reports the worktree .go files that are not
	// gofmt -s clean; any offender FAILs the audit (CI-parity gate). nil = no
	// gofmt gate. NewDefault wires gofmtCheckDefault.
	CheckGofmt func(req core.PhaseRequest) ([]string, error)
	// CheckSkillsDrift, when set, reports the worktree SKILL.md files whose
	// phase-facts region drifted from its SSOTs; any drift FAILs the audit
	// (CI TestSkills_NoDrift parity). nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault.
	CheckSkillsDrift func(req core.PhaseRequest) ([]string, error)
	// PhaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). Zero value (StageOff) = byte-identical (prose fallbacks active).
	PhaseIO config.Stage
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{genVerdict: c.GenerateVerdict, gofmtCheck: c.CheckGofmt, skillsDriftCheck: c.CheckSkillsDrift, phaseIO: c.PhaseIO},
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
	return NewDefaultWithStage(br, prm, config.StageOff)
}

// NewDefaultWithStage is NewDefault plus the EVOLVE_PHASE_IO stage (ADR-0050 §3.10
// Slice 5). The composition root (cmd_cycle.go) passes cfg.PhaseIO so the audit
// verdict extraction enforces the sentinel at >= StageEnforce. NewDefault stays as
// the StageOff (byte-identical) convenience for the registry init() and tests.
func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {
	return New(Config{Bridge: br, Prompts: prm, GenerateVerdict: generateACSVerdict, CheckGofmt: gofmtCheckDefault, CheckSkillsDrift: skillsDriftCheckDefault, PhaseIO: stage})
}

// skillsDriftCheckDefault is the production SKILL.md-drift gate: it runs the
// same projection check the CI TestSkills_NoDrift gate runs (skillcheck.Check)
// against the cycle worktree (where the builder's profile/SSOT edits live),
// in-process — no subprocess, so no fork-bomb under `go test`. The worktree is
// preferred; ProjectRoot is the fallback; an empty root is a no-op.
func skillsDriftCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	return skillcheck.Check(root)
}

// gofmtCheckDefault is the production gofmt CI-parity gate: it lists the .go
// files under the cycle worktree's go/ module that are not gofmt -s clean,
// matching CI's `gofmt -d -s .`. The worktree (where the builder wrote this
// cycle's changes) is preferred; ProjectRoot is the fallback. When no go/
// module exists the gate is a no-op (returns nil) rather than an error.
func gofmtCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	// ModuleDir is the single source for "where the .go files live", shared with
	// the post-build gofmt normalizer (normalizeBuildGofmt) so the gate and the
	// normalizer can never disagree on which tree to verify vs. format.
	return codequality.UnformattedGoFiles(codequality.ModuleDir(root))
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

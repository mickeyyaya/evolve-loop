package phasespec_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// repoRoot returns the repo root, located from this file's path.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestResearchPhasesAreConfigOnly(t *testing.T) {
	root := repoRoot(t)
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	user, warns := phasespec.DiscoverUserSpecs(filepath.Join(root, ".evolve", "phases"))
	for _, w := range warns {
		t.Logf("discover warning: %s", w)
	}
	if tracked := phasespec.TrackedUserPhaseNames(t, root); tracked != nil {
		kept := make([]phasespec.PhaseSpec, 0, len(user))
		for _, s := range user {
			if !tracked[s.Name] {
				t.Logf("untracked user phase %q: runtime/local state, not bound", s.Name)
				continue
			}
			kept = append(kept, s)
		}
		user = kept
	}
	if len(user) == 0 {
		t.Fatal("DiscoverUserSpecs found no user phases — is .evolve/phases/ present at the repo root?")
	}
	cat, mWarns := builtin.Merge(user)
	for _, w := range mWarns {
		t.Logf("merge warning: %s", w)
	}

	type want struct {
		artifact   string
		sections   []string
		hasVerdict bool
	}
	// Plan and control phases may carry verdict_on_pass, but contract derivation
	// leaves it inert, so their hasVerdict is false.
	// See ADR-0035.
	cases := map[string]want{
		"adversarial-review": {
			artifact:   "adversarial-review-report.md",
			sections:   []string{"## Threat Model", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"perf-profile": {
			artifact:   "perf-profile-report.md",
			sections:   []string{"## Benchmarks", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"incident-postmortem": {
			artifact:   "incident-postmortem-report.md",
			sections:   []string{"## Impact", "## Timeline", "## Root Cause", "## Action Items"},
			hasVerdict: true,
		},
		"runbook-draft": {
			artifact:   "runbook-draft-report.md",
			sections:   []string{"## Trigger", "## Diagnosis", "## Resolution Steps", "## Escalation"},
			hasVerdict: false,
		},
		"capacity-plan": {
			artifact:   "capacity-plan-report.md",
			sections:   []string{"## Demand Forecast", "## Current Capacity", "## Capacity Gap"},
			hasVerdict: false,
		},
		"account-reconcile": {
			artifact:   "account-reconcile-report.md",
			sections:   []string{"## GL vs Source Balance", "## Reconciling Items", "## Adjustments", "## Sign-off"},
			hasVerdict: true,
		},
		"variance-analysis": {
			artifact:   "variance-analysis-report.md",
			sections:   []string{"## Budget vs Actual", "## Classification", "## Drivers", "## Reforecast Impact"},
			hasVerdict: true,
		},
		"close-checklist": {
			artifact:   "close-checklist-report.md",
			sections:   []string{"## Tasks", "## Blocking Items", "## Sign-off"},
			hasVerdict: false,
		},
		"risk-register": {
			artifact:   "risk-register-report.md",
			sections:   []string{"## Risks", "## Scoring", "## Response Strategies", "## Owners"},
			hasVerdict: false,
		},
		"scope-baseline": {
			artifact:   "scope-baseline-report.md",
			sections:   []string{"## Deliverables", "## Acceptance Criteria", "## Exclusions", "## Constraints and Assumptions"},
			hasVerdict: false,
		},
		"dependency-map": {
			artifact:   "dependency-map-report.md",
			sections:   []string{"## Dependencies", "## Critical Path", "## Blockers"},
			hasVerdict: true,
		},
		"forces-analysis": {
			artifact:   "forces-analysis-report.md",
			sections:   []string{"## Competitive Rivalry", "## Buyer and Supplier Power", "## Entry and Substitute Threats", "## Attractiveness Verdict"},
			hasVerdict: true,
		},
		"market-sizing": {
			artifact:   "market-sizing-report.md",
			sections:   []string{"## TAM", "## SAM", "## SOM", "## Methodology and Assumptions"},
			hasVerdict: true,
		},
		"okr-draft": {
			artifact:   "okr-draft-report.md",
			sections:   []string{"## Objective", "## Key Results", "## Confidence and Scoring"},
			hasVerdict: false,
		},
		"opportunity-map": {
			artifact:   "opportunity-map-report.md",
			sections:   []string{"## Desired Outcome", "## Opportunities", "## Candidate Solutions", "## Assumption Tests"},
			hasVerdict: false,
		},
		"prd-draft": {
			artifact:   "prd-draft-report.md",
			sections:   []string{"## Problem", "## Goals and Success Metrics", "## Requirements", "## Out of Scope"},
			hasVerdict: false,
		},
		"metric-tree": {
			artifact:   "metric-tree-report.md",
			sections:   []string{"## North Star Metric", "## Input Metrics", "## Guardrail Metrics"},
			hasVerdict: true,
		},
		"premise-challenge": {
			artifact:   "premise-challenge-report.md",
			sections:   []string{"## Stated Premise", "## Falsification Attempts", "## Verdict"},
			hasVerdict: true,
		},
		"coverage-gate": {
			artifact:   "coverage-gate-report.md",
			sections:   []string{"## Coverage Delta", "## Uncovered Changed Lines", "## Verdict"},
			hasVerdict: true,
		},
		"secret-leak-scan": {
			artifact:   "secret-leak-scan-report.md",
			sections:   []string{"## Scanned Diff", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"flake-rerun-scan": {
			artifact:   "flake-rerun-scan-report.md",
			sections:   []string{"## Tests Re-run", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"race-condition-scan": {
			artifact:   "race-condition-scan-report.md",
			sections:   []string{"## Concurrent Surfaces Touched", "## Findings", "## Verdict"},
			hasVerdict: true,
		},
		"authz-gap-scan": {
			artifact:   "authz-gap-scan-report.md",
			sections:   []string{"## Protected Resources Touched", "## Authorization Findings", "## Verdict"},
			hasVerdict: true,
		},
		"compat-surface-check": {
			artifact:   "compat-surface-check-report.md",
			sections:   []string{"## Exported Surface Diff", "## Breaking Changes", "## Verdict"},
			hasVerdict: true,
		},
		"contract-fuzz-probe": {
			artifact:   "contract-fuzz-probe-report.md",
			sections:   []string{"## Boundaries Probed", "## Validation Findings", "## Verdict"},
			hasVerdict: true,
		},
		"migration-safety-check": {
			artifact:   "migration-safety-check-report.md",
			sections:   []string{"## Migration Operations", "## Reversibility Analysis", "## Verdict"},
			hasVerdict: true,
		},
		"telemetry-coverage-check": {
			artifact:   "telemetry-coverage-check-report.md",
			sections:   []string{"## New Code Paths", "## Instrumentation Gaps", "## Verdict"},
			hasVerdict: true,
		},
		"license-provenance-audit": {
			artifact:   "license-provenance-audit-report.md",
			sections:   []string{"## New Dependencies", "## License & Provenance Findings", "## Verdict"},
			hasVerdict: true,
		},
		"prompt-regression-eval": {
			artifact:   "prompt-regression-eval-report.md",
			sections:   []string{"## Instruction Changes", "## Behavioral Rubric Scores", "## Verdict"},
			hasVerdict: true,
		},
		"accessibility-audit": {
			artifact:   "accessibility-audit-report.md",
			sections:   []string{"## Components Audited", "## WCAG Findings", "## Verdict"},
			hasVerdict: true,
		},
		"frontend-design-review": {
			artifact:   "frontend-design-review-report.md",
			sections:   []string{"## UI Changes", "## Design Findings", "## Verdict"},
			hasVerdict: true,
		},
		"locale-format-check": {
			artifact:   "locale-format-check-report.md",
			sections:   []string{"## Localized Surfaces", "## Formatting Findings", "## Verdict"},
			hasVerdict: true,
		},
		"query-performance-scan": {
			artifact:   "query-performance-scan-report.md",
			sections:   []string{"## Queries Touched", "## Performance Findings", "## Verdict"},
			hasVerdict: true,
		},
		"cache-strategy-scan": {
			artifact:   "cache-strategy-scan-report.md",
			sections:   []string{"## Cache Sites Touched", "## Coherence Findings", "## Verdict"},
			hasVerdict: true,
		},
		"resilience-gap-scan": {
			artifact:   "resilience-gap-scan-report.md",
			sections:   []string{"## External Call Sites", "## Resilience Findings", "## Verdict"},
			hasVerdict: true,
		},
		"idempotency-check": {
			artifact:   "idempotency-check-report.md",
			sections:   []string{"## Message Handlers Touched", "## Idempotency Findings", "## Verdict"},
			hasVerdict: true,
		},
		"error-handling-scan": {
			artifact:   "error-handling-scan-report.md",
			sections:   []string{"## Error Paths Reviewed", "## Swallowed-Error Findings", "## Verdict"},
			hasVerdict: true,
		},
		"container-hardening-scan": {
			artifact:   "container-hardening-scan-report.md",
			sections:   []string{"## Container & Manifest Changes", "## Hardening Findings", "## Verdict"},
			hasVerdict: true,
		},
		"cicd-pipeline-audit": {
			artifact:   "cicd-pipeline-audit-report.md",
			sections:   []string{"## Workflow Changes", "## Supply-Chain & Secret Findings", "## Verdict"},
			hasVerdict: true,
		},
		"type-safety-audit": {
			artifact:   "type-safety-audit-report.md",
			sections:   []string{"## Type Surfaces Changed", "## Type-Safety Findings", "## Verdict"},
			hasVerdict: true,
		},
		"data-integrity-check": {
			artifact:   "data-integrity-check-report.md",
			sections:   []string{"## Pipeline Stages Touched", "## Integrity Findings", "## Verdict"},
			hasVerdict: true,
		},
		"resilience-design": {
			artifact:   "resilience-design-report.md",
			sections:   []string{"## Failure Modes", "## Resilience Strategy", "## Fallback & Degradation"},
			hasVerdict: false,
		},
		"data-model-design": {
			artifact:   "data-model-design-report.md",
			sections:   []string{"## Entities & Relationships", "## Schema & Indexes", "## Access Patterns"},
			hasVerdict: false,
		},
		"caching-strategy-design": {
			artifact:   "caching-strategy-design-report.md",
			sections:   []string{"## Cacheable Surfaces", "## Cache Strategy", "## Invalidation & TTL"},
			hasVerdict: false,
		},
		"observability-design": {
			artifact:   "observability-design-report.md",
			sections:   []string{"## Critical Paths", "## Instrumentation Plan", "## SLOs & Alerts"},
			hasVerdict: false,
		},
		"rollout-plan": {
			artifact:   "rollout-plan-report.md",
			sections:   []string{"## Deploy Strategy", "## Feature Flags & Kill Switch", "## Rollback Triggers"},
			hasVerdict: false,
		},
	}

	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			spec, ok := cat.Get(name)
			if !ok {
				t.Fatalf("%s not in merged catalog — should be a config-only user phase", name)
			}
			if !cat.IsUser(name) {
				t.Errorf("%s should be a user (overlay) phase, not built-in", name)
			}
			if !spec.Optional {
				t.Errorf("%s must be optional (user-phase floor)", name)
			}
			if v := phasespec.ValidateUserSpec(spec); len(v) > 0 {
				t.Errorf("%s fails ValidateUserSpec: %v", name, v)
			}
			c := phasecontract.FromSpec(spec)
			if c.ArtifactName != w.artifact {
				t.Errorf("%s artifact = %q, want %q", name, c.ArtifactName, w.artifact)
			}
			if c.Kind != phasecontract.KindMarkdown {
				t.Errorf("%s kind = %v, want markdown", name, c.Kind)
			}
			for _, want := range w.sections {
				found := false
				for _, s := range c.Sections {
					if s.Canonical == want {
						found = true
					}
				}
				if !found {
					t.Errorf("%s contract missing required section %q (have %+v)", name, want, c.Sections)
				}
			}
			if w.hasVerdict && len(c.Verdicts) == 0 {
				t.Errorf("%s should opt into a verdict vocabulary (evaluate + verdict_on_pass)", name)
			}
		})
	}
}

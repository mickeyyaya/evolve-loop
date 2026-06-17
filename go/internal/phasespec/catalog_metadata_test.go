package phasespec_test

import (
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// metadataAllowlist is the SHRINKING set of OPTIONAL catalog phases that still
// lack advisor-facing SELECT metadata (when_to_use / description). ADR-0052
// WS5-S3: it makes the metadata backlog VISIBLE and only lets it shrink — a NEW
// optional phase with no metadata fails the test (add metadata, do not pad this
// list), and a phase that GAINS metadata or is removed must be deleted here (the
// test rejects stale entries). Seeded against the live catalog at slice time
// (2026-06-17, 74 phases). Closing gap #3: the catalog metadata can no longer
// silently rot — each name here is a unit of backlog to retire, not a license.
var metadataAllowlist = map[string]bool{
	"accessibility-audit": true, "account-reconcile": true, "adversarial-review": true,
	"api-contract-design": true, "architecture-design": true, "authz-gap-scan": true,
	"behavior-baseline": true, "behavior-compare": true, "benchmark-gate": true,
	"build-planner": true, "cache-strategy-scan": true, "caching-strategy-design": true,
	"capacity-plan": true, "changelog-sync": true, "cicd-pipeline-audit": true,
	"cleanup-sweep": true, "close-checklist": true, "compat-surface-check": true,
	"container-hardening-scan": true, "context-condense": true, "contract-fuzz-probe": true,
	"coverage-gate": true, "data-integrity-check": true, "data-model-design": true,
	"dependency-audit": true, "dependency-map": true, "doc-sync": true,
	"error-handling-scan": true, "fault-localization": true, "flake-rerun-scan": true,
	"forces-analysis": true, "frontend-design-review": true, "fuzz-probe": true,
	"idempotency-check": true, "incident-postmortem": true, "intent": true,
	"license-provenance-audit": true, "locale-format-check": true, "market-sizing": true,
	"memo": true, "metric-tree": true, "migration-safety-check": true,
	"mutation-gate": true, "observability-design": true, "okr-draft": true,
	"opportunity-map": true, "perf-profile": true, "plan-review": true,
	"post-ship-monitor": true, "prd-draft": true, "premise-challenge": true,
	"prompt-regression-eval": true, "query-performance-scan": true, "race-condition-scan": true,
	"resilience-design": true, "resilience-gap-scan": true, "retrospective": true,
	"risk-register": true, "rollback-plan": true, "rollout-plan": true,
	"runbook-draft": true, "scope-baseline": true, "secret-leak-scan": true,
	"security-scan": true, "smell-scan": true, "spec-verify": true,
	"tdd": true, "telemetry-coverage-check": true, "test-amplification": true,
	"tester": true, "threat-model": true, "triage": true,
	"type-safety-audit": true, "variance-analysis": true,
}

// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata gates the phase-config
// catalog (the ~79-phase registry + user phases, NOT the 15 router PhaseCard
// defaults): every optional phase must carry when_to_use or description, except
// the shrinking allowlist above.
func TestPhaseCatalog_OptionalPhasesHaveSelectMetadata(t *testing.T) {
	cat, _, _, err := phasespec.MergedCatalog(repoRoot(t))
	if err != nil {
		t.Fatalf("MergedCatalog: %v", err)
	}

	missing := map[string]bool{}
	for _, s := range cat.All() {
		if s.Optional && s.WhenToUse == "" && s.Description == "" {
			missing[s.Name] = true
		}
	}

	// 1. No NEW optional phase may lack metadata (the gate bites).
	var newGaps []string
	for name := range missing {
		if !metadataAllowlist[name] {
			newGaps = append(newGaps, name)
		}
	}
	sort.Strings(newGaps)
	if len(newGaps) > 0 {
		t.Errorf("%d optional phase(s) lack SELECT metadata (when_to_use/description) and are NOT allowlisted — add metadata, do not pad the allowlist:\n%v", len(newGaps), newGaps)
	}

	// 2. The allowlist may only SHRINK: an entry that now has metadata or no
	// longer exists must be deleted.
	var stale []string
	for name := range metadataAllowlist {
		if !missing[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%d stale allowlist entr(ies) — they now have metadata or were removed; delete them so the allowlist keeps shrinking:\n%v", len(stale), stale)
	}
}

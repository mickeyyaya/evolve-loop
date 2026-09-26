package phasespec_test

import (
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// metadataAllowlist is the shrink-only backlog of optional phases still lacking SELECT metadata.
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
	"idempotency-check": true, "intent": true,
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

func TestPhaseCatalog_OptionalPhasesHaveSelectMetadata(t *testing.T) {
	cat, _, _, err := phasespec.MergedCatalog(repoRoot(t))
	if err != nil {
		t.Fatalf("MergedCatalog: %v", err)
	}
	trackedUser := phasespec.TrackedUserPhaseNames(t, repoRoot(t))

	missing := map[string]bool{}
	for _, s := range cat.All() {
		if trackedUser != nil && cat.IsUser(s.Name) && !trackedUser[s.Name] {
			t.Logf("untracked user phase %q: runtime/local overlay, not bound by the metadata gate", s.Name)
			continue
		}
		if s.Optional && s.WhenToUse == "" && s.Description == "" {
			missing[s.Name] = true
		}
	}

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

package phasespec

// userphases_validate_test.go — ONE table-driven port of the 8 LIVE bash
// phase-validation suites (Wave C of the bash→Go migration):
//
//	tests/test-implement-dependency-audit-phase.sh
//	tests/test-implement-security-scan-phase.sh
//	tests/test-phases-quality-gates.sh
//	tests/test-phases-release-and-memory.sh
//	tests/test-recover-wave2-phases.sh
//	tests/test-wave1-bugfix-phases.sh
//	tests/test-wave1-refactor-phases.sh
//	tests/test-wave1-router-config.sh
//
// The bash suites were authored as TDD RED contracts (cycles 214/217/246/247);
// the phases they encode have since SHIPPED, so the suites are now permanent
// regression guards. Each load-bearing bash check ran the real `evolve` binary
// (`phases list` / `phases validate`) — i.e. the DiscoverUserSpecs → Merge →
// ValidateUserSpec machinery in THIS package — plus jq/python/grep field reads
// on the same JSON the loader reads. This test reproduces that exact pipeline
// in-process: the phase.json rows go through MergedCatalog + ValidateUserSpec
// (the loader path the binary uses), and the profile / agent.md / router-doc /
// registry rows are file reads that mirror the bash jq/python/grep checks 1:1.
//
// FAITHFULNESS NOTE — stale `classify.fail_if_signal` assertions are NOT ported.
// Five bash rows assert classify.fail_if_signal keys (benchmark-gate
// perf.significant, fuzz-probe fuzz.crashers, rollback-plan rollback.ready,
// bug-reproduction repro.failing, behavior-compare behavior.preserved). The
// cycle-263 incident STRIPPED every fail_if_signal block from the shipped
// phase.json files (the Stage-3 signal bus does not exist; an inert gate makes
// the phase unconditionally FAIL at runtime — see repo_phaseconfigs_test.go,
// which asserts the ABSENCE of fail_if_signal). Those bash rows would FAIL
// against today's tree, so faking them green here would be a no-op lie. They are
// listed in the parity report (this file's package doc + the migration report)
// as "needs manual review — stale" and deliberately skipped. smell-scan's
// classify.fail_if_empty literal-JSON assertion is stale the same way (the field
// is applied by the archetype loader at discovery, not present in the shipped
// phase.json), so it is likewise not ported. The bash NEGATIVE corruption probes
// (kind:python / optional:false rejection) are covered by ValidateUserSpec unit
// tests in phasespec_test.go rather than duplicated here.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is the project root relative to this package directory
// (go/internal/phasespec → ../../..). Mirrors repo_phaseconfigs_test.go.
func repoRoot() string { return filepath.Join("..", "..", "..") }

// loadUserCatalog runs the SAME discovery+merge the `evolve phases list/validate`
// binary runs: MergedCatalog over the project root resolves the built-in
// registry and overlays .evolve/phases/<name>/phase.json. Returns the merged
// catalog. A fatal error here means the loader itself is broken.
func loadUserCatalog(t *testing.T) Catalog {
	t.Helper()
	cat, _, _, err := MergedCatalog(repoRoot())
	if err != nil {
		t.Fatalf("MergedCatalog(%q): %v", repoRoot(), err)
	}
	return cat
}

// readPhaseSpec returns the discovered, loader-parsed PhaseSpec for name, or
// fails the subtest if the phase is absent / not a user phase. This is the
// behavioral anchor: it asserts the loader actually merged the phase as a USER
// overlay (the bash `SOURCE == user` check) and hands the parsed spec to the
// field-level assertions (the bash jq checks).
func readPhaseSpec(t *testing.T, cat Catalog, name string) PhaseSpec {
	t.Helper()
	s, ok := cat.Get(name)
	if !ok {
		t.Fatalf("phase %q not in merged catalog (bash: appears in `evolve phases list`)", name)
	}
	if !cat.IsUser(name) {
		t.Fatalf("phase %q is not a USER phase (bash: SOURCE == user)", name)
	}
	return s
}

// hasInsertWhen reports whether spec has an insert_when condition matching
// field, op (any of the listed ops), and value. Mirrors the bash
// `jq '.routing.insert_when[]? | select(...)'` checks. The bash suites accept
// "==" or "eq" interchangeably for equality, so callers pass both.
func hasInsertWhen(s PhaseSpec, field string, ops []string, value string) bool {
	if s.Routing == nil {
		return false
	}
	for _, c := range s.Routing.InsertWhen {
		if c.Field != field {
			continue
		}
		opOK := false
		for _, op := range ops {
			if c.Op == op {
				opOK = true
				break
			}
		}
		if !opOK {
			continue
		}
		if condValueEquals(c.Value, value) {
			return true
		}
	}
	return false
}

// condValueEquals compares a routing condition's interface{} value against the
// string the bash suite asserted. The bash `gt 0` checks accept the value as
// either number 0 or string "0", so numeric and string forms both match.
func condValueEquals(v interface{}, want string) bool {
	switch tv := v.(type) {
	case string:
		return tv == want
	case float64:
		// JSON numbers decode to float64; "0" must match 0, "5" match 5.
		return formatNum(tv) == want
	default:
		return false
	}
}

func formatNum(f float64) string {
	if f == float64(int64(f)) {
		// integral: render without trailing ".000000"
		return strconv.FormatInt(int64(f), 10)
	}
	return ""
}

// requireSections reports whether spec.Classify.RequireSections contains every
// wanted section. Mirrors the bash `jq '.classify.require_sections | index(...)'`.
func requireSections(s PhaseSpec, want ...string) bool {
	if s.Classify == nil {
		return false
	}
	have := map[string]bool{}
	for _, sec := range s.Classify.RequireSections {
		have[sec] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

// outputSignals reports whether spec.Outputs.Signals contains every wanted
// signal. Mirrors `jq '.outputs.signals | index(...)'`.
func outputSignals(s PhaseSpec, want ...string) bool {
	have := map[string]bool{}
	for _, sig := range s.Outputs.Signals {
		have[sig] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

// fileContains reports whether the repo-relative file contains needle
// (case-insensitive). Mirrors the bash `grep -qi`.
func fileContains(t *testing.T, rel, needle string) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(needle))
}

// fileMatches reports whether the repo-relative file matches the
// case-insensitive regexp pattern. Mirrors the bash `grep -qiE`.
func fileMatches(t *testing.T, rel, pattern string) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	return regexp.MustCompile("(?i)" + pattern).Match(data)
}

// profileRequiredKeys are the keys the wave-3 profile schema check
// (test-phases-release-and-memory.sh AC3) requires. max_budget_usd is NOT a
// typed Profile field (it lives in the loader's Raw), so we read profiles as
// raw JSON exactly as the bash python3 snippet did.
var profileRequiredKeys = []string{
	"name", "cli", "model_tier_default", "role", "sandbox",
	"max_turns", "max_budget_usd", "allowed_tools", "output_artifact",
}

func loadProfileJSON(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	rel := filepath.Join(repoRoot(), ".evolve", "profiles", name+".json")
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("read profile %s: %v", name, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse profile %s: %v", name, err)
	}
	return m
}

// TestUserPhases_BashSuiteParity ports the 8 LIVE bash phase-validation suites
// into one table. Each row names the source bash script + the assertion it
// reproduces. Phase.json rows run through the real loader (MergedCatalog +
// ValidateUserSpec); profile/agent.md/router/registry rows are file reads that
// mirror the bash jq/python/grep checks. Stale fail_if_signal rows are omitted
// (see package doc).
func TestUserPhases_BashSuiteParity(t *testing.T) {
	t.Parallel()
	cat := loadUserCatalog(t)

	tests := []struct {
		name   string // <bash-script>/<assertion>
		assert func(t *testing.T)
	}{
		// ============================================================
		// test-implement-security-scan-phase.sh
		// ============================================================
		{"security-scan.sh/validate-OK", func(t *testing.T) {
			s := readPhaseSpec(t, cat, "security-scan")
			if v := ValidateUserSpec(s); len(v) != 0 {
				t.Fatalf("ValidateUserSpec(security-scan) = %v, want clean (bash: validate == OK)", v)
			}
		}},
		{"security-scan.sh/optional-true", func(t *testing.T) {
			if !readPhaseSpec(t, cat, "security-scan").Optional {
				t.Fatal("security-scan optional != true")
			}
		}},
		{"security-scan.sh/signal-security.severity_max", func(t *testing.T) {
			if !outputSignals(readPhaseSpec(t, cat, "security-scan"), "security.severity_max") {
				t.Fatal("security-scan outputs.signals missing security.severity_max")
			}
		}},
		{"security-scan.sh/routing-build.files_touched-gt-0", func(t *testing.T) {
			if !hasInsertWhen(readPhaseSpec(t, cat, "security-scan"), "build.files_touched", []string{"gt"}, "0") {
				t.Fatal("security-scan insert_when missing build.files_touched gt 0")
			}
		}},

		// ============================================================
		// test-implement-dependency-audit-phase.sh
		// ============================================================
		{"dependency-audit.sh/validate-OK", func(t *testing.T) {
			s := readPhaseSpec(t, cat, "dependency-audit")
			if v := ValidateUserSpec(s); len(v) != 0 {
				t.Fatalf("ValidateUserSpec(dependency-audit) = %v, want clean", v)
			}
		}},
		{"dependency-audit.sh/optional-true", func(t *testing.T) {
			if !readPhaseSpec(t, cat, "dependency-audit").Optional {
				t.Fatal("dependency-audit optional != true")
			}
		}},
		{"dependency-audit.sh/signal-dependency.severity_max", func(t *testing.T) {
			if !outputSignals(readPhaseSpec(t, cat, "dependency-audit"), "dependency.severity_max") {
				t.Fatal("dependency-audit outputs.signals missing dependency.severity_max")
			}
		}},
		{"dependency-audit.sh/both-user-phases-listed", func(t *testing.T) {
			// AC2.4: both new user phases present in the merged catalog.
			for _, n := range []string{"security-scan", "dependency-audit"} {
				if _, ok := cat.Get(n); !ok || !cat.IsUser(n) {
					t.Fatalf("%s not present as a user phase", n)
				}
			}
		}},

		// ============================================================
		// test-wave1-bugfix-phases.sh (fault-localization, bug-reproduction)
		// ============================================================
		{"wave1-bugfix.sh/fault-localization-validate-OK", func(t *testing.T) {
			s := readPhaseSpec(t, cat, "fault-localization")
			if v := ValidateUserSpec(s); len(v) != 0 {
				t.Fatalf("ValidateUserSpec(fault-localization) = %v", v)
			}
		}},
		{"wave1-bugfix.sh/bug-reproduction-validate-OK", func(t *testing.T) {
			s := readPhaseSpec(t, cat, "bug-reproduction")
			if v := ValidateUserSpec(s); len(v) != 0 {
				t.Fatalf("ValidateUserSpec(bug-reproduction) = %v", v)
			}
		}},
		{"wave1-bugfix.sh/fault-localization-routing-goal_type-bugfix", func(t *testing.T) {
			if !hasInsertWhen(readPhaseSpec(t, cat, "fault-localization"), "scout.goal_type", []string{"==", "eq"}, "bugfix") {
				t.Fatal("fault-localization insert_when missing scout.goal_type == bugfix")
			}
		}},
		{"wave1-bugfix.sh/fault-localization-after-triage", func(t *testing.T) {
			if got := readPhaseSpec(t, cat, "fault-localization").After; got != "triage" {
				t.Fatalf("fault-localization after = %q, want triage", got)
			}
		}},
		{"wave1-bugfix.sh/fault-localization-require-sections", func(t *testing.T) {
			if !requireSections(readPhaseSpec(t, cat, "fault-localization"), "Suspect Ranking", "Edit Locations") {
				t.Fatal("fault-localization require_sections missing Suspect Ranking / Edit Locations")
			}
		}},
		{"wave1-bugfix.sh/bug-reproduction-require-sections", func(t *testing.T) {
			if !requireSections(readPhaseSpec(t, cat, "bug-reproduction"), "Reproduction", "Verification") {
				t.Fatal("bug-reproduction require_sections missing Reproduction / Verification")
			}
		}},
		{"wave1-bugfix.sh/both-optional-not-writes-source", func(t *testing.T) {
			for _, n := range []string{"fault-localization", "bug-reproduction"} {
				s := readPhaseSpec(t, cat, n)
				if !s.Optional {
					t.Fatalf("%s optional != true", n)
				}
				if s.WritesSource {
					t.Fatalf("%s writes_source != false", n)
				}
			}
		}},

		// ============================================================
		// test-wave1-refactor-phases.sh
		// (behavior-baseline, behavior-compare, smell-scan)
		// ============================================================
		{"wave1-refactor.sh/all-three-validate-OK", func(t *testing.T) {
			for _, n := range []string{"behavior-baseline", "behavior-compare", "smell-scan"} {
				s := readPhaseSpec(t, cat, n)
				if v := ValidateUserSpec(s); len(v) != 0 {
					t.Fatalf("ValidateUserSpec(%s) = %v", n, v)
				}
			}
		}},
		{"wave1-refactor.sh/behavior-compare-require-sections", func(t *testing.T) {
			if !requireSections(readPhaseSpec(t, cat, "behavior-compare"), "Comparison", "Verdict") {
				t.Fatal("behavior-compare require_sections missing Comparison / Verdict")
			}
		}},
		{"wave1-refactor.sh/smell-scan-archetype-evaluate", func(t *testing.T) {
			if got := readPhaseSpec(t, cat, "smell-scan").RoleOrDefault(); got != RoleEvaluate {
				t.Fatalf("smell-scan archetype = %q, want evaluate", got)
			}
		}},
		{"wave1-refactor.sh/smell-scan-not-writes-source", func(t *testing.T) {
			if readPhaseSpec(t, cat, "smell-scan").WritesSource {
				t.Fatal("smell-scan writes_source != false")
			}
		}},
		{"wave1-refactor.sh/behavior-baseline-output-signals", func(t *testing.T) {
			if !outputSignals(readPhaseSpec(t, cat, "behavior-baseline"), "behavior.preserved", "behavior.delta_count") {
				t.Fatal("behavior-baseline outputs.signals missing behavior.preserved / behavior.delta_count")
			}
		}},
		{"wave1-refactor.sh/baseline-after-tdd", func(t *testing.T) {
			if got := readPhaseSpec(t, cat, "behavior-baseline").After; got != "tdd" {
				t.Fatalf("behavior-baseline after = %q, want tdd", got)
			}
		}},
		{"wave1-refactor.sh/compare-after-build", func(t *testing.T) {
			if got := readPhaseSpec(t, cat, "behavior-compare").After; got != "build" {
				t.Fatalf("behavior-compare after = %q, want build", got)
			}
		}},
		{"wave1-refactor.sh/all-three-route-refactor", func(t *testing.T) {
			for _, n := range []string{"behavior-baseline", "behavior-compare", "smell-scan"} {
				if !hasInsertWhen(readPhaseSpec(t, cat, n), "scout.goal_type", []string{"==", "eq"}, "refactor") {
					t.Fatalf("%s insert_when missing scout.goal_type == refactor", n)
				}
			}
		}},

		// ============================================================
		// test-phases-quality-gates.sh  (Wave-2:
		// benchmark-gate, fuzz-probe, cleanup-sweep, rollback-plan)
		// + test-recover-wave2-phases.sh (same 4 + mutation-gate)
		// ============================================================
		{"phases-quality-gates.sh/four-validate-OK", func(t *testing.T) {
			for _, n := range []string{"benchmark-gate", "fuzz-probe", "cleanup-sweep", "rollback-plan"} {
				s := readPhaseSpec(t, cat, n)
				if v := ValidateUserSpec(s); len(v) != 0 {
					t.Fatalf("ValidateUserSpec(%s) = %v", n, v)
				}
			}
		}},
		{"phases-quality-gates.sh/all-five-wave2-present", func(t *testing.T) {
			for _, n := range []string{"mutation-gate", "benchmark-gate", "fuzz-probe", "cleanup-sweep", "rollback-plan"} {
				if _, ok := cat.Get(n); !ok || !cat.IsUser(n) {
					t.Fatalf("%s not present as a user phase", n)
				}
			}
		}},
		{"phases-quality-gates.sh/four-optional-not-writes-source", func(t *testing.T) {
			for _, n := range []string{"benchmark-gate", "fuzz-probe", "cleanup-sweep", "rollback-plan"} {
				s := readPhaseSpec(t, cat, n)
				if !s.Optional {
					t.Fatalf("%s optional != true", n)
				}
				if s.WritesSource {
					t.Fatalf("%s writes_source != false", n)
				}
			}
		}},
		{"phases-quality-gates.sh/fuzz-probe-routing-parser-decoder-unmarshal", func(t *testing.T) {
			// Bash AC9 greps the routing JSON for pars|decod|unmarshal; the
			// shipped fuzz-probe routes on scout.surface_type ==
			// "parser/decoder/unmarshal".
			if !hasInsertWhen(readPhaseSpec(t, cat, "fuzz-probe"), "scout.surface_type", []string{"==", "eq"}, "parser/decoder/unmarshal") {
				t.Fatal("fuzz-probe insert_when missing scout.surface_type == parser/decoder/unmarshal")
			}
		}},
		{"phases-quality-gates.sh/four-profiles-name-matches-dir", func(t *testing.T) {
			for _, n := range []string{"benchmark-gate", "fuzz-probe", "cleanup-sweep", "rollback-plan"} {
				m := loadProfileJSON(t, n)
				var name string
				if raw, ok := m["name"]; ok {
					_ = json.Unmarshal(raw, &name)
				}
				if name != n {
					t.Fatalf("profile %s name = %q, want %q", n, name, n)
				}
			}
		}},
		{"phases-quality-gates.sh/benchmark-gate-agent-benchstat", func(t *testing.T) {
			if !fileContains(t, ".evolve/phases/benchmark-gate/agent.md", "benchstat") {
				t.Fatal("benchmark-gate agent.md does not reference benchstat")
			}
		}},
		{"phases-quality-gates.sh/cleanup-sweep-agent-detection-only", func(t *testing.T) {
			f := ".evolve/phases/cleanup-sweep/agent.md"
			if !fileContains(t, f, "detection-only") && !fileContains(t, f, "detection only") {
				t.Fatal("cleanup-sweep agent.md does not state detection-only")
			}
		}},
		{"phases-quality-gates.sh/benchmark-gate-agent-multi-sample", func(t *testing.T) {
			// agent.md must instruct multi-sample benchmark collection (bash grep -iE).
			if !fileMatches(t, ".evolve/phases/benchmark-gate/agent.md", `count=|multi[- ]sample|samples|[0-9]+ (runs|times|iterations)`) {
				t.Fatal("benchmark-gate agent.md does not instruct multi-sample benchmark collection")
			}
		}},
		{"phases-quality-gates.sh/cleanup-sweep-agent-forbids-edits", func(t *testing.T) {
			// agent.md must forbid source edits/removals — the detection-only intent (bash grep -iE).
			if !fileMatches(t, ".evolve/phases/cleanup-sweep/agent.md", `do not.*(edit|remove|delete|modify)|no (file )?(edits|removals|deletions)`) {
				t.Fatal("cleanup-sweep agent.md does not forbid edits/removals")
			}
		}},
		{"phases-quality-gates.sh/registry-max-optional-insertions-6", func(t *testing.T) {
			// AC: phase-registry.json config.max_optional_insertions == 6
			// (shared with test-wave1-router-config.sh AC4.1). Negative side:
			// the old cap (4) must be gone.
			if got := registryMaxOptionalInsertions(t); got != 6 {
				t.Fatalf("phase-registry max_optional_insertions = %d, want 6", got)
			}
		}},
		{"recover-wave2.sh/validate-rejects-unknown-phase", func(t *testing.T) {
			// AC6 negative: an unknown phase is not in the catalog (the binary
			// prints "no user phase named …" and exits non-zero).
			if _, ok := cat.Get("cycle247-no-such-phase"); ok {
				t.Fatal("unknown phase unexpectedly present in catalog")
			}
		}},

		// ============================================================
		// test-phases-release-and-memory.sh  (Wave-3:
		// changelog-sync, post-ship-monitor, api-contract-design,
		// context-condense)
		// ============================================================
		{"release-and-memory.sh/four-validate-OK", func(t *testing.T) {
			for _, n := range wave3 {
				s := readPhaseSpec(t, cat, n)
				if v := ValidateUserSpec(s); len(v) != 0 {
					t.Fatalf("ValidateUserSpec(%s) = %v", n, v)
				}
			}
		}},
		{"release-and-memory.sh/four-profiles-required-keys", func(t *testing.T) {
			for _, n := range wave3 {
				m := loadProfileJSON(t, n)
				for _, k := range profileRequiredKeys {
					if _, ok := m[k]; !ok {
						t.Fatalf("profile %s missing required key %q", n, k)
					}
				}
				var name string
				_ = json.Unmarshal(m["name"], &name)
				if name != n {
					t.Fatalf("profile %s name = %q, want %q", n, name, n)
				}
			}
		}},
		{"release-and-memory.sh/two-tier-name-shape", func(t *testing.T) {
			for _, n := range wave3 {
				if !twoTierNameRE.MatchString(n) {
					t.Fatalf("%s is not two-tier kebab-case", n)
				}
				if got := readPhaseSpec(t, cat, n).Name; got != n {
					t.Fatalf("phase.json name = %q, want %q", got, n)
				}
			}
		}},
		{"release-and-memory.sh/archetypes", func(t *testing.T) {
			want := map[string]Role{
				"changelog-sync":      RoleControl,
				"post-ship-monitor":   RoleControl,
				"context-condense":    RoleControl,
				"api-contract-design": RolePlan,
			}
			for n, w := range want {
				if got := readPhaseSpec(t, cat, n).RoleOrDefault(); got != w {
					t.Fatalf("%s archetype = %q, want %q", n, got, w)
				}
			}
		}},
		{"release-and-memory.sh/output-signals", func(t *testing.T) {
			want := map[string]string{
				"changelog-sync":      "changelog.drift_count",
				"post-ship-monitor":   "post_ship.health",
				"api-contract-design": "contract.surfaces",
				"context-condense":    "condense.ratio",
			}
			for n, sig := range want {
				if !outputSignals(readPhaseSpec(t, cat, n), sig) {
					t.Fatalf("%s outputs.signals missing %s", n, sig)
				}
			}
		}},
		{"release-and-memory.sh/agent-md-floor-15-lines", func(t *testing.T) {
			for _, n := range wave3 {
				if lc := lineCount(t, ".evolve/phases/"+n+"/agent.md"); lc < 15 {
					t.Fatalf("%s agent.md has %d lines, want >= 15", n, lc)
				}
			}
		}},
		{"release-and-memory.sh/router-catalog-card-rows", func(t *testing.T) {
			for _, n := range wave3 {
				if !fileContains(t, "agents/evolve-router.md", "| `"+n+"` |") {
					t.Fatalf("agents/evolve-router.md missing catalog-card row for %s", n)
				}
			}
		}},
		{"release-and-memory.sh/alphacodium-expel-cards", func(t *testing.T) {
			if !fileContains(t, "agents/evolve-spec-verify.md", "problem-reflection") {
				t.Fatal("evolve-spec-verify.md missing AlphaCodium problem-reflection card")
			}
			if !fileContains(t, "agents/evolve-architecture-design.md", "solution-ranking") {
				t.Fatal("evolve-architecture-design.md missing AlphaCodium solution-ranking card")
			}
			if !fileContains(t, "agents/evolve-retrospective.md", "lesson-extract") {
				t.Fatal("evolve-retrospective.md missing ExpeL lesson-extract note")
			}
		}},

		// ============================================================
		// test-wave1-router-config.sh
		// ============================================================
		{"wave1-router-config.sh/registry-valid-and-loads", func(t *testing.T) {
			// The registry parses and the merged catalog has >= 15 phases
			// (bash: `phases list` shows >= 15 rows). Behavioral: the loader
			// itself produced cat with no error.
			if got := len(cat.Names()); got < 15 {
				t.Fatalf("merged catalog has %d phases, want >= 15", got)
			}
		}},
		{"wave1-router-config.sh/max-optional-insertions-not-4", func(t *testing.T) {
			if got := registryMaxOptionalInsertions(t); got == 4 {
				t.Fatal("registry max_optional_insertions == 4 (old cap not removed)")
			}
		}},
		{"wave1-router-config.sh/router-goal-type-recipes-section", func(t *testing.T) {
			if !fileContains(t, "agents/evolve-router.md", "## Goal-Type Recipes") {
				t.Fatal("evolve-router.md missing '## Goal-Type Recipes' section")
			}
		}},
		{"wave1-router-config.sh/router-covers-7-goal-types", func(t *testing.T) {
			for _, gt := range []string{"bugfix", "feature", "refactor", "security", "performance", "release", "docs"} {
				if !fileContains(t, "agents/evolve-router.md", gt) {
					t.Fatalf("evolve-router.md recipe table missing goal type %q", gt)
				}
			}
		}},
		{"wave1-router-config.sh/router-bugfix-row-wires-chain", func(t *testing.T) {
			if !fileContains(t, "agents/evolve-router.md", "fault-localization") {
				t.Fatal("evolve-router.md missing fault-localization in recipe table")
			}
			if !fileContains(t, "agents/evolve-router.md", "bug-reproduction") {
				t.Fatal("evolve-router.md missing bug-reproduction in recipe table")
			}
		}},
		{"wave1-router-config.sh/router-cites-clamp-floor", func(t *testing.T) {
			if !fileContains(t, "agents/evolve-router.md", "ClampPlanToFloor") {
				t.Fatal("evolve-router.md does not cite ClampPlanToFloor as the safety net")
			}
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.assert(t)
		})
	}
}

// wave3 is the Wave-3 release/feature/memory phase set
// (test-phases-release-and-memory.sh).
var wave3 = []string{"changelog-sync", "post-ship-monitor", "api-contract-design", "context-condense"}

// registryMaxOptionalInsertions reads docs/architecture/phase-registry.json and
// returns config.max_optional_insertions. Mirrors the bash python3 read.
func registryMaxOptionalInsertions(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatalf("read phase-registry.json: %v", err)
	}
	var doc struct {
		Config struct {
			MaxOptionalInsertions int `json:"max_optional_insertions"`
		} `json:"config"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse phase-registry.json: %v", err)
	}
	return doc.Config.MaxOptionalInsertions
}

// lineCount returns the number of newline-terminated lines in a repo-relative
// file (mirrors the bash `wc -l`).
func lineCount(t *testing.T, rel string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

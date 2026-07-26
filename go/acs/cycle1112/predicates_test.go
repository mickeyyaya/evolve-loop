//go:build acs

// Package cycle1112 materialises the cycle-1112 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - exhaustion-regex-drift-failloud → arm the drift alarm for codex-tmux and
//     agy-tmux by adding controls.usage.drift_probe_regex to their manifests,
//     plus per-CLI regression coverage in exhaustion_drift_test.go.
//
// Why this is a real gap, not a cosmetic one. warnExhaustionRegexDrift
// (go/internal/bridge/exhaustion_drift.go) is generic and fail-OPEN: it keys
// entirely off manifestDriftProbePattern(cli) and returns silently when that is
// empty. claude-tmux carries a broadened drift_probe_regex; codex-tmux and
// agy-tmux do not, so a wording drift in THEIR exhausted_regex degrades to "not
// exhausted" with no diagnostic at all — the exact 8-cycle silent burn the
// watcher exists to prevent (exhaustion_drift.go header; the
// audit_quota_wording_drift incident family).
//
// Predicate strategy — each predicate COMPILES and EVALUATES the shipped regexes
// against wall/benign pane corpora, or shells the real unit suite; none is a
// source-grep of production code (the cycle-85 degenerate-predicate ban). The
// cheapest gaming fake — copy-pasting exhausted_regex verbatim into
// drift_probe_regex — satisfies "field is non-empty" but is killed by 001 (no
// drift-detectable gap) and by 004 (verbatim-copy check).
//
// RED now: neither manifest has the field, so manifest lookup returns "" and
// 001/004 fail at the presence gate; 002's per-CLI unit cases do not exist yet.
package cycle1112

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg hosts warnExhaustionRegexDrift and its unit suite. The functions are
// unexported, so behavioural coverage of the alarm itself is exercised by
// shelling `go test` at this package (002) rather than by importing it.
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// driftCLIs are the tmux CLIs that ship an exhausted_regex but no drift probe
// today. ollama-tmux is excluded on purpose: it has no controls.usage block at
// all, so there is no exhausted_regex to drift-guard (scout Deferred B1).
var driftCLIs = []string{"codex-tmux", "agy-tmux"}

// wallCorpus holds panes the CURRENT exhausted_regex already recognises. A broad
// drift probe must be a SUPERSET of the strict pattern: any wall the strict
// pattern catches, the probe must catch too, otherwise the alarm has blind spots
// in the region it is supposed to over-cover.
var wallCorpus = map[string][]string{
	"codex-tmux": {
		"Usage limit reached for this account.",
		"Weekly usage: 0% left.",
		"rate limit exceeded, retry later",
		"429 too many requests",
	},
	"agy-tmux": {
		"Usage limit reached for this account.",
		"quota exceeded for this billing period",
		"Weekly usage: 0% remaining.",
		"you are being rate-limited",
	},
}

// driftCorpus holds plausible FUTURE wall wordings that today's strict
// exhausted_regex misses. These are the drift signature: the probe must match
// them while exhausted_regex does not, which is precisely the condition under
// which warnExhaustionRegexDrift fires. A probe that cannot match any of these
// can never detect a drift and is decorative.
var driftCorpus = []string{
	"You've reached your weekly limit — resets Monday.",
	"You are out of credits. Upgrade to continue.",
	"You've hit your usage limit for this week.",
	"Upgrade to a paid plan to keep going.",
}

// benignCorpus holds ordinary working panes. The probe runs only on an
// already-failed exit-81 teardown, but it must still not cry wolf on every
// generic stall, or the loud line becomes noise and stops being read.
var benignCorpus = []string{
	"Running tests... 42/50 passing, still working.",
	"Writing the audit report now; 3 files reviewed.",
	"Applying patch to usageclassify.go; 2 hunks staged.",
}

// usageSpec mirrors the controls.usage block of a tmux manifest.
type usageSpec struct {
	Exhausted  string `json:"exhausted_regex"`
	DriftProbe string `json:"drift_probe_regex"`
}

type manifestDoc struct {
	Controls map[string]usageSpec `json:"controls"`
}

// loadUsage reads cli's manifest from the worktree and returns its controls.usage
// block. Absence of the manifest or of the usage block is a hard failure: both
// CLIs in driftCLIs are known to ship one today.
func loadUsage(t *testing.T, cli string) usageSpec {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "manifests", cli+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read manifest %s: %v", path, err)
	}
	var doc manifestDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("manifest %s is not valid JSON: %v", path, err)
	}
	spec, ok := doc.Controls["usage"]
	if !ok {
		t.Fatalf("manifest %s has no controls.usage block", path)
	}
	return spec
}

// mustCompile compiles a manifest-sourced pattern, failing loudly on a bad regex
// (an uncompilable probe silently disables the alarm at runtime — fail-open).
func mustCompile(t *testing.T, cli, field, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("%s controls.usage.%s does not compile: %v (pattern=%q)", cli, field, err, pattern)
	}
	return re
}

// requireProbe returns cli's compiled drift probe, failing when the field is
// absent or blank — the inert state this cycle exists to remove. Guarding here
// matters: regexp.Compile("") succeeds and matches EVERYTHING, so an unguarded
// superset check would spuriously pass on the un-fixed tree.
func requireProbe(t *testing.T, cli string, spec usageSpec) *regexp.Regexp {
	t.Helper()
	if strings.TrimSpace(spec.DriftProbe) == "" {
		t.Fatalf("%s has no controls.usage.drift_probe_regex — the exhaustion-regex drift alarm is INERT for this CLI", cli)
	}
	return mustCompile(t, cli, "drift_probe_regex", spec.DriftProbe)
}

// TestC1112_001_DriftProbeIsBroaderSupersetOfExhausted — AC-1
// (superset-of-exhausted-regex correctness).
//
// Evaluates both shipped patterns against real pane strings: every wall the
// strict exhausted_regex matches must also match the probe (superset), the probe
// must additionally match the drifted wordings exhausted_regex misses (that gap
// IS the alarm's firing condition), and it must stay silent on benign panes.
func TestC1112_001_DriftProbeIsBroaderSupersetOfExhausted(t *testing.T) {
	for _, cli := range driftCLIs {
		t.Run(cli, func(t *testing.T) {
			spec := loadUsage(t, cli)
			if strings.TrimSpace(spec.Exhausted) == "" {
				t.Fatalf("%s has no controls.usage.exhausted_regex — nothing to drift-guard", cli)
			}
			exhausted := mustCompile(t, cli, "exhausted_regex", spec.Exhausted)
			probe := requireProbe(t, cli, spec)

			for _, pane := range wallCorpus[cli] {
				if exhausted.MatchString(pane) && !probe.MatchString(pane) {
					t.Errorf("not a superset: exhausted_regex matches %q but drift_probe_regex does not", pane)
				}
			}
			for _, pane := range driftCorpus {
				if !probe.MatchString(pane) {
					t.Errorf("drift_probe_regex misses drifted wall %q — a drift to this wording would stay silent", pane)
				}
			}
			for _, pane := range benignCorpus {
				if probe.MatchString(pane) {
					t.Errorf("drift_probe_regex false-matches benign working pane %q — the alarm would cry wolf", pane)
				}
			}
		})
	}
}

// TestC1112_002_PerCLIDriftRegressionCoverage — AC-2 (positive-fire + silence
// regression tests per newly-configured CLI).
//
// Shells the REAL unit suite (`go test -run Drift -v` over internal/bridge) and
// requires it green AND that its verbose output names each newly-armed CLI, so
// the manifest change is pinned by executed per-CLI cases rather than by the
// manifest edit alone. -count=1 defeats the test cache.
func TestC1112_002_PerCLIDriftRegressionCoverage(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "Drift", "-v", "-count=1", bridgePkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", bridgePkg, code, err, out)
	}
	if code != 0 {
		t.Fatalf("drift regression suite is RED (exit=%d):\n%s", code, out)
	}
	for _, cli := range driftCLIs {
		if !strings.Contains(out, cli) {
			t.Errorf("no executed drift test case names %q — the newly-armed CLI has no regression coverage\n%s", cli, out)
		}
	}
	// A -run filter that matched nothing also exits 0; require real RUN lines.
	if !strings.Contains(out, "=== RUN") {
		t.Errorf("no drift tests actually ran (empty -run match):\n%s", out)
	}
}

// TestC1112_003_RepoBuildAndVetClean — AC-3 (repo-wide vet/build cleanliness).
//
// A manifest is embedded and a test file is compiled; both can break the tree.
// Executes the real toolchain rather than inspecting sources.
func TestC1112_003_RepoBuildAndVetClean(t *testing.T) {
	// Module-wide import path, not "./...": the predicate's cwd is its own
	// acs-tagged package dir, where "./..." matches no packages and go vet
	// exits 1 for the wrong reason.
	const allPkgs = "github.com/mickeyyaya/evolve-loop/go/..."
	for _, cmd := range [][]string{{"build", allPkgs}, {"vet", allPkgs}} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", cmd...)
		out := stdout + stderr
		if code < 0 {
			t.Fatalf("go %s failed to launch: code=%d err=%v\n%s", strings.Join(cmd, " "), code, err, out)
		}
		if code != 0 {
			t.Errorf("go %s is not clean (exit=%d):\n%s", strings.Join(cmd, " "), code, out)
		}
	}
}

// TestC1112_004_DriftProbeIsNotAnExhaustedRegexCopy — AC-4 (distinctness against
// the cheapest gaming fake).
//
// Copy-pasting exhausted_regex verbatim as drift_probe_regex makes the field
// non-empty while guaranteeing the alarm can NEVER fire: warnExhaustionRegexDrift
// requires probe-match AND exhausted-miss, which is unsatisfiable for identical
// patterns. This asserts the two are distinct AND that the gap is real — at least
// one drifted pane matches the probe while exhausted_regex misses it.
func TestC1112_004_DriftProbeIsNotAnExhaustedRegexCopy(t *testing.T) {
	for _, cli := range driftCLIs {
		t.Run(cli, func(t *testing.T) {
			spec := loadUsage(t, cli)
			probe := requireProbe(t, cli, spec)
			if strings.TrimSpace(spec.DriftProbe) == strings.TrimSpace(spec.Exhausted) {
				t.Fatalf("%s drift_probe_regex is a verbatim copy of exhausted_regex — the alarm can never fire", cli)
			}
			exhausted := mustCompile(t, cli, "exhausted_regex", spec.Exhausted)
			gap := 0
			for _, pane := range driftCorpus {
				if probe.MatchString(pane) && !exhausted.MatchString(pane) {
					gap++
				}
			}
			if gap == 0 {
				t.Errorf("%s drift_probe_regex has no detectable gap over exhausted_regex: no drifted pane matches the probe while missing the strict pattern", cli)
			}
		})
	}
}

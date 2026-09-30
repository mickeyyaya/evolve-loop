//go:build acs

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

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

var driftCLIs = []string{"codex-tmux", "agy-tmux"}

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

var driftCorpus = []string{
	"You've reached your weekly limit — resets Monday.",
	"You are out of credits. Upgrade to continue.",
	"You've hit your usage limit for this week.",
	"Upgrade to a paid plan to keep going.",
}

var benignCorpus = []string{
	"Running tests... 42/50 passing, still working.",
	"Writing the audit report now; 3 files reviewed.",
	"Applying patch to usageclassify.go; 2 hunks staged.",
}

type usageSpec struct {
	Exhausted  string `json:"exhausted_regex"`
	DriftProbe string `json:"drift_probe_regex"`
}

type manifestDoc struct {
	Controls map[string]usageSpec `json:"controls"`
}

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

func mustCompile(t *testing.T, cli, field, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("%s controls.usage.%s does not compile: %v (pattern=%q)", cli, field, err, pattern)
	}
	return re
}

func requireProbe(t *testing.T, cli string, spec usageSpec) *regexp.Regexp {
	t.Helper()
	if strings.TrimSpace(spec.DriftProbe) == "" {
		t.Fatalf("%s has no controls.usage.drift_probe_regex — the exhaustion-regex drift alarm is INERT for this CLI", cli)
	}
	return mustCompile(t, cli, "drift_probe_regex", spec.DriftProbe)
}

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
	if !strings.Contains(out, "=== RUN") {
		t.Errorf("no drift tests actually ran (empty -run match):\n%s", out)
	}
}

func TestC1112_003_RepoBuildAndVetClean(t *testing.T) {
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

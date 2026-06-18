package consensusdispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExec(t *testing.T, path, content string) {
	t.Helper()
	writeFile(t, path, content)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeProfile(t *testing.T, path string, consensus map[string]any) {
	t.Helper()
	doc := map[string]any{
		"model_tier_default": "sonnet",
		"consensus":          consensus,
	}
	b, _ := json.Marshal(doc)
	writeFile(t, path, string(b))
}

// installFakeEvolve writes a fake `evolve` binary into dir/bin and points
// EVOLVE_GO_BIN at it. Because Run resolves the native `evolve` for THREE
// subcommands — `bridge launch` (the per-voter worker, post-bridge cutover),
// `fanout-dispatch`, and `aggregator` — via the same resolveEvolveBin, one
// shim must handle all three:
//
//   - bridge launch ... --artifact=PATH : writes artifactBody to PATH (the
//     bridge's artifact-write contract). Empty artifactBody writes nothing
//     (modeling a worker that produces no artifact). Exits launchExit.
//   - fanout-dispatch <commandsTSV> <resultsTSV> : runs each TSV worker command
//     (so the bridge-launch workers actually execute), recording ok/fail.
//   - aggregator <mode> <output> <artifacts...> : concatenates the artifacts and
//     appends a verdict; exits aggExit (non-zero to model consensus FAIL).
//
// Because it sets a process-global env var, callers must NOT t.Parallel().
func installFakeEvolve(t *testing.T, dir, artifactBody string, launchExit, aggExit int) {
	t.Helper()
	bin := filepath.Join(dir, "bin", "evolve")
	script := fmt.Sprintf(`#!/usr/bin/env bash
sub="$1"; shift
case "$sub" in
  bridge)
    # form: bridge launch --cli=... --artifact=PATH ...
    art=""
    for a in "$@"; do
      case "$a" in --artifact=*) art="${a#--artifact=}" ;; esac
    done
    body=%q
    if [ -n "$art" ] && [ -n "$body" ]; then
      mkdir -p "$(dirname "$art")"
      printf '%%s' "$body" > "$art"
    fi
    exit %d
    ;;
  fanout-dispatch)
    commands="$1"; results="$2"
    while IFS=$'\t' read -r name cmd; do
      bash -c "$cmd" >/dev/null 2>&1 && printf '%%s\tok\n' "$name" >> "$results" || printf '%%s\tfail\n' "$name" >> "$results"
    done < "$commands"
    exit 0
    ;;
  aggregator)
    mode="$1"; output="$2"; shift 2
    printf '# Cross-CLI Consensus (%%s)\n' "$mode" > "$output"
    for a in "$@"; do printf '## %%s\n' "$a" >> "$output"; cat "$a" >> "$output"; done
    printf '## Verdict: PASS\n' >> "$output"
    exit %d
    ;;
  *)
    exit 0
    ;;
esac
`, artifactBody, launchExit, aggExit)
	writeExec(t, bin, script)
	t.Setenv("EVOLVE_GO_BIN", bin)
}

func TestParseProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "auditor.json")
	writeProfile(t, p, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"claude", "gemini", "codex"},
		"quorum":           2,
		"require_min_tier": "hybrid",
	})
	prof, err := ParseProfile(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !prof.Enabled || len(prof.CLIVoters) != 3 || prof.Quorum != 2 || prof.RequireMinTier != "hybrid" {
		t.Errorf("bad parse: %+v", prof)
	}
}

func TestParseProfile_DefaultsApplied(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "p.json")
	writeFile(t, p, `{"consensus":{"enabled":true,"cli_voters":["a","b"]}}`)
	prof, _ := ParseProfile(p)
	if prof.ModelTierDefault != "sonnet" || prof.RequireMinTier != "hybrid" {
		t.Errorf("defaults missing: %+v", prof)
	}
}

func TestParseProfile_FileMissing(t *testing.T) {
	t.Parallel()
	_, err := ParseProfile("/nonexistent/path")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestParseProfile_BadJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.json")
	writeFile(t, p, "not json")
	_, err := ParseProfile(p)
	if err == nil {
		t.Fatal("want parse error")
	}
}

func TestFilterEligibleAgainstTiers(t *testing.T) {
	t.Parallel()
	voters := []string{"claude", "gemini", "codex", "agy"}
	tiers := map[string]string{
		"claude": "full",
		"gemini": "hybrid",
		"codex":  "degraded",
		"agy":    "none",
	}
	tests := []struct {
		name     string
		min      string
		expected []string
	}{
		{"full-only", "full", []string{"claude"}},
		{"hybrid-and-above", "hybrid", []string{"claude", "gemini"}},
		{"degraded-and-above", "degraded", []string{"claude", "gemini", "codex"}},
		{"none-includes-all", "none", []string{"claude", "gemini", "codex", "agy"}},
		{"empty-defaults-to-include-all", "", []string{"claude", "gemini", "codex", "agy"}},
		{"unknown-includes-all", "experimental", []string{"claude", "gemini", "codex", "agy"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FilterEligibleAgainstTiers(voters, tiers, tc.min)
			if !equalStrings(got, tc.expected) {
				t.Errorf("got %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestBuildCommandsTSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	workers := filepath.Join(dir, "workers")
	if err := os.MkdirAll(workers, 0o755); err != nil {
		t.Fatal(err)
	}
	// Post-bridge cutover: a worker is included when its resolved CLI projects
	// onto a REGISTERED bridge driver (bridge.DriverFor + LookupDriver), not when
	// a <cli>.sh exists. Bare "claude"/"gemini" map to claude-tmux; "codex" is
	// itself a registered driver so it passes through unchanged (DriverFor keeps
	// already-registered names). "no-such-cli" has no driver → skipped.
	tsv, count, err := BuildCommandsTSV([]string{"claude", "gemini", "codex", "no-such-cli"},
		"/tmp/profile.json", "/tmp/prompt.md", "42", workers, "/usr/local/bin/evolve", "sonnet")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if count != 3 {
		t.Errorf("want 3 driver-backed workers, got %d", count)
	}
	for _, cli := range []string{"claude", "gemini", "codex"} {
		if !strings.Contains(tsv, cli+"\t") {
			t.Errorf("missing voter line for %s:\n%s", cli, tsv)
		}
	}
	if strings.Contains(tsv, "no-such-cli\t") {
		t.Errorf("driverless no-such-cli should be skipped:\n%s", tsv)
	}
	// command routes through the bridge, not bash <cli>.sh, and projects bare
	// names onto registered drivers (claude → claude-tmux).
	if !strings.Contains(tsv, "/usr/local/bin/evolve bridge launch --cli='claude-tmux'") {
		t.Errorf("claude voter not routed through bridge launch with driver name:\n%s", tsv)
	}
	// codex is already a registered driver → passes through as codex.
	if !strings.Contains(tsv, "--cli='codex'") {
		t.Errorf("codex voter not dispatched via its registered driver:\n%s", tsv)
	}
	if strings.Contains(tsv, "bash '") {
		t.Errorf("worker command must not shell bash <cli>.sh anymore:\n%s", tsv)
	}
	// deterministic ordering
	if strings.Index(tsv, "claude\t") > strings.Index(tsv, "gemini\t") {
		t.Errorf("voters not sorted: %s", tsv)
	}
}

func TestRun_MissingInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   Inputs
		want int
	}{
		{"no-cycle", Inputs{}, ExitRuntimeErr},
		{"no-workspace", Inputs{Cycle: "1"}, ExitRuntimeErr},
		{"no-profile-path", Inputs{Cycle: "1", WorkspacePath: t.TempDir()}, ExitProfileErr},
		{"no-prompt-path", Inputs{Cycle: "1", WorkspacePath: t.TempDir(), ProfilePath: "/nonexistent", PromptFile: ""}, ExitRuntimeErr},
		{"profile-file-missing", Inputs{Cycle: "1", WorkspacePath: t.TempDir(), ProfilePath: "/nonexistent", PromptFile: "/also-nonexistent"}, ExitProfileErr},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			rc := Run(tc.in, &stdout, &stderr)
			if rc != tc.want {
				t.Errorf("got rc=%d, want %d (stderr=%s)", rc, tc.want, stderr.String())
			}
		})
	}
}

func TestRun_ConsensusEnvOff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{"enabled": true, "cli_voters": []string{"claude", "gemini"}, "quorum": 2})
	writeFile(t, prompt, "audit me")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt,
		ConsensusEnvOff: true,
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("env-off should refuse, got rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "EVOLVE_CONSENSUS_AUDIT=0") {
		t.Errorf("missing opt-out reason in stderr: %s", stderr.String())
	}
}

func TestRun_DisabledProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{"enabled": false, "cli_voters": []string{"claude"}, "quorum": 1})
	writeFile(t, prompt, "audit")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt}, &stdout, &stderr)
	if rc != ExitProfileErr {
		t.Errorf("disabled profile should fail, got rc=%d", rc)
	}
}

func TestRun_EmptyVoters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{"enabled": true, "cli_voters": []string{}, "quorum": 1})
	writeFile(t, prompt, "audit")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt}, &stdout, &stderr)
	if rc != ExitProfileErr {
		t.Errorf("empty voters should fail-profile, got rc=%d", rc)
	}
}

func TestRun_InsufficientVoters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"only-one"},
		"quorum":           1,
		"require_min_tier": "none",
	})
	writeFile(t, prompt, "audit")
	// No capability-check binary in adapters dir → all included, but only 1 voter
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: filepath.Join(dir, "fake-adapters"),
		DispatchDir: filepath.Join(dir, "fake-dispatch"),
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("solo-voter should fail-runtime, got rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "consensus requires at least 2 eligible voters") {
		t.Errorf("missing reason: %s", stderr.String())
	}
}

// TestRun_E2E_WithFakeBash drives Run() through a full pipeline. Post-bridge
// cutover the per-voter worker command is `evolve bridge launch ...`, and the
// fanout/aggregator steps already resolve the native `evolve` binary, so the
// test installs ONE fake `evolve` shim (via EVOLVE_GO_BIN) that handles all
// three subcommands — proving the bridge-launch command is well-formed,
// executes through fanout, and produces both worker artifacts + the aggregate.
func TestRun_E2E_WithFakeBash(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("/bin/bash not present")
	}
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	adapters := filepath.Join(dir, "adapters")
	dispatch := filepath.Join(dir, "dispatch")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dispatch, 0o755); err != nil {
		t.Fatal(err)
	}

	// One fake `evolve` shim covers bridge launch (writes the artifact), plus
	// fanout-dispatch + aggregator (both resolve the native evolve binary).
	installFakeEvolve(t, dir, "## Verdict: PASS from bridge\n", 0, 0)

	prof := filepath.Join(dir, "auditor.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"claude", "gemini"},
		"quorum":           2,
		"require_min_tier": "hybrid",
	})
	writeFile(t, prompt, "audit request")

	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "42", WorkspacePath: workspace, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: adapters, DispatchDir: dispatch,
		TierFor: func(string) (string, error) { return "full", nil },
	}, &stdout, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, stderr=%s", rc, stderr.String())
	}
	// expect audit-report.md produced
	report := filepath.Join(workspace, "audit-report.md")
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("audit-report.md not created: %v", err)
	}
	if !strings.Contains(string(body), "Verdict: PASS") {
		t.Errorf("aggregate missing verdict: %s", body)
	}
	// expect both worker artifacts present
	for _, cli := range []string{"claude", "gemini"} {
		artifact := filepath.Join(workspace, "consensus-workers", cli+"-audit.md")
		if _, err := os.Stat(artifact); err != nil {
			t.Errorf("worker artifact missing: %s", artifact)
		}
	}
}

// TestRun_AggregatorFailPropagates checks that aggregator's non-zero exit
// is passed through as the run's exit code (consensus FAIL).
func TestRun_AggregatorFailPropagates(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash unavailable")
	}
	dir := t.TempDir()
	workspace := filepath.Join(dir, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	adapters := filepath.Join(dir, "adapters")
	dispatch := filepath.Join(dir, "dispatch")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dispatch, 0o755); err != nil {
		t.Fatal(err)
	}
	// One fake `evolve` shim: bridge launch writes a PASS artifact; aggregator
	// exits 1 to model consensus FAIL (the non-zero must propagate as the run rc).
	installFakeEvolve(t, dir, "PASS\n", 0, 1)
	prof := filepath.Join(dir, "p.json")
	prompt := filepath.Join(dir, "pr.md")
	writeProfile(t, prof, map[string]any{
		"enabled": true, "cli_voters": []string{"claude", "gemini"},
		"quorum": 2, "require_min_tier": "hybrid",
	})
	writeFile(t, prompt, "x")

	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: workspace, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: adapters, DispatchDir: dispatch,
		TierFor: func(string) (string, error) { return "full", nil },
	}, &stdout, &stderr)
	if rc != ExitConsensusFAIL {
		t.Errorf("expected aggregator-fail rc=1, got %d", rc)
	}
}

// TestRun_NoArtifactsProducedFails covers the path where every worker fails
// to write its artifact — aggregator can't run.
func TestRun_NoArtifactsProducedFails(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash unavailable")
	}
	dir := t.TempDir()
	workspace := filepath.Join(dir, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	adapters := filepath.Join(dir, "adapters")
	dispatch := filepath.Join(dir, "dispatch")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dispatch, 0o755); err != nil {
		t.Fatal(err)
	}
	// fake `evolve bridge launch` exits 0 but writes NO artifact (empty
	// artifactBody → shim skips the write), so the "no worker artifacts" branch
	// fires; the aggregator step is never reached.
	installFakeEvolve(t, dir, "", 0, 0)
	prof := filepath.Join(dir, "p.json")
	prompt := filepath.Join(dir, "pr.md")
	writeProfile(t, prof, map[string]any{
		"enabled": true, "cli_voters": []string{"claude", "gemini"},
		"quorum": 2, "require_min_tier": "hybrid",
	})
	writeFile(t, prompt, "x")

	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: workspace, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: adapters, DispatchDir: dispatch,
		TierFor: func(string) (string, error) { return "full", nil },
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("expected runtime-fail rc=2 when no artifacts, got %d", rc)
	}
	if !strings.Contains(stderr.String(), "no worker artifacts produced") {
		t.Errorf("missing reason in stderr: %s", stderr.String())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

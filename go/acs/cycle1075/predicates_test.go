//go:build acs

package cycle1075

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runGo(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	full := append([]string{"-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil && stdout == "" && stderr == "" {
		t.Fatalf("could not execute `go %s`: %v", strings.Join(full, " "), err)
	}
	return stdout, stderr, code
}

func dryRunChainMode(t *testing.T, extraArgs ...string) (bool, string) {
	t.Helper()
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatalf("seed temp project: %v", err)
	}
	args := []string{"run", "./cmd/evolve", "loop",
		"--dry-run",
		"--project-root", proj,
		"--goal-text", "acs-1075-chain-probe",
	}
	args = append(args, extraArgs...)
	stdout, stderr, code := runGo(t, args...)
	if code != 0 {
		t.Fatalf("`evolve loop --dry-run %s` exited %d (must be accepted and exit 0)\nstdout:\n%s\nstderr:\n%s",
			strings.Join(extraArgs, " "), code, stdout, stderr)
	}
	var doc struct {
		DryRun bool            `json:"dry_run"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("dry-run stdout is not the expected config JSON: %v\nstdout:\n%s", err, stdout)
	}
	var cfg map[string]any
	if err := json.Unmarshal(doc.Config, &cfg); err != nil {
		t.Fatalf("dry-run `config` is not a JSON object: %v", err)
	}
	v, ok := cfg["chain_mode"]
	if !ok {
		return false, string(doc.Config)
	}
	b, isBool := v.(bool)
	if !isBool {
		t.Fatalf("dry-run config field chain_mode is %T, want bool\nconfig:\n%s", v, string(doc.Config))
	}
	return b, string(doc.Config)
}

func TestC1075_001_UntilInboxEmptyFlagDrivesChainMode(t *testing.T) {
	on, cfgOn := dryRunChainMode(t, "--until-inbox-empty")
	if !on {
		t.Errorf("`evolve loop --until-inbox-empty --dry-run` resolved chain mode OFF; the flag must set loopConfig chain_mode=true\nconfig:\n%s", cfgOn)
	}

	off, cfgOff := dryRunChainMode(t)
	if off {
		t.Errorf("`evolve loop --dry-run` (no --until-inbox-empty) resolved chain mode ON; chaining must stay opt-in\nconfig:\n%s", cfgOff)
	}
}

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy fixture: %v", err)
	}
	return path
}

func TestC1075_002_ChainPolicyBlockOverrideAndDefault(t *testing.T) {
	base, err := policy.Load(writePolicy(t, `{"floor":[]}`))
	if err != nil {
		t.Fatalf("policy.Load on a chain-less policy failed: %v", err)
	}
	def := base.ChainConfig()
	if def.Enabled {
		t.Errorf("absent chain block resolved Enabled=true; chaining must default OFF (CLI opt-in)")
	}
	if def.MaxBatches <= 0 {
		t.Errorf("absent chain block resolved MaxBatches=%d; the compiled default cap must be positive", def.MaxBatches)
	}

	p, err := policy.Load(writePolicy(t, `{"chain":{"enabled":true,"max_batches":7}}`))
	if err != nil {
		t.Fatalf("policy.Load on a chain policy failed: %v", err)
	}
	got := p.ChainConfig()
	if !got.Enabled {
		t.Errorf("chain.enabled=true in policy.json resolved Enabled=false")
	}
	if got.MaxBatches != 7 {
		t.Errorf("chain.max_batches=7 resolved MaxBatches=%d, want 7", got.MaxBatches)
	}

	for _, body := range []string{
		`{"chain":{"enabled":true,"max_batches":0}}`,
		`{"chain":{"enabled":true,"max_batches":-3}}`,
	} {
		bp, err := policy.Load(writePolicy(t, body))
		if err != nil {
			t.Fatalf("policy.Load on %s failed: %v", body, err)
		}
		if c := bp.ChainConfig(); c.MaxBatches <= 0 {
			t.Errorf("policy %s resolved MaxBatches=%d; a non-positive cap must fall back to the positive compiled default", body, c.MaxBatches)
		}
	}
}

func passedTests(out string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--- PASS:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "--- PASS:"))
		if i := strings.Index(rest, " "); i > 0 {
			rest = rest[:i]
		}
		if rest != "" {
			names[rest] = true
		}
	}
	return names
}

func anyNameMatches(names map[string]bool, alternatives [][]string) bool {
	for n := range names {
		low := strings.ToLower(n)
		for _, frags := range alternatives {
			all := true
			for _, f := range frags {
				if !strings.Contains(low, f) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}

func runChainTests(t *testing.T, pattern string) map[string]bool {
	t.Helper()
	stdout, stderr, code := runGo(t, "test", "-count=1", "-v", "-run", pattern, "./cmd/evolve")
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("`go test -run %s ./cmd/evolve` exited %d (chain behaviour tests must PASS)\n%s", pattern, code, tail(combined))
	}
	names := passedTests(combined)
	if len(names) == 0 {
		t.Fatalf("`go test -run %s ./cmd/evolve` matched no passing tests — the chain behaviour tests are missing\n%s", pattern, tail(combined))
	}
	return names
}

func tail(s string) string {
	if len(s) <= 4000 {
		return s
	}
	return "…\n" + s[len(s)-4000:]
}

func TestC1075_003_ChainBoundaryStopConditionsCovered(t *testing.T) {
	names := runChainTests(t, "TestRunLoopChain")

	conditions := []struct {
		label string
		alts  [][]string
	}{
		{"inbox drained → chained batch + clean exit", [][]string{{"inbox"}, {"drain"}}},
		{"quota wall → checkpoint/defer, no relaunch", [][]string{{"quota"}, {"exhaust"}}},
		{"max_batches cap", [][]string{{"maxbatch"}, {"batchcap"}, {"cap"}}},
		{"`.evolve/loop-stop` operator brake", [][]string{{"stopfile"}, {"loopstop"}, {"brake"}}},
	}
	for _, c := range conditions {
		if !anyNameMatches(names, c.alts) {
			t.Errorf("no PASSING TestRunLoopChain* test covers the %s stop condition (saw: %s)", c.label, sortedKeys(names))
		}
	}
	if len(names) < 4 {
		t.Errorf("only %d TestRunLoopChain* tests passed; all four boundary stop conditions need distinct coverage (saw: %s)", len(names), sortedKeys(names))
	}
}

func TestC1075_004_FleetWidthPreservedAcrossBatches(t *testing.T) {
	names := runChainTests(t, "TestRunLoopChain.*(Fleet|Width)|Fleet.*Chain")
	if !anyNameMatches(names, [][]string{{"width"}, {"lane"}}) {
		t.Errorf("no PASSING test asserts fleet width/lane count is preserved across chained batches (saw: %s)", sortedKeys(names))
	}
}

func sortedKeys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	if len(out) == 0 {
		return "<none>"
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return strings.Join(out, ", ")
}

//go:build acs

package cycle2384

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	toolImportPath = "github.com/mickeyyaya/evolve-loop/go/cmd/testlatency"
	modulePrefix   = "github.com/mickeyyaya/evolve-loop/go/"
	toolTimeout    = 2 * time.Minute
	bridge         = "internal/bridge"
	docPath        = "docs/architecture/packages/cmd-testlatency.md"
	ratchetFile    = "go/.test-latency.json"
)

var toolDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "testlatency-c2384-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	toolDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type builtTool struct {
	path string
	err  error
}

var buildTool = sync.OnceValue(func() builtTool {
	out := filepath.Join(toolDir, "testlatency")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", out, toolImportPath)
	if err != nil || code != 0 {
		return builtTool{err: fmt.Errorf("go build exited %d: %v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))}
	}
	return builtTool{path: out}
})

func requireTool(t *testing.T) string {
	t.Helper()
	built := buildTool()
	if built.err != nil {
		t.Fatalf("RED: %v", built.err)
	}
	return built.path
}

type outcome struct {
	code           int
	stdout, stderr string
}

func (o outcome) all() string { return o.stdout + o.stderr }

func execIn(t *testing.T, dir string, env []string, name string, args ...string) outcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(), env...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %s: %v", name, err)
		}
		code = ee.ExitCode()
	}
	return outcome{code: code, stdout: out.String(), stderr: errOut.String()}
}

type series struct {
	order    []string
	baseline map[string]float64
}

func (s series) runCPU(speed float64, factor map[string]float64) map[string]float64 {
	cpu := map[string]float64{}
	for _, name := range s.order {
		f := 1.0
		if v, ok := factor[name]; ok {
			f = v
		}
		cpu[name] = s.baseline[name] * speed * f
	}
	return cpu
}

var macOS = series{
	order:    []string{bridge, "cmd/evolve", "internal/core", "internal/phases/ship", "internal/looppreflight"},
	baseline: map[string]float64{bridge: 390, "cmd/evolve": 220, "internal/core": 140, "internal/phases/ship": 60, "internal/looppreflight": 8},
}

var ubuntu = series{
	order: []string{bridge, "cmd/evolve", "internal/core", "internal/phases/audit", "internal/phases/ship", "test/structure"},
	baseline: map[string]float64{
		bridge: 165, "cmd/evolve": 90, "internal/core": 70, "internal/phases/audit": 40, "internal/phases/ship": 30, "test/structure": 25,
	},
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err == nil {
		err = os.WriteFile(path, body, 0o644)
	}
	if err != nil {
		t.Fatalf("writing %s: %v", filepath.Base(path), err)
	}
}

func ratchetBody(order []string, baseline map[string]float64) map[string]any {
	records := map[string]any{}
	for _, name := range order {
		records[name] = map[string]any{"class": "small", "p90": 5.0, "cpu": map[string]float64{runtime.GOOS: baseline[name]}}
	}
	return map[string]any{"packages": records}
}

func writeRun(t *testing.T, dir string, order []string, cpu map[string]float64) {
	t.Helper()
	var stream, usage strings.Builder
	for _, name := range order {
		fmt.Fprintf(&stream, `{"Action":"pass","Package":%q,"Elapsed":30}`+"\n", modulePrefix+name)
		fmt.Fprintf(&usage, `{"package":%q,"wall_s":30,"user_s":%v,"system_s":%v,"exit_code":0}`+"\n", name, cpu[name]*0.6, cpu[name]*0.4)
	}
	for file, body := range map[string]string{"test-integration.json": stream.String(), "test-rusage.ndjson": usage.String()} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func gate(t *testing.T, dir, ratchet string) outcome {
	t.Helper()
	tool := requireTool(t)
	stdin := filepath.Join(dir, "test-integration.json")
	script := fmt.Sprintf("exec %q -ratchet %q -rusage test-rusage.ndjson < %q", tool, ratchet, stdin)
	return execIn(t, dir, nil, "sh", "-c", script)
}

func failedPackages(o outcome) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(o.stderr), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, strings.Fields(line)[0])
		}
	}
	sort.Strings(out)
	return out
}

type replayRun struct {
	id     string
	speed  float64
	factor map[string]float64
	failed bool
}

func replay(t *testing.T, s series, runs []replayRun) {
	t.Helper()
	for _, r := range runs {
		dir := t.TempDir()
		writeJSON(t, filepath.Join(dir, ".test-latency.json"), ratchetBody(s.order, s.baseline))
		writeRun(t, dir, s.order, s.runCPU(r.speed, r.factor))
		got := gate(t, dir, ".test-latency.json")
		switch {
		case r.failed && (got.code != 1 || strings.Join(failedPackages(got), ",") != bridge):
			t.Errorf("RED: run %s: exit %d with failures %v, want exit 1 naming only %s\n%s", r.id, got.code, failedPackages(got), bridge, acsassert.OutputTail(got.all(), 8))
		case !r.failed && got.code != 0:
			t.Errorf("RED: run %s: exit %d, want 0\n%s", r.id, got.code, acsassert.OutputTail(got.all(), 8))
		}
	}
}

func TestC2384_001_TheMacOSReplayPassesTheTwoCleanRunsAndFailsTheDefectRunsOnBridge(t *testing.T) {
	replay(t, macOS, []replayRun{
		{id: "40582532714 (base)", speed: 1.00},
		{id: "40611567029 (3-core host)", speed: 1.46, factor: map[string]float64{bridge: 0.987, "internal/looppreflight": 1.29}},
		{id: "40563673621 (clean)", speed: 0.91, factor: map[string]float64{bridge: 1.02}},
		{id: "40526866678 (defect)", speed: 0.90, factor: map[string]float64{bridge: 1.48}, failed: true},
		{id: "40576371034 (defect)", speed: 1.02, factor: map[string]float64{bridge: 1.52}, failed: true},
	})
}

func TestC2384_002_TheUbuntuReplayPassesTheTwoCleanRunsAndFailsTheDefectRunsOnBridge(t *testing.T) {
	replay(t, ubuntu, []replayRun{
		{id: "40582532714 (base)", speed: 1.00},
		{id: "40611567029 (clean)", speed: 1.06, factor: map[string]float64{bridge: 1.07}},
		{id: "40563673621 (clean)", speed: 1.26, factor: map[string]float64{bridge: 1.05}},
		{id: "40526866678 (defect)", speed: 1.52, factor: map[string]float64{bridge: 2.01}, failed: true},
		{id: "40576371034 (defect)", speed: 1.29, factor: map[string]float64{bridge: 2.00}, failed: true},
	})
}

func TestC2384_003_BelowThreeReferencePackagesTheDefectRunIsReportedAndNeverFails(t *testing.T) {
	order := ubuntu.order[:3]
	baseline := map[string]float64{}
	for _, name := range order {
		baseline[name] = ubuntu.baseline[name]
	}
	two := series{order: order, baseline: baseline}
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".test-latency.json"), ratchetBody(order, baseline))
	writeRun(t, dir, order, two.runCPU(1.52, map[string]float64{bridge: 2.01}))
	got := gate(t, dir, ".test-latency.json")
	if got.code != 0 || !strings.Contains(got.stdout, "reported only") {
		t.Errorf("RED: two other reference packages and a 2x bridge: exit %d, want 0 with a reported only row\n%s", got.code, acsassert.OutputTail(got.all(), 10))
	}
	three := series{order: ubuntu.order[:4], baseline: ubuntu.baseline}
	writeJSON(t, filepath.Join(dir, ".test-latency.json"), ratchetBody(three.order, ubuntu.baseline))
	writeRun(t, dir, three.order, three.runCPU(1.52, map[string]float64{bridge: 2.01}))
	if got := gate(t, dir, ".test-latency.json"); got.code != 1 {
		t.Errorf("RED: three other reference packages and a 2x bridge: exit %d, want 1\n%s", got.code, acsassert.OutputTail(got.all(), 10))
	}
}

func TestC2384_004_APRThatAddsRealWorkPassesOnlyWhenItRaisesThePackageBaselineInTheSamePR(t *testing.T) {
	order := ubuntu.order
	realWork := map[string]float64{"internal/core": 1.60}
	dir := t.TempDir()
	writeRun(t, dir, order, ubuntu.runCPU(1.0, realWork))

	baseRef := ratchetBody(order, ubuntu.baseline)
	writeJSON(t, filepath.Join(dir, "base-ref.json"), baseRef)

	raisedBaseline := map[string]float64{}
	for name, cpu := range ubuntu.baseline {
		raisedBaseline[name] = cpu
	}
	raisedBaseline["internal/core"] = ubuntu.baseline["internal/core"] * 1.60
	writeJSON(t, filepath.Join(dir, "head-raised.json"), ratchetBody(order, raisedBaseline))

	otherRaised := map[string]float64{}
	for name, cpu := range ubuntu.baseline {
		otherRaised[name] = cpu
	}
	otherRaised["internal/phases/audit"] = ubuntu.baseline["internal/phases/audit"] * 2
	writeJSON(t, filepath.Join(dir, "head-unrelated.json"), ratchetBody(order, otherRaised))

	cases := []struct {
		label, file string
		want        int
	}{
		{"1.6x of work in core, the baseline file as at the base ref", "base-ref.json", 1},
		{"1.6x of work in core, core raised in the same PR", "head-raised.json", 0},
		{"1.6x of work in core, the PR raised only another package", "head-unrelated.json", 1},
	}
	for _, c := range cases {
		got := gate(t, dir, c.file)
		if got.code != c.want {
			t.Errorf("RED: %s: exit %d, want %d\n%s", c.label, got.code, c.want, acsassert.OutputTail(got.all(), 8))
		}
		if c.want == 1 && !strings.Contains(got.stderr, "internal/core") {
			t.Errorf("RED: %s: the failure must name internal/core\n%s", c.label, got.stderr)
		}
	}
}

func ratchetCommands(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "go.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if cmd := strings.TrimSpace(line); strings.HasPrefix(cmd, "go run ./cmd/testlatency") && strings.Contains(cmd, "-ratchet ") {
			out = append(out, cmd)
		}
	}
	return out
}

var darwinTimeout = regexp.MustCompile(`"\$\(make -s print-test-timeout-darwin\)"`)

func TestC2384_005_BothCIStepsUseTheCPUGateAsWrittenAndPassNoReportOnlyFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	commands := ratchetCommands(t, root)
	if len(commands) < 2 {
		t.Fatalf("RED: want a ratchet command in the ubuntu job and in the macOS job of go.yml, found %d", len(commands))
	}
	tool := requireTool(t)
	for i, cmd := range commands {
		if strings.Contains(cmd, "report-only") || !strings.Contains(cmd, "-rusage ") {
			t.Errorf("RED: ratchet command %d must pass -rusage and no report-only flag:\n%s", i, cmd)
			continue
		}
		script := darwinTimeout.ReplaceAllString(strings.Replace(cmd, "go run ./cmd/testlatency", "'"+tool+"'", 1), "20m")
		for label, factor := range map[string]float64{"a 2.2x regression in bridge": 2.2, "no regression": 1.0} {
			dir := t.TempDir()
			writeJSON(t, filepath.Join(dir, ".test-latency.json"), ratchetBody(ubuntu.order, ubuntu.baseline))
			writeRun(t, dir, ubuntu.order, ubuntu.runCPU(1.0, map[string]float64{bridge: factor}))
			got := execIn(t, dir, []string{"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary.md")}, "sh", "-c", script)
			want := 0
			if factor > 1.35 {
				want = 1
			}
			if got.code != want {
				t.Errorf("RED: ratchet command %d on %s: exit %d, want %d\n%s", i, label, got.code, want, acsassert.OutputTail(got.all(), 8))
			}
		}
	}
}

func TestC2384_006_TheToolDocPassesSteLintWithNoFinding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(filepath.Join(root, docPath), stelint.Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	var real []string
	for _, f := range findings {
		if !f.IsSkipNotice() {
			real = append(real, fmt.Sprintf("line %d %s: %s", f.Line, f.Rule, f.Excerpt))
		}
	}
	if len(real) > 0 {
		t.Errorf("RED: %d ste-lint finding(s) in %s, first: %v", len(real), docPath, real[:min(5, len(real))])
	}
}

// acs-predicate: config-check
func TestC2384_007_TheToolDocStatesThatAPRRaisesABaselineInTheSamePRAndNamesTheFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, docPath))
	if err != nil {
		t.Fatal(err)
	}
	var block []string
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "- **CPU judgment.**") {
			in = true
		} else if in && !strings.HasPrefix(line, "  ") {
			break
		}
		if in {
			block = append(block, line)
		}
	}
	text := strings.ToLower(strings.Join(block, "\n"))
	if len(block) == 0 {
		t.Fatal("RED: the CPU judgment item is missing from the tool doc")
	}
	for _, need := range []string{"same pr", "raise", ratchetFile, "baseline"} {
		if !strings.Contains(text, need) {
			t.Errorf("RED: the CPU judgment item of %s must state that a PR raises a baseline in the same PR in %s; %q is missing", docPath, ratchetFile, need)
		}
	}
	if !strings.Contains(text, "fails") {
		t.Errorf("RED: the CPU judgment item must state that a raise without the file change fails")
	}
}

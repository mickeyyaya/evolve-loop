//go:build acs

package cycle2360

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
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
)

var toolDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "testlatency-acs-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	toolDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

var buildTool = sync.OnceValues(func() (string, error) {
	out := filepath.Join(toolDir, "testlatency")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", out, toolImportPath)
	if err != nil || code != 0 {
		return "", fmt.Errorf("go build exited %d: %v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))
	}
	return out, nil
})

func requireTool(t *testing.T) string {
	t.Helper()
	path, err := buildTool()
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	return path
}

type pkgCase struct {
	name    string
	baseCPU float64
	runCPU  float64
	wall    float64
}

type fixture struct {
	pkgs       []pkgCase
	baselineOS string
}

func (f fixture) osKey() string {
	if f.baselineOS != "" {
		return f.baselineOS
	}
	return runtime.GOOS
}

func (f fixture) write(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "go")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	records := map[string]any{}
	var stream, usage strings.Builder
	for _, p := range f.pkgs {
		rec := map[string]any{"class": "small", "p90": 5.0}
		if p.baseCPU > 0 {
			rec["cpu"] = map[string]float64{f.osKey(): p.baseCPU}
		}
		records[p.name] = rec
		fmt.Fprintf(&stream, `{"Action":"pass","Package":%q,"Elapsed":%v}`+"\n", modulePrefix+p.name, p.wall)
		fmt.Fprintf(&usage, `{"package":%q,"wall_s":%v,"user_s":%v,"system_s":%v,"exit_code":0}`+"\n",
			p.name, p.wall, p.runCPU*0.6, p.runCPU*0.4)
	}
	body, err := json.Marshal(map[string]any{"packages": records})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".test-latency.json":    string(body),
		"test-integration.json": stream.String(),
		"test-rusage.ndjson":    usage.String(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type outcome struct {
	code   int
	stdout string
	stderr string
}

func (o outcome) all() string { return o.stdout + o.stderr }

func execIn(t *testing.T, dir string, stdin *os.File, env []string, name string, args ...string) outcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = stdin
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %s: %v", name, err)
		}
		code = ee.ExitCode()
	}
	return outcome{code: code, stdout: out.String(), stderr: errOut.String()}
}

func gate(t *testing.T, dir string, extra ...string) outcome {
	t.Helper()
	tool := requireTool(t)
	stream, err := os.Open(filepath.Join(dir, "test-integration.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	args := append([]string{"-ratchet", ".test-latency.json", "-rusage", "test-rusage.ndjson"}, extra...)
	return execIn(t, dir, stream, nil, tool, args...)
}

func refs(count int, base, run float64) []pkgCase {
	out := make([]pkgCase, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, pkgCase{name: fmt.Sprintf("internal/ref%d", i), baseCPU: base, runCPU: run, wall: 30})
	}
	return out
}

func target(base, run, wall float64) pkgCase {
	return pkgCase{name: "internal/target", baseCPU: base, runCPU: run, wall: wall}
}

func firstLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.Join(lines[:min(n, len(lines))], "\n")
}

func wantExit(t *testing.T, label string, got outcome, want int) {
	t.Helper()
	if got.code != want {
		t.Errorf("RED: %s: exit %d, want %d\nstdout: %s\nstderr: %s", label, got.code, want, acsassert.OutputTail(got.stdout, 6), firstLines(got.stderr, 4))
	}
}

var wallNeverFlagged = fixture{pkgs: []pkgCase{
	target(100, 150, 140),
	{name: "internal/ref0", baseCPU: 20, runCPU: 30, wall: 140},
	{name: "internal/ref1", baseCPU: 20, runCPU: 30, wall: 135},
	{name: "internal/ref2", baseCPU: 20, runCPU: 30, wall: 130},
	{name: "internal/ref3", baseCPU: 20, runCPU: 30, wall: 125},
}}

func TestC2360_001_ASlowerHostWithTwoTimesTheWallPassesTheHardRatchet(t *testing.T) {
	dir := wallNeverFlagged.write(t)
	wantExit(t, "every package 1.5x slower in CPU, wall above the old class limit", gate(t, dir), 0)
}

func TestC2360_002_ARealCPURegressionFailsAndNamesThePackage(t *testing.T) {
	f := fixture{pkgs: append(refs(5, 20, 20), target(100, 200, 90))}
	got := gate(t, f.write(t))
	wantExit(t, "one package at twice its CPU baseline", got, 1)
	if !strings.Contains(got.all(), "internal/target") {
		t.Errorf("RED: the failure must name internal/target\n%s", acsassert.OutputTail(got.all(), 12))
	}
}

func TestC2360_003_TheSpeedIndexIsAMedianSoSlowMinorityReferencesDoNotHideARegression(t *testing.T) {
	slowMinority := []pkgCase{
		{name: "internal/ref0", baseCPU: 20, runCPU: 20, wall: 30},
		{name: "internal/ref1", baseCPU: 20, runCPU: 20, wall: 30},
		{name: "internal/ref2", baseCPU: 20, runCPU: 20, wall: 30},
		{name: "internal/ref3", baseCPU: 2, runCPU: 6, wall: 30},
		{name: "internal/ref4", baseCPU: 2, runCPU: 6, wall: 30},
	}
	regressed := fixture{pkgs: append(append([]pkgCase{}, slowMinority...), target(100, 150, 60))}
	wantExit(t, "target 1.5x while two of five references run 3x", gate(t, regressed.write(t)), 1)
	steady := fixture{pkgs: append(append([]pkgCase{}, slowMinority...), target(100, 100, 60))}
	wantExit(t, "target unchanged while two of five references run 3x", gate(t, steady.write(t)), 0)
}

func TestC2360_004_AGrowthUnderTheRatioOrUnderTheFifteenSecondFloorPasses(t *testing.T) {
	cases := []struct {
		label string
		t     pkgCase
		want  int
	}{
		{"1.30x on 100 s of baseline (+30 s)", target(100, 130, 60), 0},
		{"1.40x on 100 s of baseline (+40 s)", target(100, 140, 60), 1},
		{"2.00x on 5 s of baseline (+5 s, under the floor)", target(5, 10, 60), 0},
		{"2.00x on 10 s of baseline (+10 s, under the floor)", target(10, 20, 60), 0},
		{"1.80x on 40 s of baseline (+32 s)", target(40, 72, 60), 1},
	}
	for _, c := range cases {
		f := fixture{pkgs: append(refs(5, 20, 20), c.t)}
		wantExit(t, c.label, gate(t, f.write(t)), c.want)
	}
}

func TestC2360_005_FewerThanThreeReferencePackagesReportButNeverFail(t *testing.T) {
	twoRefs := fixture{pkgs: append(refs(2, 20, 20), target(100, 300, 60))}
	got := gate(t, twoRefs.write(t))
	wantExit(t, "two reference packages and a 3x regression", got, 0)
	if !strings.Contains(strings.ToLower(got.all()), "report") {
		t.Errorf("RED: the output must say the package is only reported\n%s", acsassert.OutputTail(got.all(), 12))
	}
	smallOnes := fixture{pkgs: append(refs(2, 20, 20),
		pkgCase{name: "internal/tiny0", baseCPU: 1, runCPU: 1, wall: 3},
		pkgCase{name: "internal/tiny1", baseCPU: 1, runCPU: 1, wall: 3},
		pkgCase{name: "internal/tiny2", baseCPU: 1, runCPU: 1, wall: 3},
		target(100, 300, 60))}
	wantExit(t, "two references of 2 s or more beside three under 2 s", gate(t, smallOnes.write(t)), 0)
	threeRefs := fixture{pkgs: append(refs(3, 20, 20), target(100, 300, 60))}
	wantExit(t, "exactly three reference packages and a 3x regression", gate(t, threeRefs.write(t)), 1)
}

func TestC2360_006_AWallAboveTheGoTestTimeoutStillFails(t *testing.T) {
	f := fixture{pkgs: append(refs(5, 20, 20), target(100, 100, 150))}
	dir := f.write(t)
	got := gate(t, dir, "-test-timeout", "100s")
	wantExit(t, "wall 150 s against a 100 s go test timeout", got, 1)
	if !strings.Contains(got.all(), "internal/target") || strings.Contains(got.stderr, "flag provided but not defined") {
		t.Errorf("RED: the timeout failure must come from the judgment and name internal/target\n%s", acsassert.OutputTail(got.all(), 12))
	}
	wantExit(t, "the same wall under the default 20m timeout", gate(t, dir), 0)
}

func TestC2360_007_ABaselineOfAnotherOSNeverGatesThisHost(t *testing.T) {
	other := "darwin"
	if runtime.GOOS == "darwin" {
		other = "linux"
	}
	f := fixture{baselineOS: other, pkgs: append(refs(5, 20, 20), target(100, 500, 60))}
	wantExit(t, "5x regression against a baseline recorded for "+other, gate(t, f.write(t)), 0)
}

func TestC2360_008_AnUnreadableRusageFileFailsLoudly(t *testing.T) {
	f := fixture{pkgs: append(refs(5, 20, 20), target(100, 100, 60))}
	dir := f.write(t)
	if err := os.Remove(filepath.Join(dir, "test-rusage.ndjson")); err != nil {
		t.Fatal(err)
	}
	got := gate(t, dir)
	wantExit(t, "the rusage file is absent", got, 2)
	if !strings.Contains(got.all(), "test-rusage.ndjson") {
		t.Errorf("RED: the error must name the absent file\n%s", acsassert.OutputTail(got.all(), 12))
	}
}

func cpuOf(t *testing.T, dir, pkg string) (float64, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".test-latency.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Packages map[string]struct {
			CPU map[string]float64 `json:"cpu"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("RED: the ratchet file no longer parses: %v", err)
	}
	rec, ok := doc.Packages[pkg]
	if !ok {
		return 0, false
	}
	v, ok := rec.CPU[runtime.GOOS]
	return v, ok
}

func retighten(t *testing.T, dir string, usage string) outcome {
	t.Helper()
	tool := requireTool(t)
	if err := os.WriteFile(filepath.Join(dir, "test-rusage.ndjson"), []byte(usage), 0o644); err != nil {
		t.Fatal(err)
	}
	stream, err := os.Open(filepath.Join(dir, "test-integration.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	return execIn(t, dir, stream, nil, tool, "tighten", "-ratchet", ".test-latency.json", "-rusage", "test-rusage.ndjson")
}

func usageLine(pkg string, cpu float64) string {
	return fmt.Sprintf(`{"package":%q,"wall_s":10,"user_s":%v,"system_s":%v,"exit_code":0}`+"\n", pkg, cpu/2, cpu/2)
}

func TestC2360_009_TightenLowersAndSeedsTheCPUBaselineAndNeverRaisesIt(t *testing.T) {
	f := fixture{pkgs: []pkgCase{target(100, 100, 5), {name: "internal/fresh", wall: 5}}}
	dir := f.write(t)
	first := retighten(t, dir, usageLine("internal/target", 80)+usageLine("internal/fresh", 30)+usageLine("internal/ghost", 9))
	wantExit(t, "tighten with a faster run", first, 0)
	if v, ok := cpuOf(t, dir, "internal/target"); !ok || math.Abs(v-80) > 0.01 {
		t.Errorf("RED: a faster run must lower the baseline to 80, got %v (recorded %v)", v, ok)
	}
	if v, ok := cpuOf(t, dir, "internal/fresh"); !ok || math.Abs(v-30) > 0.01 {
		t.Errorf("RED: a package with no baseline for this OS must be seeded with 30, got %v (recorded %v)", v, ok)
	}
	if _, ok := cpuOf(t, dir, "internal/ghost"); ok {
		t.Errorf("RED: tighten must not add a package that the ratchet file does not list")
	}
	retighten(t, dir, usageLine("internal/target", 120))
	if v, _ := cpuOf(t, dir, "internal/target"); math.Abs(v-80) > 0.01 {
		t.Errorf("RED: a slower run must never raise the baseline, got %v", v)
	}
}

func TestC2360_010_TheRealRatchetFileStillLoadsUnderTheNewJudgment(t *testing.T) {
	tool := requireTool(t)
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "go", ".test-latency.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		".test-latency.json":    string(raw),
		"test-integration.json": fmt.Sprintf(`{"Action":"pass","Package":%q,"Elapsed":0.2}`+"\n", modulePrefix+"cmd/commentaudit"),
		"test-rusage.ndjson":    usageLine("cmd/commentaudit", 0.2),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stream, err := os.Open(filepath.Join(dir, "test-integration.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	got := execIn(t, dir, stream, nil, tool, "-ratchet", ".test-latency.json", "-rusage", "test-rusage.ndjson")
	wantExit(t, "the committed go/.test-latency.json with one quick package", got, 0)
}

func workflowRatchetScripts(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "go.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	indentOf := func(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }
	var scripts []string
	for i := 0; i < len(lines); i++ {
		if !strings.Contains(lines[i], "- name:") || !strings.Contains(strings.ToLower(lines[i]), "package wall ratchet") {
			continue
		}
		for j := i + 1; j < len(lines) && !strings.Contains(lines[j], "- name:"); j++ {
			if strings.TrimSpace(lines[j]) != "run: |" {
				continue
			}
			runIndent := indentOf(lines[j])
			var body []string
			for k := j + 1; k < len(lines) && (strings.TrimSpace(lines[k]) == "" || indentOf(lines[k]) > runIndent); k++ {
				body = append(body, strings.TrimSpace(lines[k]))
			}
			scripts = append(scripts, strings.Join(body, "\n"))
			break
		}
	}
	return scripts
}

func runWorkflowStep(t *testing.T, script string, f fixture) outcome {
	t.Helper()
	tool := requireTool(t)
	dir := f.write(t)
	summary := filepath.Join(dir, "summary.md")
	rewritten := strings.ReplaceAll(script, "go run ./cmd/testlatency", "'"+tool+"'")
	return execIn(t, dir, nil, []string{"GITHUB_STEP_SUMMARY=" + summary, "GITHUB_WORKSPACE=" + filepath.Dir(dir)}, "sh", "-c", rewritten)
}

func TestC2360_011_BothWorkflowRatchetStepsAsWrittenFailARegressionAndKeepNoReportOnlyFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scripts := workflowRatchetScripts(t, root)
	if len(scripts) < 2 {
		t.Fatalf("RED: want a package wall ratchet step in both the ubuntu and the macOS job, found %d", len(scripts))
	}
	regression := fixture{pkgs: append(refs(5, 20, 20), target(100, 220, 90))}
	for i, script := range scripts {
		if strings.Contains(script, "ratchet-report-only") {
			t.Errorf("RED: ratchet step %d still passes -ratchet-report-only:\n%s", i, script)
			continue
		}
		wantExit(t, fmt.Sprintf("workflow ratchet step %d on a 2.2x CPU regression", i), runWorkflowStep(t, script, regression), 1)
	}
}

func TestC2360_012_BothWorkflowRatchetStepsAsWrittenPassASlowNoisyHost(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scripts := workflowRatchetScripts(t, root)
	if len(scripts) < 2 {
		t.Fatalf("RED: want a package wall ratchet step in both the ubuntu and the macOS job, found %d", len(scripts))
	}
	for i, script := range scripts {
		if strings.Contains(script, "ratchet-report-only") {
			t.Errorf("RED: ratchet step %d passes only because -ratchet-report-only swallows every breach", i)
			continue
		}
		wantExit(t, fmt.Sprintf("workflow ratchet step %d on a 1.5x slower host with walls above the old limit", i),
			runWorkflowStep(t, script, wallNeverFlagged), 0)
	}
}

var (
	cellSplit  = regexp.MustCompile(`\s*\|\s*`)
	testName   = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9]*(?:_[A-Za-z0-9]+)*_?`)
	runID      = regexp.MustCompile(`\b\d{11}\b`)
	percent    = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	waitVocab  = regexp.MustCompile(`(?i)\b(cpu|process starts?|tmux|git|sleeps?|deadline|real clock|i/o|disk|sandbox-exec|go build|scheduling)\b`)
	backticked = regexp.MustCompile("`([a-z0-9][a-z0-9-]+)`")
)

type table struct {
	header []string
	rows   [][]string
}

func (tb table) col(keyword string) int {
	for i, h := range tb.header {
		if strings.Contains(strings.ToLower(h), keyword) {
			return i
		}
	}
	return -1
}

func parseTables(md string) []table {
	var tables []table
	var cur *table
	for _, line := range strings.Split(md, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "|") {
			cur = nil
			continue
		}
		cells := cellSplit.Split(strings.Trim(trim, "| "), -1)
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if cur != nil {
			for len(cells) < len(cur.header) {
				cells = append(cells, "")
			}
		}
		if strings.Trim(strings.Join(cells, ""), "-: ") == "" {
			continue
		}
		if cur == nil {
			tables = append(tables, table{header: cells})
			cur = &tables[len(tables)-1]
			continue
		}
		cur.rows = append(cur.rows, cells)
	}
	return tables
}

type rankedDoc struct {
	path    string
	text    string
	ranking table
}

func rankingDoc(t *testing.T) rankedDoc {
	t.Helper()
	root := acsassert.RepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "docs", "research", "ci-wall-variance*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range parseTables(string(raw)) {
			if tb.col("share") >= 0 && tb.col("waits on") >= 0 && tb.col("evidence") >= 0 {
				return rankedDoc{path: p, text: string(raw), ranking: tb}
			}
		}
	}
	t.Fatalf("RED: no doc under docs/research (ci-wall-variance*.md) has a ranking table with the columns Share, Waits on and Evidence")
	return rankedDoc{}
}

func TestC2360_013_TheDocRanksComponentsByShareOfTheVarianceSummingToAWhole(t *testing.T) {
	doc := rankingDoc(t)
	shareCol := doc.ranking.col("share")
	if len(doc.ranking.rows) < 4 {
		t.Fatalf("RED: the ranking has %d rows, want at least 4 components", len(doc.ranking.rows))
	}
	sum, prev := 0.0, math.Inf(1)
	for _, row := range doc.ranking.rows {
		m := percent.FindStringSubmatch(row[shareCol])
		if m == nil {
			t.Fatalf("RED: the share cell %q of row %v is not a percentage", row[shareCol], row)
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		if v > prev {
			t.Errorf("RED: the ranking is not in descending order of share at %q (%.1f after %.1f)", row[0], v, prev)
		}
		prev, sum = v, sum+v
	}
	if math.Abs(sum-100) > 2 {
		t.Errorf("RED: the shares sum to %.1f%%, want 100%% within 2 points", sum)
	}
	for _, id := range []string{"40526866678", "40563673621", "40576371034", "40582532714", "40611567029"} {
		if !strings.Contains(doc.text, id) {
			t.Errorf("RED: the doc never names run %s", id)
		}
	}
}

func repoTestFunctions(t *testing.T, root string) map[string]bool {
	t.Helper()
	declared := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
	found := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "go"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range declared.FindAllStringSubmatch(string(raw), -1) {
			found[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestC2360_014_EveryTestTheDocCitesAsEvidenceExistsInTheRepository(t *testing.T) {
	doc := rankingDoc(t)
	cited := map[string]bool{}
	for _, name := range testName.FindAllString(doc.text, -1) {
		cited[name] = true
	}
	if len(cited) < 8 {
		t.Fatalf("RED: the doc cites %d distinct tests, want at least 8 as per-test evidence", len(cited))
	}
	declared := repoTestFunctions(t, acsassert.RepoRoot(t))
	for name := range cited {
		if strings.HasSuffix(name, "_") {
			hit := false
			for fn := range declared {
				if strings.HasPrefix(fn, name) {
					hit = true
					break
				}
			}
			if !hit {
				t.Errorf("RED: no test function starts with the cited prefix %s", name)
			}
			continue
		}
		if !declared[name] {
			t.Errorf("RED: the doc cites %s, which no _test.go file declares", name)
		}
	}
	evidence := doc.ranking.col("evidence")
	for _, row := range doc.ranking.rows {
		if len(strings.TrimSpace(row[evidence])) < 3 {
			t.Errorf("RED: the ranked component %q has an empty evidence cell", row[0])
		}
	}
}

func TestC2360_015_EveryRankedComponentNamesWhatItWaitsOn(t *testing.T) {
	doc := rankingDoc(t)
	waits := doc.ranking.col("waits on")
	for i, row := range doc.ranking.rows {
		if !waitVocab.MatchString(row[waits]) {
			t.Errorf("RED: row %d (%q) waits-on cell %q names none of CPU, process start, tmux, git, sleep, deadline, I/O", i+1, row[0], row[waits])
		}
	}
	joined := ""
	for _, row := range doc.ranking.rows {
		joined += " " + row[waits]
	}
	kinds := map[string]bool{}
	for _, m := range waitVocab.FindAllString(joined, -1) {
		kinds[strings.ToLower(m)] = true
	}
	if len(kinds) < 3 {
		t.Errorf("RED: the doc separates only %d kinds of wait (%v), want at least 3", len(kinds), kinds)
	}
}

func TestC2360_016_TheDocPassesTheSteLintWithNoFinding(t *testing.T) {
	doc := rankingDoc(t)
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(doc.path, stelint.Options{Words: words})
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
		t.Errorf("RED: %d ste-lint finding(s) in %s, first: %v", len(real), filepath.Base(doc.path), real[:min(5, len(real))])
	}
}

func inboxItemFile(root, id string) string {
	for _, sub := range []string{"", "consumed", "processed"} {
		matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", sub, "*-"+id+".json"))
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

func TestC2360_017_EachFixOfTheDocHasAFiledInboxItemWithAcceptance(t *testing.T) {
	doc := rankingDoc(t)
	root := acsassert.RepoRoot(t)
	var fixes table
	for _, tb := range parseTables(doc.text) {
		if tb.col("fix") >= 0 && tb.col("follow-up") >= 0 {
			fixes = tb
			break
		}
	}
	if len(fixes.rows) < 5 {
		t.Fatalf("RED: the doc needs a table with the columns Fix and Follow-up and at least 5 fixes, found %d rows", len(fixes.rows))
	}
	followCol := fixes.col("follow-up")
	distinct := map[string]bool{}
	for _, row := range fixes.rows {
		m := backticked.FindStringSubmatch(row[followCol])
		if m == nil {
			t.Errorf("RED: fix %q names no inbox item id in backticks", row[0])
			continue
		}
		path := inboxItemFile(root, m[1])
		if path == "" {
			t.Errorf("RED: fix %q names inbox item %s, but no file .evolve/inbox/*-%s.json exists", row[0], m[1], m[1])
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var item struct {
			ID         string   `json:"id"`
			Title      string   `json:"title"`
			Acceptance []string `json:"acceptance"`
		}
		if err := json.Unmarshal(raw, &item); err != nil || item.ID != m[1] || item.Title == "" || len(item.Acceptance) == 0 {
			t.Errorf("RED: inbox item %s must parse with its id, a title and at least one acceptance line (err %v)", m[1], err)
		}
		distinct[m[1]] = true
	}
	if len(distinct) < 4 {
		t.Errorf("RED: the fixes share only %d distinct inbox items, want at least 4", len(distinct))
	}
}

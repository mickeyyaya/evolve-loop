//go:build acs

package cycle1748

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ratchetPkg  = "./internal/rawgitratchet"
	ratchetTest = "TestRatchet_NoNewRawGitFixtures"
	baselineRel = "internal/rawgitratchet/baseline.json"
	shipPkg     = "./internal/phases/ship"
	releasePkg  = "./internal/releasepipeline"

	shipRemoteAnchor = "TestShip_CommitMessage_TypedVsMap"
)

var (
	shipFiles = []string{
		"internal/phases/ship/explanation_gate_test.go",
		"internal/phases/ship/pushonly_test.go",
		"internal/phases/ship/realgit_testhelpers_test.go",
		"internal/phases/ship/stage_deleted_test.go",
	}
	releaseFiles = []string{
		"internal/releasepipeline/bridges_changelog_test.go",
		"internal/releasepipeline/classify_test.go",
		"internal/releasepipeline/default_release_verify_test.go",
		"internal/releasepipeline/git_helpers_test.go",
	}
)

func moduleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func tail(s string) string {
	const n = 6000
	if len(s) <= n {
		return s
	}
	return "...\n" + s[len(s)-n:]
}

func requireLeftRatchet(t *testing.T, files []string) {
	t.Helper()
	goDir := moduleDir(t)
	baseline, err := rawgitratchet.LoadBaseline(filepath.Join(goDir, filepath.FromSlash(baselineRel)))
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	bound, _, err := rawgitratchet.BoundTestFiles(goDir)
	if err != nil {
		t.Fatalf("bind test files: %v", err)
	}
	sites, err := rawgitratchet.Sites(goDir, bound)
	if err != nil {
		t.Fatalf("scan raw git sites: %v", err)
	}
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(goDir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("RED: %s is gone (%v) — migrate its fixtures, do not delete its tests", rel, err)
			continue
		}
		if n, listed := baseline[rel]; listed {
			t.Errorf("RED: %s is still listed in %s (%d raw init sites allowed) — migrate it and remove the entry", rel, baselineRel, n)
		}
		if n := sites[rel]; n > 0 {
			t.Errorf("RED: %s still builds a raw git repo (%d init sites by the ratchet scanner) — use gittest.Fixture / gittest.Bare / gittest.Clone", rel, n)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "^"+ratchetTest+"$", ratchetPkg)
	cmd.Dir = goDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("RED: %s failed: %v\n%s", ratchetTest, err, tail(string(out)))
	}
}

func testNames(t *testing.T, goDir string, files []string) []string {
	t.Helper()
	var names []string
	for _, rel := range files {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(goDir, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("RED: parse %s: %v", rel, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && fn.Type.Params.NumFields() == 1 {
				names = append(names, fn.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func runPattern(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

type gitOp struct {
	sub, maintenanceAuto, gcAuto, gitDir string
}

func (o gitOp) quiet() bool { return o.maintenanceAuto == "false" && o.gcAuto == "0" }

const shimScript = `#!@BASH@
set -uo pipefail
real=@REAL@
log=@LOG@
"$real" "$@"
rc=$?
[ "$rc" -eq 0 ] || exit "$rc"
globals=()
sub=""
while [ $# -gt 0 ]; do
  case "$1" in
    -C|-c)
      [ $# -ge 2 ] || exit 0
      globals+=("$1" "$2"); shift 2 ;;
    -*) globals+=("$1"); shift ;;
    *) sub="$1"; shift; break ;;
  esac
done
case "$sub" in
  commit|merge|fetch|pull|am|rebase|cherry-pick|revert) ;;
  push)
    dest=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --repo=*) dest="${1#--repo=}"; break ;;
        -o|--push-option) [ $# -ge 2 ] || exit 0; shift 2 ;;
        -*) shift ;;
        *) dest="$1"; break ;;
      esac
    done
    [ -n "$dest" ] || exit 0
    if ! "$real" ${globals[@]+"${globals[@]}"} -C "$dest" rev-parse --git-dir >/dev/null 2>&1; then
      dest=$("$real" ${globals[@]+"${globals[@]}"} remote get-url --push "$dest" 2>/dev/null) || exit 0
    fi
    globals+=(-C "${dest#file://}") ;;
  *) exit 0 ;;
esac
gd=$("$real" ${globals[@]+"${globals[@]}"} rev-parse --absolute-git-dir 2>/dev/null) || exit 0
ma=$("$real" ${globals[@]+"${globals[@]}"} config --get maintenance.auto 2>/dev/null) || ma="<unset>"
ga=$("$real" ${globals[@]+"${globals[@]}"} config --get gc.auto 2>/dev/null) || ga="<unset>"
printf '%s\t%s\t%s\t%s\n' "$sub" "$ma" "$ga" "$gd" >> "$log"
exit 0
`

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func runUnderProbe(t *testing.T, dir, pkg, pattern string, env ...string) (string, []gitOp, error) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash not on PATH: %v", err)
	}
	probe, err := os.MkdirTemp("", "acs-gitprobe-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(probe); err != nil {
			t.Logf("probe dir %s left behind: %v", probe, err)
		}
	})
	probe, err = filepath.EvalSymlinks(probe)
	if err != nil {
		t.Fatal(err)
	}
	binDir, tmpDir, logPath := filepath.Join(probe, "bin"), filepath.Join(probe, "tmp"), filepath.Join(probe, "git-ops.tsv")
	for _, d := range []string{binDir, tmpDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := strings.NewReplacer("@BASH@", bash, "@REAL@", shQuote(realGit), "@LOG@", shQuote(logPath)).Replace(shimScript)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{"test", "-count=1", "-v"}
	if pattern != "" {
		args = append(args, "-run", pattern)
	}
	cmd := exec.Command("go", append(args, pkg)...)
	cmd.Dir = dir
	cmd.Env = append(ipcenv.Scrub(os.Environ()),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+tmpDir)
	cmd.Env = append(cmd.Env, env...)
	out, runErr := cmd.CombinedOutput()

	body, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read probe log: %v", err)
	}
	var ops []gitOp
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 || !strings.HasPrefix(f[3], tmpDir+string(filepath.Separator)) {
			continue
		}
		ops = append(ops, gitOp{sub: f[0], maintenanceAuto: f[1], gcAuto: f[2], gitDir: strings.TrimPrefix(f[3], tmpDir)})
	}
	return string(out), ops, runErr
}

func requireQuietFixtures(t *testing.T, pkg, pattern string, mustPass []string) {
	t.Helper()
	out, ops, err := runUnderProbe(t, moduleDir(t), pkg, pattern)
	if err != nil {
		t.Errorf("RED: go test %s failed: %v\n%s", pkg, err, tail(out))
	}
	if strings.Contains(out, "directory not empty") {
		t.Errorf("RED: a test's temp dir removal raced a git child (`directory not empty`):\n%s", tail(out))
	}
	for _, name := range mustPass {
		if !strings.Contains(out, "--- PASS: "+name+" (") {
			t.Errorf("RED: %s did not pass under go test -count=1", name)
		}
	}
	if len(ops) == 0 {
		t.Fatalf("RED: the probe saw no commit/fetch/merge/push in a test repository — the git shim was not reached, nothing was judged")
	}
	var loud []string
	seen := map[string]bool{}
	for _, op := range ops {
		key := fmt.Sprintf("%s in %s (maintenance.auto=%s gc.auto=%s)", op.sub, op.gitDir, op.maintenanceAuto, op.gcAuto)
		if !op.quiet() && !seen[key] {
			seen[key] = true
			loud = append(loud, key)
		}
	}
	if len(loud) > 0 {
		sort.Strings(loud)
		t.Errorf("RED: %d of %d git ops ran in a test repository that leaves git's automatic maintenance on; build it with gittest.Fixture / gittest.Bare / gittest.Clone:\n  %s",
			len(loud), len(ops), strings.Join(loud, "\n  "))
	}
}

func TestC1748_001_ShipFixturesLeaveRatchetBaseline(t *testing.T) {
	requireLeftRatchet(t, shipFiles)
}

func TestC1748_002_ShipFixtureReposKeepMaintenanceOff(t *testing.T) {
	names := append(testNames(t, moduleDir(t), shipFiles), shipRemoteAnchor)
	if len(names) < 10 {
		t.Fatalf("RED: only %d tests found in the ship target files — their tests were dropped", len(names)-1)
	}
	requireQuietFixtures(t, shipPkg, runPattern(names), names)
}

func TestC1748_003_ReleasePipelineFixturesLeaveRatchetBaseline(t *testing.T) {
	requireLeftRatchet(t, releaseFiles)
}

func TestC1748_004_ReleasePipelineFixtureReposKeepMaintenanceOff(t *testing.T) {
	names := testNames(t, moduleDir(t), releaseFiles)
	if len(names) < 10 {
		t.Fatalf("RED: only %d tests found in the releasepipeline target files — their tests were dropped", len(names))
	}
	requireQuietFixtures(t, releasePkg, "", names)
}

const controlModule = `package probecontrol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func seed(t *testing.T, prefix string, config ...[2]string) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	run(t, dir, "init", "-q")
	for _, kv := range config {
		run(t, dir, "config", kv[0], kv[1])
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "a.txt")
	run(t, dir, "commit", "-q", "-m", "seed")
}

func TestRawFixture(t *testing.T) { seed(t, "raw-") }

func TestQuietFixture(t *testing.T) {
	seed(t, "quiet-", [2]string{"maintenance.auto", "false"}, [2]string{"gc.auto", "0"})
}
`

func TestC1748_005_MaintenanceProbeTellsRawFromQuietFixtures(t *testing.T) {
	mod := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":          "module probecontrol\n\ngo 1.23\n",
		"control_test.go": controlModule,
	} {
		if err := os.WriteFile(filepath.Join(mod, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, ops, err := runUnderProbe(t, mod, ".", "", "GOFLAGS=", "GOWORK=off")
	if err != nil {
		t.Fatalf("control module failed: %v\n%s", err, tail(out))
	}
	var raw, quiet []gitOp
	for _, op := range ops {
		switch {
		case strings.Contains(op.gitDir, string(filepath.Separator)+"raw-"):
			raw = append(raw, op)
		case strings.Contains(op.gitDir, string(filepath.Separator)+"quiet-"):
			quiet = append(quiet, op)
		}
	}
	if len(raw) == 0 || len(quiet) == 0 {
		t.Fatalf("probe saw %d raw and %d quiet commits, want both > 0; ops=%v", len(raw), len(quiet), ops)
	}
	for _, op := range raw {
		if op.quiet() {
			t.Errorf("probe judged the raw fixture quiet: %+v", op)
		}
	}
	for _, op := range quiet {
		if !op.quiet() {
			t.Errorf("probe judged the quiet fixture loud: %+v", op)
		}
	}
}

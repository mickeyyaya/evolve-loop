//go:build acs

package cycle1194

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const followTestFile = "go/cmd/evolve/cmd_bridge_watch_test.go"

var observingFollowTests = []string{
	"TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines",
	"TestRunBridgeWatchFollow_TailsNewLines",
}

const minFollowDeadline = 10

var (
	secondsDeadlineRe = regexp.MustCompile(`WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second\s*\)`)
	msSleepRe         = regexp.MustCompile(`time\.Sleep\([^)]*time\.Millisecond`)
)

func goFuncBody(path, funcName string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return "", err
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		return string(raw[fset.Position(fd.Pos()).Offset:fset.Position(fd.End()).Offset]), nil
	}
	return "", os.ErrNotExist
}

func TestC1194_001_follow_tests_race_clean_under_repetition(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-race", "-count=50",
		"-run", "TestRunBridgeWatchFollow", "./cmd/evolve/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test -race -count=50 -run TestRunBridgeWatchFollow ./cmd/evolve/ must be green: %v\n%s", err, out)
	}
}

func TestC1194_002_follow_waits_are_event_driven_with_long_deadline(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), followTestFile)
	for _, fn := range observingFollowTests {
		body, err := goFuncBody(path, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		m := secondsDeadlineRe.FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no seconds-scale context deadline found — a follow test that must OBSERVE an appended "+
				"line needs an event wait bounded by a deadline >= %ds, not a millisecond window that a loaded "+
				"macOS runner can miss", fn, minFollowDeadline)
			continue
		}
		secs, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			t.Errorf("%s: unparsable deadline literal %q: %v", fn, m[1], convErr)
			continue
		}
		if secs < minFollowDeadline {
			t.Errorf("%s: context deadline is %ds, want >= %ds", fn, secs, minFollowDeadline)
		}
		if n := len(msSleepRe.FindAllString(body, -1)); n != 0 {
			t.Errorf("%s: %d fixed millisecond sleep(s) remain — the wait must synchronise on the observable "+
				"event (the rendered line), retrying at the poll interval, never on a wall-clock guess", fn, n)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=acs@example.com",
		"-c", "user.name=acs",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", "acs: "+name)
}

func behindBaseRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	for _, d := range []string{origin, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	git(t, origin, "init", "--bare", "-b", "main")
	git(t, work, "init", "-b", "main")
	git(t, work, "remote", "add", "origin", origin)
	commitFile(t, work, "a.txt", "one")
	git(t, work, "push", "origin", "main")
	commitFile(t, work, "b.txt", "two")
	git(t, work, "push", "origin", "main")
	git(t, work, "reset", "--hard", "HEAD~1")
	return work
}

func TestC1194_003_boot_halts_on_base_behind_origin(t *testing.T) {
	work := behindBaseRepo(t)
	evolveDir := filepath.Join(work, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	res, err := looppreflight.Run(looppreflight.Options{
		ProjectRoot: work,
		EvolveDir:   evolveDir,
		ProfileDir:  filepath.Join(work, "profiles"),
		Stderr:      io.Discard,
		SkipBoot:    true,
	})
	if err != nil {
		t.Fatalf("looppreflight.Run harness fault: %v", err)
	}
	var found *looppreflight.CheckResult
	for i := range res.Checks {
		if res.Checks[i].Name == "base-divergence" {
			found = &res.Checks[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(res.Checks))
		for _, c := range res.Checks {
			names = append(names, c.Name)
		}
		t.Fatalf("boot preflight runs no `base-divergence` check — lanes would still be cut from a stale base; checks=%v", names)
	}
	if found.Level != looppreflight.LevelHalt {
		t.Errorf("local main is 1 commit behind origin/main: base-divergence must HALT the batch before any lane spawns, got level=%v (%s / %s)",
			found.Level, found.Message, found.Detail)
	}
	if !strings.Contains(found.Detail, "evolve sync-main") {
		t.Errorf("the halt must name the reconcile command `evolve sync-main` so the stop comes with a next step; detail=%q", found.Detail)
	}
}

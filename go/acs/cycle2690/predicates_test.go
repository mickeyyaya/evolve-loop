//go:build acs

package cycle2690

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	syncFile      = "go/internal/integration/sync.go"
	mainsyncFile  = "go/internal/mainsync/mainsync.go"
	gitexecFetch  = "go/internal/gitexec/fetch.go"
	integrationMd = "docs/architecture/packages/internal-integration.md"
	mainsyncMd    = "docs/architecture/packages/internal-mainsync.md"
)

var remainingBareFetchSites = map[string]bool{
	"go/cmd/evolve/cmd_boundary_ratchet.go":          true,
	"go/internal/marketplacepoll/marketplacepoll.go": true,
}

func pushMain(repo *gittest.Repo, file string) string {
	repo.Write(file, file+"\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", file)
	repo.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	return strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
}

func TestC2690_001_SyncToOriginSeesTheNewOriginMainWhateverRemoteOriginFetchHolds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		shape func(work *gittest.Repo)
	}{
		{"fetch config omits main", func(w *gittest.Repo) {
			w.Git("config", "remote.origin.fetch", "+refs/heads/other:refs/remotes/origin/other")
		}},
		{"fetch config is unset", func(w *gittest.Repo) {
			w.Git("config", "--unset-all", "remote.origin.fetch")
		}},
		{"fetch config is the default", func(w *gittest.Repo) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin := gittest.Bare(t)
			work := gittest.Clone(t, origin.Dir)
			pushMain(work, "a.txt")
			work.Git("fetch", "-q", "origin")
			tc.shape(work)
			tip := pushMain(gittest.Clone(t, origin.Dir), "b.txt")

			got, err := integration.SyncToOrigin(context.Background(), integration.Repo{Dir: work.Dir, Git: integration.SysGit()})

			if err != nil || got != integration.SyncMoved {
				t.Errorf("RED: SyncToOrigin = %q, %v, want %q", got, err, integration.SyncMoved)
			}
			if head := strings.TrimSpace(work.Git("rev-parse", "refs/heads/main")); head != tip {
				t.Errorf("RED: local main = %s, want the new origin tip %s", head, tip)
			}
		})
	}
}

func TestC2690_002_SyncToOriginStillReportsCurrentAndStillFailsLoudlyWhenOriginIsGone(t *testing.T) {
	origin := gittest.Bare(t)
	work := gittest.Clone(t, origin.Dir)
	pushMain(work, "a.txt")
	work.Git("config", "remote.origin.fetch", "+refs/heads/other:refs/remotes/origin/other")
	repo := integration.Repo{Dir: work.Dir, Git: integration.SysGit()}

	if got, err := integration.SyncToOrigin(context.Background(), repo); err != nil || got != integration.SyncCurrent {
		t.Errorf("RED: SyncToOrigin with nothing new = %q, %v, want %q", got, err, integration.SyncCurrent)
	}

	work.Git("remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	if got, err := integration.SyncToOrigin(context.Background(), repo); err == nil {
		t.Errorf("RED: SyncToOrigin with an unreachable origin = %q, nil, want an error", got)
	}
}

func fetchTokens(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if v, err := strconv.Unquote(e.Value); err == nil {
				return v
			}
		}
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func isBareMainFetch(tokens []string) bool {
	for i, tok := range tokens {
		if tok != "fetch" {
			continue
		}
		var rest []string
		for _, next := range tokens[i+1:] {
			if next != "" && !strings.HasPrefix(next, "-") {
				rest = append(rest, next)
			}
		}
		if len(rest) >= 2 && (rest[0] == "origin" || rest[0] == "Remote") &&
			(rest[1] == "main" || rest[1] == "mainBranch" || rest[1] == "MainBranch") {
			return true
		}
	}
	return false
}

func bareMainFetches(t *testing.T, path string) int {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	hits := 0
	ast.Inspect(parsed, func(n ast.Node) bool {
		var elems []ast.Expr
		switch node := n.(type) {
		case *ast.CallExpr:
			elems = node.Args
		case *ast.CompositeLit:
			elems = node.Elts
		default:
			return true
		}
		tokens := make([]string, len(elems))
		for i, el := range elems {
			tokens[i] = fetchTokens(el)
		}
		if isBareMainFetch(tokens) {
			hits++
		}
		return true
	})
	return hits
}

func writeSource(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.go")
	if err := os.WriteFile(path, []byte("package probe\n\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestC2690_003_TheBareFetchScannerFlagsEveryBareShapeAndNoRefspecShape(t *testing.T) {
	bare := map[string]string{
		"call arguments":    `func f() { git(ctx, dir, "", "fetch", "origin", mainBranch) }`,
		"slice literal":     `var s = []string{"fetch", "--quiet", "origin", "main"}`,
		"selector constant": `func f() { run("fetch", gitexec.Remote, gitexec.MainBranch) }`,
	}
	for name, body := range bare {
		if got := bareMainFetches(t, writeSource(t, body)); got != 1 {
			t.Errorf("RED: scanner missed the bare fetch shape %q: %d hits", name, got)
		}
	}
	explicit := map[string]string{
		"refspec constant": `func f() { git(ctx, dir, "", "fetch", "-q", "origin", mainFetchRefspec) }`,
		"pull request":     `func f() { git(ctx, dir, "", "fetch", "-q", "origin", "+refs/pull/"+n+"/head:x") }`,
		"other verb":       `func f() { git(ctx, dir, "", "pull", "origin", "main") }`,
	}
	for name, body := range explicit {
		if got := bareMainFetches(t, writeSource(t, body)); got != 0 {
			t.Errorf("RED: scanner flagged the explicit shape %q: %d hits", name, got)
		}
	}
}

func TestC2690_004_NoProductionCodeOutsideMainsyncRunsABareFetchOfOriginMain(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files, err := acsassert.WalkModule(filepath.Join(root, "go"), func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") &&
			!strings.HasPrefix(rel, "acs/") && !strings.HasPrefix(rel, "internal/mainsync/")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 100 {
		t.Fatalf("RED: the scan walked %d files; the module walk is wrong", len(files))
	}
	var offenders []string
	for _, rel := range files {
		rel = "go/" + strings.TrimPrefix(filepath.ToSlash(rel), "go/")
		if bareMainFetches(t, filepath.Join(root, rel)) > 0 && !remainingBareFetchSites[rel] {
			offenders = append(offenders, rel)
		}
	}
	sort.Strings(offenders)
	for _, rel := range offenders {
		t.Errorf("RED: %s runs a bare fetch of origin main outside internal/mainsync", rel)
	}
	if bareMainFetches(t, filepath.Join(root, syncFile)) != 0 {
		t.Errorf("RED: %s still runs a bare fetch of origin main", syncFile)
	}
}

func runGo(t *testing.T, args ...string) (string, error) {
	t.Helper()
	module, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(module, "go", args...)
	if err == nil && code != 0 {
		err = fmt.Errorf("exit status %d", code)
	}
	return stdout + stderr, err
}

func requireFullFileCoverage(t *testing.T, pkg, runPattern, fileSuffix string) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	args := []string{"test", "-count=1", "-coverprofile=" + profile}
	if runPattern != "" {
		args = append(args, "-run", runPattern)
	}
	if out, err := runGo(t, append(args, pkg)...); err != nil {
		t.Errorf("RED: go test %s exited with %v\n%s", pkg, err, acsassert.OutputTail(out, 30))
		return
	}
	report, err := runGo(t, "tool", "cover", "-func="+profile)
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, report)
	}
	seen := 0
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], fileSuffix+":") {
			continue
		}
		seen++
		if pct := fields[len(fields)-1]; pct != "100.0%" {
			t.Errorf("RED: %s %s is covered %s, want 100.0%%", fileSuffix, fields[1], pct)
		}
	}
	if seen == 0 {
		t.Errorf("RED: the cover report lists no function of %s", fileSuffix)
	}
}

func TestC2690_005_EveryFunctionOfTheChangedSyncFilesIsFullyCovered(t *testing.T) {
	requireFullFileCoverage(t, "./internal/integration", "^TestSyncToOrigin", "internal/integration/sync.go")
	requireFullFileCoverage(t, "./internal/mainsync", "", "internal/mainsync/mainsync.go")
	requireFullFileCoverage(t, "./internal/gitexec", "Fetch", "internal/gitexec/fetch.go")
}

func TestC2690_006_TheChangedFilesAndTheNewTestsCarryNoComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{syncFile, mainsyncFile, gitexecFetch, "go/internal/integration/sync_narrowed_fetch_test.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, group := range parsed.Comments {
			t.Errorf("RED: %s holds a comment: %s", rel, strings.TrimSpace(group.Text()))
		}
	}
	walkErr := filepath.WalkDir(filepath.Join(root, "go", "internal", "integration"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "sync") || !strings.HasSuffix(d.Name(), "_test.go") {
			return err
		}
		parsed, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if perr != nil {
			return perr
		}
		for _, group := range parsed.Comments {
			t.Errorf("RED: %s holds a comment: %s", d.Name(), strings.TrimSpace(group.Text()))
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
}

func TestC2690_007_TheIntegrationAndMainsyncPackageNotesRecordTheSharedFetchAndPassSte(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, rel := range []string{integrationMd, mainsyncMd} {
		findings, err := stelint.LintFile(filepath.Join(root, rel), stelint.Options{Words: words})
		if err != nil {
			t.Fatalf("lint %s: %v", rel, err)
		}
		for _, finding := range findings {
			t.Errorf("RED: %s:%d %s", rel, finding.Line, finding.Message)
		}
	}
	note := filepath.Join(root, integrationMd)
	if !acsassert.FileNotContains(t, note, "own refspec until `integration-sync-to-origin-uses-the-mainsync-fetch` lands") {
		t.Errorf("RED: %s still says the batcher fetch waits for this item", integrationMd)
	}
	if !acsassert.FileContains(t, note, "SyncToOrigin") {
		t.Errorf("RED: %s does not record how SyncToOrigin fetches origin main", integrationMd)
	}
}

//go:build acs

package cycle1837

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	overlongTitleFile      = "2026-10-01T00-00-00Z-overlong-title-item.json"
	overlongAcceptanceFile = "2026-10-01T00-00-01Z-overlong-acceptance-item.json"
	overlongFilesFile      = "2026-10-01T00-00-02Z-overlong-files-item.json"
	controlCharFile        = "2026-10-01T00-00-03Z-control-char-item.json"
	cleanFile              = "2026-10-01T00-00-04Z-clean-item.json"
	unparsableFile         = "2026-10-01T00-00-05Z-unparsable-item.json"
	wrongShapeFile         = "2026-10-01T00-00-06Z-wrong-shape-item.json"
	loadedFixtureCount     = 5
	timeOfDayStampFragment = "15-04-05"
	inboxbatchImportPath   = "github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1837: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1837-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

type cliRun struct {
	stdout, stderr string
	code           int
}

func (r cliRun) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func runEvolve(t *testing.T, projectRoot string, args ...string) cliRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = projectRoot, isolatedEnv(), 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := cliRun{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("evolve %s did not return within its budget: %s", strings.Join(args, " "), r)
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve %s: %v", strings.Join(args, " "), err)
	}
	return r
}

func loaderFixtures() map[string]string {
	return map[string]string{
		overlongTitleFile:      `{"id":"overlong-title-item","title":"` + strings.Repeat("t", 200) + `","acceptance":["ok"]}`,
		overlongAcceptanceFile: `{"id":"overlong-acceptance-item","title":"short","acceptance":["` + strings.Repeat("a", 700) + `"]}`,
		overlongFilesFile:      `{"id":"overlong-files-item","title":"short","files":["go/internal/` + strings.Repeat("f", 200) + `.go"],"acceptance":["ok"]}`,
		controlCharFile:        `{"id":"control-char-item","title":"line one\u0007line two","acceptance":["ok"]}`,
		cleanFile:              `{"id":"clean-item","title":"short","acceptance":["ok"]}`,
		unparsableFile:         `{not json`,
		wrongShapeFile:         `["not","an","object"]`,
	}
}

type loaderVerb struct {
	name        string
	args        []string
	inboxRel    string
	loadedCount string
}

func loaderVerbs() []loaderVerb {
	return []loaderVerb{
		{name: "batches", args: []string{"inbox", "batches"}, inboxRel: filepath.Join(".evolve", "inbox"), loadedCount: fmt.Sprintf("%d items -> ", loadedFixtureCount)},
		{name: "quarantine list", args: []string{"inbox", "quarantine", "list"}, inboxRel: filepath.Join(".evolve", "inbox", "quarantine"), loadedCount: fmt.Sprintf("%d quarantined item(s)", loadedFixtureCount)},
	}
}

func runVerbOverFixtures(t *testing.T, verb loaderVerb) cliRun {
	t.Helper()
	projectRoot := t.TempDir()
	inboxDir := filepath.Join(projectRoot, verb.inboxRel)
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range loaderFixtures() {
		if err := os.WriteFile(filepath.Join(inboxDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return runEvolve(t, projectRoot, verb.args...)
}

func warningLinesFor(stderr, fileName string) []string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, fileName) && strings.Contains(line, "WARN") {
			lines = append(lines, strings.ToLower(line))
		}
	}
	return lines
}

func anyLineContains(lines []string, word string) bool {
	for _, line := range lines {
		if strings.Contains(line, word) {
			return true
		}
	}
	return false
}

func TestC1837_001_LoadedButTruncatedItemPrintsAsTruncatedNeverSkipped(t *testing.T) {
	for _, verb := range loaderVerbs() {
		t.Run(verb.name, func(t *testing.T) {
			r := runVerbOverFixtures(t, verb)
			for _, truncatedFile := range []string{overlongTitleFile, overlongAcceptanceFile, overlongFilesFile} {
				lines := warningLinesFor(r.stderr, truncatedFile)
				switch {
				case len(lines) == 0:
					t.Errorf("evolve inbox %s printed no warning for %s, whose fields it cut to bound; the truncation must stay visible:\n%s", verb.name, truncatedFile, r)
				case anyLineContains(lines, "skipped"):
					t.Errorf("evolve inbox %s reports %s as skipped, but it loaded that item with truncated fields:\n%s", verb.name, truncatedFile, r)
				case !anyLineContains(lines, "truncat"):
					t.Errorf("evolve inbox %s does not report %s as truncated:\n%s", verb.name, truncatedFile, r)
				}
			}
		})
	}
}

func TestC1837_002_LoadedControlCharacterItemNeverPrintsAsSkipped(t *testing.T) {
	for _, verb := range loaderVerbs() {
		t.Run(verb.name, func(t *testing.T) {
			r := runVerbOverFixtures(t, verb)
			if anyLineContains(warningLinesFor(r.stderr, controlCharFile), "skipped") {
				t.Errorf("evolve inbox %s reports %s as skipped, but it loaded that item with its control characters replaced:\n%s", verb.name, controlCharFile, r)
			}
		})
	}
}

func TestC1837_003_UnparsableItemStillPrintsAsSkippedAndLoadedItemsStayCounted(t *testing.T) {
	for _, verb := range loaderVerbs() {
		t.Run(verb.name, func(t *testing.T) {
			r := runVerbOverFixtures(t, verb)
			if r.code != 0 {
				t.Fatalf("evolve inbox %s exited %d over a dir with unreadable records; a bad record is a warning, not a failure:\n%s", verb.name, r.code, r)
			}
			for _, skippedFile := range []string{unparsableFile, wrongShapeFile} {
				lines := warningLinesFor(r.stderr, skippedFile)
				if !anyLineContains(lines, "skipped") {
					t.Errorf("evolve inbox %s does not report the unparsable %s as skipped:\n%s", verb.name, skippedFile, r)
				}
				if anyLineContains(lines, "truncat") {
					t.Errorf("evolve inbox %s reports the unparsable %s as truncated; it never loaded:\n%s", verb.name, skippedFile, r)
				}
			}
			if !strings.Contains(r.stdout, verb.loadedCount) {
				t.Errorf("evolve inbox %s must still count every loaded item, truncated ones included (want %q):\n%s", verb.name, verb.loadedCount, r)
			}
			if lines := warningLinesFor(r.stderr, cleanFile); len(lines) != 0 {
				t.Errorf("evolve inbox %s warns about the clean record %s: %v", verb.name, cleanFile, lines)
			}
		})
	}
}

func nonTestGoFiles(t *testing.T, dir string, skipDirs map[string]bool) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}

func parseGoFile(t *testing.T, fset *token.FileSet, path string, src []byte) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func stampLayoutLiteralSites(t *testing.T, goDir string) []string {
	t.Helper()
	var sites []string
	fset := token.NewFileSet()
	for _, path := range nonTestGoFiles(t, goDir, map[string]bool{"vendor": true, "testdata": true, "acs": true}) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.Contains(src, []byte(timeOfDayStampFragment)) {
			continue
		}
		ast.Inspect(parseGoFile(t, fset, path, src), func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if value, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(value, timeOfDayStampFragment) {
				rel, _ := filepath.Rel(goDir, path)
				sites = append(sites, fmt.Sprintf("%s:%d", filepath.ToSlash(rel), fset.Position(lit.Pos()).Line))
			}
			return true
		})
	}
	return sites
}

func referencesIdent(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func inboxbatchStampSources(t *testing.T, goDir string) map[string]bool {
	t.Helper()
	sources := map[string]bool{"FilenameStampLayout": true}
	fset := token.NewFileSet()
	for _, path := range nonTestGoFiles(t, filepath.Join(goDir, "internal", "inboxbatch"), nil) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, decl := range parseGoFile(t, fset, path, src).Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil && referencesIdent(fn.Body, "FilenameStampLayout") {
				sources[fn.Name.Name] = true
			}
		}
	}
	return sources
}

func inboxbatchLocalName(f *ast.File) string {
	for _, imp := range f.Imports {
		if path, err := strconv.Unquote(imp.Path.Value); err == nil && path == inboxbatchImportPath {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return "inboxbatch"
		}
	}
	return ""
}

func ciwatchStampReferences(t *testing.T, goDir string) []string {
	t.Helper()
	sources := inboxbatchStampSources(t, goDir)
	var refs []string
	fset := token.NewFileSet()
	for _, path := range nonTestGoFiles(t, filepath.Join(goDir, "internal", "ciwatch"), nil) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		f := parseGoFile(t, fset, path, src)
		local := inboxbatchLocalName(f)
		if local == "" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == local && sources[sel.Sel.Name] {
				refs = append(refs, fmt.Sprintf("%s:%d %s.%s", filepath.Base(path), fset.Position(sel.Pos()).Line, local, sel.Sel.Name))
			}
			return true
		})
	}
	return refs
}

func TestC1837_004_FilenameStampLayoutHasOneNonTestDeclarationAndCiwatchWritesThroughIt(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	sites := stampLayoutLiteralSites(t, goDir)
	if len(sites) != 1 || !strings.HasPrefix(sites[0], "internal/inboxbatch/") {
		t.Errorf("want exactly one non-test string literal spelling the filename-stamp layout, declared in internal/inboxbatch; found %d: %v", len(sites), sites)
	}
	if refs := ciwatchStampReferences(t, goDir); len(refs) == 0 {
		t.Errorf("internal/ciwatch references neither inboxbatch.FilenameStampLayout nor an inboxbatch function built on it, so its escalation file name does not write through the single layout source")
	}
}

func TestC1837_005_CiwatchEscalationFileNameRoundTripsThroughFiledAt(t *testing.T) {
	inboxDir := t.TempDir()
	escalatedAt := time.Date(2026, 10, 8, 12, 34, 56, 0, time.UTC)
	redRun := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: "failure", FailingTest: "TestRed"}, nil
	}
	_, err := ciwatch.Watch(context.Background(), ciwatch.Options{
		SHA:      "0123456789abcdef0123",
		InboxDir: inboxDir,
		Fetch:    redRun,
		Now:      func() time.Time { return escalatedAt },
		Sleep:    func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("ciwatch.Watch over a red run: %v", err)
	}
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		t.Fatal(err)
	}
	const wantName = "2026-10-08T12-34-56Z-ci-red-0123456789ab.json"
	if len(entries) != 1 || entries[0].Name() != wantName {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Fatalf("ciwatch escalation file name changed: want exactly [%s], got %v", wantName, got)
	}
	it, _, err := inboxbatch.LoadFile(filepath.Join(inboxDir, wantName))
	if err != nil {
		t.Fatalf("inboxbatch.LoadFile on the ciwatch escalation: %v", err)
	}
	it.CreatedAt = ""
	if got := it.FiledAt(); !got.Equal(escalatedAt) {
		t.Errorf("FiledAt's file-name fallback reads the ciwatch stamp as %v, want %v", got, escalatedAt)
	}
}

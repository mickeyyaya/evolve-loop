//go:build acs

package cycle1742

import (
	"bytes"
	"encoding/json"
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

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	setupDir           = "internal/setup"
	releasepipelineDir = "internal/releasepipeline"
	baseCommit         = "d29ffcb1"
	offendersRel       = "go/internal/sizeratchet/offenders.json"
	mutantWorkers      = 6
)

var packageDirs = []string{setupDir, releasepipelineDir}

type target struct {
	key, pkg, file string
	allowance      int
}

var targets = []target{
	{key: releasepipelineDir + ".defaultReleaseVerify", pkg: releasepipelineDir, file: "releasepipeline.go", allowance: 60},
	{key: releasepipelineDir + ".newReleaseRun", pkg: releasepipelineDir, file: "release_run.go", allowance: 62},
	{key: releasepipelineDir + ".releaseRun.prePublish", pkg: releasepipelineDir, file: "release_run.go", allowance: 68},
	{key: setupDir + ".Apply", pkg: setupDir, file: "apply.go", allowance: 63},
	{key: setupDir + ".Detect", pkg: setupDir, file: "setup.go", allowance: 104},
	{key: setupDir + ".Recommend", pkg: setupDir, file: "recommend.go", allowance: 65},
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel))
}

func walkModule(t *testing.T) []sizeratchet.FuncSpan {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	return spans
}

func TestC1742_001_SixSetupReleasepipelineFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key, tg.pkg)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1742_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling; this lane leaves the file byte-unchanged:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key]; !listed || got != tg.allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key, got, listed, tg.allowance)
		}
	}
}

func TestC1742_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1742_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(diff) != "" {
			t.Errorf("RED: baseline %s test files modified or deleted vs %s (existing tests must stay unmodified):\n%s", dir, baseCommit, diff)
		}
	}
}

func TestC1742_005_SetupPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+setupDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", setupDir, code, out)
	}
}

func TestC1742_006_ReleasepipelinePackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+releasepipelineDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", releasepipelineDir, code, out)
	}
}

func TestC1742_007_BothPackagesVetClean(t *testing.T) {
	for _, dir := range packageDirs {
		if out, code := runGo(t, goModuleDir(t), "vet", "./"+dir); code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d):\n%s", dir, code, out)
		}
	}
}

func TestC1742_008_NoCommentLinesAdded(t *testing.T) {
	var out, errOut strings.Builder
	args := []string{"comments", "-base", baseCommit}
	for _, dir := range packageDirs {
		args = append(args, filepath.Join(goModuleDir(t), dir))
	}
	if code := commentaudit.Main(args, &out, &errOut, worktreeGit{root: acsassert.RepoRoot(t)}); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent):\n%s%s", code, strings.Join(packageDirs, ", "), out.String(), errOut.String())
	}
}

type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }

// acs-predicate: config-check — the doc comment text IS the contract under
func TestC1742_009_TargetFunctionDocsMatchBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	current := map[string]string{}
	for _, dir := range packageDirs {
		for key, doc := range funcDocs(t, dir, packageSources(t, dir)) {
			current[key] = doc
		}
	}
	for _, tg := range targets {
		src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+tg.pkg+"/"+tg.file)
		if code != 0 {
			t.Fatalf("git show %s:%s failed: %v\n%s", baseCommit, tg.file, err, stderr)
		}
		want, found := funcDocs(t, tg.pkg, map[string][]byte{tg.file: []byte(src)})[tg.key]
		if !found {
			t.Fatalf("baseline %s has no func %s", tg.file, tg.key)
		}
		got, found := current[tg.key]
		if !found {
			t.Errorf("RED: func %s not found in %s", tg.key, tg.pkg)
			continue
		}
		if got != want {
			t.Errorf("RED: doc comment of %s differs from baseline %s\n--- baseline ---\n%s--- worktree ---\n%s", tg.key, baseCommit, want, got)
		}
	}
}

// acs-predicate: config-check — comment text IS the contract under test
func TestC1742_010_NoBaselineCommentDeleted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	for _, dir := range packageDirs {
		var baseline, current []byte
		scoped := 0
		for _, rel := range changed {
			if filepath.Dir(rel) != "go/"+dir || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				continue
			}
			scoped++
			before, err := git.Show(baseCommit, rel)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
			}
			after, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("read %s: %v", rel, err)
			}
			baseline = append(append(baseline, before...), '\n')
			current = append(append(current, after...), '\n')
		}
		if scoped == 0 {
			t.Errorf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", dir, baseCommit)
			continue
		}
		if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
			t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s (restore each where its code now lives; code-comments.md:43):\n  %s", len(deleted), dir, baseCommit, strings.Join(deleted, "\n  "))
		}
	}
}

type mutant struct{ fn, name, anchor, replacement string }

var setupMutants = []mutant{
	{"Apply", "a whitespace-only policy is refused as malformed", `strings.TrimSpace(string(existingPolicyJSON))`, `string(existingPolicyJSON)`},
	{"Apply", "a malformed pins block is clobbered", `json.Unmarshal(raw, &pins); err != nil`, `json.Unmarshal(raw, &pins); err != nil && false`},
	{"Apply", "a floor-breaching pin is written", `policy.ValidatePin(a.Role, pin, &prof); verr != nil`, `policy.ValidatePin(a.Role, pin, &prof); verr != nil && false`},
	{"Apply", "an emptied pins block keeps the stale pins", `delete(obj, "pins")`, `delete(obj, "pinz")`},
	{"Apply", "the policy bytes lose their trailing newline", `append(out, '\n')`, `append(out)`},
	{"Apply", "the policy is re-encoded with tab indentation", `json.MarshalIndent(obj, "", "  ")`, `json.MarshalIndent(obj, "", "\t")`},
	{"Recommend", "a configured default is overridden by the first preset", `def == "" && len(cfg.Presets) > 0`, `len(cfg.Presets) > 0`},
	{"Recommend", "an unset default falls to the last preset", `cfg.Presets[0].Name`, `cfg.Presets[len(cfg.Presets)-1].Name`},
	{"Recommend", "a builder moved off its default family is not flagged as a fallback", `bldFam, bldFam != prefBase`, `bldFam, false`},
	{"Recommend", "an auditor moved off its default family is not flagged as a fallback", `audFam, audFam != prefBase`, `audFam, false`},
	{"Recommend", "an unpaired auditor gets an empty CLI", `ps.Role == "auditor" && audFam != ""`, `ps.Role == "auditor"`},
	{"Recommend", "a degraded assignment's rationale drops the warning", `rationale(spec.Name, cli, tier, clamped, fallback, warn)`, `rationale(spec.Name, cli, tier, clamped, fallback, "")`},
	{"Recommend", "the builder/auditor pair is never chosen", `hasBuilder && hasAuditor`, `hasBuilder && hasAuditor && false`},
	{"Detect", "the binary path is dropped", `r.Binary.Path`, `r.Binary.Path[:0]`},
	{"Detect", "env warnings are dropped", `r.EnvWarnings`, `r.EnvWarnings[:0]`},
	{"Detect", "auth-configured is dropped", `r.Auth.Configured`, `(r.Auth.Configured && false)`},
	{"Detect", "tier models are not surfaced", `tierModelsFor(b)`, `map[string]string(nil)`},
	{"Detect", "CLIs sort descending", `clis[i].CLI < clis[j].CLI`, `clis[i].CLI > clis[j].CLI`},
	{"Detect", "an unresolvable phase has no source", `Source: "unresolved"`, `Source: ""`},
	{"Detect", "allowed CLIs are dropped", `pc.AllowedCLIs`, `pc.AllowedCLIs[:0]`},
	{"Detect", "a CLI-less pin blanks the current CLI", `pin.CLI != ""`, `(pin.CLI != "" || true)`},
	{"Detect", "a model-less pin blanks the current tier", `pin.Model != ""`, `(pin.Model != "" || true)`},
	{"Detect", "the scan time is not UTC", `ScannedAt: now().UTC()`, `ScannedAt: now()`},
	{"Detect", "the default capability probe reads the plugin root", `capTierFromManifest(o.AdaptersDir, base)`, `capTierFromManifest(o.PluginRoot, base)`},
}

var releasepipelineMutants = []mutant{
	{"defaultReleaseVerify", "a disk binary that differs from the committed blob passes", `diskSHA != blobSHA`, `(false && diskSHA != blobSHA)`},
	{"defaultReleaseVerify", "the committed blob is read from HEAD, not the release commit", `commitSHA+":"+binRel`, `commitSHA[:0]+"HEAD:"+binRel`},
	{"defaultReleaseVerify", "an already-pinned state.json is rewritten", `st["expected_ship_sha"].(string)`, `st["expected_ship_sha+"].(string)`},
	{"defaultReleaseVerify", "a malformed state.json is written through", `json.Unmarshal(raw, &st)`, `func() error { _ = json.Unmarshal(raw, &st); return nil }()`},
	{"defaultReleaseVerify", "a binary whose --version lacks the target passes", `!strings.Contains(string(verOut), target)`, `(false && !strings.Contains(string(verOut), target))`},
	{"defaultReleaseVerify", "the released binary is never executed", `exec.Command(binAbs, "--version")`, `exec.Command("echo", target, binAbs[:0])`},
	{"defaultReleaseVerify", "an existing local tag is recreated", `"tag", "-l", tag`, `"tag", "-l", tag+"-absent"`},
	{"defaultReleaseVerify", "the local tag lands on HEAD, not the release commit", `"tag", tag, commitSHA`, `"tag", tag, commitSHA[:0]+"HEAD"`},
	{"defaultReleaseVerify", "a failed tag creation is ignored", `terr != nil`, `(terr != nil && false)`},
	{"newReleaseRun", "log lines lose the [release-pipeline] prefix", `"[release-pipeline] "`, `""`},
	{"newReleaseRun", "a given FromTag is overridden by the resolved one", `opts.FromTag`, `opts.FromTag[:0]`},
	{"newReleaseRun", "the previous tag is never used as the range start", `err == nil && t != ""`, `(err == nil && t != "" && false)`},
	{"newReleaseRun", "a tagless repo does not fall back to its initial commit", `r.fromTag = init`, `r.fromTag = init[:0]`},
	{"newReleaseRun", "the no-previous-tag warning is blank", `"WARN: no previous tag found; changelog range will start from initial commit"`, `""`},
	{"newReleaseRun", "a given Now seam is ignored", `r.now == nil`, `(r.now == nil || true)`},
	{"newReleaseRun", "the banner hides the target", `"target: v%s"`, `"target: v%.0s"`},
	{"newReleaseRun", "the banner hides the changelog range start", `"changelog range: %s..HEAD"`, `"changelog range: %.0s..HEAD"`},
	{"newReleaseRun", "the banner swaps the dry-run and skip-tests flags", `"dry-run: %v | no-rollback: %v | skip-tests: %v"`, `"skip-tests: %v | no-rollback: %v | dry-run: %v"`},
	{"newReleaseRun", "the banner hides the journal path", `"journal: %s"`, `"journal: %.0s"`},
	{"releaseRun.prePublish", "preflight gets DryRun and SkipTests swapped", `o.DryRun, o.SkipTests)`, `o.SkipTests, o.DryRun)`},
	{"releaseRun.prePublish", "changelog-gen gets the unresolved FromTag", `r.fromTag, "HEAD"`, `r.opts.FromTag, "HEAD"`},
	{"releaseRun.prePublish", "changelog-gen never dry-runs", `"HEAD", o.Target, o.DryRun)`, `"HEAD", o.Target, false)`},
	{"releaseRun.prePublish", "version-bump never dry-runs", `VersionBump(o.RepoRoot, o.Target, o.DryRun)`, `VersionBump(o.RepoRoot, o.Target, false)`},
	{"releaseRun.prePublish", "rebuild-binary is told it is a dry run", `RebuildBinary(o.RepoRoot, o.Target, false)`, `RebuildBinary(o.RepoRoot, o.Target, true)`},
	{"releaseRun.prePublish", "release.sh check gets a v-prefixed target", `ReleaseSh(o.RepoRoot, o.Target)`, `ReleaseSh(o.RepoRoot, "v"+o.Target)`},
	{"releaseRun.prePublish", "the dry-run rebuild-binary line hides the target", `…version=%s …`, `…version=%.0s …`},
	{"releaseRun.prePublish", "the dry-run release.sh-check line is blank", `"step: release.sh-check (DRY-RUN — skipping; markers not actually bumped)"`, `""`},
}

func TestC1742_011_SetupTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, setupDir, setupMutants)
}

func TestC1742_012_ReleasepipelineTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, releasepipelineDir, releasepipelineMutants)
}

func assertMutantsKilled(t *testing.T, dir string, mutants []mutant) {
	t.Helper()
	goDir := goModuleDir(t)
	overlays := make([]string, len(mutants)+1)
	for i, m := range mutants {
		overlays[i+1] = writeMutantOverlay(t, dir, m)
	}
	outcomes := make([]goRun, len(overlays))
	sem := make(chan struct{}, mutantWorkers)
	var wg sync.WaitGroup
	for i, overlay := range overlays {
		wg.Add(1)
		go func(i int, overlay string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			args := []string{"test", "-count=1", "-failfast"}
			if overlay != "" {
				args = append(args, "-overlay", overlay)
			}
			outcomes[i] = execGo(goDir, append(args, "./"+dir)...)
		}(i, overlay)
	}
	wg.Wait()
	if base := outcomes[0]; base.err != nil || base.code != 0 {
		t.Fatalf("RED: %s tests fail on the unmutated code (exit=%d, err=%v):\n%s", dir, base.code, base.err, base.out)
	}
	var survivors []string
	for i, m := range mutants {
		o := outcomes[i+1]
		switch {
		case o.err != nil:
			t.Fatalf("mutant %s: %s: %v", m.fn, m.name, o.err)
		case strings.Contains(o.out, "[build failed]") || strings.Contains(o.out, "[setup failed]"):
			survivors = append(survivors, fmt.Sprintf("%s: %s (mutant does not compile — keep the anchor %q verbatim in an expression that still compiles)", m.fn, m.name, m.anchor))
		case o.code == 0 || !strings.Contains(o.out, "--- FAIL"):
			survivors = append(survivors, fmt.Sprintf("%s: %s (exit=%d)", m.fn, m.name, o.code))
		}
	}
	if len(survivors) > 0 {
		t.Errorf("RED: %d/%d behavior mutants survive the %s tests — add characterization tests that pin them:\n  %s", len(survivors), len(mutants), dir, strings.Join(survivors, "\n  "))
	}
}

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return goRun{out: string(out)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return goRun{out: string(out), err: fmt.Errorf("go %s: %w", strings.Join(args, " "), err)}
	}
	return goRun{out: string(out), code: exitErr.ExitCode()}
}

func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	r := execGo(dir, args...)
	if r.err != nil {
		t.Fatal(r.err)
	}
	return r.out, r.code
}

func packageSources(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	abs := filepath.Join(goModuleDir(t), dir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	sources := make(map[string][]byte)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sources[name] = src
	}
	return sources
}

func funcDocs(t *testing.T, dir string, sources map[string][]byte) map[string]string {
	t.Helper()
	docs := make(map[string]string)
	fset := token.NewFileSet()
	for name, src := range sources {
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				docs[dir+"."+receiverPrefix(fn)+fn.Name.Name] = fn.Doc.Text()
			}
		}
	}
	return docs
}

func receiverPrefix(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name + "."
	}
	return fmt.Sprintf("%T.", expr)
}

func writeMutantOverlay(t *testing.T, dir string, m mutant) string {
	t.Helper()
	pkgDir := filepath.Join(goModuleDir(t), dir)
	replace := map[string]string{}
	tmp := t.TempDir()
	for name, src := range packageSources(t, dir) {
		if !strings.Contains(string(src), m.anchor) {
			continue
		}
		mutantPath := filepath.Join(tmp, name)
		if err := os.WriteFile(mutantPath, []byte(strings.ReplaceAll(string(src), m.anchor, m.replacement)), 0o644); err != nil {
			t.Fatalf("write mutant: %v", err)
		}
		replace[filepath.Join(pkgDir, name)] = mutantPath
	}
	if len(replace) == 0 {
		t.Fatalf("mutation anchor %q (%s: %s) no longer occurs in the %s non-test sources — keep it verbatim when extracting", m.anchor, m.fn, m.name, dir)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlayPath
}

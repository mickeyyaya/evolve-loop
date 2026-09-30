//go:build acs

package cycle1729

import (
	"bufio"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ratchetPkg    = "./internal/rawgitratchet"
	baselineRel   = "internal/rawgitratchet/baseline.json"
	scratchDir    = "internal/zzrawgitratchet"
	mirrorMaxFile = 1 << 20
)

var (
	gitBin   = "gi" + "t"
	gitWord  = strconv.Quote(gitBin)
	initWord = strconv.Quote("in" + "it")

	literalSite = regexp.MustCompile(`exec\.Command(?:Context)?\(\s*(?:[A-Za-z_][A-Za-z0-9_.()]*\s*,\s*)?` +
		regexp.QuoteMeta(gitWord) + `\s*,\s*` + regexp.QuoteMeta(initWord))
)

const literalInit = `package @PKG@

import (
	"os/exec"
	"testing"
)

func TestScratchLiteralInit(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command(@GIT@, @INIT@, "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
}
`

const wrappedInit = `package @PKG@

import (
	"os/exec"
	"testing"
)

func TestScratchWrappedInit(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(@GIT@, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run(@INIT@, "-q")
}
`

const extraInit = `
func zzExtraRawGitFixture(dir string) error {
	return exec.Command(@GIT@, @INIT@, "-q", dir).Run()
}
`

func fill(tmpl, pkg string) string {
	return strings.NewReplacer("@PKG@", pkg, "@GIT@", gitWord, "@INIT@", initWord).Replace(tmpl)
}

func moduleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requireRatchetPackage(t *testing.T, goDir string) {
	t.Helper()
	dir := filepath.Join(goDir, filepath.FromSlash(strings.TrimPrefix(ratchetPkg, "./")))
	matches, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if len(matches) == 0 {
		t.Fatalf("RED: %s has no test files — the ratchet package does not exist yet", dir)
	}
}

type mirror struct {
	goDir   string
	ceiling string
	repo    *gittest.Repo
}

func gitMirror(t *testing.T, src string) mirror {
	t.Helper()
	r := gittest.Fixture(t)
	dst := filepath.Join(r.Dir, "go")
	copyTree(t, src, dst)
	r.Git("add", "-A")
	return mirror{goDir: dst, ceiling: realDir(t, filepath.Dir(r.Dir)), repo: r}
}

func plainMirror(t *testing.T, src string) mirror {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "plain")
	dst := filepath.Join(parent, "go")
	copyTree(t, src, dst)
	return mirror{goDir: dst, ceiling: realDir(t, parent)}
}

func realDir(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > mirrorMaxFile {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (m mirror) write(t *testing.T, rel, src string, stage bool) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), rel, src, 0); err != nil {
		t.Fatalf("scratch %s is not valid Go: %v", rel, err)
	}
	path := filepath.Join(m.goDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if stage {
		m.stage(t, rel)
	}
}

func (m mirror) stage(t *testing.T, rel string) {
	t.Helper()
	m.repo.Git("add", "--", "go/"+rel)
}

func (m mirror) remove(t *testing.T, rel string) {
	t.Helper()
	m.repo.Git("rm", "-q", "-f", "--", "go/"+rel)
}

func runGo(t *testing.T, ceiling string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	if ceiling != "" {
		cmd.Env = append(cmd.Env, "GIT_CEILING_DIRECTORIES="+ceiling)
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("go %v did not run: %v", args, err)
	}
	return string(out), ee.ExitCode()
}

func runRatchet(t *testing.T, goDir, ceiling string) (string, int) {
	t.Helper()
	return runGo(t, ceiling, "test", "-C", goDir, "-count=1", "-v", ratchetPkg)
}

func expectPass(t *testing.T, what, out string, code int) {
	t.Helper()
	if code != 0 || !strings.Contains(out, "--- PASS: ") || strings.Contains(out, "--- FAIL: ") {
		t.Errorf("%s: want the ratchet suite to run and pass, exit %d\n%s", what, code, out)
	}
}

func expectCaught(t *testing.T, what, out string, code int, files ...string) {
	t.Helper()
	if code == 0 {
		t.Fatalf("%s: the ratchet passed; want a failure naming %v\n%s", what, files, out)
	}
	if !strings.Contains(out, "--- FAIL: ") {
		t.Errorf("%s: exit %d without a failing test — a build or setup failure, not the ratchet\n%s", what, code, out)
	}
	for _, f := range files {
		if !strings.Contains(out, f) {
			t.Errorf("%s: the ratchet's failure does not name %s\n%s", what, f, out)
		}
	}
}

func literalSites(t *testing.T, goDir string) []string {
	t.Helper()
	gittestDir := filepath.Join(goDir, "internal", "gittest")
	var sites []string
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == gittestDir || (path != goDir && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), "."))) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if literalSite.Match(src) {
			rel, err := filepath.Rel(goDir, path)
			if err != nil {
				return err
			}
			sites = append(sites, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", goDir, err)
	}
	if len(sites) == 0 {
		t.Skip("no literal raw init remains outside internal/gittest; nothing left to migrate")
	}
	sort.Strings(sites)
	return sites
}

func TestC1729_001_RatchetPassesOnCurrentTree(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	out, code := runRatchet(t, goDir, "")
	expectPass(t, "current tree", out, code)
	if _, _, rc, _ := acsassert.SubprocessOutput(gitBin, "-C", root, "ls-files", "--error-unmatch", "go/"+baselineRel); rc != 0 {
		t.Errorf("go/%s is not tracked or staged — the list of existing call sites would not ship", baselineRel)
	}
}

func TestC1729_002_NewLiteralRawInitFails(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := gitMirror(t, goDir)
	m.write(t, scratchDir+"/literal_init_test.go", fill(literalInit, "zzrawgitratchet"), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "new literal raw init", out, code, scratchDir+"/literal_init_test.go")
}

func TestC1729_003_NewHelperWrappedRawInitFails(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := gitMirror(t, goDir)
	m.write(t, scratchDir+"/wrapped_init_test.go", fill(wrappedInit, "zzrawgitratchet"), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "new helper-wrapped raw init", out, code, scratchDir+"/wrapped_init_test.go")
}

func TestC1729_004_NewRawInitOutsideInternalFails(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := gitMirror(t, goDir)
	m.write(t, "cmd/evolve/zz_rawgit_ratchet_test.go", fill(literalInit, "main"), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "new raw init under cmd/", out, code, "cmd/evolve/zz_rawgit_ratchet_test.go")
}

func TestC1729_005_NewRawInitInListedFileFails(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	site := literalSites(t, goDir)[0]
	m := gitMirror(t, goDir)
	src, err := os.ReadFile(filepath.Join(m.goDir, filepath.FromSlash(site)))
	if err != nil {
		t.Fatal(err)
	}
	m.write(t, site, string(src)+fill(extraInit, ""), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "one more call site in a listed file", out, code, site)
}

func TestC1729_006_MigratedSitesMustLeaveTheList(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	sites := literalSites(t, goDir)
	m := gitMirror(t, goDir)
	for _, site := range sites {
		m.remove(t, site)
	}
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "migrated sites still listed", out, code, sites...)
}

func TestC1729_007_MigratingOneSiteDoesNotAdmitANewOne(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	site := literalSites(t, goDir)[0]
	m := gitMirror(t, goDir)
	m.remove(t, site)
	m.write(t, scratchDir+"/swap_init_test.go", fill(literalInit, "zzrawgitratchet"), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "a migration swapped for a new site", out, code, scratchDir+"/swap_init_test.go")
}

func TestC1729_008_RawInitInsideGittestIsAllowed(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := gitMirror(t, goDir)
	m.write(t, "internal/gittest/zz_owned_init_test.go", fill(literalInit, "gittest"), true)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectPass(t, "raw init inside internal/gittest", out, code)
}

func TestC1729_009_UntrackedRawInitDoesNotRedUntilStaged(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := gitMirror(t, goDir)
	rel := scratchDir + "/untracked_init_test.go"
	m.write(t, rel, fill(literalInit, "zzrawgitratchet"), false)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectPass(t, "untracked raw init (runtime state that cannot land)", out, code)

	m.stage(t, rel)
	out, code = runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "the same raw init once staged", out, code, rel)
}

func TestC1729_010_ModuleOutsideGitBindsEveryTestFile(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	m := plainMirror(t, goDir)
	out, code := runRatchet(t, m.goDir, m.ceiling)
	expectPass(t, "untouched module outside git", out, code)

	m.write(t, scratchDir+"/plain_init_test.go", fill(literalInit, "zzrawgitratchet"), false)
	out, code = runRatchet(t, m.goDir, m.ceiling)
	expectCaught(t, "raw init in a module outside git", out, code, scratchDir+"/plain_init_test.go")
}

func repoContractPackages(t *testing.T, goDir string) []string {
	t.Helper()
	path := filepath.Join(goDir, "internal", "phases", "ship", "repocontract.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if name.Name != "repoContractPackages" || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					t.Fatalf("repoContractPackages in %s is not a []string literal", path)
				}
				var pkgs []string
				for _, elt := range lit.Elts {
					bl, ok := elt.(*ast.BasicLit)
					if !ok || bl.Kind != token.STRING {
						t.Fatalf("repoContractPackages in %s has a non-string-literal entry", path)
					}
					s, err := strconv.Unquote(bl.Value)
					if err != nil {
						t.Fatal(err)
					}
					pkgs = append(pkgs, s)
				}
				return pkgs
			}
		}
	}
	t.Fatalf("no var repoContractPackages in %s", path)
	return nil
}

func goList(t *testing.T, goDir, pattern string) []string {
	t.Helper()
	out, code := runGo(t, "", "list", "-C", goDir, pattern)
	if code != 0 {
		t.Fatalf("go list %s: exit %d\n%s", pattern, code, out)
	}
	return strings.Fields(out)
}

type testEvent struct {
	Action, Package, Test, Output string
}

func TestC1729_011_ShipRepoContractPackReachesRatchet(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	want := goList(t, goDir, ratchetPkg)
	if len(want) != 1 {
		t.Fatalf("go list %s = %v, want one package", ratchetPkg, want)
	}
	entries := repoContractPackages(t, goDir)
	entry := ""
	for _, e := range entries {
		for _, p := range goList(t, goDir, e) {
			if p == want[0] {
				entry = e
			}
		}
	}
	if entry == "" {
		t.Fatalf("RED: no ship.repoContractPackages entry %v reaches %s — a lane can land a new raw git fixture and red main", entries, want[0])
	}

	m := gitMirror(t, goDir)
	rel := scratchDir + "/pack_init_test.go"
	m.write(t, rel, fill(literalInit, "zzrawgitratchet"), true)
	out, code := runGo(t, m.ceiling, "test", "-C", m.goDir, "-json", "-count=1", entry)
	if code == 0 {
		t.Fatalf("the pack entry %s stayed green with a new staged raw init\n%s", entry, out)
	}
	namedFail, named := false, false
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for sc.Scan() {
		var ev testEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Package != want[0] {
			continue
		}
		namedFail = namedFail || (ev.Action == "fail" && ev.Test != "")
		named = named || strings.Contains(ev.Output, rel)
	}
	if !namedFail {
		t.Errorf("no named-test fail event in %s — the gate would class this red as infra, not a contract violation\n%s", want[0], out)
	}
	if !named {
		t.Errorf("the pack's failure output does not name %s\n%s", rel, out)
	}
}

func TestC1729_012_PackageIsApicoverCleanAndEnrolled(t *testing.T) {
	goDir := moduleDir(t)
	requireRatchetPackage(t, goDir)
	// acs-predicate: config-check — enrolment is a line in the gate's SSOT; the
	enrolled := false
	body, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		enrolled = enrolled || strings.TrimSpace(line) == ratchetPkg
	}
	if !enrolled {
		t.Errorf("go/.apicover-enforce does not list %s", ratchetPkg)
	}

	tmp := t.TempDir()
	profile := filepath.Join(tmp, "cover.out")
	if out, code := runGo(t, "", "test", "-C", goDir, "-count=1", "-coverprofile="+profile, ratchetPkg); code != 0 {
		t.Fatalf("coverage run of %s: exit %d\n%s", ratchetPkg, code, out)
	}
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
	cmd.Dir = goDir
	funcs, err := cmd.Output()
	if err != nil {
		t.Fatalf("go tool cover -func: %v", err)
	}
	funcsPath := filepath.Join(tmp, "cover-func.txt")
	if err := os.WriteFile(funcsPath, funcs, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runGo(t, "", "run", "-C", goDir, "./cmd/apicover", "-enforce", "-require-doc", "-cover", funcsPath, ratchetPkg)
	if code != 0 {
		t.Errorf("apicover -enforce -require-doc %s: exit %d — every export must be named, executed and documented\n%s", ratchetPkg, code, out)
	}
}

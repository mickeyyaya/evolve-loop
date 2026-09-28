//go:build acs

// Package cycle1726 holds the acceptance predicates for the repo-wide
// function-size ratchet (go/internal/sizeratchet): a function past 50 lines
// fails CI unless it is a listed offender, and listed offenders may only shrink.
package cycle1726

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ratchetPkg    = "./internal/sizeratchet"
	offendersFile = "go/internal/sizeratchet/offenders.json"
	namedTestFile = "go/internal/sizeratchet/apicover_named_test.go"
	probeFunc     = "SizeRatchetPlantedProbe"
)

func TestC1726_001_NewFunctionPastFiftyLinesFailsAndExactlyFiftyPasses(t *testing.T) {
	if sizeratchet.MaxLines != 50 {
		t.Fatalf("MaxLines = %d, want 50", sizeratchet.MaxLines)
	}
	if err := sizeratchet.Check(spans("a.AtLimit", 50, "a.Small", 3), map[string]int{}); err != nil {
		t.Errorf("an unlisted 50-line function was rejected: %v", err)
	}
	err := sizeratchet.Check(spans("a.AtLimit", 50, "a.PastLimit", 51), map[string]int{})
	requireNames(t, "unlisted 51-line function", err, []string{"a.PastLimit"}, []string{"a.AtLimit"})
	err = sizeratchet.Check(spans("a.PastLimit", 51), nil)
	requireNames(t, "unlisted 51-line function with a nil offender list", err, []string{"a.PastLimit"}, nil)
}

func TestC1726_002_ListedOffenderGrowingPastItsAllowanceFails(t *testing.T) {
	offenders := map[string]int{"ship.consume": 76, "ship.stage": 93}
	if err := sizeratchet.Check(spans("ship.consume", 76, "ship.stage", 93), offenders); err != nil {
		t.Errorf("offenders exactly at their allowances were rejected: %v", err)
	}
	err := sizeratchet.Check(spans("ship.consume", 77, "ship.stage", 93), offenders)
	requireNames(t, "listed offender grown by one line", err, []string{"ship.consume"}, []string{"ship.stage"})
}

func TestC1726_003_AnAllowanceIsACeilingAndSlackPasses(t *testing.T) {
	for name, tc := range map[string]struct {
		spans     []sizeratchet.FuncSpan
		offenders map[string]int
	}{
		"a shrunk offender keeps its allowance": {spans("a.Shrunk", 59), map[string]int{"a.Shrunk": 60}},
		"an offender back within the limit":     {spans("a.Healed", 50), map[string]int{"a.Healed": 60}},
		"a deleted offender's entry":            {nil, map[string]int{"a.Deleted": 80}},
		"same-key twins within the allowance":   {spans("a.Twin", 60, "a.Twin", 61), map[string]int{"a.Twin": 61}},
		"an empty tree with an empty list":      {nil, nil},
	} {
		if err := sizeratchet.Check(tc.spans, tc.offenders); err != nil {
			t.Errorf("%s is slack a boundary tighten removes, never a failure: %v", name, err)
		}
	}
	requireNames(t, "every violation is reported", sizeratchet.Check(spans("a.Fresh", 51, "a.Grew", 71, "a.Steady", 60),
		map[string]int{"a.Grew": 70, "a.Steady": 60, "a.Deleted": 80}),
		[]string{"a.Fresh", "a.Grew"}, []string{"a.Steady", "a.Deleted"})
	requireNames(t, "a same-key twin cannot grow past the allowance", sizeratchet.Check(spans("a.Twin", 60, "a.Twin", 62), map[string]int{"a.Twin": 61}),
		[]string{"a.Twin"}, nil)
	requireNames(t, "a grown twin fails whatever the walk order", sizeratchet.Check(spans("a.Twin", 62, "a.Twin", 60), map[string]int{"a.Twin": 61}),
		[]string{"a.Twin"}, nil)
}

func TestC1726_004_WalkMeasuresEveryNonTestFunctionUnderTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"pkg/alpha/alpha.go": "package alpha\n\ntype T struct{}\n\ntype U struct{}\n\ntype G[K any] struct{}\n\n" +
			strings.Repeat("// Doc comment lines sit outside the declaration.\n", 10) +
			funcSrc("func AtLimit() {", 50) + "\n" +
			funcSrc("func PastLimit() {", 51) + "\n" +
			funcSrc("func (T) Close() {", 60) + "\n" +
			funcSrc("func (*U) Close() {", 55) + "\n" +
			funcSrc("func (g *G[K]) Close() {", 52) + "\n" +
			funcSrc("func Small() {", 3),
		"pkg/alpha/alpha_test.go":        "package alpha\n\n" + funcSrc("func TestHuge(t *testing.T) {", 80),
		"pkg/alpha/notes.txt":            "not Go\n",
		"pkg/beta/beta_windows.go":       "//go:build windows\n\npackage beta\n\n" + funcSrc("func OnlyWindows() {", 55),
		"pkg/alpha/testdata/broken.go":   "this is not Go source\n",
		"vendor/example.com/v/v.go":      "this is not Go source\n",
		".hidden/hidden.go":              "this is not Go source\n",
		"_scratch/scratch.go":            "this is not Go source\n",
		"pkg/alpha/testdata/nested/x.go": "this is not Go source\n",
	})
	got, err := sizeratchet.Walk(root)
	if err != nil {
		t.Fatalf("Walk(fixture): %v", err)
	}
	want := map[string]int{
		"pkg/alpha.AtLimit": 50, "pkg/alpha.PastLimit": 51, "pkg/alpha.T.Close": 60,
		"pkg/alpha.U.Close": 55, "pkg/alpha.G.Close": 52, "pkg/alpha.Small": 3, "pkg/beta.OnlyWindows": 55,
	}
	if diff := diffCensus(want, spanMap(t, got)); diff != "" {
		t.Errorf("Walk(fixture) key/line mismatch (want -> got):\n%s", diff)
	}

	listed := map[string]int{"pkg/alpha.PastLimit": 51, "pkg/alpha.T.Close": 60, "pkg/alpha.U.Close": 55,
		"pkg/alpha.G.Close": 52, "pkg/beta.OnlyWindows": 55}
	if err := sizeratchet.Check(got, listed); err != nil {
		t.Errorf("fixture with every offender listed at its size was rejected: %v", err)
	}
	delete(listed, "pkg/alpha.U.Close")
	requireNames(t, "U.Close unlisted while T.Close is listed", sizeratchet.Check(got, listed),
		[]string{"pkg/alpha.U.Close"}, []string{"pkg/alpha.T.Close"})

	if _, err := sizeratchet.Walk(filepath.Join(root, "missing")); err == nil {
		t.Error("Walk on a missing root returned no error")
	}
	writeFixture(t, root, map[string]string{"pkg/gamma/broken.go": "this is not Go source\n"})
	if _, err := sizeratchet.Walk(root); err == nil || !strings.Contains(err.Error(), "broken.go") {
		t.Errorf("Walk over an unparsable non-test source = %v, want an error naming broken.go", err)
	}
}

func TestC1726_005_LoadOffendersReadsTheFlatListAndRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	got, err := sizeratchet.LoadOffenders(write("ok.json", `{"pkg/a.F": 51, "pkg/a.T.M": 120}`))
	if err != nil {
		t.Fatalf("LoadOffenders(valid list): %v", err)
	}
	if diff := diffCensus(map[string]int{"pkg/a.F": 51, "pkg/a.T.M": 120}, got); diff != "" {
		t.Errorf("LoadOffenders(valid list) mismatch (want -> got):\n%s", diff)
	}
	bad := map[string]string{
		"missing file":           filepath.Join(dir, "absent.json"),
		"malformed JSON":         write("malformed.json", `{"pkg/a.F": `),
		"non-object JSON":        write("array.json", `[51]`),
		"non-integer allowance":  write("string.json", `{"pkg/a.F": "76"}`),
		"allowance at the limit": write("atlimit.json", `{"pkg/a.F": 50}`),
		"allowance below limit":  write("below.json", `{"pkg/a.F": 3}`),
	}
	for name, path := range bad {
		if _, err := sizeratchet.LoadOffenders(path); err == nil {
			t.Errorf("LoadOffenders(%s) returned no error", name)
		}
	}
}

func TestC1726_006_CheckedInListCoversEveryLiveOffenderAtOrAboveItsSize(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	modRoot := filepath.Join(repo, "go")
	requireTracked(t, repo, offendersFile)
	listed, err := sizeratchet.LoadOffenders(filepath.Join(repo, offendersFile))
	if err != nil {
		t.Fatalf("LoadOffenders(%s): %v", offendersFile, err)
	}
	census := liveOffenderCensus(t, modRoot)
	t.Logf("independent census: %d functions over %d lines; list holds %d entries", len(census), 50, len(listed))
	for key, size := range census {
		if allowance, ok := listed[key]; !ok || allowance < size {
			t.Errorf("%s does not cover live offender %s at %d lines (allowance %d, listed %v)", offendersFile, key, size, allowance, ok)
		}
	}
	for _, key := range []string{
		"internal/phases/ship.consumeCommittedItems", "internal/phases/ship.itemConsumer.stage",
		"internal/phases/ship.stageExplicitPaths", "internal/bridge.codexDriver.Launch", "internal/bridge.claudePDriver.Launch",
	} {
		if _, ok := listed[key]; !ok || census[key] <= 50 {
			t.Errorf("known offender %s must be listed and measured as a live offender (listed %v, census %d lines)", key, ok, census[key])
		}
	}
	walked, err := sizeratchet.Walk(modRoot)
	if err != nil {
		t.Fatalf("Walk(module root): %v", err)
	}
	if diff := diffCensus(census, overLimit(spanMax(walked))); diff != "" {
		t.Errorf("Walk(module root) disagrees with the independent census (census -> walk):\n%s", diff)
	}
	if err := sizeratchet.Check(walked, listed); err != nil {
		t.Errorf("the ratchet is red on the current tree with the checked-in list: %v", err)
	}
}

func TestC1726_007_CIRatchetTestFailsOnARealModuleRegression(t *testing.T) {
	modRoot := filepath.Join(acsassert.RepoRoot(t), "go")
	copyRoot := copyModule(t, modRoot)
	probe := filepath.Join(copyRoot, "internal", "sizeratchetprobe", "probe.go")
	writeProbe := func(lines int) {
		writeFixture(t, copyRoot, map[string]string{
			"internal/sizeratchetprobe/probe.go": "package sizeratchetprobe\n\n" + funcSrc("func "+probeFunc+"() {", lines),
		})
	}

	writeProbe(50)
	if out, code := runGo(t, copyRoot, ciTestArgs()...); code != 0 {
		t.Fatalf("the CI ratchet test is red on a faithful module copy whose only addition is a 50-line function "+
			"(exit %d); it must pass the current tree and accept exactly 50 lines:\n%s", code, tail(out))
	}
	writeProbe(51)
	if out, code := runGo(t, copyRoot, ciTestArgs()...); code == 0 || !strings.Contains(out, probeFunc) {
		t.Errorf("the CI ratchet test did not fail naming %s after a new 51-line function landed in %s "+
			"(exit %d):\n%s", probeFunc, probe, code, tail(out))
	}
	writeProbe(50)
	growConsumeCommittedItems(t, copyRoot)
	out, code := runGo(t, copyRoot, ciTestArgs()...)
	if code == 0 || !strings.Contains(out, "consumeCommittedItems") {
		t.Errorf("the CI ratchet test did not fail naming consumeCommittedItems after it grew one line past "+
			"its allowance (exit %d):\n%s", code, tail(out))
	}
	if strings.Contains(out, probeFunc) {
		t.Errorf("the CI ratchet test blamed the 50-line probe for the consumeCommittedItems growth:\n%s", tail(out))
	}
}

func TestC1726_008_SizeratchetGraduatesIntoTheApicoverEnforcedSet(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	modRoot := filepath.Join(repo, "go")
	// acs-predicate: config-check — the enrolment line is what the CI apicover-enforce step reads.
	enforce, err := os.ReadFile(filepath.Join(modRoot, ".apicover-enforce"))
	if err != nil {
		t.Fatalf("read .apicover-enforce: %v", err)
	}
	enrolled := false
	for _, line := range strings.Split(string(enforce), "\n") {
		enrolled = enrolled || strings.TrimSpace(line) == ratchetPkg
	}
	if !enrolled {
		t.Errorf("go/.apicover-enforce does not list %s", ratchetPkg)
	}
	requireTracked(t, repo, namedTestFile)

	profile := filepath.Join(t.TempDir(), "cover.out")
	if out, code := runGo(t, modRoot, "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, ratchetPkg); code != 0 {
		t.Fatalf("go test %s on the current tree exited %d:\n%s", ratchetPkg, code, tail(out))
	}
	funcs, code := runGo(t, modRoot, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d:\n%s", code, tail(funcs))
	}
	funcTxt := filepath.Join(t.TempDir(), "coverage.func.txt")
	if err := os.WriteFile(funcTxt, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"-enforce", "-cover", funcTxt, filepath.Join(modRoot, "internal", "sizeratchet")}
	if rc := apicover.Main(args, &stdout, &stderr); rc != 0 {
		t.Errorf("apicover -enforce over %s exited %d:\n%s%s", ratchetPkg, rc, tail(stdout.String()), stderr.String())
	}
}

func spans(pairs ...any) []sizeratchet.FuncSpan {
	out := make([]sizeratchet.FuncSpan, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, sizeratchet.FuncSpan{Key: pairs[i].(string), Lines: pairs[i+1].(int)})
	}
	return out
}

func requireNames(t *testing.T, scenario string, err error, want, notWant []string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: Check returned nil, want an error naming %v", scenario, want)
		return
	}
	for _, key := range want {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("%s: error does not name %s: %v", scenario, key, err)
		}
	}
	for _, key := range notWant {
		if strings.Contains(err.Error(), key) {
			t.Errorf("%s: error blames compliant %s: %v", scenario, key, err)
		}
	}
}

// funcSrc renders a declaration spanning exactly total lines, header and
// closing brace included.
func funcSrc(header string, total int) string {
	return header + "\n" + strings.Repeat("\t// body\n", total-2) + "}\n"
}

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func spanMap(t *testing.T, in []sizeratchet.FuncSpan) map[string]int {
	t.Helper()
	out := make(map[string]int, len(in))
	for _, s := range in {
		if _, dup := out[s.Key]; dup {
			t.Errorf("Walk returned key %s twice for distinct functions", s.Key)
		}
		out[s.Key] = s.Lines
	}
	return out
}

func spanMax(in []sizeratchet.FuncSpan) map[string]int {
	out := make(map[string]int, len(in))
	for _, s := range in {
		if s.Lines > out[s.Key] {
			out[s.Key] = s.Lines
		}
	}
	return out
}

func overLimit(in map[string]int) map[string]int {
	out := make(map[string]int)
	for k, v := range in {
		if v > 50 {
			out[k] = v
		}
	}
	return out
}

func diffCensus(want, got map[string]int) string {
	var lines []string
	for k, w := range want {
		if g, ok := got[k]; !ok {
			lines = append(lines, fmt.Sprintf("  missing %s (%d)", k, w))
		} else if g != w {
			lines = append(lines, fmt.Sprintf("  %s: %d -> %d", k, w, g))
		}
	}
	for k, g := range got {
		if _, ok := want[k]; !ok {
			lines = append(lines, fmt.Sprintf("  extra %s (%d)", k, g))
		}
	}
	sort.Strings(lines)
	if len(lines) > 40 {
		lines = append(lines[:40], fmt.Sprintf("  ... and %d more", len(lines)-40))
	}
	return strings.Join(lines, "\n")
}

// liveOffenderCensus is an oracle independent of package sizeratchet: every
// non-test function declaration over 50 lines under modRoot, keyed
// "<slash dir>.<Name>" or "<slash dir>.<Receiver>.<Name>".
func liveOffenderCensus(t *testing.T, modRoot string) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	census := map[string]int{}
	err := filepath.WalkDir(modRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != modRoot && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(modRoot, filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			lines := fset.Position(fn.End()).Line - fset.Position(fn.Pos()).Line + 1
			if key := censusKey(filepath.ToSlash(rel), fn); lines > 50 && lines > census[key] {
				census[key] = lines
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("independent census walk: %v", err)
	}
	return census
}

func censusKey(dir string, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return dir + "." + fn.Name.Name
	}
	expr := fn.Recv.List[0].Type
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return dir + "." + e.Name + "." + fn.Name.Name
		default:
			return fmt.Sprintf("%s.%T.%s", dir, e, fn.Name.Name)
		}
	}
}

func requireTracked(t *testing.T, repo, rel string) {
	t.Helper()
	if !acsassert.FileExists(t, filepath.Join(repo, rel)) {
		t.Errorf("%s missing on disk", rel)
		return
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", repo, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is not git-tracked (it would be dropped at ship)", rel)
	}
}

// ciTestArgs mirrors the CI recipe (make test-integration) for the one package.
func ciTestArgs() []string {
	return []string{"test", "-count=1", "-tags", "integration", ratchetPkg}
}

func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = isolatedGoEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	return "", -1
}

// isolatedGoEnv drops EVOLVE_* so the ratchet cannot be pointed back at the
// real tree, and pins module mode so the copy resolves only its own go.mod.
func isolatedGoEnv() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") || strings.HasPrefix(kv, "GOWORK=") || strings.HasPrefix(kv, "GOFLAGS=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOWORK=off", "GOFLAGS=")
}

// copyModule copies what `go test ./internal/sizeratchet` needs outside the
// repo: every non-test source file (the ratchet's input), go.mod, go.sum,
// vendor/, and the full directories of the ratchet package and its in-module
// dependencies.
func copyModule(t *testing.T, modRoot string) string {
	t.Helper()
	depDirs := ratchetDepDirs(t, modRoot)
	dst := filepath.Join(t.TempDir(), "go")
	sep := string(filepath.Separator)
	err := filepath.WalkDir(modRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(modRoot, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel == "bin" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		keep := rel == "go.mod" || rel == "go.sum" || strings.HasPrefix(rel, "vendor"+sep) ||
			(strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go"))
		for _, dep := range depDirs {
			keep = keep || strings.HasPrefix(path, dep+sep)
		}
		if !keep || !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, filepath.Join(dst, rel))
	})
	if err != nil {
		t.Fatalf("copy module: %v", err)
	}
	return dst
}

func ratchetDepDirs(t *testing.T, modRoot string) []string {
	t.Helper()
	out, code := runGo(t, modRoot, "list", "-deps", "-test", "-tags", "integration",
		"-f", "{{if not .Standard}}{{.Dir}}{{end}}", ratchetPkg)
	if code != 0 {
		t.Fatalf("go list -deps %s exited %d:\n%s", ratchetPkg, code, tail(out))
	}
	var dirs []string
	for _, dir := range strings.Split(out, "\n") {
		if dir = strings.TrimSpace(dir); dir != "" && strings.HasPrefix(dir, modRoot+string(filepath.Separator)) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func copyFile(src, dst string) error {
	body, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, body, 0o644)
}

func growConsumeCommittedItems(t *testing.T, copyRoot string) {
	t.Helper()
	path := filepath.Join(copyRoot, "internal", "phases", "ship", "consume.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "\nfunc consumeCommittedItems("
	start := bytes.Index(src, []byte(anchor))
	if start < 0 {
		t.Fatalf("%q not found in the copied consume.go", anchor)
	}
	eol := bytes.IndexByte(src[start+1:], '\n')
	if eol < 0 {
		t.Fatalf("consumeCommittedItems header has no line end")
	}
	cut := start + 1 + eol + 1
	grown := append(append(append([]byte{}, src[:cut]...), "\t// one planted line of growth\n"...), src[cut:]...)
	if err := os.WriteFile(path, grown, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tail(out string) string {
	const max = 4000
	if len(out) > max {
		return "..." + out[len(out)-max:]
	}
	return out
}

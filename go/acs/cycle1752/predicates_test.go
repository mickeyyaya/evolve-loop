//go:build acs

// Package cycle1752 pins the shrinks of fleet.PartitionGraph and committedset.DispositionsFrom to the size ratchet.
package cycle1752

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseCommit      = "aedac036be451da3dd8e017ce1aadd3b657eab96"
	offendersRel    = "go/internal/sizeratchet/offenders.json"
	fleetDir        = "internal/fleet"
	committedsetDir = "internal/committedset"
	mutantWorkers   = 4
)

var packageDirs = []string{fleetDir, committedsetDir}

type target struct {
	dir, file, fn string
	allowance     int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var (
	partitionGraphTarget   = target{dir: fleetDir, file: "packagegraph.go", fn: "PartitionGraph", allowance: 53}
	dispositionsFromTarget = target{dir: committedsetDir, file: "committedset.go", fn: "DispositionsFrom", allowance: 57}
	targets                = []target{partitionGraphTarget, dispositionsFromTarget}
)

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

func TestC1752_001_TwoTargetFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key()]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key(), tg.dir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key()+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1752_002_ShrinkExtractsCodeRatherThanStrippingBlankLines(t *testing.T) {
	for _, tg := range targets {
		base := nonBlankLines(baselineFuncSource(t, tg))
		site := currentFuncSite(t, tg)
		cur := nonBlankLines(string(site.src[site.start:site.end]))
		if cur >= base {
			t.Errorf("RED: %s still has %d non-blank lines (baseline %d) — deleting blank lines is not the extraction; move a block of its logic into a named helper", tg.key(), cur, base)
		}
	}
}

func TestC1752_003_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling and a lane never edits the file; a shrunk function's entry is slack the boundary tighten removes:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key()]; !listed || got != tg.allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key(), got, listed, tg.allowance)
		}
	}
}

func TestC1752_004_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1752_005_BaselineTestFunctionsPreserved(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	for _, dir := range packageDirs {
		baseline := map[string][]string{}
		names, err := git.run("ls-tree", "--name-only", baseCommit, "go/"+dir+"/")
		if err != nil {
			t.Fatalf("list baseline %s: %v", dir, err)
		}
		for _, rel := range strings.Fields(string(names)) {
			if !strings.HasSuffix(rel, "_test.go") {
				continue
			}
			src, err := git.Show(baseCommit, rel)
			if err != nil {
				t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
			}
			collectFuncDecls(t, rel, src, baseline)
		}
		current := map[string][]string{}
		for _, path := range packageGoFiles(t, filepath.Join(goModuleDir(t), dir), true) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			collectFuncDecls(t, path, src, current)
		}
		var lost []string
		for name, bodies := range baseline {
			if missing := subtractStrings(bodies, current[name]); len(missing) > 0 {
				lost = append(lost, name)
			}
		}
		sort.Strings(lost)
		if len(lost) > 0 {
			t.Errorf("RED: %d baseline test function(s) under %s were deleted or edited since %s — add characterization tests beside them, never change one:\n  %s", len(lost), dir, baseCommit, strings.Join(lost, "\n  "))
		}
	}
}

func TestC1752_006_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, tail(r.out))
		}
	}
}

func TestC1752_007_TargetPackagesVetAndGofmtClean(t *testing.T) {
	goDir := goModuleDir(t)
	for _, dir := range packageDirs {
		if r := execGo(goDir, "vet", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
		for _, path := range packageGoFiles(t, filepath.Join(goDir, dir), true) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Errorf("RED: %s does not parse for gofmt: %v", path, err)
				continue
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("RED: %s is not gofmt-formatted", path)
			}
		}
	}
}

func TestC1752_008_NoCommentLinesAdded(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	args := []string{"comments", "-base", baseCommit}
	var scoped []string
	for _, dir := range packageDirs {
		for _, rel := range changed {
			if strings.HasPrefix(rel, "go/"+dir+"/") && strings.HasSuffix(rel, ".go") {
				scoped = append(scoped, dir)
				args = append(args, filepath.Join(goModuleDir(t), dir))
				break
			}
		}
	}
	if len(scoped) == 0 {
		return
	}
	var out, errOut strings.Builder
	if code := commentaudit.Main(args, &out, &errOut, git); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent):\n%s%s", code, strings.Join(scoped, ", "), out.String(), errOut.String())
	}
}

func TestC1752_009_ShrunkSourcesChangedAndKeepEveryComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	for _, dir := range packageDirs {
		touched := 0
		for _, rel := range changed {
			if filepath.Dir(rel) == "go/"+dir && strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
				touched++
			}
		}
		if touched == 0 {
			t.Errorf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", dir, baseCommit)
			continue
		}
		var baseline []string
		names, err := git.run("ls-tree", "--name-only", baseCommit, "go/"+dir+"/")
		if err != nil {
			t.Fatalf("list baseline %s: %v", dir, err)
		}
		for _, rel := range strings.Fields(string(names)) {
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				continue
			}
			src, err := git.Show(baseCommit, rel)
			if err != nil {
				t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
			}
			baseline = append(baseline, commentTexts(t, rel, src)...)
		}
		var current []string
		for _, path := range packageGoFiles(t, filepath.Join(root, "go", dir), false) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			current = append(current, commentTexts(t, path, src)...)
		}
		if lost := subtractStrings(baseline, current); len(lost) > 0 {
			t.Errorf("RED: %d baseline comment(s) under %s are gone since %s — move each with its code, never strip one to save a line:\n  %s", len(lost), dir, baseCommit, strings.Join(lost, "\n  "))
		}
		if added := subtractStrings(current, baseline); len(added) > 0 {
			t.Errorf("RED: %d comment(s) added to the non-test sources under %s since %s, trailing ones included — names carry the intent:\n  %s", len(added), dir, baseCommit, strings.Join(added, "\n  "))
		}
	}
}

type mutant struct{ name, anchor, replacement string }

var partitionGraphMutants = []mutant{
	{"a bucket count below one is not clamped to one", `if n < 1 {`, `if n < 0 {`},
	{"a package-set failure loses its cause", `"PartitionGraph: todo %s: %w", td.ID, perr)`, `"PartitionGraph: todo %s: %v", td.ID, perr)`},
	{"a package-set failure returns the partial buckets", `return nil, nil, fmt.Errorf(`, `return buckets, deferred, fmt.Errorf(`},
	{"a todo after a global-zone todo in bucket zero is not pulled into that bucket", `} else if gzBucket >= 0 {`, `} else if gzBucket > 0 {`},
	{"a global-zone todo never claims its bucket", `gzBucket = chosen`, `_ = chosen`},
	{"a todo conflicting with two buckets lands in bucket zero instead of deferring", `deferred = append(deferred, td)`, `buckets[0] = append(buckets[0], td)`},
	{"package ownership is credited to bucket zero", `owner[pkg] = chosen`, `owner[pkg] = 0`},
}

var dispositionsFromMutants = []mutant{
	{"a decision with a mistyped field still answers", `if json.Unmarshal(body, &decision) != nil {`, `if json.Unmarshal(body, &decision) != nil && len(body) == 0 {`},
	{"an id is not trimmed", `if id = strings.TrimSpace(id); id == "" || seen[id] {`, `if id == "" || seen[id] {`},
	{"an answer's evidence is not trimmed", `Reason: strings.TrimSpace(evidence)})`, `Reason: evidence})`},
	{"a blank escalation reason counts as stated", `evidence := strings.TrimSpace(e.Reason)`, `evidence := e.Reason`},
	{"a stated escalation reason loses to its fail_count", `case evidence != "":`, `case evidence != "" && e.FailCount == 0:`},
	{"a negative fail_count is reported as the evidence", `case e.FailCount > 0:`, `case e.FailCount != 0:`},
	{"a blank or padded sha is not trimmed", `if sha := strings.TrimSpace(e.GitSHA); sha != "" {`, `if sha := e.GitSHA; sha != "" {`},
}

func TestC1752_010_PartitionGraphCharacterizationKillsBaselineMutants(t *testing.T) {
	assertBaselineMutantsKilled(t, partitionGraphTarget, partitionGraphMutants)
}

func TestC1752_011_DispositionsFromCharacterizationKillsBaselineMutants(t *testing.T) {
	assertBaselineMutantsKilled(t, dispositionsFromTarget, dispositionsFromMutants)
}

const dispositionsEquivalenceTest = "TestACS1752DispositionsFromMatchesBaseline"

const dispositionsEquivalenceHarness = `package committedset

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestACS1752DispositionsFromMatchesBaseline(t *testing.T) {
	type single struct {
		bucket string
		entry  map[string]any
	}
	var singles []single
	for _, id := range []string{"a", " a ", "b", "", "  "} {
		singles = append(singles, single{"skip_rejected", map[string]any{"task_id": id}})
		for _, text := range []string{"", "  ", "r", " r "} {
			singles = append(singles, single{"skip_shipped", map[string]any{"task_id": id, "git_sha": text}})
			singles = append(singles, single{"dropped", map[string]any{"id": id, "reason": text}})
			for _, count := range []int{-1, 0, 2} {
				singles = append(singles, single{"escalate_block", map[string]any{"task_id": id, "reason": text, "fail_count": count}})
			}
		}
	}
	bodies := []string{
		"", "null", "{}", "[]", "{not json", "{\"dropped\":\"x\"}",
		"{\"dropped\":[{\"id\":\"x\",\"reason\":\"r\"}],\"escalate_block\":\"oops\"}",
		"{\"skip_shipped\":[{\"task_id\":\"s\",\"git_sha\":7}]}",
		"{\"top_n\":[{\"id\":\"c\"}],\"deferred\":[{\"id\":\"d\"}],\"dropped\":[{\"id\":\"d\",\"reason\":\"r\"}]}",
	}
	for _, x := range singles {
		for _, y := range singles {
			decision := map[string][]map[string]any{}
			decision[x.bucket] = append(decision[x.bucket], x.entry)
			decision[y.bucket] = append(decision[y.bucket], y.entry)
			body, err := json.Marshal(decision)
			if err != nil {
				t.Fatal(err)
			}
			bodies = append(bodies, string(body))
		}
	}
	for _, body := range bodies {
		got, want := DispositionsFrom([]byte(body)), acs1752BaselineDispositionsFrom([]byte(body))
		if (len(got) != 0 || len(want) != 0) && !reflect.DeepEqual(got, want) {
			t.Errorf("DispositionsFrom(%s) = %+v, baseline %+v", body, got, want)
		}
	}
}

`

const partitionEquivalenceTest = "TestACS1752PartitionGraphMatchesBaseline"

const partitionEquivalenceHarness = `package fleet

import (
	"fmt"
	"strings"
	"testing"
)

func TestACS1752PartitionGraphMatchesBaseline(t *testing.T) {
	const (
		acs    = "internal/acsrunner/runner.go"
		flt    = "internal/fleet/partition.go"
		ipc    = "internal/ipcenv/ipcenv.go"
		gomod  = "go.mod"
		policy = ".evolve/policy.json"
		absent = "internal/does/not/exist.go"
	)
	scenarios := []struct {
		n     int
		files [][]string
	}{
		{2, nil},
		{0, [][]string{{acs}}},
		{-1, [][]string{{acs}, {flt}, {ipc}}},
		{1, [][]string{{acs}, {flt}}},
		{2, [][]string{{acs}, {flt}}},
		{2, [][]string{{flt}, {ipc}}},
		{2, [][]string{{gomod}, {acs}}},
		{2, [][]string{{acs}, {gomod}}},
		{2, [][]string{{acs}, {flt}, {gomod}}},
		{2, [][]string{{acs}, {flt}, {ipc}}},
		{2, [][]string{{gomod}, {policy}, {acs}}},
		{2, [][]string{{acs, gomod}, {flt}}},
		{2, [][]string{{acs}, {flt}, {acs, ipc}}},
		{2, [][]string{{}, {acs}, {}}},
		{3, [][]string{{acs}, {flt}, {ipc}, {gomod}, {policy}}},
		{3, [][]string{{gomod}, {acs}, {flt}}},
		{2, [][]string{{acs}, {absent}}},
		{2, [][]string{{absent}, {acs}}},
	}
	for i, sc := range scenarios {
		todos := make([]Todo, len(sc.files))
		for j, files := range sc.files {
			todos[j] = Todo{ID: fmt.Sprintf("t%d", j), Files: files}
		}
		got := acs1752Render(PartitionGraph(todos, sc.n, "../.."))
		want := acs1752Render(acs1752BaselinePartitionGraph(todos, sc.n, "../.."))
		if got != want {
			t.Errorf("scenario %d (n=%d, files=%q): PartitionGraph gives %s, baseline %s", i, sc.n, sc.files, got, want)
		}
	}
}

func acs1752Render(buckets [][]Todo, deferred []Todo, err error) string {
	var b strings.Builder
	if buckets == nil {
		b.WriteString("buckets=nil")
	} else {
		fmt.Fprintf(&b, "buckets=%d", len(buckets))
		for i, bucket := range buckets {
			fmt.Fprintf(&b, " [%d:%s]", i, acs1752IDs(bucket))
		}
	}
	fmt.Fprintf(&b, " deferred=[%s]", acs1752IDs(deferred))
	if err != nil {
		fmt.Fprintf(&b, " err=%q", err.Error())
	}
	return b.String()
}

func acs1752IDs(todos []Todo) string {
	ids := make([]string, len(todos))
	for i, td := range todos {
		ids[i] = td.ID
	}
	return strings.Join(ids, ",")
}

`

func TestC1752_012_DispositionsFromMatchesBaselineOverAGeneratedCorpus(t *testing.T) {
	assertMatchesBaseline(t, dispositionsFromTarget, dispositionsEquivalenceHarness, dispositionsEquivalenceTest, "func acs1752BaselineDispositionsFrom(")
}

func TestC1752_013_PartitionGraphMatchesBaselineOverAScenarioTable(t *testing.T) {
	assertMatchesBaseline(t, partitionGraphTarget, partitionEquivalenceHarness, partitionEquivalenceTest, "func acs1752BaselinePartitionGraph(")
}

func TestC1752_014_NoProtectedSurfaceTouched(t *testing.T) {
	changed, err := worktreeGit{root: acsassert.RepoRoot(t)}.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var touched []string
	for _, rel := range changed {
		if guards.IsProtectedSurface(rel) {
			touched = append(touched, rel)
		}
	}
	if len(touched) > 0 {
		t.Errorf("RED: the lane touched protected surfaces since %s:\n  %s", baseCommit, strings.Join(touched, "\n  "))
	}
}

const explanationRunID = "01M3NKCSXD128FNS9F5XS8MX93"

var (
	sentenceEnd = regexp.MustCompile(`[.;!?](?:\s+|$)|\n`)
	testNoun    = regexp.MustCompile(`(?i)\b(?:tests?|testing|characteri[sz]\w*)\b`)
	shrinkNoun  = regexp.MustCompile(`(?i)\b(?:shr[iu]nk\w*|extract\w*|refactor\w*|hoist\w*)\b`)
	orderWord   = regexp.MustCompile(`(?i)\b(?:before|beforehand|prior to|ahead of|preceding|first|then|after(?:wards?)?|subsequently|earlier|later)\b`)
)

func explanationDocument(t *testing.T) (string, string) {
	t.Helper()
	rel, err := explanationdocs.DocumentPath(1752, explanationRunID)
	if err != nil {
		t.Fatalf("explanationdocs.DocumentPath: %v", err)
	}
	root := acsassert.RepoRoot(t)
	changed, err := worktreeGit{root: root}.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	if !slices.Contains(changed, rel) {
		t.Fatalf("RED: %s is not in the build diff since %s — correct the cycle's explanation document in place, never drop it", rel, baseCommit)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("RED: read %s: %v", rel, err)
	}
	return rel, string(body)
}

func predicateRef(number string) *regexp.Regexp {
	return regexp.MustCompile(`(?:^|\D)` + number + `(?:\D|$)`)
}

func TestC1752_015_ExplanationDocumentClaimsNoTestFirstOrder(t *testing.T) {
	rel, body := explanationDocument(t)
	var claims []string
	for _, sentence := range sentenceEnd.Split(body, -1) {
		if testNoun.MatchString(sentence) && shrinkNoun.MatchString(sentence) && orderWord.MatchString(sentence) {
			claims = append(claims, strings.TrimSpace(sentence))
		}
	}
	if len(claims) > 0 {
		t.Errorf("RED: %s orders the characterization tests against the shrink — the build transcript shows the extraction first and the tree cannot prove any order; say what the tests kill and cite predicates 010/011, with no before/after/first/then relating the tests to the shrink in one sentence:\n  %s", rel, strings.Join(claims, "\n  "))
	}
}

func TestC1752_016_ExplanationSummaryGroundsTheMutantKillClaimInPredicates010And011(t *testing.T) {
	rel, body := explanationDocument(t)
	summary, found, err := reportdoc.Section(body, "Summary")
	if err != nil || !found {
		t.Fatalf("RED: %s has no single ## Summary section (found=%v, err=%v)", rel, found, err)
	}
	var missing []string
	for _, want := range []struct {
		what string
		re   *regexp.Regexp
	}{
		{"the characterization tests", testNoun},
		{"the mutants they kill", regexp.MustCompile(`(?i)\bmutants?\b`)},
		{"the baseline those mutants come from", regexp.MustCompile(`(?i)\bbaseline\b`)},
		{"predicate 010, the PartitionGraph mutant-kill proof", predicateRef("010")},
		{"predicate 011, the DispositionsFrom mutant-kill proof", predicateRef("011")},
	} {
		if !want.re.MatchString(summary) {
			missing = append(missing, want.what)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RED: the ## Summary of %s must keep the true claim that the characterization tests kill the baseline mutants and ground it in the predicates that prove it; missing:\n  %s", rel, strings.Join(missing, "\n  "))
	}
}

func assertMatchesBaseline(t *testing.T, tg target, harness, testName, renamedHeader string) {
	t.Helper()
	baseline := baselineFuncSource(t, tg)
	renamed := strings.Replace(baseline, "func "+tg.fn+"(", renamedHeader, 1)
	if renamed == baseline {
		t.Fatalf("baseline %s has no func %s( header to rename", tg.file, tg.fn)
	}
	tmp := t.TempDir()
	name := "zz_acs1752_" + strings.ToLower(tg.fn) + "_equivalence_test.go"
	harnessPath := filepath.Join(tmp, name)
	if err := os.WriteFile(harnessPath, []byte(harness+renamed+"\n"), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	pkgDir := filepath.Join(goModuleDir(t), tg.dir)
	overlay := writeOverlay(t, tmp, map[string]string{filepath.Join(pkgDir, name): harnessPath})
	r := execGo(goModuleDir(t), "test", "-count=1", "-json", "-run", "^"+testName+"$", "-overlay", overlay, "./"+tg.dir)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.code != 0 || !testPassed(r.out, testName) {
		t.Errorf("RED: %s no longer matches its baseline (exit=%d) — the extraction changed behavior:\n%s", tg.key(), r.code, tail(r.out))
	}
}

func assertBaselineMutantsKilled(t *testing.T, tg target, mutants []mutant) {
	t.Helper()
	baseline := baselineFuncSource(t, tg)
	site := currentFuncSite(t, tg)
	bodies := []string{baseline}
	for _, m := range mutants {
		if n := strings.Count(baseline, m.anchor); n != 1 {
			t.Fatalf("mutation anchor %q (%s) occurs %d times in the baseline %s, want exactly 1", m.anchor, m.name, n, tg.key())
		}
		bodies = append(bodies, strings.Replace(baseline, m.anchor, m.replacement, 1))
	}
	outcomes := make([]goRun, len(bodies))
	sem := make(chan struct{}, mutantWorkers)
	var wg sync.WaitGroup
	for i, body := range bodies {
		overlay := site.overlay(t, body)
		wg.Add(1)
		go func(i int, overlay string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			outcomes[i] = execGo(goModuleDir(t), "test", "-count=1", "-failfast", "-json", "-overlay", overlay, "./"+tg.dir)
		}(i, overlay)
	}
	wg.Wait()
	control := outcomes[0]
	if control.err != nil {
		t.Fatal(control.err)
	}
	if control.code != 0 {
		t.Fatalf("RED: the %s tests fail with the baseline %s substituted back (exit=%d) — they pin behavior the extraction changed, or the baseline no longer compiles beside the extracted helpers:\n%s", tg.dir, tg.fn, control.code, tail(control.out))
	}
	var survivors []string
	for i, m := range mutants {
		o := outcomes[i+1]
		switch {
		case o.err != nil:
			t.Fatalf("mutant %s: %v", m.name, o.err)
		case o.code == 0:
			survivors = append(survivors, m.name)
		case !testFailed(o.out):
			survivors = append(survivors, fmt.Sprintf("%s (the mutated baseline does not compile here, exit=%d):\n%s", m.name, o.code, tail(o.out)))
		}
	}
	if len(survivors) > 0 {
		t.Errorf("RED: %d/%d behavior mutants of the baseline %s survive the %s tests — add characterization tests that pin them:\n  %s", len(survivors), len(mutants), tg.fn, tg.dir, strings.Join(survivors, "\n  "))
	}
}

func nonBlankLines(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func packageGoFiles(t *testing.T, dir string, withTests bool) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || (!withTests && strings.HasSuffix(name, "_test.go")) {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

func collectFuncDecls(t *testing.T, name string, src []byte, into map[string][]string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		key := file.Name.Name + "." + d.Name.Name
		if d.Recv != nil && len(d.Recv.List) > 0 {
			key = file.Name.Name + "." + string(src[fset.Position(d.Recv.Pos()).Offset:fset.Position(d.Recv.End()).Offset]) + "." + d.Name.Name
		}
		into[key] = append(into[key], string(src[fset.Position(d.Pos()).Offset:fset.Position(d.End()).Offset]))
	}
}

func commentTexts(t *testing.T, name string, src []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var texts []string
	for _, group := range file.Comments {
		for _, c := range group.List {
			texts = append(texts, c.Text)
		}
	}
	return texts
}

func subtractStrings(lines, held []string) []string {
	count := map[string]int{}
	for _, line := range held {
		count[line]++
	}
	var rest []string
	for _, line := range lines {
		if count[line] > 0 {
			count[line]--
			continue
		}
		rest = append(rest, line)
	}
	return rest
}

func baselineFile(t *testing.T, tg target) []byte {
	t.Helper()
	root := acsassert.RepoRoot(t)
	src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+tg.dir+"/"+tg.file)
	if code != 0 {
		t.Fatalf("git show %s:go/%s/%s failed: %v\n%s", baseCommit, tg.dir, tg.file, err, stderr)
	}
	return []byte(src)
}

func baselineFuncSource(t *testing.T, tg target) string {
	t.Helper()
	src := baselineFile(t, tg)
	start, end, found := funcSpan(t, tg.file, src, tg.fn)
	if !found {
		t.Fatalf("baseline go/%s/%s has no func %s", tg.dir, tg.file, tg.fn)
	}
	return string(src[start:end])
}

type importSpec struct{ name, path string }

func fileImports(t *testing.T, name string, src []byte) []importSpec {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", name, err)
	}
	var specs []importSpec
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("import path %s in %s: %v", spec.Path.Value, name, err)
		}
		n := p[strings.LastIndex(p, "/")+1:]
		if spec.Name != nil {
			n = spec.Name.Name
		}
		specs = append(specs, importSpec{name: n, path: p})
	}
	return specs
}

func reconcileImports(t *testing.T, name string, src []byte, baseline []importSpec) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse substituted %s: %v", name, err)
	}
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	var keep []importSpec
	seen := map[string]bool{}
	for _, spec := range append(fileImports(t, name, src), baseline...) {
		if seen[spec.path] || !(used[spec.name] || spec.name == "_" || spec.name == ".") {
			continue
		}
		seen[spec.path] = true
		keep = append(keep, spec)
	}
	var block strings.Builder
	block.WriteString("import (\n")
	for _, spec := range keep {
		fmt.Fprintf(&block, "\t%s %q\n", spec.name, spec.path)
	}
	block.WriteString(")")
	start, end := -1, -1
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			if start < 0 {
				start = fset.Position(gen.Pos()).Offset
			}
			end = fset.Position(gen.End()).Offset
		}
	}
	if start < 0 {
		start = fset.Position(file.Name.End()).Offset
		return append(append(append([]byte{}, src[:start]...), "\n\n"+block.String()+"\n"...), src[start:]...)
	}
	return append(append(append([]byte{}, src[:start]...), block.String()...), src[end:]...)
}

type funcSite struct {
	path       string
	src        []byte
	start, end int
	baseline   []importSpec
}

func currentFuncSite(t *testing.T, tg target) funcSite {
	t.Helper()
	for _, path := range packageGoFiles(t, filepath.Join(goModuleDir(t), tg.dir), false) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if start, end, found := funcSpan(t, filepath.Base(path), src, tg.fn); found {
			return funcSite{path: path, src: src, start: start, end: end, baseline: fileImports(t, tg.file, baselineFile(t, tg))}
		}
	}
	t.Fatalf("RED: no non-test file under %s declares func %s — keep the function (and its signature) when extracting", tg.dir, tg.fn)
	return funcSite{}
}

func (s funcSite) overlay(t *testing.T, body string) string {
	t.Helper()
	tmp := t.TempDir()
	substituted := filepath.Join(tmp, filepath.Base(s.path))
	var buf bytes.Buffer
	buf.Write(s.src[:s.start])
	buf.WriteString(body)
	buf.Write(s.src[s.end:])
	if err := os.WriteFile(substituted, reconcileImports(t, s.path, buf.Bytes(), s.baseline), 0o644); err != nil {
		t.Fatalf("write substituted source: %v", err)
	}
	return writeOverlay(t, tmp, map[string]string{s.path: substituted})
}

func funcSpan(t *testing.T, name string, src []byte, fn string) (start, end int, found bool) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == fn && d.Body != nil {
			return fset.Position(d.Pos()).Offset, fset.Position(d.End()).Offset, true
		}
	}
	return 0, 0, false
}

func writeOverlay(t *testing.T, dir string, replace map[string]string) string {
	t.Helper()
	body, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return path
}

type testEvent struct {
	Action string
	Test   string
}

func testEvents(out string) []testEvent {
	var events []testEvent
	for _, line := range strings.Split(out, "\n") {
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) == nil && ev.Action != "" {
			events = append(events, ev)
		}
	}
	return events
}

func testFailed(out string) bool {
	for _, ev := range testEvents(out) {
		if ev.Action == "fail" && ev.Test != "" {
			return true
		}
	}
	return false
}

func testPassed(out, name string) bool {
	for _, ev := range testEvents(out) {
		if ev.Action == "pass" && ev.Test == name {
			return true
		}
	}
	return false
}

func tail(out string) string {
	const keep = 4000
	if len(out) <= keep {
		return out
	}
	return "…" + out[len(out)-keep:]
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

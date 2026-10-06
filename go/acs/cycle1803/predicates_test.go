//go:build acs

package cycle1803

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const pruneLifecycleState = `{"failedApproaches":[` +
	`{"cycle":21,"classification":"code-build-fail","summary":"live","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":22,"classification":"code-audit-fail","summary":"expired","recordedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-02-01T00:00:00Z"},` +
	`{"cycle":23,"classification":"infrastructure-transient","summary":"legacy expired","recordedAt":"2020-01-01T00:00:00Z"},` +
	`{"cycle":24,"classification":"code-audit-fail","summary":"untimed legacy"}],` +
	`"carryoverTodos":[` +
	`{"id":"todo-c1803-live","cycles_unpicked":3,"expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"id":"todo-c1803-expired","cycles_unpicked":4,"expiresAt":"2020-02-01T00:00:00Z"},` +
	`{"id":"todo-c1803-untimed","cycles_unpicked":0}]}`

const resetLifecycleState = `{"failedApproaches":[` +
	`{"cycle":31,"classification":"infrastructure-systemic","summary":"infra","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":32,"classification":"code-build-fail","summary":"build","recordedAt":"2026-10-02T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":33,"classification":"ship-gate-config","summary":"gate","recordedAt":"2026-10-03T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"}]}`

const unparseableState = "{not json"

const absentSalvageHead = "0123456789abcdef0123456789abcdef01234567"

var (
	reportedCycleRe = regexp.MustCompile(`\bcycle=(\d+)`)
	reportedIDRe    = regexp.MustCompile(`\bid=(\S+)`)
)

func reportedCycles(t *testing.T, stdout string) map[int]bool {
	t.Helper()
	set := map[int]bool{}
	for _, m := range reportedCycleRe.FindAllStringSubmatch(stdout, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		set[n] = true
	}
	return set
}

func reportedIDs(stdout string) map[string]bool {
	set := map[string]bool{}
	for _, m := range reportedIDRe.FindAllStringSubmatch(stdout, -1) {
		set[m[1]] = true
	}
	return set
}

func TestC1803_001_FailuresResetAcksTheFingerprintWithoutLaunchingALoop(t *testing.T) {
	const fingerprint = "fp-c1803-reset"
	root, evolveDir := projectWithState(t, resetLifecycleState)

	run := runEvolve(t, "failures", "reset", "--fingerprint", fingerprint, "--project-root", root)

	if run.code != 0 {
		t.Fatalf("failures reset rc=%d want 0; stdout=%q stderr=%q", run.code, run.stdout, run.stderr)
	}
	acked, err := core.LoadResolvedFingerprints(evolveDir)
	if err != nil || !acked[fingerprint] {
		t.Errorf("resolved-fingerprints.json does not acknowledge %q (acked=%v err=%v)", fingerprint, acked, err)
	}
	var records []core.ResolvedFingerprint
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(evolveDir, "resolved-fingerprints.json"))), &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ResolvedBy != "operator-reset" {
		t.Errorf("ledger records %+v, want exactly one operator-reset ack", records)
	}
	if got := failedCycleSet(readStateArrays(t, evolveDir)); !reflect.DeepEqual(got, map[int]bool{32: true}) {
		t.Errorf("failedApproaches after reset = %v, want only cycle 32 (the loop --reset prune of infrastructure + ship-gate-config)", got)
	}
	entries, err := os.ReadDir(evolveDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name := e.Name(); name != "state.json" && name != "resolved-fingerprints.json" {
			t.Errorf("failures reset left .evolve/%s behind: the verb must not launch or prepare a loop", name)
		}
	}

	refusedRoot, refusedEvolveDir := projectWithState(t, resetLifecycleState)
	before := currentStamp(t, filepath.Join(refusedEvolveDir, "state.json"))
	refused := runEvolveIn(t, refusedRoot, "failures", "reset", "--fingerprint", fingerprint)
	if refused.code != 1 {
		t.Errorf("failures reset without --project-root rc=%d want 1 (mutating refusal); stderr=%q", refused.code, refused.stderr)
	}
	if after := currentStamp(t, filepath.Join(refusedEvolveDir, "state.json")); after.body != before.body {
		t.Errorf("a refused reset rewrote state.json: %q", after.body)
	}
	if _, err := os.Stat(filepath.Join(refusedEvolveDir, "resolved-fingerprints.json")); !os.IsNotExist(err) {
		t.Errorf("a refused reset still wrote resolved-fingerprints.json (stat err=%v)", err)
	}
}

func TestC1803_002_FailuresPruneDryRunReportsTheExpiredEntriesAndChangesNothing(t *testing.T) {
	root, evolveDir := projectWithState(t, pruneLifecycleState)
	statePath := filepath.Join(evolveDir, "state.json")
	before := stampAgedFile(t, statePath)

	for _, invocation := range []struct {
		name string
		dir  string
		args []string
	}{
		{"explicit root", t.TempDir(), []string{"failures", "prune", "--dry-run", "--project-root", root}},
		{"cwd root", root, []string{"failures", "prune", "--dry-run"}},
	} {
		t.Run(invocation.name, func(t *testing.T) {
			run := runEvolveIn(t, invocation.dir, invocation.args...)

			if run.code != 0 {
				t.Fatalf("failures prune --dry-run rc=%d want 0; stdout=%q stderr=%q", run.code, run.stdout, run.stderr)
			}
			if got := reportedCycles(t, run.stdout); !reflect.DeepEqual(got, map[int]bool{22: true, 23: true}) {
				t.Errorf("dry run reported failedApproaches cycles %v, want exactly the expired {22, 23}; stdout=%q", got, run.stdout)
			}
			if got := reportedIDs(run.stdout); !reflect.DeepEqual(got, map[string]bool{"todo-c1803-expired": true}) {
				t.Errorf("dry run reported carryoverTodos ids %v, want exactly the expired todo-c1803-expired; stdout=%q", got, run.stdout)
			}
			if after := currentStamp(t, statePath); after != before {
				t.Errorf("failures prune --dry-run changed state.json (mtime %v→%v, body changed=%v)", before.modTime, after.modTime, after.body != before.body)
			}
		})
	}
}

func TestC1803_003_FailuresPruneRemovesExactlyTheDryRunSetAndLeavesCyclesUnpicked(t *testing.T) {
	root, evolveDir := projectWithState(t, pruneLifecycleState)
	seeded := readStateArrays(t, evolveDir)
	seededTodos := carryoverByID(seeded)

	preview := runEvolve(t, "failures", "prune", "--dry-run", "--project-root", root)
	if preview.code != 0 {
		t.Fatalf("failures prune --dry-run rc=%d want 0; stderr=%q", preview.code, preview.stderr)
	}
	run := runEvolve(t, "failures", "prune", "--project-root", root)

	if run.code != 0 {
		t.Fatalf("failures prune rc=%d want 0; stdout=%q stderr=%q", run.code, run.stdout, run.stderr)
	}
	pruned := readStateArrays(t, evolveDir)
	removedCycles := map[int]bool{}
	for c := range failedCycleSet(seeded) {
		if !failedCycleSet(pruned)[c] {
			removedCycles[c] = true
		}
	}
	keptTodos := carryoverByID(pruned)
	removedIDs := map[string]bool{}
	for id := range seededTodos {
		if _, ok := keptTodos[id]; !ok {
			removedIDs[id] = true
		}
	}
	if want := map[int]bool{22: true, 23: true}; !reflect.DeepEqual(removedCycles, want) {
		t.Errorf("failures prune removed failedApproaches cycles %v, want %v", removedCycles, want)
	}
	if want := map[string]bool{"todo-c1803-expired": true}; !reflect.DeepEqual(removedIDs, want) {
		t.Errorf("failures prune removed carryoverTodos %v, want %v", removedIDs, want)
	}
	if got := reportedCycles(t, preview.stdout); !reflect.DeepEqual(got, removedCycles) {
		t.Errorf("dry run reported cycles %v but the prune removed %v", got, removedCycles)
	}
	if got := reportedIDs(preview.stdout); !reflect.DeepEqual(got, removedIDs) {
		t.Errorf("dry run reported carryoverTodos %v but the prune removed %v", got, removedIDs)
	}
	for id, todo := range keptTodos {
		if !reflect.DeepEqual(todo, seededTodos[id]) {
			t.Errorf("failures prune changed surviving carryoverTodos %q from %v to %v: cycles_unpicked and expiresAt stay per-batch bookkeeping", id, seededTodos[id], todo)
		}
	}
}

func TestC1803_004_FailuresPruneDryRunOnEmptyStateExitsZeroAndUsageErrorsExitTen(t *testing.T) {
	emptyDir := t.TempDir()
	run := runEvolveIn(t, emptyDir, "failures", "prune", "--dry-run")
	if run.code != 0 {
		t.Errorf("failures prune --dry-run with no state rc=%d want 0; stderr=%q", run.code, run.stderr)
	}
	if entries, err := os.ReadDir(emptyDir); err != nil || len(entries) != 0 {
		t.Errorf("a dry run with no state created %v (err=%v)", entries, err)
	}

	if bogus := runEvolve(t, "failures", "bogus"); bogus.code != 10 {
		t.Errorf("failures bogus rc=%d want 10", bogus.code)
	}

	root, evolveDir := projectWithState(t, pruneLifecycleState)
	statePath := filepath.Join(evolveDir, "state.json")
	before := stampAgedFile(t, statePath)
	for _, args := range [][]string{
		{"failures", "prune", "--project-root", root, "dry-run"},
		{"failures", "prune", "--project-root", root, "--dry-run", "extra"},
		{"failures", "prune", "--project-root", root, "--no-such-flag"},
	} {
		misuse := runEvolve(t, args...)
		if misuse.code != 10 {
			t.Errorf("evolve %v rc=%d want 10 (usage); stdout=%q", args, misuse.code, misuse.stdout)
		}
		if after := currentStamp(t, statePath); after != before {
			t.Fatalf("evolve %v rewrote state.json: a usage error must never fall through to the mutating prune", args)
		}
	}
}

func TestC1803_005_FailuresListJSONIncludesEveryClassification(t *testing.T) {
	var entries []string
	want := map[string]bool{}
	for i, c := range failurelog.KnownClassifications() {
		want[string(c)] = true
		entries = append(entries, `{"cycle":`+strconv.Itoa(40+i)+`,"classification":"`+string(c)+`","summary":"s","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"}`)
	}
	entries = append(entries, `{"cycle":99,"classification":"code-audit-fail","summary":"expired","recordedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-02-01T00:00:00Z"}`)
	root, _ := projectWithState(t, `{"failedApproaches":[`+strings.Join(entries, ",")+`]}`)

	run := runEvolve(t, "failures", "list", "--json", "--project-root", root)

	if run.code != 0 {
		t.Fatalf("failures list --json rc=%d want 0; stderr=%q", run.code, run.stderr)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &listed); err != nil {
		t.Fatalf("failures list --json is not a JSON array: %v\n%s", err, run.stdout)
	}
	got := map[string]bool{}
	for _, e := range listed {
		cls, _ := e["classification"].(string)
		got[cls] = true
	}
	if len(listed) != len(entries) || !reflect.DeepEqual(got, want) {
		t.Errorf("failures list --json listed %d entries with classes %v, want %d entries covering every class %v", len(listed), got, len(entries), want)
	}
	if bad := runEvolve(t, "failures", "list", "--json", "--class", "no-such-class", "--project-root", root); bad.code != 10 {
		t.Errorf("failures list --class no-such-class rc=%d want 10", bad.code)
	}
}

func TestC1803_006_FailuresListTextShowsExpiryAndAcknowledgedFingerprints(t *testing.T) {
	root, evolveDir := projectWithState(t, `{"failedApproaches":[`+
		`{"cycle":51,"classification":"code-build-fail","summary":"timed","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-03-04T05:06:07Z"},`+
		`{"cycle":52,"classification":"code-audit-fail","summary":"legacy"}]}`)
	for _, fp := range []string{"fp-c1803-zulu", "fp-c1803-alpha"} {
		if err := core.AppendResolvedFingerprint(evolveDir, fp, "operator-reset", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}

	run := runEvolve(t, "failures", "list", "--project-root", root)

	if run.code != 0 {
		t.Fatalf("failures list rc=%d want 0; stderr=%q", run.code, run.stderr)
	}
	if !strings.Contains(run.stdout, "expiresAt=2099-03-04T05:06:07Z") {
		t.Errorf("failures list does not show the entry's expiry; stdout=%q", run.stdout)
	}
	alpha, zulu := strings.Index(run.stdout, "fp-c1803-alpha"), strings.Index(run.stdout, "fp-c1803-zulu")
	if alpha < 0 || zulu < 0 {
		t.Fatalf("failures list does not show the acknowledged fingerprints; stdout=%q", run.stdout)
	}
	if alpha > zulu {
		t.Errorf("acknowledged fingerprints are not sorted ascending; stdout=%q", run.stdout)
	}
	filtered := runEvolve(t, "failures", "list", "--class", "code-build-fail", "--project-root", root)
	if filtered.code != 0 || !strings.Contains(filtered.stdout, "fp-c1803-alpha") || strings.Contains(filtered.stdout, "cycle=52") {
		t.Errorf("failures list --class code-build-fail rc=%d stdout=%q: want only class rows filtered, fingerprints still shown", filtered.code, filtered.stdout)
	}
}

func TestC1803_007_FailuresStateIOErrorsExitTwoAndNeverRewriteState(t *testing.T) {
	cases := []struct {
		name     string
		state    string
		resolved string
		args     []string
		want     int
	}{
		{"list unparseable state", unparseableState, "", []string{"list"}, 2},
		{"list --json unparseable state", unparseableState, "", []string{"list", "--json"}, 2},
		{"list unparseable ledger", resetLifecycleState, "{corrupt ledger", []string{"list"}, 2},
		{"prune --dry-run unparseable state", unparseableState, "", []string{"prune", "--dry-run"}, 2},
		{"prune unparseable state", unparseableState, "", []string{"prune"}, 2},
		{"reset unparseable state", unparseableState, "", []string{"reset"}, 2},
		{"reset unparseable ledger", resetLifecycleState, "{corrupt ledger", []string{"reset", "--fingerprint", "fp-c1803-io"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, evolveDir := projectWithState(t, tc.state)
			if tc.resolved != "" {
				writeFile(t, filepath.Join(evolveDir, "resolved-fingerprints.json"), tc.resolved)
			}

			run := runEvolve(t, append(append([]string{"failures"}, tc.args...), "--project-root", root)...)

			if run.code != tc.want {
				t.Errorf("failures %v rc=%d want %d (state I/O); stderr=%q", tc.args, run.code, tc.want, run.stderr)
			}
			if strings.TrimSpace(run.stderr) == "" {
				t.Errorf("failures %v failed silently: stderr is empty", tc.args)
			}
			if tc.state == unparseableState {
				if got := readFile(t, filepath.Join(evolveDir, "state.json")); got != tc.state {
					t.Errorf("failures %v rewrote an unparseable state.json: %q", tc.args, got)
				}
			}
			if tc.resolved != "" {
				if got := readFile(t, filepath.Join(evolveDir, "resolved-fingerprints.json")); got != tc.resolved {
					t.Errorf("failures %v rewrote an unparseable ledger: %q", tc.args, got)
				}
			}
		})
	}
	root, _ := projectWithState(t, pruneLifecycleState)
	if refused := runEvolveIn(t, root, "failures", "prune"); refused.code != 1 {
		t.Errorf("failures prune without --project-root rc=%d want 1 (mutating refusal)", refused.code)
	}
}

type callGraph struct {
	edges     map[string]map[string]bool
	selectors map[string]map[string]bool
}

func cmdEvolveCallGraph(t *testing.T) callGraph {
	t.Helper()
	dir := filepath.Join(goDir(t), "cmd", "evolve")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	pkg, ok := pkgs["main"]
	if !ok {
		t.Fatalf("no package main in %s", dir)
	}
	topLevel := map[string]bool{}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					topLevel[decl.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, s := range decl.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							topLevel[n.Name] = true
						}
					}
				}
			}
		}
	}
	g := callGraph{edges: map[string]map[string]bool{}, selectors: map[string]map[string]bool{}}
	record := func(owner string, node ast.Node) {
		if g.edges[owner] == nil {
			g.edges[owner], g.selectors[owner] = map[string]bool{}, map[string]bool{}
		}
		notReferences := map[*ast.Ident]bool{}
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				notReferences[x.Sel] = true
				if id, ok := x.X.(*ast.Ident); ok {
					g.selectors[owner][id.Name+"."+x.Sel.Name] = true
				}
			case *ast.KeyValueExpr:
				if key, ok := x.Key.(*ast.Ident); ok {
					notReferences[key] = true
				}
			case *ast.Ident:
				if topLevel[x.Name] && x.Name != owner && !notReferences[x] {
					g.edges[owner][x.Name] = true
				}
			}
			return true
		})
	}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && decl.Body != nil {
					record(decl.Name.Name, decl.Body)
				}
			case *ast.GenDecl:
				for _, s := range decl.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok {
						for i, n := range vs.Names {
							if i < len(vs.Values) {
								record(n.Name, vs.Values[i])
							}
						}
					}
				}
			}
		}
	}
	return g
}

func (g callGraph) ownersOf(selector string) []string {
	var owners []string
	for owner, sels := range g.selectors {
		if sels[selector] {
			owners = append(owners, owner)
		}
	}
	sort.Strings(owners)
	return owners
}

func (g callGraph) reaches(from, to string) bool {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == to {
			return true
		}
		for next := range g.edges[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

func TestC1803_008_LoopResetAndTheFailuresVerbsShareOnePruneAndAckFunction(t *testing.T) {
	parityTests := []string{"TestFailuresResetMatchesTheLoopReset", "TestFailuresPruneMatchesTheLaunchPruneWithoutCarryoverBookkeeping"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(parityTests, "|")+")$", "./cmd/evolve")
	cmd.Dir = goDir(t)
	cmd.Env = isolatedEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("the loop-vs-verb parity tests fail: %v\n%s", err, out)
	}
	for _, name := range parityTests {
		if !strings.Contains(string(out), "--- PASS: "+name) {
			t.Errorf("parity test %s did not run and pass:\n%s", name, out)
		}
	}

	g := cmdEvolveCallGraph(t)
	for _, shared := range []struct {
		what      string
		selector  string
		companion string
		verb      string
	}{
		{"expiry prune", "failurelog.PruneExpired", "failurelog.PruneExpiredCarryoverTodos", "runFailuresPrune"},
		{"reset prune+ack", "failurelog.PruneByClassification", "core.AppendResolvedFingerprint", "runFailuresReset"},
	} {
		owners := g.ownersOf(shared.selector)
		if len(owners) != 1 {
			t.Errorf("%s: %s is called from %v in cmd/evolve, want exactly one shared function", shared.what, shared.selector, owners)
			continue
		}
		owner := owners[0]
		if !g.selectors[owner][shared.companion] {
			t.Errorf("%s: the shared function %s does not also call %s", shared.what, owner, shared.companion)
		}
		if companions := g.ownersOf(shared.companion); shared.what == "expiry prune" && !reflect.DeepEqual(companions, []string{owner}) {
			t.Errorf("%s: %s is called from %v, want only the shared function %s", shared.what, shared.companion, companions, owner)
		}
		for _, entry := range []string{"runLoopBatch", shared.verb} {
			if !g.reaches(entry, owner) {
				t.Errorf("%s: %s never reaches the shared function %s", shared.what, entry, owner)
			}
		}
	}
	for _, bookkeeping := range []string{"failurelog.BackfillLegacyCarryoverExpiry", "failurelog.IncrementCarryoverUnpicked"} {
		for _, owner := range g.ownersOf(bookkeeping) {
			if g.reaches("runFailuresPrune", owner) {
				t.Errorf("failures prune reaches %s, which calls %s: that bookkeeping stays per-batch", owner, bookkeeping)
			}
		}
	}
}

type salvageFixture struct {
	root, salvageDir     string
	landedHead, laneHead string
	lanePatch            string
}

func salvageProject(t *testing.T) salvageFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "f.txt"), "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	landed := repo.Git("rev-parse", "HEAD")
	repo.Git("update-ref", "refs/remotes/origin/main", landed)
	repo.Git("checkout", "-q", "-b", "cycle-cd3ae73e-1690")
	writeFile(t, filepath.Join(repo.Dir, "f.txt"), "base\nlane\n")
	repo.Git("commit", "-q", "-am", "lane work")
	lane := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "main")

	salvageDir := filepath.Join(repo.Dir, ".evolve", "operator-salvage")
	writeFile(t, filepath.Join(salvageDir, "cycle-cd3ae73e-1677", "HEAD"), landed+" cycle-cd3ae73e-1677\n")
	writeFile(t, filepath.Join(salvageDir, "cycle-cd3ae73e-1688", "HEAD"), absentSalvageHead+"\n")
	patch := patchTouching("f.txt", "g.txt")
	writeFile(t, filepath.Join(salvageDir, "cycle-cd3ae73e-1690", "HEAD"), lane+" cycle-cd3ae73e-1690\n")
	writeFile(t, filepath.Join(salvageDir, "cycle-cd3ae73e-1690", "uncommitted.patch"), patch)
	writeTarGz(t, filepath.Join(salvageDir, "cycle-cd3ae73e-1690", "untracked.tgz"), "a.txt", "sub/b.txt", "sub/c.txt")
	writeFile(t, filepath.Join(salvageDir, ".DS_Store"), "not a leaf")
	return salvageFixture{root: repo.Dir, salvageDir: salvageDir, landedHead: landed, laneHead: lane, lanePatch: patch}
}

func TestC1803_009_SalvageListJSONReportsEachLeafsCycleHeadPatchStatsAndLanded(t *testing.T) {
	fx := salvageProject(t)
	before := treeSnapshot(t, fx.salvageDir)

	run := runEvolve(t, "salvage", "list", "--json", "--project-root", fx.root)

	if run.code != 0 {
		t.Fatalf("salvage list --json rc=%d want 0; stdout=%q stderr=%q", run.code, run.stdout, run.stderr)
	}
	rows := decodeRows(t, run.stdout)
	var order []string
	for _, r := range rows {
		leaf, _ := r["leaf"].(string)
		order = append(order, leaf)
	}
	if want := []string{"cycle-cd3ae73e-1677", "cycle-cd3ae73e-1688", "cycle-cd3ae73e-1690"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("salvage list rows %v, want one row per leaf sorted by cycle %v (stray files are not leaves)", order, want)
	}
	wants := map[string]map[string]any{
		"cycle-cd3ae73e-1677": {"cycle": 1677.0, "head": fx.landedHead, "branch": "cycle-cd3ae73e-1677", "changed_files": 0.0, "patch_bytes": 0.0, "untracked_files": 0.0, "landed": true},
		"cycle-cd3ae73e-1688": {"cycle": 1688.0, "head": absentSalvageHead, "branch": "", "changed_files": 0.0, "patch_bytes": 0.0, "untracked_files": 0.0, "landed": nil},
		"cycle-cd3ae73e-1690": {"cycle": 1690.0, "head": fx.laneHead, "branch": "cycle-cd3ae73e-1690", "changed_files": 2.0, "patch_bytes": float64(len(fx.lanePatch)), "untracked_files": 3.0, "landed": false},
	}
	for leaf, row := range rowByLeaf(rows) {
		for key, want := range wants[leaf] {
			got, present := row[key]
			if !present || !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s = %#v (present=%v), want %#v", leaf, key, got, present, want)
			}
		}
		if at, _ := row["salvaged_at"].(string); at == "" {
			t.Errorf("%s has no salvaged_at (the age clock): %v", leaf, row)
		} else if _, err := time.Parse(time.RFC3339, at); err != nil {
			t.Errorf("%s salvaged_at %q is not RFC3339: %v", leaf, at, err)
		}
	}
	if after := treeSnapshot(t, fx.salvageDir); !reflect.DeepEqual(after, before) {
		t.Errorf("salvage list changed the salvage directory: it must be read-only")
	}
}

func TestC1803_010_SalvageListTextPrintsOneRowPerLeafWithAgeAndLanded(t *testing.T) {
	fx := salvageProject(t)

	run := runEvolve(t, "salvage", "list", "--project-root", fx.root)

	if run.code != 0 {
		t.Fatalf("salvage list rc=%d want 0; stdout=%q stderr=%q", run.code, run.stdout, run.stderr)
	}
	lines := nonBlankLines(run.stdout)
	if len(lines) != 3 {
		t.Fatalf("salvage list printed %d lines, want one per leaf (3):\n%s", len(lines), run.stdout)
	}
	wants := []struct{ leaf, head, landed string }{
		{"cycle-cd3ae73e-1677", fx.landedHead, "landed=yes"},
		{"cycle-cd3ae73e-1688", absentSalvageHead, "landed=unknown"},
		{"cycle-cd3ae73e-1690", fx.laneHead, "landed=no"},
	}
	for i, want := range wants {
		line := lines[i]
		for _, needle := range []string{want.leaf, "head=" + want.head, want.landed, "age="} {
			if !strings.Contains(line, needle) {
				t.Errorf("row %d %q lacks %q", i, line, needle)
			}
		}
	}
}

func TestC1803_011_SalvageListUnreadableLeafExitsTwoNamingItAndStillListsTheReadable(t *testing.T) {
	fx := salvageProject(t)
	badLeaves := []string{"cycle-cd3ae73e-1699", "cycle-cd3ae73e-1700", "cycle-cd3ae73e-1701"}
	writeFile(t, filepath.Join(fx.salvageDir, badLeaves[0], "HEAD"), "not-a-sha cycle-cd3ae73e-1699\n")
	writeFile(t, filepath.Join(fx.salvageDir, badLeaves[1], "HEAD"), fx.laneHead+" cycle-cd3ae73e-1700\n")
	writeFile(t, filepath.Join(fx.salvageDir, badLeaves[1], "untracked.tgz"), "this is not gzip")
	writeFile(t, filepath.Join(fx.salvageDir, badLeaves[2], "uncommitted.patch"), patchTouching("f.txt"))

	for _, form := range [][]string{{"--json"}, {}} {
		args := append(append([]string{"salvage", "list"}, form...), "--project-root", fx.root)
		run := runEvolve(t, args...)

		if run.code != 2 {
			t.Errorf("evolve %v with unreadable leaves rc=%d want 2; stderr=%q", args, run.code, run.stderr)
		}
		for _, bad := range badLeaves {
			if !strings.Contains(run.stderr, bad) {
				t.Errorf("evolve %v stderr does not name the unreadable leaf %s: %q", args, bad, run.stderr)
			}
			if strings.Contains(run.stdout, bad) {
				t.Errorf("evolve %v printed a row for the unreadable leaf %s", args, bad)
			}
		}
		if len(form) == 1 {
			if got := rowByLeaf(decodeRows(t, run.stdout)); len(got) != 3 {
				t.Errorf("salvage list --json listed %d readable leaves, want the 3 readable ones: %v", len(got), got)
			}
		} else if !strings.Contains(run.stdout, "cycle-cd3ae73e-1690") {
			t.Errorf("salvage list dropped the readable leaves; stdout=%q", run.stdout)
		}
	}
}

func TestC1803_012_SalvageListEmptyOrMissingDirPrintsNothingAndExitsZero(t *testing.T) {
	missing, _ := projectWithState(t, "{}")
	empty, emptyEvolveDir := projectWithState(t, "{}")
	if err := os.MkdirAll(filepath.Join(emptyEvolveDir, "operator-salvage"), 0o755); err != nil {
		t.Fatal(err)
	}
	strayOnly, strayEvolveDir := projectWithState(t, "{}")
	writeFile(t, filepath.Join(strayEvolveDir, "operator-salvage", ".DS_Store"), "not a leaf")

	for _, root := range []string{missing, empty, strayOnly} {
		for _, form := range [][]string{{"--json"}, {}} {
			args := append(append([]string{"salvage", "list"}, form...), "--project-root", root)
			run := runEvolve(t, args...)
			if run.code != 0 || run.stdout != "" || run.stderr != "" {
				t.Errorf("evolve %v on a leafless salvage dir: rc=%d stdout=%q stderr=%q, want rc=0 and no output", args, run.code, run.stdout, run.stderr)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(missing, ".evolve", "operator-salvage")); !os.IsNotExist(err) {
		t.Errorf("salvage list created the missing salvage directory (stat err=%v)", err)
	}

	for _, args := range [][]string{
		{"salvage", "list", "--no-such-flag", "--project-root", missing},
		{"salvage", "list", "--project-root", missing, "extra"},
		{"salvage", "bogus"},
	} {
		if run := runEvolve(t, args...); run.code != 10 {
			t.Errorf("evolve %v rc=%d want 10 (usage)", args, run.code)
		}
	}
}

func closeOutCycle(t *testing.T, projectRoot string, cycle int, at time.Time) {
	t.Helper()
	p := filepath.Join(dossier.CyclesDir(projectRoot), "cycle-"+strconv.Itoa(cycle)+".json")
	writeFile(t, p, `{}`)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func goStringLiteralSites(t *testing.T, root, value string) []string {
	t.Helper()
	var sites []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(raw), value) {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, raw, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && s == value {
					rel, _ := filepath.Rel(root, path)
					sites = append(sites, rel)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sites
}

func TestC1803_013_GCAndSalvageListReadOneExportedSalvagePathHelper(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "f.txt"), "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("update-ref", "refs/remotes/origin/main", repo.Git("rev-parse", "HEAD"))
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	const leaf = "cycle-cd3ae73e-1677"
	wt := filepath.Join(base, leaf)
	repo.Git("worktree", "add", "-q", wt, "-b", leaf)
	writeFile(t, filepath.Join(wt, "f.txt"), "base\nlane edit\n")
	repo.Git("-C", wt, "commit", "-q", "-am", "lane work")
	writeFile(t, filepath.Join(wt, "f.txt"), "base\nlane edit\nuncommitted edit\n")
	writeFile(t, filepath.Join(wt, "sub", "new.txt"), "untracked\n")
	tip := repo.Git("rev-parse", leaf)
	now := time.Now()
	closeOutCycle(t, repo.Dir, 1677, now.Add(-30*time.Hour))
	o := gc.WorktreeOptions{ProjectRoot: repo.Dir, WorktreeBase: base, EvolveDir: filepath.Join(repo.Dir, ".evolve"),
		Policy: gc.WorktreesPolicy{SalvageAfterHours: 24}, Now: func() time.Time { return now }, Exec: sysexec.DefaultRunner}
	m, err := gc.PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.ApplyWorktrees(o, m); err != nil {
		t.Fatalf("gc salvage: %v", err)
	}

	if head := readFile(t, filepath.Join(gc.OperatorSalvageDir(o.EvolveDir), leaf, "HEAD")); !strings.HasPrefix(head, tip) {
		t.Errorf("gc did not salvage under gc.OperatorSalvageDir: HEAD=%q want prefix %s", head, tip)
	}
	leaves, err := gc.ListSalvage(o.EvolveDir)
	if err != nil {
		t.Fatalf("gc.ListSalvage: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("gc.ListSalvage returned %+v, want the one leaf gc salvaged", leaves)
	}
	got := leaves[0]
	if got.Leaf != leaf || got.Cycle != 1677 || got.Head != tip || got.Branch != leaf || got.ChangedFiles != 1 || got.PatchBytes <= 0 || got.UntrackedFiles != 1 || got.SalvagedAt.IsZero() {
		t.Errorf("gc.ListSalvage disagrees with what gc salvaged: %+v (want leaf=%s cycle=1677 head=%s branch=%s changed=1 untracked=1)", got, leaf, tip, leaf)
	}

	run := runEvolve(t, "salvage", "list", "--json", "--project-root", repo.Dir)
	if run.code != 0 {
		t.Fatalf("salvage list --json rc=%d; stderr=%q", run.code, run.stderr)
	}
	row := rowByLeaf(decodeRows(t, run.stdout))[leaf]
	if row == nil || row["head"] != tip || row["branch"] != leaf || row["changed_files"] != 1.0 || row["untracked_files"] != 1.0 || row["landed"] != false {
		t.Errorf("salvage list row %v disagrees with gc's salvage of %s at %s (want changed=1 untracked=1 landed=false)", row, leaf, tip)
	}

	if sites := goStringLiteralSites(t, goDir(t), "operator-salvage"); len(sites) != 1 || !strings.HasPrefix(sites[0], filepath.Join("internal", "gc")+string(filepath.Separator)) {
		t.Errorf("the salvage directory name is spelled at %v in non-test Go, want exactly one site inside internal/gc (the exported helper)", sites)
	}
}

func TestC1803_014_UsageLinesNameTheNewVerbs(t *testing.T) {
	salvageUsage := runEvolve(t, "salvage")
	if salvageUsage.code != 10 || !strings.Contains(salvageUsage.stderr, "salvage list") {
		t.Errorf("evolve salvage rc=%d stderr=%q: want rc=10 and a usage line naming `salvage list`", salvageUsage.code, salvageUsage.stderr)
	}
	failuresUsage := runEvolve(t, "failures")
	if failuresUsage.code != 10 || !strings.Contains(failuresUsage.stderr, "--dry-run") {
		t.Errorf("evolve failures rc=%d stderr=%q: want rc=10 and a usage line naming prune --dry-run", failuresUsage.code, failuresUsage.stderr)
	}
}

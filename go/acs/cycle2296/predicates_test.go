//go:build acs

package cycle2296

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	stateMerged = "MERGED"
	stateOpen   = "OPEN"
	stateClosed = "CLOSED"
)

type journal struct {
	mu    sync.Mutex
	lines []string
}

func (j *journal) add(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, line)
}

func (j *journal) index(fragment string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, l := range j.lines {
		if strings.Contains(l, fragment) {
			return i
		}
	}
	return -1
}

func (j *journal) gitPushes() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	var pushes []string
	for _, l := range j.lines {
		if strings.HasPrefix(l, "git ") && slices.Contains(strings.Fields(l), "push") {
			pushes = append(pushes, l)
		}
	}
	return pushes
}

func recordingExec(j *journal, failBundleVerify bool) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		j.add(name + " " + strings.Join(args, " "))
		if failBundleVerify && name == "git" && slices.Contains(args, "bundle") && slices.Contains(args, "verify") {
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

type world struct {
	t         *testing.T
	project   *gittest.Repo
	bare      *gittest.Repo
	evolveDir string
	seedTip   string
	tips      map[string]string
}

func newWorld(t *testing.T, branches ...string) *world {
	t.Helper()
	w := &world{t: t, project: gittest.Fixture(t), bare: gittest.Bare(t), tips: map[string]string{}}
	w.evolveDir = filepath.Join(w.project.Dir, ".evolve")
	w.commit("seed.txt", "seed")
	w.project.Git("remote", "add", "origin", w.bare.Dir)
	w.project.Git("push", "-q", "origin", "main")
	w.seedTip = w.project.Git("rev-parse", "HEAD")
	w.tips["main"] = w.seedTip
	for _, b := range branches {
		w.project.Git("checkout", "-q", "-b", b, "main")
		w.commit(b+".txt", b)
		w.project.Git("push", "-q", "origin", b)
		w.tips[b] = w.project.Git("rev-parse", "HEAD")
		w.project.Git("checkout", "-q", "main")
		w.project.Git("branch", "-q", "-D", b)
	}
	return w
}

func (w *world) commit(file, body string) {
	w.t.Helper()
	if err := os.WriteFile(filepath.Join(w.project.Dir, file), []byte(body+"\n"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	w.project.Git("add", file)
	w.project.Git("commit", "-q", "-m", "work on "+body)
}

func (w *world) advanceRemote(branch string) string {
	w.t.Helper()
	w.project.Git("fetch", "-q", "origin")
	w.project.Git("checkout", "-q", "-b", "advance-tmp", "origin/"+branch)
	w.commit(branch+"-later.txt", branch+" later")
	tip := w.project.Git("rev-parse", "HEAD")
	w.project.Git("push", "-q", "origin", "advance-tmp:refs/heads/"+branch)
	w.project.Git("checkout", "-q", "main")
	w.project.Git("branch", "-q", "-D", "advance-tmp")
	return tip
}

func (w *world) remoteBranches() []string {
	names := strings.Fields(w.bare.Git("for-each-ref", "--format=%(refname:short)", "refs/heads"))
	sort.Strings(names)
	return names
}

func (w *world) hasRemoteBranch(branch string) bool {
	return slices.Contains(w.remoteBranches(), branch)
}

func (w *world) bundledTips() string {
	w.t.Helper()
	var heads strings.Builder
	for _, file := range w.bundleFiles() {
		heads.WriteString(w.project.Git("bundle", "list-heads", file))
		heads.WriteString("\n")
	}
	return heads.String()
}

func (w *world) bundleFiles() []string {
	var files []string
	_ = filepath.WalkDir(gc.OperatorSalvageDir(w.evolveDir), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && strings.HasSuffix(path, ".bundle") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

type fakeAPI struct {
	w         *world
	journal   *journal
	prs       []gc.RemotePR
	prsErr    error
	protected map[string]bool
	lookupErr map[string]error
	deleted   []string
}

func (f *fakeAPI) PullRequests(context.Context) ([]gc.RemotePR, error) {
	f.journal.add("api pull-requests")
	return f.prs, f.prsErr
}

func (f *fakeAPI) RulesetProtects(_ context.Context, branch string) (bool, error) {
	if err := f.lookupErr[branch]; err != nil {
		return false, err
	}
	return f.protected[branch], nil
}

func (f *fakeAPI) DeleteBranch(_ context.Context, branch string) error {
	f.journal.add("api delete " + branch)
	f.deleted = append(f.deleted, branch)
	f.w.bare.Git("update-ref", "-d", "refs/heads/"+branch)
	return nil
}

func pr(branch, state, oid string) gc.RemotePR {
	return gc.RemotePR{Branch: branch, State: state, HeadRefOid: oid}
}

func (w *world) options(stage gc.RemoteStage, api gc.RemoteAPI, run sysexec.RunFunc, log io.Writer) gc.RemoteSweepOptions {
	return gc.RemoteSweepOptions{
		ProjectRoot: w.project.Dir,
		EvolveDir:   w.evolveDir,
		Remote:      "origin",
		Stage:       stage,
		Exec:        run,
		API:         api,
		Log:         log,
	}
}

func itemFor(m gc.RemoteManifest, branch string) (gc.RemoteItem, bool) {
	for _, it := range m.Items {
		if it.Branch == branch {
			return it, true
		}
	}
	return gc.RemoteItem{}, false
}

func standardWorld(t *testing.T) (*world, *fakeAPI, *journal) {
	t.Helper()
	w := newWorld(t, "merged-ok", "merged-moved", "closed-unmerged", "open-pr", "no-pr")
	j := &journal{}
	api := &fakeAPI{w: w, journal: j, prs: []gc.RemotePR{
		pr("merged-ok", stateMerged, w.tips["merged-ok"]),
		pr("merged-moved", stateMerged, w.seedTip),
		pr("closed-unmerged", stateClosed, w.tips["closed-unmerged"]),
		pr("open-pr", stateOpen, w.tips["open-pr"]),
		pr("main", stateMerged, w.tips["main"]),
	}}
	return w, api, j
}

func TestC2296_001_ShadowReportsTheSweepAndDeletesNothing(t *testing.T) {
	w, api, j := standardWorld(t)
	var log bytes.Buffer
	opts := w.options(gc.RemoteStageShadow, api, recordingExec(j, false), &log)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanRemoteSweep at shadow: %v", err)
	}
	if err := gc.ApplyRemoteSweep(context.Background(), opts, m); err != nil {
		t.Fatalf("ApplyRemoteSweep at shadow: %v", err)
	}
	it, ok := itemFor(m, "merged-ok")
	if !ok || it.Action != gc.RemoteActionDelete {
		t.Errorf("shadow plan must report merged-ok as the branch enforce would delete, got %+v (found=%v)", it, ok)
	}
	if !strings.Contains(log.String(), "merged-ok") {
		t.Errorf("shadow must report merged-ok in its log, got %q", log.String())
	}
	if len(api.deleted) != 0 {
		t.Errorf("shadow called the delete API for %v", api.deleted)
	}
	if !w.hasRemoteBranch("merged-ok") {
		t.Errorf("shadow removed merged-ok from the remote")
	}
	if files := w.bundleFiles(); len(files) != 0 {
		t.Errorf("shadow wrote bundles %v; it must only report", files)
	}
	if pushes := j.gitPushes(); len(pushes) != 0 {
		t.Errorf("shadow ran git push: %v", pushes)
	}
}

func TestC2296_002_EnforceDeletesOnlyAMergedBranchWhoseTipMatches(t *testing.T) {
	w, api, j := standardWorld(t)
	var log bytes.Buffer
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, false), &log)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanRemoteSweep at enforce: %v", err)
	}
	if err := gc.ApplyRemoteSweep(context.Background(), opts, m); err != nil {
		t.Fatalf("ApplyRemoteSweep at enforce: %v", err)
	}
	if !slices.Equal(api.deleted, []string{"merged-ok"}) {
		t.Fatalf("enforce deleted %v, want exactly [merged-ok]", api.deleted)
	}
	want := []string{"closed-unmerged", "main", "merged-moved", "no-pr", "open-pr"}
	if got := w.remoteBranches(); !slices.Equal(got, want) {
		t.Errorf("remote branches after enforce = %v, want %v", got, want)
	}
	for _, kept := range want {
		it, ok := itemFor(m, kept)
		if ok && (it.Action != gc.RemoteActionKeep || strings.TrimSpace(it.Reason) == "") {
			t.Errorf("%s must be kept with a reason, got %+v", kept, it)
		}
	}
	if !strings.Contains(log.String(), "merged-ok") {
		t.Errorf("enforce must report the deleted branch in its log, got %q", log.String())
	}
}

func TestC2296_003_AVerifiedBundleOfTheRemoteTipComesBeforeTheDelete(t *testing.T) {
	w, api, j := standardWorld(t)
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, false), io.Discard)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanRemoteSweep: %v", err)
	}
	if err := gc.ApplyRemoteSweep(context.Background(), opts, m); err != nil {
		t.Fatalf("ApplyRemoteSweep: %v", err)
	}
	if !strings.Contains(w.bundledTips(), w.tips["merged-ok"]) {
		t.Errorf("no bundle under the salvage dir holds the remote tip %s of merged-ok", w.tips["merged-ok"])
	}
	verify, del := j.index("bundle verify"), j.index("api delete merged-ok")
	if verify < 0 || del < 0 || verify > del {
		t.Errorf("git bundle verify (journal index %d) must run before the delete API call (index %d)", verify, del)
	}
	if pushes := j.gitPushes(); len(pushes) != 0 {
		t.Errorf("the delete must go through the API, not git push: %v", pushes)
	}
}

func TestC2296_004_AFailedBundleVerifyKeepsTheRemoteBranch(t *testing.T) {
	w, api, j := standardWorld(t)
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, true), io.Discard)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanRemoteSweep: %v", err)
	}
	applyErr := gc.ApplyRemoteSweep(context.Background(), opts, m)
	if applyErr == nil || !strings.Contains(applyErr.Error(), "merged-ok") {
		t.Errorf("a failed bundle verify must surface an error naming merged-ok, got %v", applyErr)
	}
	if len(api.deleted) != 0 {
		t.Errorf("the delete API ran after a failed bundle verify: %v", api.deleted)
	}
	if !w.hasRemoteBranch("merged-ok") {
		t.Errorf("merged-ok left the remote after a failed bundle verify")
	}
}

func TestC2296_005_AnOpenPullRequestARulesetOrALookupFailureKeepsTheBranch(t *testing.T) {
	w := newWorld(t, "clean", "reused", "ruled", "flaky")
	j := &journal{}
	api := &fakeAPI{
		w:       w,
		journal: j,
		prs: []gc.RemotePR{
			pr("clean", stateMerged, w.tips["clean"]),
			pr("reused", stateMerged, w.tips["reused"]),
			pr("reused", stateOpen, w.tips["reused"]),
			pr("ruled", stateMerged, w.tips["ruled"]),
			pr("flaky", stateMerged, w.tips["flaky"]),
		},
		protected: map[string]bool{"ruled": true},
		lookupErr: map[string]error{"flaky": errors.New("ruleset api unavailable")},
	}
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, false), io.Discard)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("a per-branch ruleset failure must keep that branch, not fail the plan: %v", err)
	}
	if err := gc.ApplyRemoteSweep(context.Background(), opts, m); err != nil {
		t.Fatalf("ApplyRemoteSweep: %v", err)
	}
	if !slices.Equal(api.deleted, []string{"clean"}) {
		t.Errorf("deleted %v, want only the unobstructed merged branch [clean]", api.deleted)
	}
	for _, kept := range []string{"reused", "ruled", "flaky"} {
		if !w.hasRemoteBranch(kept) {
			t.Errorf("%s left the remote", kept)
		}
	}
}

func TestC2296_006_AFailedPullRequestReadPlansNothing(t *testing.T) {
	w, api, j := standardWorld(t)
	api.prsErr = errors.New("gh unavailable")
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, false), io.Discard)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err == nil {
		t.Errorf("a failed pull request read must fail the plan, got manifest %+v", m)
	}
	for _, it := range m.Items {
		if it.Action == gc.RemoteActionDelete {
			t.Errorf("a failed pull request read still planned a delete: %+v", it)
		}
	}
	if len(api.deleted) != 0 {
		t.Errorf("deleted %v after a failed pull request read", api.deleted)
	}
}

func TestC2296_007_ATipThatMovedAfterThePlanIsNotDeleted(t *testing.T) {
	w, api, j := standardWorld(t)
	opts := w.options(gc.RemoteStageEnforce, api, recordingExec(j, false), io.Discard)
	m, err := gc.PlanRemoteSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanRemoteSweep: %v", err)
	}
	moved := w.advanceRemote("merged-ok")
	_ = gc.ApplyRemoteSweep(context.Background(), opts, m)
	if len(api.deleted) != 0 {
		t.Errorf("deleted %v although the remote tip moved after the plan", api.deleted)
	}
	if got := w.bare.Git("rev-parse", "refs/heads/merged-ok"); got != moved {
		t.Errorf("remote merged-ok tip = %s, want the advanced tip %s", got, moved)
	}
}

func TestC2296_008_TheStageComesFromPolicyDataAndFailsSafeToShadow(t *testing.T) {
	cases := map[string]gc.RemoteStage{
		`{}`:                                         gc.RemoteStageShadow,
		`{"remote_sweep":{}}`:                        gc.RemoteStageShadow,
		`{"remote_sweep":{"stage":"shadow"}}`:        gc.RemoteStageShadow,
		`{"remote_sweep":{"stage":"enforce"}}`:       gc.RemoteStageEnforce,
		`{"remote_sweep":{"stage":"Enforce"}}`:       gc.RemoteStageShadow,
		`{"remote_sweep":{"stage":"on"}}`:            gc.RemoteStageShadow,
		`{"remote_sweep":{"stage":"enforce "}}`:      gc.RemoteStageShadow,
		`{"mode":"enforce","remote_sweep":null}`:     gc.RemoteStageShadow,
		`{"remote_sweep":{"stage":"enforce"},"x":1}`: gc.RemoteStageEnforce,
	}
	for doc, want := range cases {
		var pol gcpolicy.Policy
		if err := json.Unmarshal([]byte(doc), &pol); err != nil {
			t.Fatalf("policy %s: %v", doc, err)
		}
		if got := gc.RemoteStage(pol.RemoteStage()); got != want {
			t.Errorf("policy %s: stage = %q, want %q", doc, got, want)
		}
	}
}

func TestC2296_009_TheGCCommandReachesTheRemoteSweepThroughGh(t *testing.T) {
	root := acsassert.RepoRoot(t)
	mod, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	bin := filepath.Join(scratch, "evolve")
	build := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	build.Dir = mod
	build.WaitDelay = 10 * time.Second
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/evolve: %v\n%s", err, acsassert.OutputTail(string(out), 20))
	}

	w, _, _ := standardWorld(t)
	policyPath := filepath.Join(w.evolveDir, "policy.json")
	if err := os.MkdirAll(w.evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prs := []map[string]string{
		{"headRefName": "merged-ok", "headRefOid": w.tips["merged-ok"], "state": stateMerged},
		{"headRefName": "open-pr", "headRefOid": w.tips["open-pr"], "state": stateOpen},
	}
	prsJSON, err := json.Marshal(prs)
	if err != nil {
		t.Fatal(err)
	}
	prsFile := filepath.Join(scratch, "prs.json")
	if err := os.WriteFile(prsFile, prsJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	ghLog := filepath.Join(scratch, "gh.log")
	fakeBin := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho \"$*\" >> \"$GH_LOG\"\ncase \"$1 $2\" in\n\"pr list\") cat \"$GH_PRS\" ;;\n*) case \"$*\" in *DELETE*) exit 0 ;; *) echo '[]' ;; esac ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(scratch, "home")
	for _, d := range []string{home, filepath.Join(scratch, "tmp"), filepath.Join(scratch, "gocache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	run := exec.Command(bin, "gc", "--dry-run", "--project-root", w.project.Dir)
	run.Dir = w.project.Dir
	run.WaitDelay = 10 * time.Second
	run.Env = []string{
		"PATH=" + fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home,
		"TMPDIR=" + filepath.Join(scratch, "tmp"),
		"GOCACHE=" + filepath.Join(scratch, "gocache"),
		"GH_LOG=" + ghLog,
		"GH_PRS=" + prsFile,
	}
	out, runErr := run.CombinedOutput()
	t.Logf("evolve gc --dry-run exit error: %v", runErr)

	var remoteLine string
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "open-pr") && strings.Contains(lower, "would-delete") {
			t.Errorf("a branch with an open pull request is reported for deletion: %q", line)
		}
		if strings.Contains(lower, "remote") && strings.Contains(lower, "merged-ok") && strings.Contains(lower, "would") {
			remoteLine = line
		}
	}
	if remoteLine == "" {
		t.Errorf("evolve gc --dry-run at remote_sweep.stage enforce printed no remote WOULD line for merged-ok:\n%s", acsassert.OutputTail(string(out), 40))
	}
	logged, _ := os.ReadFile(ghLog)
	if !strings.Contains(string(logged), "pr list") {
		t.Errorf("the production sweep did not read pull requests through gh; gh saw:\n%s", logged)
	}
	if strings.Contains(string(logged), "DELETE") {
		t.Errorf("a dry run called the delete API:\n%s", logged)
	}
	if !w.hasRemoteBranch("merged-ok") {
		t.Errorf("a dry run removed merged-ok from the remote")
	}
}

func changedFunctionsAreFullyCovered(t *testing.T, pkgRel, runFilter string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	moduleDir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	args := []string{"test", "-count=1", "-tags", "integration", "-coverprofile=" + profile}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	test := exec.Command("go", append(args, "./"+pkgRel)...)
	test.Dir = moduleDir
	test.WaitDelay = 10 * time.Second
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./%s failed: %v\n%s", pkgRel, err, acsassert.OutputTail(string(out), 30))
	}
	packages, err := covchanged.ParsePackageSet([]byte(pkgRel))
	if err != nil {
		t.Fatalf("package set: %v", err)
	}
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: sysexec.DefaultRunner}
	verdict, err := ws.Check(context.Background(), covchanged.Options{Base: "main", Profile: profile, Tags: covchanged.DefaultTags, Packages: packages})
	if err != nil {
		t.Fatalf("covchanged: %v", err)
	}
	if verdict.Changed == 0 {
		t.Fatalf("no function of %s changed against main", pkgRel)
	}
	if !verdict.Clean() {
		t.Errorf("changed functions of %s below 100%% coverage:\n%s", pkgRel, strings.Join(verdict.Problems(), "\n"))
	}
}

func TestC2296_010_ChangedGcFunctionsAreFullyCovered(t *testing.T) {
	changedFunctionsAreFullyCovered(t, "internal/gc", "")
}

func TestC2296_011_ChangedGcPolicyFunctionsAreFullyCovered(t *testing.T) {
	changedFunctionsAreFullyCovered(t, "internal/gcpolicy", "")
}

func TestC2296_012_ChangedGcCommandFunctionsAreFullyCovered(t *testing.T) {
	changedFunctionsAreFullyCovered(t, "cmd/evolve", "^TestGC")
}

func TestC2296_013_ExplanationNamesTheDecisionAndComponentIDs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2296-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("docs/explain/builds holds %d cycle-2296 document(s), want exactly 1", len(matches))
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}
	for _, id := range []string{"D10", "X8"} {
		if !strings.Contains(string(body), id) {
			t.Errorf("%s does not name %s", filepath.Base(matches[0]), id)
		}
	}
}

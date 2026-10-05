//go:build acs

package cycle1792

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fakeGHStateEnv  = "C1792_FAKE_GH_STATE"
	neverCompletes  = 1 << 30
	pollEverySecond = `{"ci_watch":{"poll_s":1}}`
	classifyHeader  = "label\tpackage\ttest\trule\ttouched\tbase_red\trecurred_on_main\trerun_green"
	alphaPkg        = "example.com/fix/internal/alpha"
	gammaPkg        = "example.com/fix/internal/gamma"
	deltaPkg        = "example.com/fix/internal/delta"
	epsilonPkg      = "example.com/fix/internal/epsilon"
	ubuntuJob       = "test (ubuntu-latest)"
	macosJob        = "test (macos-latest)"
	commentOnlyOK   = "comment-only: 1 changed Go file(s) verified\n"
)

var stampTime = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func at(offset time.Duration) string { return stampTime.Add(offset).Format(time.RFC3339) }

func TestMain(m *testing.M) {
	if state := os.Getenv(fakeGHStateEnv); state != "" {
		os.Exit(serveFakeGH(state, os.Args[1:], os.Stdout, os.Stderr))
	}
	code := m.Run()
	for _, b := range []*builtBinary{evolveBuild, commentauditBuild} {
		if b.dir == "" {
			continue
		}
		if err := os.RemoveAll(b.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1792: remove %s: %v\n", b.dir, err)
		}
	}
	os.Exit(code)
}

type builtBinary struct {
	once      sync.Once
	pkg, name string
	dir, path string
	failure   string
}

var (
	evolveBuild       = &builtBinary{pkg: "./cmd/evolve", name: "evolve"}
	commentauditBuild = &builtBinary{pkg: "./cmd/commentaudit", name: "commentaudit"}
)

func (b *builtBinary) get(t *testing.T) string {
	t.Helper()
	b.once.Do(func() { b.failure = b.build(filepath.Join(acsassert.RepoRoot(t), "go")) })
	if b.failure != "" {
		t.Fatalf("go build %s: %s", b.pkg, b.failure)
	}
	return b.path
}

func (b *builtBinary) build(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1792-"+b.name+"-")
	if err != nil {
		return err.Error()
	}
	b.dir, b.path = dir, filepath.Join(dir, b.name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", b.path, b.pkg)
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

func evolveBin(t *testing.T) string       { return evolveBuild.get(t) }
func commentauditBin(t *testing.T) string { return commentauditBuild.get(t) }

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func run(t *testing.T, dir string, env []string, bin string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("%s %s: %v", filepath.Base(bin), strings.Join(args, " "), err)
	}
	return r
}

func knownVerb(t *testing.T, r result, verb string) bool {
	t.Helper()
	if strings.Contains(r.stderr, "unknown command") {
		t.Errorf("evolve has no %q verb: %s", verb, r)
		return false
	}
	return true
}

func hermeticEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if name == "PATH" || strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") ||
			strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "C1792_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	return append(env, extra...)
}

func toolsDir(t *testing.T, withFakeGH bool) string {
	t.Helper()
	dir := t.TempDir()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("the fixtures need git: %v", err)
	}
	links := map[string]string{"git": gitBin}
	if withFakeGH {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		links["gh"] = self
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func hexOf(seed string) string { return fmt.Sprintf("%x", sha1.Sum([]byte(seed))) }

type fakePR struct {
	State       string `json:"state"`
	IsDraft     bool   `json:"is_draft"`
	Mergeable   string `json:"mergeable"`
	Head        string `json:"head"`
	Base        string `json:"base"`
	UpdatedHead string `json:"updated_head"`
	MergeSHA    string `json:"merge_sha"`
	MergeLies   bool   `json:"merge_lies"`
}

type fakeRerun struct {
	Conclusion string `json:"conclusion"`
	Log        string `json:"log"`
}

type fakeRun struct {
	ID           int64      `json:"id"`
	Head         string     `json:"head"`
	Branch       string     `json:"branch"`
	CreatedAt    string     `json:"created_at"`
	Conclusion   string     `json:"conclusion"`
	PendingPolls int        `json:"pending_polls"`
	Log          string     `json:"log"`
	LogFails     bool       `json:"log_fails"`
	Rerun        *fakeRerun `json:"rerun,omitempty"`
}

type ghScenario struct {
	Broken  bool              `json:"broken"`
	RepoDir string            `json:"repo_dir"`
	PRs     map[string]fakePR `json:"prs"`
	Behind  map[string]int    `json:"behind"`
	Runs    []fakeRun         `json:"runs"`
}

type runState struct {
	status, conclusion, log string
	attempt                 int
}

type fakeGH struct {
	state string
	sc    ghScenario
}

func serveFakeGH(state string, args []string, stdout, stderr io.Writer) int {
	g, err := loadFakeGH(state, args)
	if err != nil {
		fmt.Fprintf(stderr, "fake gh: %v\n", err)
		return 3
	}
	c := parseGHCall(args)
	if g.sc.Broken && !c.is("repo", "view") && !c.is("auth", "status") {
		fmt.Fprintln(stderr, "error connecting to api.github.com")
		return 1
	}
	if refused := templateFlag(c); refused != "" {
		fmt.Fprintf(stderr, "fake gh: %s is not modelled; request --json fields and decode them in Go\n", refused)
		return 1
	}
	out, err := g.answer(c)
	if err != nil {
		fmt.Fprintf(stderr, "fake gh: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}

func loadFakeGH(state string, args []string) (fakeGH, error) {
	logged, err := json.Marshal(args)
	if err != nil {
		return fakeGH{}, err
	}
	if err := appendLine(filepath.Join(state, "calls.log"), string(logged)); err != nil {
		return fakeGH{}, err
	}
	raw, err := os.ReadFile(filepath.Join(state, "scenario.json"))
	if err != nil {
		return fakeGH{}, err
	}
	var sc ghScenario
	if err := json.Unmarshal(raw, &sc); err != nil {
		return fakeGH{}, err
	}
	return fakeGH{state: state, sc: sc}, nil
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func fakeStateFailed(err error) {
	fmt.Fprintf(os.Stderr, "fake gh: state: %v\n", err)
	os.Exit(3)
}

func (g fakeGH) path(name string) string { return filepath.Join(g.state, name) }

func (g fakeGH) marked(name string) bool {
	_, err := os.Stat(g.path(name))
	return err == nil
}

func (g fakeGH) mark(name string) {
	if err := os.WriteFile(g.path(name), nil, 0o644); err != nil {
		fakeStateFailed(err)
	}
}

func (g fakeGH) count(name string) int {
	raw, err := os.ReadFile(g.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		fakeStateFailed(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		fakeStateFailed(err)
	}
	return n
}

func (g fakeGH) setCount(name string, n int) {
	if err := os.WriteFile(g.path(name), []byte(strconv.Itoa(n)), 0o644); err != nil {
		fakeStateFailed(err)
	}
}

func (g fakeGH) event(line string) {
	if err := appendLine(g.path("events.log"), line); err != nil {
		fakeStateFailed(err)
	}
}

func templateFlag(c ghCall) string {
	names := []string{"--jq", "-q", "--template"}
	if !c.is("pr", "merge") {
		names = append(names, "-t")
	}
	for _, n := range names {
		if _, ok := c.flags[n]; ok {
			return n
		}
	}
	return ""
}

func (g fakeGH) answer(c ghCall) (string, error) {
	switch {
	case c.is("pr", "view"):
		return g.prView(c.arg(2))
	case c.is("pr", "update-branch"):
		return g.prUpdateBranch(c.arg(2))
	case c.is("pr", "merge"):
		return g.prMerge(c)
	case c.is("run", "list"):
		return g.runList(c)
	case c.is("run", "view"):
		return g.runView(c)
	case c.is("run", "rerun"):
		return g.runRerun(c.arg(2))
	case c.is("run", "watch"):
		return g.runWatch(c)
	case c.is("repo", "view"):
		return asJSON(map[string]any{"nameWithOwner": "acme/widgets", "name": "widgets", "owner": map[string]any{"login": "acme"},
			"defaultBranchRef": map[string]any{"name": "main"}, "url": "https://github.com/acme/widgets"})
	case c.is("auth", "status"):
		return "github.com: logged in\n", nil
	case c.is("api"):
		return g.api(c.arg(1))
	}
	return "", fmt.Errorf("not modelled: gh %s", strings.Join(c.raw, " "))
}

func asJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

func (g fakeGH) headOf(n string, pr fakePR) string {
	if g.marked("updated-" + n) {
		return pr.UpdatedHead
	}
	return pr.Head
}

func (g fakeGH) prView(n string) (string, error) {
	pr, ok := g.sc.PRs[n]
	if !ok {
		return "", fmt.Errorf("no pull requests found for %q", n)
	}
	number, err := strconv.Atoi(n)
	if err != nil {
		return "", err
	}
	head := g.headOf(n, pr)
	state, mergeCommit := pr.State, any(nil)
	if g.marked("merged-"+n) && !pr.MergeLies {
		state, mergeCommit = "MERGED", map[string]any{"oid": pr.MergeSHA}
	}
	mergeState := "CLEAN"
	switch {
	case pr.IsDraft:
		mergeState = "DRAFT"
	case pr.Mergeable == "CONFLICTING":
		mergeState = "DIRTY"
	case g.sc.Behind[head] > 0:
		mergeState = "BEHIND"
	}
	return asJSON(map[string]any{
		"number": number, "state": state, "isDraft": pr.IsDraft, "mergeable": pr.Mergeable,
		"headRefOid": head, "headRefName": "feature-" + n, "baseRefName": pr.Base,
		"mergeStateStatus": mergeState, "mergeCommit": mergeCommit, "title": "fixture PR " + n,
		"url": "https://github.com/acme/widgets/pull/" + n,
	})
}

func (g fakeGH) prUpdateBranch(n string) (string, error) {
	pr, ok := g.sc.PRs[n]
	if !ok || pr.UpdatedHead == "" {
		return "", fmt.Errorf("pull request %q cannot be updated in this scenario", n)
	}
	g.mark("updated-" + n)
	g.event("update-branch " + n)
	return "", nil
}

func (g fakeGH) prMerge(c ghCall) (string, error) {
	n := c.arg(2)
	pr, ok := g.sc.PRs[n]
	if !ok {
		return "", fmt.Errorf("no pull requests found for %q", n)
	}
	head := g.headOf(n, pr)
	if want, ok := c.flag("--match-head-commit"); ok && want != head {
		return "", fmt.Errorf("head branch of pull request %s was modified: %s is not %s", n, head, want)
	}
	g.mark("merged-" + n)
	g.event("merge " + n + " " + head)
	return "", nil
}

func (g fakeGH) newestFirst() []fakeRun {
	runs := slices.Clone(g.sc.Runs)
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].CreatedAt != runs[j].CreatedAt {
			return runs[i].CreatedAt > runs[j].CreatedAt
		}
		return runs[i].ID > runs[j].ID
	})
	return runs
}

func (g fakeGH) runList(c ghCall) (string, error) {
	limit := 20
	if v, ok := c.flag("--limit", "-L"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return "", fmt.Errorf("invalid limit %q", v)
		}
		limit = n
	}
	docs := []map[string]any{}
	for _, r := range g.newestFirst() {
		keep, err := listed(c, r)
		if err != nil {
			return "", err
		}
		if !keep {
			continue
		}
		st := g.observe(r)
		if v, ok := c.flag("--status", "-s"); ok && v != st.status && v != st.conclusion {
			continue
		}
		docs = append(docs, runDoc(r, st))
		if len(docs) == limit {
			break
		}
	}
	return asJSON(docs)
}

func listed(c ghCall, r fakeRun) (bool, error) {
	if v, ok := c.flag("--commit", "-c"); ok && (len(v) < 7 || !strings.HasPrefix(r.Head, v)) {
		return false, nil
	}
	if v, ok := c.flag("--branch", "-b"); ok && r.Branch != v {
		return false, nil
	}
	if v, ok := c.flag("--event", "-e"); ok && eventOf(r) != v {
		return false, nil
	}
	if v, ok := c.flag("--created"); ok {
		return createdWithin(r.CreatedAt, v)
	}
	return true, nil
}

func createdWithin(created, filter string) (bool, error) {
	when, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return false, err
	}
	if lo, hi, ok := strings.Cut(filter, ".."); ok {
		return withinBounds(when, [][2]string{{">=", lo}, {"<=", hi}})
	}
	for _, op := range []string{"<=", ">=", "<", ">"} {
		if v, ok := strings.CutPrefix(filter, op); ok {
			return withinBounds(when, [][2]string{{op, v}})
		}
	}
	return false, fmt.Errorf("--created %q is not modelled", filter)
}

func withinBounds(when time.Time, bounds [][2]string) (bool, error) {
	for _, b := range bounds {
		if b[1] == "*" {
			continue
		}
		bound, err := parseGHDate(b[1])
		if err != nil {
			return false, err
		}
		holds := map[string]bool{"<": when.Before(bound), "<=": !when.After(bound), ">": when.After(bound), ">=": !when.Before(bound)}
		if !holds[b[0]] {
			return false, nil
		}
	}
	return true, nil
}

func parseGHDate(v string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, v); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("--created bound %q is not modelled", v)
}

func (g fakeGH) runByID(id string) (fakeRun, error) {
	for _, r := range g.sc.Runs {
		if strconv.FormatInt(r.ID, 10) == id {
			return r, nil
		}
	}
	return fakeRun{}, fmt.Errorf("could not find any workflow run with ID %s", id)
}

func (g fakeGH) runView(c ghCall) (string, error) {
	r, err := g.runByID(c.arg(2))
	if err != nil {
		return "", err
	}
	if c.bools["--log-failed"] || c.bools["--log"] {
		if r.LogFails {
			return "", errors.New("failed to get run log: HTTP 502: Server Error")
		}
		if v, ok := c.flag("--attempt"); ok && v == "1" {
			return r.Log, nil
		}
		return g.current(r, max(g.count(observedKey(r))-1, 0)).log, nil
	}
	if _, ok := c.flag("--json"); ok {
		return asJSON(runDoc(r, g.observe(r)))
	}
	return "", fmt.Errorf("not modelled: gh %s", strings.Join(c.raw, " "))
}

func (g fakeGH) runRerun(id string) (string, error) {
	r, err := g.runByID(id)
	if err != nil {
		return "", err
	}
	if r.Rerun == nil {
		return "", fmt.Errorf("run %d cannot be rerun in this scenario", r.ID)
	}
	g.mark(rerunKey(r))
	g.event("rerun " + id)
	return "", nil
}

func (g fakeGH) runWatch(c ghCall) (string, error) {
	r, err := g.runByID(c.arg(2))
	if err != nil {
		return "", err
	}
	st := g.observe(r)
	if c.bools["--exit-status"] && st.conclusion != "success" {
		return "", fmt.Errorf("run %d concluded %q", r.ID, st.conclusion)
	}
	return "", nil
}

func observedKey(r fakeRun) string { return fmt.Sprintf("observed-%d", r.ID) }
func rerunKey(r fakeRun) string    { return fmt.Sprintf("rerun-%d", r.ID) }

func (g fakeGH) current(r fakeRun, seen int) runState {
	if r.Rerun != nil && g.marked(rerunKey(r)) {
		return runState{status: "completed", conclusion: r.Rerun.Conclusion, log: r.Rerun.Log, attempt: 2}
	}
	if seen < r.PendingPolls {
		return runState{status: "in_progress", attempt: 1}
	}
	return runState{status: "completed", conclusion: r.Conclusion, log: r.Log, attempt: 1}
}

func (g fakeGH) observe(r fakeRun) runState {
	seen := g.count(observedKey(r))
	g.setCount(observedKey(r), seen+1)
	st := g.current(r, seen)
	g.event(strings.TrimSpace(fmt.Sprintf("observe %d %s %s", r.ID, st.status, st.conclusion)))
	return st
}

func runURL(id int64) string {
	return fmt.Sprintf("https://github.com/acme/widgets/actions/runs/%d", id)
}

func eventOf(r fakeRun) string {
	if r.Branch == "main" {
		return "push"
	}
	return "pull_request"
}

func runDoc(r fakeRun, st runState) map[string]any {
	return map[string]any{
		"attempt": st.attempt, "conclusion": st.conclusion, "createdAt": r.CreatedAt, "databaseId": r.ID,
		"displayTitle": "fixture", "event": eventOf(r), "headBranch": r.Branch, "headSha": r.Head,
		"jobs": jobsOf(st.log), "name": "required CI", "number": r.ID, "path": ".github/workflows/required.yml",
		"startedAt": r.CreatedAt, "status": st.status, "updatedAt": r.CreatedAt, "url": runURL(r.ID),
		"workflowName": "required CI",
	}
}

func jobsOf(log string) []map[string]any {
	jobs := []map[string]any{}
	seen := map[string]bool{}
	for _, line := range strings.Split(log, "\n") {
		name, _, ok := strings.Cut(line, "\t")
		if !ok || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		jobs = append(jobs, map[string]any{"name": name, "status": "completed", "conclusion": "failure", "databaseId": len(jobs) + 1})
	}
	return jobs
}

func (g fakeGH) api(path string) (string, error) {
	path, _, _ = strings.Cut(path, "?")
	if _, spec, ok := strings.Cut(path, "/compare/"); ok {
		return g.compare(spec)
	}
	if _, ref, ok := strings.Cut(path, "/commits/"); ok {
		return g.commit(ref)
	}
	return "", fmt.Errorf("not modelled: gh api %s", path)
}

func (g fakeGH) compare(spec string) (string, error) {
	base, head, ok := strings.Cut(spec, "...")
	if !ok {
		return "", fmt.Errorf("compare %q is not base...head", spec)
	}
	if behind, ok := g.sc.Behind[head]; ok {
		return asJSON(compareDoc(hexOf("merge-base-"+head), behind, 1, nil))
	}
	mergeBase, err := g.git("merge-base", base, head)
	if err != nil {
		return "", err
	}
	behind, err := g.revCount(head + ".." + base)
	if err != nil {
		return "", err
	}
	ahead, err := g.revCount(base + ".." + head)
	if err != nil {
		return "", err
	}
	names, err := g.git("diff", "--name-only", mergeBase, head)
	if err != nil {
		return "", err
	}
	return asJSON(compareDoc(mergeBase, behind, ahead, strings.Fields(names)))
}

func compareDoc(mergeBase string, behind, ahead int, names []string) map[string]any {
	status := map[[2]bool]string{
		{false, false}: "identical", {false, true}: "ahead", {true, false}: "behind", {true, true}: "diverged",
	}[[2]bool{behind > 0, ahead > 0}]
	files := []map[string]any{}
	for _, name := range names {
		files = append(files, map[string]any{"filename": name, "status": "modified"})
	}
	return map[string]any{
		"status": status, "behind_by": behind, "ahead_by": ahead, "total_commits": ahead,
		"merge_base_commit": map[string]any{"sha": mergeBase}, "base_commit": map[string]any{"sha": mergeBase},
		"files": files, "commits": []any{},
	}
}

func (g fakeGH) commit(ref string) (string, error) {
	line, err := g.git("rev-list", "--parents", "-n", "1", ref)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("no commit %q", ref)
	}
	parents := []map[string]any{}
	for _, p := range fields[1:] {
		parents = append(parents, map[string]any{"sha": p})
	}
	return asJSON(map[string]any{"sha": fields[0], "parents": parents, "commit": map[string]any{"message": "fixture"}})
}

func (g fakeGH) git(args ...string) (string, error) {
	if g.sc.RepoDir == "" {
		return "", fmt.Errorf("git %s: no repository in this scenario", strings.Join(args, " "))
	}
	out, err := exec.Command("git", append([]string{"-C", g.sc.RepoDir}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g fakeGH) revCount(spec string) (int, error) {
	out, err := g.git("rev-list", "--count", spec)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

type ghCall struct {
	raw   []string
	pos   []string
	flags map[string]string
	bools map[string]bool
}

var ghBoolFlags = map[string]map[string]bool{
	"pr merge": {"--merge": true, "-m": true, "--squash": true, "-s": true, "--rebase": true, "-r": true,
		"--admin": true, "--auto": true, "--disable-auto": true, "--delete-branch": true, "-d": true},
	"pr update-branch": {"--rebase": true},
	"pr view":          {"--web": true, "-w": true, "--comments": true, "-c": true},
	"run view":         {"--log-failed": true, "--log": true, "--exit-status": true, "--verbose": true, "-v": true, "--web": true, "-w": true},
	"run rerun":        {"--failed": true, "--debug": true, "-d": true},
	"run watch":        {"--exit-status": true, "--compact": true},
	"run list":         {"--all": true, "-a": true},
	"api":              {"--paginate": true, "--slurp": true, "--include": true, "-i": true, "--silent": true, "--verbose": true},
}

func ghCommandKey(args []string) string {
	if len(args) > 0 && args[0] == "api" {
		return "api"
	}
	if len(args) > 1 {
		return args[0] + " " + args[1]
	}
	return strings.Join(args, " ")
}

func parseGHCall(args []string) ghCall {
	c := ghCall{raw: args, flags: map[string]string{}, bools: map[string]bool{}}
	bools := ghBoolFlags[ghCommandKey(args)]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			c.pos = append(c.pos, a)
			continue
		}
		name, value, inline := strings.Cut(a, "=")
		if bools[name] {
			c.bools[name] = !inline || value != "false"
			continue
		}
		if !inline && i+1 < len(args) {
			i++
			value = args[i]
		}
		c.flags[name] = value
	}
	return c
}

func (c ghCall) arg(i int) string {
	if i < len(c.pos) {
		return c.pos[i]
	}
	return ""
}

func (c ghCall) is(words ...string) bool {
	return len(c.pos) >= len(words) && slices.Equal(c.pos[:len(words)], words)
}

func (c ghCall) flag(names ...string) (string, bool) {
	for _, n := range names {
		if v, ok := c.flags[n]; ok {
			return v, true
		}
	}
	return "", false
}

type ghFixture struct {
	root, state string
	env         []string
}

func newGHFixture(t *testing.T, root string, sc ghScenario) ghFixture {
	t.Helper()
	state := t.TempDir()
	raw, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(state, "scenario.json"), string(raw))
	return ghFixture{root: root, state: state, env: hermeticEnv("PATH="+toolsDir(t, true), fakeGHStateEnv+"="+state)}
}

func (fx ghFixture) evolve(t *testing.T, args ...string) result {
	t.Helper()
	return run(t, fx.root, fx.env, evolveBin(t), slices.Concat(args, []string{"--project-root", fx.root})...)
}

func (fx ghFixture) evolveBare(t *testing.T, args ...string) result {
	t.Helper()
	return run(t, fx.root, fx.env, evolveBin(t), args...)
}

func (fx ghFixture) lines(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fx.state, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func (fx ghFixture) calls(t *testing.T) []ghCall {
	t.Helper()
	var calls []ghCall
	for _, line := range fx.lines(t, "calls.log") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatalf("calls.log line %q: %v", line, err)
		}
		calls = append(calls, parseGHCall(args))
	}
	return calls
}

func (fx ghFixture) events(t *testing.T) []string { return fx.lines(t, "events.log") }

func callsTo(calls []ghCall, words ...string) []ghCall {
	var out []ghCall
	for _, c := range calls {
		if c.is(words...) {
			out = append(out, c)
		}
	}
	return out
}

func describe(calls []ghCall) string {
	if len(calls) == 0 {
		return "(no gh call)\n"
	}
	var b strings.Builder
	for _, c := range calls {
		fmt.Fprintf(&b, "  gh %s\n", strings.Join(c.raw, " "))
	}
	return b.String()
}

func newPlane(t *testing.T, policyJSON string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, ".gitignore"), ".evolve/\n")
	writeFile(t, filepath.Join(repo.Dir, "README.md"), "plane\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "plane")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if policyJSON != "" {
		writeFile(t, filepath.Join(repo.Dir, ".evolve", "policy.json"), policyJSON)
	}
	return repo
}

func writeLease(t *testing.T, runsDir, runID string, heartbeat time.Time) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"run_id": runID, "owner_pid": os.Getpid(), "heartbeat_at": heartbeat.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runsDir, runID, ".lease"), string(body)+"\n")
}

func prKey(n int) string { return strconv.Itoa(n) }

func prScenario(numbers ...int) ghScenario {
	sc := ghScenario{PRs: map[string]fakePR{}, Behind: map[string]int{}}
	for _, n := range numbers {
		head := hexOf("head-" + prKey(n))
		sc.PRs[prKey(n)] = fakePR{State: "OPEN", Mergeable: "MERGEABLE", Head: head, Base: "main", MergeSHA: hexOf("merge-" + prKey(n))}
		sc.Behind[head] = 0
		sc.Runs = append(sc.Runs, fakeRun{ID: int64(1000 + n), Head: head, Branch: "feature-" + prKey(n), CreatedAt: at(0), Conclusion: "success"})
	}
	return sc
}

func (sc *ghScenario) runOn(t *testing.T, head string) *fakeRun {
	t.Helper()
	for i := range sc.Runs {
		if sc.Runs[i].Head == head {
			return &sc.Runs[i]
		}
	}
	t.Fatalf("scenario has no run on %s", head)
	return nil
}

func (sc *ghScenario) dropRunsOn(head string) {
	sc.Runs = slices.DeleteFunc(sc.Runs, func(r fakeRun) bool { return r.Head == head })
}

func (sc *ghScenario) editPR(n int, edit func(*fakePR)) {
	pr := sc.PRs[prKey(n)]
	edit(&pr)
	sc.PRs[prKey(n)] = pr
}

func (sc *ghScenario) updatable(n int, pendingPolls int, conclusion string) string {
	updated := hexOf("updated-" + prKey(n))
	sc.editPR(n, func(pr *fakePR) { pr.UpdatedHead = updated })
	old := sc.PRs[prKey(n)].Head
	sc.Behind[old], sc.Behind[updated] = 2, 0
	sc.dropRunsOn(old)
	sc.Runs = append(sc.Runs, fakeRun{ID: int64(2000 + n), Head: updated, Branch: "feature-" + prKey(n), CreatedAt: at(time.Minute),
		Conclusion: conclusion, PendingPolls: pendingPolls, Log: widgetFailureFor(conclusion)})
	return updated
}

func widgetFailureFor(conclusion string) string {
	if conclusion == "success" {
		return ""
	}
	return goTestLog(ubuntuJob, pkgFailure{"example.com/fix/internal/widget", []string{"TestWidget"}})
}

func mergedLine(sc ghScenario, n int) string {
	return fmt.Sprintf("merged #%d %s\n", n, sc.PRs[prKey(n)].MergeSHA)
}

func requireMergeAt(t *testing.T, calls []ghCall, n int, head, method string) {
	t.Helper()
	merges := callsTo(calls, "pr", "merge", prKey(n))
	if len(merges) != 1 {
		t.Errorf("want exactly one `gh pr merge %d`, got %d:\n%s", n, len(merges), describe(calls))
		return
	}
	m := merges[0]
	if got, _ := m.flag("--match-head-commit"); got != head {
		t.Errorf("`gh pr merge %d` must bind the verified head with --match-head-commit %s, got %q", n, head, got)
	}
	used := map[string]bool{
		"--merge":  m.bools["--merge"] || m.bools["-m"],
		"--squash": m.bools["--squash"] || m.bools["-s"],
		"--rebase": m.bools["--rebase"] || m.bools["-r"],
	}
	for flag, on := range used {
		if on != (flag == method) {
			t.Errorf("`gh pr merge %d` must merge with %s and no other method; %s passed=%v", n, method, flag, on)
		}
	}
	for _, banned := range []string{"--admin", "--auto", "--delete-branch", "-d"} {
		if m.bools[banned] {
			t.Errorf("`gh pr merge %d` must never pass %s", n, banned)
		}
	}
}

func observations(events []string, runID int64) (total, pending int) {
	prefix := fmt.Sprintf("observe %d ", runID)
	for _, e := range events {
		if strings.HasPrefix(e, prefix) {
			total++
			if strings.Contains(e, "in_progress") {
				pending++
			}
		}
	}
	return total, pending
}

func TestC1792_001_PRMergeMergesAGreenPRAtItsVerifiedHeadAndPrintsTheMergeSHA(t *testing.T) {
	cases := []struct {
		name string
		args []string
		prs  []int
	}{
		{"one green PR", []string{"12"}, []int{12}},
		{"green PRs in operand order with a flag between the operands", []string{"12", "--wait", "0s", "13"}, []int{12, 13}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := prScenario(c.prs...)
			fx := newGHFixture(t, newPlane(t, "").Dir, sc)
			r := fx.evolve(t, slices.Concat([]string{"pr", "merge"}, c.args)...)
			if !knownVerb(t, r, "pr") {
				return
			}
			want := ""
			for _, n := range c.prs {
				want += mergedLine(sc, n)
			}
			if r.code != 0 || r.stdout != want {
				t.Fatalf("want exit 0 and exactly %q on stdout:\n%s\ngh calls:\n%s", want, r, describe(fx.calls(t)))
			}
			for _, n := range c.prs {
				requireMergeAt(t, fx.calls(t), n, sc.PRs[prKey(n)].Head, "--merge")
			}
		})
	}
}

func TestC1792_002_PRMergeRefusesWithExitOneAndMergesNothingFurther(t *testing.T) {
	cases := []struct {
		name     string
		arrange  func(t *testing.T, plane *gittest.Repo, sc *ghScenario)
		names    string
		atLaunch bool
	}{
		{"a live run lease in the plane", func(t *testing.T, plane *gittest.Repo, _ *ghScenario) {
			writeLease(t, filepath.Join(plane.Dir, ".evolve", "runs"), "cycle-41", time.Now())
		}, "cycle-41", true},
		{"a live run lease in a sibling worktree of the plane", func(t *testing.T, plane *gittest.Repo, _ *ghScenario) {
			sibling := filepath.Join(t.TempDir(), "sibling")
			plane.Git("worktree", "add", "-q", "-b", "sibling", sibling)
			writeLease(t, filepath.Join(sibling, ".evolve", "runs"), "cycle-42", time.Now())
		}, "cycle-42", true},
		{"a pending required run", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.runOn(t, sc.PRs["12"].Head).PendingPolls = neverCompletes
		}, "#12", false},
		{"no required run yet", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.dropRunsOn(sc.PRs["12"].Head)
		}, "#12", false},
		{"a failed required run, naming its failing job", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			failed := sc.runOn(t, sc.PRs["12"].Head)
			failed.Conclusion, failed.Log = "failure", widgetFailureFor("failure")
		}, ubuntuJob, false},
		{"a branch behind its base without --update-branch", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.Behind[sc.PRs["12"].Head] = 3
		}, "--update-branch", false},
		{"a draft PR", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.editPR(12, func(pr *fakePR) { pr.IsDraft = true })
		}, "#12", false},
		{"a closed PR", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.editPR(12, func(pr *fakePR) { pr.State = "CLOSED" })
		}, "#12", false},
		{"a conflicting PR", func(t *testing.T, _ *gittest.Repo, sc *ghScenario) {
			sc.editPR(12, func(pr *fakePR) { pr.Mergeable = "CONFLICTING" })
		}, "#12", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plane := newPlane(t, "")
			sc := prScenario(12, 13)
			c.arrange(t, plane, &sc)
			fx := newGHFixture(t, plane.Dir, sc)
			r := fx.evolve(t, "pr", "merge", "12", "13")
			if !knownVerb(t, r, "pr") {
				return
			}
			calls := fx.calls(t)
			if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, c.names) {
				t.Errorf("want exit 1, empty stdout and a refusal naming %q:\n%s", c.names, r)
			}
			if merges := callsTo(calls, "pr", "merge"); len(merges) != 0 {
				t.Errorf("a refusal must merge nothing:\n%s", describe(calls))
			}
			if later := callsTo(calls, "pr", "view", "13"); len(later) != 0 {
				t.Errorf("after refusing #12 the next PR must not be touched:\n%s", describe(calls))
			}
			if updates := callsTo(calls, "pr", "update-branch"); len(updates) != 0 {
				t.Errorf("without --update-branch no branch may be updated:\n%s", describe(calls))
			}
			if prCalls := callsTo(calls, "pr"); c.atLaunch && len(prCalls) != 0 {
				t.Errorf("a live lease must refuse before any PR is read:\n%s", describe(calls))
			}
		})
	}
}

func TestC1792_003_PRMergeStopsAtTheFirstRefusalAndKeepsEarlierMerges(t *testing.T) {
	sc := prScenario(12, 13, 14)
	failed := sc.runOn(t, sc.PRs["13"].Head)
	failed.Conclusion, failed.Log = "failure", widgetFailureFor("failure")
	fx := newGHFixture(t, newPlane(t, "").Dir, sc)
	r := fx.evolve(t, "pr", "merge", "12", "13", "14")
	if !knownVerb(t, r, "pr") {
		return
	}
	calls := fx.calls(t)
	if r.code != 1 || r.stdout != mergedLine(sc, 12) {
		t.Errorf("want exit 1 with only #12's merge line on stdout:\n%s", r)
	}
	requireMergeAt(t, calls, 12, sc.PRs["12"].Head, "--merge")
	for _, n := range []string{"13", "14"} {
		if merges := callsTo(calls, "pr", "merge", n); len(merges) != 0 {
			t.Errorf("#%s must not be merged after the refusal of #13:\n%s", n, describe(calls))
		}
	}
	if later := callsTo(calls, "pr", "view", "14"); len(later) != 0 {
		t.Errorf("#14 must not be touched after the refusal of #13:\n%s", describe(calls))
	}
	if !strings.Contains(r.stderr, "not attempted") || !strings.Contains(r.stderr, "#14") {
		t.Errorf("stderr must list #14 as not attempted:\n%s", r)
	}
}

func TestC1792_004_PRMergeIgnoresStaleLeasesAndLeasesOutsideThePlane(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, plane *gittest.Repo)
	}{
		{"a lease whose heartbeat stopped days ago", func(t *testing.T, plane *gittest.Repo) {
			writeLease(t, filepath.Join(plane.Dir, ".evolve", "runs"), "cycle-43", stampTime.AddDate(0, 0, -1))
		}},
		{"a live lease in a directory that is no worktree of the plane", func(t *testing.T, _ *gittest.Repo) {
			writeLease(t, filepath.Join(t.TempDir(), ".evolve", "runs"), "cycle-44", time.Now())
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plane := newPlane(t, "")
			c.arrange(t, plane)
			sc := prScenario(12)
			fx := newGHFixture(t, plane.Dir, sc)
			r := fx.evolve(t, "pr", "merge", "12")
			if !knownVerb(t, r, "pr") {
				return
			}
			if r.code != 0 || r.stdout != mergedLine(sc, 12) {
				t.Errorf("a lease that proves no live run in the plane must not block the merge:\n%s", r)
			}
		})
	}
}

func TestC1792_005_PRMergeUpdateBranchWaitMergesOnlyAfterTheUpdatedHeadGoesGreen(t *testing.T) {
	sc := prScenario(12)
	updated := sc.updatable(12, 2, "success")
	fx := newGHFixture(t, newPlane(t, pollEverySecond).Dir, sc)
	r := fx.evolve(t, "pr", "merge", "12", "--update-branch", "--wait", "60s")
	if !knownVerb(t, r, "pr") {
		return
	}
	calls := fx.calls(t)
	if r.code != 0 || r.stdout != mergedLine(sc, 12) {
		t.Fatalf("want the updated PR merged once its head is green:\n%s\ngh calls:\n%s", r, describe(calls))
	}
	updates := callsTo(calls, "pr", "update-branch", "12")
	if len(updates) != 1 || updates[0].bools["--rebase"] {
		t.Errorf("want one `gh pr update-branch 12` that merges the base (never --rebase):\n%s", describe(calls))
	}
	requireMergeAt(t, calls, 12, updated, "--merge")
	events := fx.events(t)
	mergeAt := slices.Index(events, "merge 12 "+updated)
	if mergeAt < 0 {
		t.Fatalf("no merge of the updated head recorded:\n%s", strings.Join(events, "\n"))
	}
	if updateAt := slices.Index(events, "update-branch 12"); updateAt < 0 || updateAt > mergeAt {
		t.Errorf("the branch must be updated before the merge:\n%s", strings.Join(events, "\n"))
	}
	if _, pending := observations(events[:mergeAt], 2012); pending != 2 {
		t.Errorf("the merge must wait through both pending observations of the updated head, saw %d:\n%s", pending, strings.Join(events, "\n"))
	}
	if last := lastObservation(events[:mergeAt], 2012); last != "observe 2012 completed success" {
		t.Errorf("the last observation of the updated head before the merge must be green, got %q", last)
	}
}

func lastObservation(events []string, runID int64) string {
	prefix := fmt.Sprintf("observe %d ", runID)
	last := ""
	for _, e := range events {
		if strings.HasPrefix(e, prefix) {
			last = e
		}
	}
	return last
}

func TestC1792_006_PRMergeRefusesAnUpdatedHeadThatIsNotGreenInTime(t *testing.T) {
	cases := []struct {
		name         string
		pendingPolls int
		conclusion   string
		flags        []string
		observations int
	}{
		{"--update-branch without --wait while the updated head is pending", neverCompletes, "success", []string{"--update-branch"}, -1},
		{"the --wait deadline passes while the updated head is still pending", neverCompletes, "success", []string{"--update-branch", "--wait", "2s"}, -1},
		{"the updated head's required run fails, refused at once despite --wait", 0, "failure", []string{"--update-branch", "--wait", "60s"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := prScenario(12)
			sc.updatable(12, c.pendingPolls, c.conclusion)
			fx := newGHFixture(t, newPlane(t, pollEverySecond).Dir, sc)
			r := fx.evolve(t, slices.Concat([]string{"pr", "merge", "12"}, c.flags)...)
			if !knownVerb(t, r, "pr") {
				return
			}
			calls := fx.calls(t)
			if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "#12") {
				t.Errorf("want exit 1 refusing #12 with nothing merged:\n%s", r)
			}
			if merges := callsTo(calls, "pr", "merge"); len(merges) != 0 {
				t.Errorf("a head that is not green must not be merged:\n%s", describe(calls))
			}
			if updates := callsTo(calls, "pr", "update-branch", "12"); len(updates) != 1 {
				t.Errorf("--update-branch must update the behind branch once:\n%s", describe(calls))
			}
			if total, _ := observations(fx.events(t), 2012); c.observations >= 0 && total != c.observations {
				t.Errorf("a red required run must refuse at once, observed %d times, want %d", total, c.observations)
			}
		})
	}
}

func TestC1792_007_PRMergeUsageErrorsExitTenWithoutCallingGH(t *testing.T) {
	usages := [][]string{
		{"pr"}, {"pr", "bogus"}, {"pr", "merge"}, {"pr", "merge", "0"}, {"pr", "merge", "abc"}, {"pr", "merge", "-3"},
		{"pr", "merge", "12", "12"}, {"pr", "merge", "12", "--wait", "-1s"}, {"pr", "merge", "12", "--wait", "soon"},
		{"pr", "merge", "12", "--bogus"},
	}
	for _, args := range usages {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			fx := newGHFixture(t, newPlane(t, "").Dir, prScenario(12))
			r := fx.evolveBare(t, args...)
			if !knownVerb(t, r, "pr") {
				return
			}
			if r.code != 10 || r.stdout != "" {
				t.Errorf("want usage exit 10 and no stdout:\n%s", r)
			}
			if calls := fx.calls(t); len(calls) != 0 {
				t.Errorf("a usage error must not call gh:\n%s", describe(calls))
			}
		})
	}
}

func TestC1792_008_PRMergeMethodComesFromPolicyAndFailsClosed(t *testing.T) {
	for _, c := range []struct{ word, flag string }{{"merge", "--merge"}, {"squash", "--squash"}, {"rebase", "--rebase"}} {
		t.Run("pr.merge_method "+c.word, func(t *testing.T) {
			sc := prScenario(12)
			fx := newGHFixture(t, newPlane(t, `{"pr":{"merge_method":"`+c.word+`"}}`).Dir, sc)
			r := fx.evolve(t, "pr", "merge", "12")
			if !knownVerb(t, r, "pr") {
				return
			}
			if r.code != 0 || r.stdout != mergedLine(sc, 12) {
				t.Fatalf("want the PR merged with the policy method:\n%s", r)
			}
			requireMergeAt(t, fx.calls(t), 12, sc.PRs["12"].Head, c.flag)
		})
	}
	for _, policyJSON := range []string{`{"pr":{"merge_method":"fast-forward"}}`, `{"pr":{"merge_method":"Squash"}}`, `{"pr":`} {
		t.Run("rejects "+policyJSON, func(t *testing.T) {
			fx := newGHFixture(t, newPlane(t, policyJSON).Dir, prScenario(12))
			r := fx.evolve(t, "pr", "merge", "12")
			if !knownVerb(t, r, "pr") {
				return
			}
			if r.code != 2 || r.stdout != "" {
				t.Errorf("an invalid or unreadable merge method must exit 2 before merging:\n%s", r)
			}
			if calls := fx.calls(t); len(calls) != 0 {
				t.Errorf("a bad policy must fail before any gh call:\n%s", describe(calls))
			}
		})
	}
}

func TestC1792_009_PRMergeGHFailuresExitTwo(t *testing.T) {
	t.Run("gh cannot reach GitHub", func(t *testing.T) {
		sc := prScenario(12)
		sc.Broken = true
		fx := newGHFixture(t, newPlane(t, "").Dir, sc)
		r := fx.evolve(t, "pr", "merge", "12")
		if !knownVerb(t, r, "pr") {
			return
		}
		if r.code != 2 || r.stdout != "" {
			t.Errorf("a gh I/O failure must exit 2 with nothing merged:\n%s", r)
		}
		if calls := fx.calls(t); len(calls) == 0 {
			t.Errorf("the verb must have reached gh before failing")
		}
	})
	t.Run("a merge gh reports done that GitHub never recorded", func(t *testing.T) {
		sc := prScenario(12)
		sc.editPR(12, func(pr *fakePR) { pr.MergeLies = true })
		fx := newGHFixture(t, newPlane(t, "").Dir, sc)
		r := fx.evolve(t, "pr", "merge", "12")
		if !knownVerb(t, r, "pr") {
			return
		}
		if r.code != 2 || strings.Contains(r.stdout, "merged #12") {
			t.Errorf("a merge whose PR is not MERGED with a merge SHA must exit 2 and print no merged line:\n%s", r)
		}
		if merges := callsTo(fx.calls(t), "pr", "merge", "12"); len(merges) != 1 {
			t.Errorf("the merge must have been attempted once:\n%s", describe(fx.calls(t)))
		}
	})
}

var backtickSpan = regexp.MustCompile("`([^`]+)`")

func documentedCommand(text string, verb ...string) []string {
	for _, m := range backtickSpan.FindAllStringSubmatch(text, -1) {
		f := strings.Fields(m[1])
		if len(f) > len(verb) && f[0] == "evolve" && slices.Equal(f[1:1+len(verb)], verb) {
			return f[1:]
		}
	}
	return nil
}

func runnableArgs(tokens []string, placeholder string, appendWhenAbsent bool) []string {
	var out []string
	square, angle, substituted := 0, 0, false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if square > 0 || strings.Contains(tok, "[") {
			square += strings.Count(tok, "[") - strings.Count(tok, "]")
			continue
		}
		if angle > 0 || strings.HasPrefix(tok, "<") {
			angle += strings.Count(tok, "<") - strings.Count(tok, ">")
			if !substituted {
				out, substituted = append(out, placeholder), true
			}
			continue
		}
		switch {
		case tok == "--project-root":
			i++
		case strings.HasPrefix(tok, "--project-root="), tok == "...":
		default:
			out = append(out, tok)
		}
	}
	if !substituted && appendWhenAbsent {
		out = append(out, placeholder)
	}
	return out
}

func boundaryMergeStep(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(readRepoFile(t, "docs/operations/runtime-reference.md"), "\n") {
		step := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))
		if strings.HasPrefix(step, "2.") && strings.Contains(step, "**Merge") {
			return line
		}
	}
	t.Fatalf("runtime-reference.md has no boundary step 2 (**Merge**)")
	return ""
}

func TestC1792_010_BoundaryMergeStepRunsAnEvolvePRMerge(t *testing.T) {
	step := boundaryMergeStep(t)
	tokens := documentedCommand(step, "pr", "merge")
	if tokens == nil {
		t.Fatalf("boundary step 2 must name an `evolve pr merge` command:\n%s", step)
	}
	args := runnableArgs(tokens, "12", true)
	t.Run("refuses while a loop runs", func(t *testing.T) {
		plane := newPlane(t, pollEverySecond)
		writeLease(t, filepath.Join(plane.Dir, ".evolve", "runs"), "cycle-45", time.Now())
		fx := newGHFixture(t, plane.Dir, prScenario(12))
		r := fx.evolve(t, args...)
		if r.code != 1 || len(callsTo(fx.calls(t), "pr", "merge")) != 0 {
			t.Errorf("the documented `evolve %s` must refuse with exit 1 while a run lease is live:\n%s", strings.Join(args, " "), r)
		}
	})
	t.Run("merges a green PR", func(t *testing.T) {
		sc := prScenario(12)
		fx := newGHFixture(t, newPlane(t, pollEverySecond).Dir, sc)
		r := fx.evolve(t, args...)
		if r.code != 0 || r.stdout != mergedLine(sc, 12) {
			t.Errorf("the documented `evolve %s` must merge a green PR and print its merge SHA:\n%s", strings.Join(args, " "), r)
		}
	})
}

type pkgFailure struct {
	pkg   string
	tests []string
}

func goTestLog(job string, failures ...pkgFailure) string {
	var b strings.Builder
	line := func(text string) { fmt.Fprintf(&b, "%s\tRun make test\t2026-10-01T12:01:00.0000000Z %s\n", job, text) }
	for _, f := range failures {
		for _, test := range f.tests {
			indent := ""
			if strings.Contains(test, "/") {
				indent = "    "
			}
			line(indent + "--- FAIL: " + test + " (0.01s)")
			line(indent + "    fixture_test.go:12: boom")
		}
		line("FAIL")
		line("FAIL\t" + f.pkg + "\t0.123s")
	}
	line("make: *** [Makefile:51: test] Error 1")
	return b.String()
}

type ciRepo struct {
	dir, base, head string
}

func newCIRepo(t *testing.T) ciRepo {
	t.Helper()
	origin := gittest.Bare(t)
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, ".gitignore"), ".evolve/\n")
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/fix\n\ngo 1.23\n")
	for _, pkg := range []string{"alpha", "gamma", "delta", "epsilon"} {
		writeFile(t, filepath.Join(repo.Dir, "go", "internal", pkg, pkg+".go"), "package "+pkg+"\n\nfunc Value() int { return 1 }\n")
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "alpha", "alpha.go"), "package alpha\n\nfunc Value() int { return 2 }\n")
	repo.Git("commit", "-q", "-am", "head")
	head := repo.Git("rev-parse", "HEAD")
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("push", "-q", "origin", "main")
	writeFile(t, filepath.Join(repo.Dir, ".evolve", "policy.json"), pollEverySecond)
	return ciRepo{dir: repo.Dir, base: base, head: head}
}

func (ci ciRepo) target(conclusion, log string) fakeRun {
	return fakeRun{ID: 501, Head: ci.head, Branch: "main", CreatedAt: at(0), Conclusion: conclusion, Log: log}
}

func (ci ciRepo) baseRun(conclusion, log string) fakeRun {
	return fakeRun{ID: 401, Head: ci.base, Branch: "main", CreatedAt: at(-time.Hour), Conclusion: conclusion, Log: log}
}

func (ci ciRepo) mixed() ghScenario {
	target := ci.target("failure", goTestLog(ubuntuJob,
		pkgFailure{alphaPkg, []string{"TestAlpha", "TestAlphaLegacy"}},
		pkgFailure{deltaPkg, []string{"TestDelta", "TestDelta/sub_case"}},
		pkgFailure{gammaPkg, []string{"TestGamma"}},
	)+goTestLog(macosJob, pkgFailure{alphaPkg, []string{"TestAlpha"}}))
	return ghScenario{RepoDir: ci.dir, Runs: []fakeRun{
		target,
		ci.baseRun("failure", goTestLog(ubuntuJob, pkgFailure{alphaPkg, []string{"TestAlphaLegacy"}})),
		{ID: 301, Head: hexOf("main-301"), Branch: "main", CreatedAt: at(-2 * time.Hour), Conclusion: "failure",
			Log: goTestLog(ubuntuJob, pkgFailure{gammaPkg, []string{"TestGamma"}})},
		{ID: 502, Head: ci.head, Branch: "main", CreatedAt: at(-10 * time.Minute), Conclusion: "failure",
			Log: goTestLog(ubuntuJob, pkgFailure{deltaPkg, []string{"TestDelta"}})},
		{ID: 701, Head: hexOf("main-701"), Branch: "main", CreatedAt: at(time.Hour), Conclusion: "failure",
			Log: goTestLog(ubuntuJob, pkgFailure{deltaPkg, []string{"TestDelta"}})},
	}}
}

func (ci ciRepo) rerunnable(rerunConclusion string) ghScenario {
	target := ci.target("failure", goTestLog(ubuntuJob, pkgFailure{epsilonPkg, []string{"TestEpsilon"}}))
	target.Rerun = &fakeRerun{Conclusion: rerunConclusion}
	if rerunConclusion != "success" {
		target.Rerun.Log = target.Log
	}
	return ghScenario{RepoDir: ci.dir, Runs: []fakeRun{target, ci.baseRun("success", "")}}
}

func (ci ciRepo) green() ghScenario {
	return ghScenario{RepoDir: ci.dir, Runs: []fakeRun{ci.target("success", ""), ci.baseRun("success", "")}}
}

func (ci ciRepo) unfinished() ghScenario {
	target := ci.target("success", "")
	target.PendingPolls = neverCompletes
	return ghScenario{RepoDir: ci.dir, Runs: []fakeRun{target, ci.baseRun("success", "")}}
}

var mixedRows = []string{
	"real\t" + alphaPkg + "\tTestAlpha\ttouched\tyes\tno\tno\tunknown",
	"pre-existing\t" + alphaPkg + "\tTestAlphaLegacy\tbase-red\tyes\tyes\tyes\tunknown",
	"unknown\t" + deltaPkg + "\tTestDelta\tdefault\tno\tno\tno\tunknown",
	"flake-evidence\t" + gammaPkg + "\tTestGamma\trecurred-on-main\tno\tno\tyes\tunknown",
}

func classify(t *testing.T, root string, sc ghScenario, args ...string) (ghFixture, result) {
	t.Helper()
	fx := newGHFixture(t, root, sc)
	return fx, fx.evolve(t, slices.Concat([]string{"ci", "classify"}, args)...)
}

type classifiedFailure struct {
	Package  string            `json:"package"`
	Test     string            `json:"test"`
	Jobs     []string          `json:"jobs"`
	Evidence map[string]string `json:"evidence"`
	Label    string            `json:"label"`
	Rule     string            `json:"rule"`
}

type classifyReport struct {
	Kind       string              `json:"kind"`
	RunID      int64               `json:"run_id"`
	Status     string              `json:"status"`
	Conclusion string              `json:"conclusion"`
	HeadSHA    string              `json:"head_sha"`
	BaseSHA    string              `json:"base_sha"`
	Failures   []classifiedFailure `json:"failures"`
	RetrySafe  *bool               `json:"retry_safe"`
}

func decodeReport(t *testing.T, r result) classifyReport {
	t.Helper()
	var rep classifyReport
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("--json must print exactly one report object: %v\n%s", err, r)
	}
	if rep.RetrySafe == nil {
		t.Fatalf("the report carries no retry_safe field:\n%s", r.stdout)
	}
	return rep
}

func (rep classifyReport) failure(t *testing.T, pkg, test string) classifiedFailure {
	t.Helper()
	for _, f := range rep.Failures {
		if f.Package == pkg && f.Test == test {
			return f
		}
	}
	t.Fatalf("the report has no failure %s %s: %+v", pkg, test, rep.Failures)
	return classifiedFailure{}
}

func (f classifiedFailure) evidence() string {
	return strings.Join([]string{f.Evidence["touched"], f.Evidence["base_red"], f.Evidence["recurred_on_main"], f.Evidence["rerun_green"]}, " ")
}

type classifyText struct {
	head, header, verdict string
	rows                  []string
}

func parseClassifyText(t *testing.T, r result) classifyText {
	t.Helper()
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("text output needs a run line, the header and a retry-safe line:\n%s", r)
	}
	rows := slices.Clone(lines[2 : len(lines)-1])
	sort.Strings(rows)
	return classifyText{head: lines[0], header: lines[1], verdict: lines[len(lines)-1], rows: rows}
}

func TestC1792_011_CIClassifyUsageErrorsExitTenWithoutCallingGH(t *testing.T) {
	usages := [][]string{
		{"ci"}, {"ci", "bogus"}, {"ci", "classify"}, {"ci", "classify", "501", "502"}, {"ci", "classify", ""},
		{"ci", "classify", "#12"}, {"ci", "classify", "run:"}, {"ci", "classify", "0"}, {"ci", "classify", "pr:abc"},
		{"ci", "classify", "pr:0"}, {"ci", "classify", "sha:xyz"}, {"ci", "classify", "abcde1"},
		{"ci", "classify", strings.Repeat("a", 41)}, {"ci", "classify", "501", "--bogus"},
	}
	ci := newCIRepo(t)
	for _, args := range usages {
		t.Run(fmt.Sprintf("%q", args), func(t *testing.T) {
			fx := newGHFixture(t, ci.dir, ci.mixed())
			r := fx.evolveBare(t, args...)
			if !knownVerb(t, r, "ci") {
				return
			}
			if r.code != 10 || r.stdout != "" {
				t.Errorf("want usage exit 10 and no stdout:\n%s", r)
			}
			if calls := fx.calls(t); len(calls) != 0 {
				t.Errorf("a usage error must not call gh:\n%s", describe(calls))
			}
		})
	}
}

func TestC1792_012_CIClassifyAcceptsEveryTargetFormAndReportsGHFailureAsExitTwo(t *testing.T) {
	fortyDigits := strings.Repeat("1234567890", 4)
	upperSHA := strings.ToUpper(hexOf("upper"))
	cases := []struct {
		target   string
		reaches  []string
		mustSkip string
	}{
		{"run:501", []string{"run", "view", "501"}, ""},
		{"501", []string{"run", "view", "501"}, ""},
		{"pr:12", []string{"pr", "view", "12"}, ""},
		{"sha:ABCDEF1", nil, "ABCDEF1"},
		{"abcdef1", nil, ""},
		{fortyDigits, nil, ""},
		{upperSHA, nil, upperSHA},
	}
	ci := newCIRepo(t)
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			sc := ci.mixed()
			sc.Broken = true
			fx, r := classify(t, ci.dir, sc, c.target)
			if !knownVerb(t, r, "ci") {
				return
			}
			calls := fx.calls(t)
			if r.code != 2 || strings.Contains(r.stdout, "retry-safe") {
				t.Errorf("a valid target whose evidence cannot be fetched must exit 2, not 10 or a verdict:\n%s\n%s", r, describe(calls))
			}
			if c.reaches != nil && len(callsTo(calls, c.reaches...)) == 0 {
				t.Errorf("target %q must be resolved as `gh %s`:\n%s", c.target, strings.Join(c.reaches, " "), describe(calls))
			}
			if c.reaches == nil && (len(callsTo(calls, "run", "view")) != 0 || len(callsTo(calls, "pr", "view")) != 0) {
				t.Errorf("target %q is a commit SHA, never a run id or a PR:\n%s", c.target, describe(calls))
			}
			if c.mustSkip != "" && strings.Contains(describe(calls), c.mustSkip) {
				t.Errorf("a SHA target must reach gh lowercased, not as %q:\n%s", c.mustSkip, describe(calls))
			}
		})
	}
}

func TestC1792_013_CIClassifyPrintsEachFailingTestWithItsEvidenceAndLabel(t *testing.T) {
	ci := newCIRepo(t)
	_, r := classify(t, ci.dir, ci.mixed(), "501")
	if !knownVerb(t, r, "ci") {
		return
	}
	if r.code != 1 {
		t.Errorf("a run with a real and an unknown failure must exit 1:\n%s", r)
	}
	out := parseClassifyText(t, r)
	for _, part := range []string{"run 501 ", runURL(501), "head=" + ci.head[:12], "base=" + ci.base[:12], "conclusion=failure"} {
		if !strings.Contains(out.head, part) {
			t.Errorf("the run line must carry %q: %q", part, out.head)
		}
	}
	if out.header != classifyHeader {
		t.Errorf("header = %q, want %q", out.header, classifyHeader)
	}
	want := slices.Clone(mixedRows)
	sort.Strings(want)
	if !slices.Equal(out.rows, want) {
		t.Errorf("one row per failing test with its evidence and label:\ngot:\n%s\nwant:\n%s", strings.Join(out.rows, "\n"), strings.Join(want, "\n"))
	}
	if out.verdict != "retry-safe: no" {
		t.Errorf("last line = %q, want %q", out.verdict, "retry-safe: no")
	}
}

func TestC1792_014_CIClassifyLabelsTouchedAsRealBaseRedAsPreExistingAndRerunGreenAsFlake(t *testing.T) {
	ci := newCIRepo(t)
	_, r := classify(t, ci.dir, ci.mixed(), "501", "--json")
	if !knownVerb(t, r, "ci") {
		return
	}
	rep := decodeReport(t, r)
	if rep.Kind != "run" || rep.RunID != 501 || rep.Status != "completed" || rep.Conclusion != "failure" || rep.HeadSHA != ci.head || rep.BaseSHA != ci.base {
		t.Errorf("report header: kind=%q run_id=%d status=%q conclusion=%q head=%q base=%q; want run 501 completed failure head=%s base=%s (the head's first parent)",
			rep.Kind, rep.RunID, rep.Status, rep.Conclusion, rep.HeadSHA, rep.BaseSHA, ci.head, ci.base)
	}
	if touched := rep.failure(t, alphaPkg, "TestAlpha"); touched.Label != "real" || touched.Evidence["touched"] != "yes" {
		t.Errorf("a failing test in a package the diff touches must be real: %+v", touched)
	}
	if touched := rep.failure(t, alphaPkg, "TestAlpha"); !slices.Equal(touched.Jobs, []string{macosJob, ubuntuJob}) {
		t.Errorf("a test failing in both matrix jobs is one failure carrying both jobs, sorted: %v", touched.Jobs)
	}
	if alsoBaseRed := rep.failure(t, alphaPkg, "TestAlphaLegacy"); alsoBaseRed.Label != "pre-existing" || alsoBaseRed.evidence() != "yes yes yes unknown" {
		t.Errorf("a touched failing test that is also red on the base must be pre-existing: %+v", alsoBaseRed)
	}
	if r.code != 1 || *rep.RetrySafe {
		t.Errorf("a real failure makes the run not retry-safe and exits 1: exit=%d retry_safe=%v", r.code, *rep.RetrySafe)
	}
	_, r = classify(t, ci.dir, ci.rerunnable("success"), "501", "--json", "--rerun")
	rep = decodeReport(t, r)
	if flake := rep.failure(t, epsilonPkg, "TestEpsilon"); flake.Label != "flake-evidence" || flake.evidence() != "no no no yes" {
		t.Errorf("an untouched failing test that is green on --rerun must be flake-evidence: %+v", flake)
	}
	if r.code != 0 || !*rep.RetrySafe {
		t.Errorf("a run whose every failure is flake-evidence is retry-safe and exits 0:\n%s", r)
	}
}

func TestC1792_015_CIClassifyEveryRuleTableRowHasItsOwnTest(t *testing.T) {
	ruleLabels := map[string]string{
		"base-red": "pre-existing", "touched": "real", "rerun-green": "flake-evidence",
		"recurred-on-main": "flake-evidence", "default": "unknown",
	}
	rows := []struct {
		rule, pkg, test, evidence string
		rerun                     bool
	}{
		{"base-red", alphaPkg, "TestAlphaLegacy", "yes yes yes unknown", false},
		{"touched", alphaPkg, "TestAlpha", "yes no no unknown", false},
		{"rerun-green", epsilonPkg, "TestEpsilon", "no no no yes", true},
		{"recurred-on-main", gammaPkg, "TestGamma", "no no yes unknown", false},
		{"default", deltaPkg, "TestDelta", "no no no unknown", false},
	}
	ci := newCIRepo(t)
	for _, row := range rows {
		t.Run(row.rule, func(t *testing.T) {
			sc, args := ci.mixed(), []string{"501", "--json"}
			if row.rerun {
				sc, args = ci.rerunnable("success"), append(args, "--rerun")
			}
			_, r := classify(t, ci.dir, sc, args...)
			if !knownVerb(t, r, "ci") {
				return
			}
			rep := decodeReport(t, r)
			got := rep.failure(t, row.pkg, row.test)
			if got.Rule != row.rule || got.Label != ruleLabels[row.rule] || got.evidence() != row.evidence {
				t.Errorf("evidence %q must match rule %q -> %q; got rule=%q label=%q evidence=%q",
					row.evidence, row.rule, ruleLabels[row.rule], got.Rule, got.Label, got.evidence())
			}
			for _, f := range rep.Failures {
				if want, known := ruleLabels[f.Rule]; !known || f.Label != want {
					t.Errorf("every label comes from the one rule table: %s %s has rule %q label %q", f.Package, f.Test, f.Rule, f.Label)
				}
			}
		})
	}
}

func TestC1792_016_CIClassifyExitCodeFollowsRetrySafety(t *testing.T) {
	ci := newCIRepo(t)
	t.Run("a green run is retry-safe with no failures", func(t *testing.T) {
		_, r := classify(t, ci.dir, ci.green(), "501", "--json")
		if !knownVerb(t, r, "ci") {
			return
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(r.stdout), &raw); err != nil {
			t.Fatalf("--json: %v\n%s", err, r)
		}
		if r.code != 0 || string(bytes.TrimSpace(raw["failures"])) != "[]" || string(raw["retry_safe"]) != "true" {
			t.Errorf("want exit 0, \"failures\": [] and retry_safe true:\n%s", r)
		}
		_, text := classify(t, ci.dir, ci.green(), "501")
		if text.code != 0 || !strings.HasSuffix(text.stdout, "retry-safe: yes\n") {
			t.Errorf("the text verdict of a green run must end with retry-safe: yes:\n%s", text)
		}
	})
	t.Run("a run still in progress exits 1", func(t *testing.T) {
		_, r := classify(t, ci.dir, ci.unfinished(), "501")
		if !knownVerb(t, r, "ci") {
			return
		}
		if r.code != 1 || strings.Contains(r.stdout, "retry-safe: yes") {
			t.Errorf("an unfinished run cannot be classified retry-safe:\n%s", r)
		}
	})
}

func TestC1792_017_CIClassifyRerunIsTheOnlySideEffectAndNeedsAGreenNewerAttempt(t *testing.T) {
	ci := newCIRepo(t)
	cases := []struct {
		name, rerun, evidence, label string
		flags                        []string
		reruns, code                 int
	}{
		{"without --rerun nothing is rerun", "success", "no no no unknown", "unknown", nil, 0, 1},
		{"--rerun reruns the failed jobs once and a green attempt is flake evidence", "success", "no no no yes", "flake-evidence", []string{"--rerun"}, 1, 0},
		{"--rerun whose newer attempt fails the same test is no flake evidence", "failure", "no no no no", "unknown", []string{"--rerun"}, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx, r := classify(t, ci.dir, ci.rerunnable(c.rerun), slices.Concat([]string{"501", "--json"}, c.flags)...)
			if !knownVerb(t, r, "ci") {
				return
			}
			rep := decodeReport(t, r)
			got := rep.failure(t, epsilonPkg, "TestEpsilon")
			if got.evidence() != c.evidence || got.Label != c.label || r.code != c.code {
				t.Errorf("want evidence %q label %q exit %d; got evidence %q label %q exit %d", c.evidence, c.label, c.code, got.evidence(), got.Label, r.code)
			}
			calls := fx.calls(t)
			reruns := callsTo(calls, "run", "rerun", "501")
			if len(reruns) != c.reruns || (c.reruns == 1 && !reruns[0].bools["--failed"]) {
				t.Errorf("want %d `gh run rerun 501 --failed`:\n%s", c.reruns, describe(calls))
			}
			if all := callsTo(calls, "run", "rerun"); len(all) != c.reruns {
				t.Errorf("no other run may be rerun:\n%s", describe(calls))
			}
		})
	}
}

func TestC1792_018_CIClassifyPropagatesEvidenceFetchFailuresAsExitTwo(t *testing.T) {
	ci := newCIRepo(t)
	for _, failing := range []int64{501, 401} {
		t.Run(fmt.Sprintf("the failed-log fetch of run %d fails", failing), func(t *testing.T) {
			sc := ci.mixed()
			for i := range sc.Runs {
				sc.Runs[i].LogFails = sc.Runs[i].ID == failing
			}
			fx, r := classify(t, ci.dir, sc, "501")
			if !knownVerb(t, r, "ci") {
				return
			}
			if r.code != 2 || strings.Contains(r.stdout, "retry-safe") {
				t.Errorf("a gh failure while gathering evidence must exit 2, never degrade to a verdict:\n%s\n%s", r, describe(fx.calls(t)))
			}
		})
	}
}

func markdownSection(t *testing.T, doc, heading string) string {
	t.Helper()
	start := strings.Index(doc, heading+"\n")
	if start < 0 {
		t.Fatalf("no %q section", heading)
	}
	section := doc[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

func TestC1792_019_CIFailuresSectionCitesARunnableCIClassify(t *testing.T) {
	section := markdownSection(t, readRepoFile(t, "CLAUDE.md"), "## CI Failures")
	tokens := documentedCommand(section, "ci", "classify")
	if tokens == nil {
		t.Fatalf("CLAUDE.md's CI Failures section must cite an `evolve ci classify` command:\n%s", section)
	}
	if !strings.Contains(readRepoFile(t, "docs/operations/runtime-reference.md"), "`evolve ci classify") {
		t.Errorf("runtime-reference.md must document `evolve ci classify`")
	}
	ci := newCIRepo(t)
	args := runnableArgs(tokens, "501", true)
	_, r := classify(t, ci.dir, ci.mixed(), args[2:]...)
	if r.code != 1 || !strings.Contains(r.stdout, "TestAlpha") {
		t.Errorf("the cited `evolve %s` must classify run 501, naming its real failure, and exit 1:\n%s", strings.Join(args, " "), r)
	}
}

func newCommentRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "x.go"), sumSource("the sum of a and b", "+"))
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "y", "y.go"), "package y\n\nfunc Y() int { return 1 }\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "fixture")
	return repo
}

func sumSource(doc, op string) string {
	return "package x\n\n// Add returns " + doc + ".\nfunc Add(a, b int) int { return a " + op + " b }\n"
}

func editX(t *testing.T, repo *gittest.Repo, doc, op string) {
	t.Helper()
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "x", "x.go"), sumSource(doc, op))
}

func bothEntryPoints(t *testing.T, dir string, args ...string) (viaEvolve, viaCommentaudit result) {
	t.Helper()
	env := hermeticEnv("PATH=" + toolsDir(t, false))
	viaEvolve = run(t, dir, env, evolveBin(t), slices.Concat([]string{"comments"}, args)...)
	viaCommentaudit = run(t, dir, env, commentauditBin(t), args...)
	return viaEvolve, viaCommentaudit
}

func TestC1792_020_CommentsVerifyReportsCommentOnlyEquivalenceLikeCommentaudit(t *testing.T) {
	repo := newCommentRepo(t)
	editX(t, repo, "a plus b", "+")
	viaEvolve, viaTool := bothEntryPoints(t, repo.Dir, "verify", "-base", "HEAD")
	if !knownVerb(t, viaEvolve, "comments") {
		return
	}
	if viaTool.code != 0 || viaTool.stdout != commentOnlyOK {
		t.Fatalf("fixture: commentaudit itself must verify the comment-only edit:\n%s", viaTool)
	}
	if viaEvolve != viaTool {
		t.Errorf("evolve comments verify must report what commentaudit verify reports:\nevolve:\n%s\ncommentaudit:\n%s", viaEvolve, viaTool)
	}
	editX(t, repo, "the sum of a and b", "-")
	viaEvolve, viaTool = bothEntryPoints(t, repo.Dir, "verify", "-base", "HEAD")
	if viaTool.code != 1 {
		t.Fatalf("fixture: commentaudit must reject a code edit:\n%s", viaTool)
	}
	if viaEvolve != viaTool || strings.Contains(viaEvolve.stdout, "comment-only:") {
		t.Errorf("a code edit must fail evolve comments verify exactly as commentaudit verify:\nevolve:\n%s\ncommentaudit:\n%s", viaEvolve, viaTool)
	}
}

func TestC1792_021_CommentsVerifyResolvesRelativeDirectories(t *testing.T) {
	repo := newCommentRepo(t)
	editX(t, repo, "a plus b", "+")
	cases := []struct {
		name, cwd, dir string
		code           int
	}{
		{"a cwd-relative dir from go/", "go", "internal/x", 0},
		{"a repo-root-relative dir from go/", "go", "go/internal/x", 0},
		{"dot from the package directory", "go/internal/x", ".", 0},
		{"a relative dir with no changed Go file", "go", "internal/y", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			viaEvolve, viaTool := bothEntryPoints(t, filepath.Join(repo.Dir, filepath.FromSlash(c.cwd)), "verify", "-base", "HEAD", c.dir)
			if !knownVerb(t, viaEvolve, "comments") {
				return
			}
			if viaTool.code != c.code {
				t.Fatalf("fixture: commentaudit verify %s from %s must exit %d:\n%s", c.dir, c.cwd, c.code, viaTool)
			}
			if viaEvolve != viaTool {
				t.Errorf("evolve comments verify %s from %s must match commentaudit:\nevolve:\n%s\ncommentaudit:\n%s", c.dir, c.cwd, viaEvolve, viaTool)
			}
		})
	}
}

func TestC1792_022_CommentsDelegatesEverySubcommandToCommentauditMain(t *testing.T) {
	repo := newCommentRepo(t)
	editX(t, repo, "a plus b", "+")
	argSets := [][]string{
		{}, {"bogus"}, {"verify"}, {"verify", "-bogus"}, {"verify", "-base", "HEAD"}, {"check", "-base", "HEAD"},
		{"comments", "-base", "HEAD"}, {"history", "-base", "HEAD"}, {"rank", "-n", "3", "."},
	}
	for _, args := range argSets {
		t.Run(fmt.Sprintf("%q", args), func(t *testing.T) {
			viaEvolve, viaTool := bothEntryPoints(t, repo.Dir, args...)
			if !knownVerb(t, viaEvolve, "comments") {
				return
			}
			if viaEvolve != viaTool {
				t.Errorf("evolve comments must run commentaudit.Main unchanged:\nevolve:\n%s\ncommentaudit:\n%s", viaEvolve, viaTool)
			}
			if len(args) == 0 || (len(args) == 1 && args[0] == "bogus") {
				if viaEvolve.code != 2 || !strings.Contains(viaEvolve.stderr, "usage: commentaudit") {
					t.Errorf("no subcommand or an unknown one is commentaudit's usage error, exit 2:\n%s", viaEvolve)
				}
			}
		})
	}
}

func TestC1792_023_CommentConventionDocumentsARunnableEvolveCommentsVerify(t *testing.T) {
	doc := readRepoFile(t, "docs/conventions/code-comments.md")
	tokens := documentedCommand(doc, "comments", "verify")
	if tokens == nil {
		t.Fatalf("docs/conventions/code-comments.md must document an `evolve comments verify` command")
	}
	if !strings.Contains(readRepoFile(t, "docs/operations/runtime-reference.md"), "`evolve comments") {
		t.Errorf("runtime-reference.md must document `evolve comments`")
	}
	args := runnableArgs(tokens, "HEAD", false)
	repo := newCommentRepo(t)
	editX(t, repo, "a plus b", "+")
	env := hermeticEnv("PATH=" + toolsDir(t, false))
	r := run(t, repo.Dir, env, evolveBin(t), args...)
	if r.code != 0 || r.stdout != commentOnlyOK {
		t.Errorf("the documented `evolve %s` must verify a comment-only edit:\n%s", strings.Join(args, " "), r)
	}
	editX(t, repo, "a plus b", "-")
	if r := run(t, repo.Dir, env, evolveBin(t), args...); r.code != 1 {
		t.Errorf("the documented `evolve %s` must reject a code edit:\n%s", strings.Join(args, " "), r)
	}
}

//go:build acs

package cycle1845

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	subprocessHangGuard = 8 * time.Minute
	hungGHSleep         = 3 * time.Minute
	fakeGHStateEnv      = "CYCLE1845_FAKE_GH_STATE"
	fakeGHCallLog       = "calls.log"
	fakeGHStateFile     = "state.json"
	requiredWorkflow    = "required.yml"
	releaseWorkflow     = "release.yml"
	fixtureTag          = "v9.9.9"
	exitUsage           = 10
	redTestName         = "TestWidgetRed"
	redRunLog           = "test (ubuntu-latest)\tRun go test\t2026-10-01T00:00:01.0000000Z --- FAIL: " + redTestName + " (0.00s)\n" +
		"test (ubuntu-latest)\tRun go test\t2026-10-01T00:00:01.0000000Z FAIL\tgithub.com/acme/widget/internal/widget\t0.01s\n"
)

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "gh" {
		os.Exit(serveFakeGH(os.Args[1:]))
	}
	code := m.Run()
	if evolveBuild.dir != "" {
		_ = os.RemoveAll(evolveBuild.dir)
	}
	os.Exit(code)
}

type builtEvolve struct {
	once     sync.Once
	dir, bin string
	err      error
}

var evolveBuild builtEvolve

func evolveBin(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		evolveBuild.dir, evolveBuild.err = os.MkdirTemp("", "evolve-acs-1845-")
		if evolveBuild.err != nil {
			return
		}
		evolveBuild.bin = filepath.Join(evolveBuild.dir, "evolve")
		cmd := exec.Command("go", "build", "-o", evolveBuild.bin, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		if out, err := cmd.CombinedOutput(); err != nil {
			evolveBuild.err = fmt.Errorf("go build ./cmd/evolve: %w\n%s", err, out)
		}
	})
	if evolveBuild.err != nil {
		t.Fatal(evolveBuild.err)
	}
	return evolveBuild.bin
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s\n--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func (r result) combined() string { return r.stdout + "\n" + r.stderr }

func runEvolve(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	return runEvolveWithin(t, subprocessHangGuard, dir, env, args...)
}

func runEvolveWithin(t *testing.T, guard time.Duration, dir string, env []string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), guard)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("evolve %v outlived the hang guard: %v\n%s\n%s", args, ctx.Err(), stdout.String(), stderr.String())
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run evolve %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func hostEnvWithout(prefixes ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		dropped := false
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				dropped = true
				break
			}
		}
		if !dropped {
			env = append(env, kv)
		}
	}
	return env
}

func pathWith(dir string) string {
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

type fakeRun struct {
	Workflow   string `json:"workflow"`
	ID         int64  `json:"id"`
	Conclusion string `json:"conclusion"`
	Pending    int    `json:"pending"`
}

type fakeGHState struct {
	Runs    []fakeRun `json:"runs"`
	HeadSHA string    `json:"headSha"`
	BaseSHA string    `json:"baseSha"`
	Branch  string    `json:"branch"`
	Event   string    `json:"event"`
	Broken  bool      `json:"broken"`
	Hang    bool      `json:"hang"`
}

type ciFixture struct {
	root, stateDir, head string
	env                  []string
}

func newCIFixture(t *testing.T, timeoutS int, gh fakeGHState) ciFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-q", "-m", "base")
	gh.BaseSHA = repo.Git("rev-parse", "HEAD")
	repo.Git("commit", "--allow-empty", "-q", "-m", "head")
	gh.HeadSHA = repo.Git("rev-parse", "HEAD")
	repo.Git("tag", fixtureTag)
	if gh.Branch == "" {
		gh.Branch, gh.Event = "main", "push"
	}
	policyBody := fmt.Sprintf(`{"ci_watch":{"enabled":true,"timeout_s":%d,"poll_s":1}}`, timeoutS)
	writeFile(t, filepath.Join(repo.Dir, ".evolve", "policy.json"), policyBody)
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	raw, err := json.Marshal(gh)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stateDir, fakeGHStateFile), string(raw))
	tools := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(tools, "gh")); err != nil {
		t.Fatal(err)
	}
	env := append(hostEnvWithout("EVOLVE_", "GH_", "GITHUB_", fakeGHStateEnv+"="),
		pathWith(tools), "EVOLVE_PROJECT_ROOT="+repo.Dir, fakeGHStateEnv+"="+stateDir)
	return ciFixture{root: repo.Dir, stateDir: stateDir, head: gh.HeadSHA, env: env}
}

func (f ciFixture) watch(t *testing.T, args ...string) result {
	t.Helper()
	return runEvolve(t, f.root, f.env, append([]string{"ci", "watch"}, args...)...)
}

func (f ciFixture) watchWithin(t *testing.T, guard time.Duration, args ...string) result {
	t.Helper()
	return runEvolveWithin(t, guard, f.root, f.env, append([]string{"ci", "watch"}, args...)...)
}

func (f ciFixture) ghCalls(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.stateDir, fakeGHCallLog))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func (f ciFixture) observations(t *testing.T, workflow string) int {
	t.Helper()
	n, err := readCount(filepath.Join(f.stateDir, "seen-"+workflow))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (f ciFixture) inboxItems(t *testing.T) []string {
	t.Helper()
	var items []string
	err := filepath.WalkDir(filepath.Join(f.root, ".evolve", "inbox"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			items = append(items, path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return items
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

func hasLineWithAll(text string, needles ...string) bool {
	for _, line := range strings.Split(text, "\n") {
		all := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func serveFakeGH(args []string) int {
	dir := os.Getenv(fakeGHStateEnv)
	if dir == "" {
		fmt.Fprintln(os.Stderr, "fake gh: "+fakeGHStateEnv+" is unset")
		return 1
	}
	if err := appendLine(filepath.Join(dir, fakeGHCallLog), strings.Join(args, " ")); err != nil {
		fmt.Fprintf(os.Stderr, "fake gh: log call: %v\n", err)
		return 1
	}
	raw, err := os.ReadFile(filepath.Join(dir, fakeGHStateFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake gh: read state: %v\n", err)
		return 1
	}
	var st fakeGHState
	if err := json.Unmarshal(raw, &st); err != nil {
		fmt.Fprintf(os.Stderr, "fake gh: parse state: %v\n", err)
		return 1
	}
	if st.Hang && hasWords(leadingWords(args), "run", "list") {
		time.Sleep(hungGHSleep)
	}
	if st.Broken {
		fmt.Fprintln(os.Stderr, "fake gh: HTTP 502: api.github.com unreachable")
		return 1
	}
	out, err := fakeGHAnswer(dir, st, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake gh: %v\n", err)
		return 1
	}
	fmt.Print(out)
	return 0
}

func fakeGHAnswer(dir string, st fakeGHState, args []string) (string, error) {
	words := leadingWords(args)
	switch {
	case hasWords(words, "run", "list"):
		return runListAnswer(dir, st, args)
	case hasWords(words, "run", "view") && len(words) > 2:
		return runViewAnswer(dir, st, words[2], args)
	case hasWords(words, "pr", "view"):
		return asJSON(map[string]any{"headRefOid": st.HeadSHA, "baseRefName": "main", "headRefName": "feature", "number": 7, "state": "OPEN"})
	case hasWords(words, "api") && len(words) > 1:
		return apiAnswer(st, words[1])
	}
	return "", fmt.Errorf("unsupported call %q", strings.Join(args, " "))
}

func leadingWords(args []string) []string {
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-R" || a == "--repo":
			i++
		case strings.HasPrefix(a, "--repo="):
		case strings.HasPrefix(a, "-"):
			return words
		default:
			words = append(words, a)
		}
	}
	return words
}

func hasWords(words []string, want ...string) bool {
	if len(words) < len(want) {
		return false
	}
	for i, w := range want {
		if words[i] != w {
			return false
		}
	}
	return true
}

func flagValue(args []string, names ...string) string {
	for i, a := range args {
		for _, n := range names {
			if a == n && i+1 < len(args) {
				return args[i+1]
			}
			if v, ok := strings.CutPrefix(a, n+"="); ok {
				return v
			}
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func sameWorkflow(asked, have string) bool {
	asked = filepath.Base(asked)
	return asked == have || asked == strings.TrimSuffix(have, filepath.Ext(have))
}

func runListAnswer(dir string, st fakeGHState, args []string) (string, error) {
	if flagValue(args, "--created") != "" || flagValue(args, "--status", "-s") == "completed" {
		return "[]\n", nil
	}
	asked := flagValue(args, "--workflow", "-w")
	docs := []map[string]any{}
	for _, r := range st.Runs {
		if asked != "" && !sameWorkflow(asked, r.Workflow) {
			continue
		}
		seen, err := bumpCount(filepath.Join(dir, "seen-"+r.Workflow))
		if err != nil {
			return "", err
		}
		docs = append(docs, runDoc(st, r, seen))
	}
	return asJSON(docs)
}

func runViewAnswer(dir string, st fakeGHState, id string, args []string) (string, error) {
	for _, r := range st.Runs {
		if strconv.FormatInt(r.ID, 10) != id {
			continue
		}
		if hasFlag(args, "--log-failed") {
			if r.Conclusion == "success" {
				return "", nil
			}
			return redRunLog, nil
		}
		seen, err := readCount(filepath.Join(dir, "seen-"+r.Workflow))
		if err != nil {
			return "", err
		}
		return asJSON(runDoc(st, r, seen))
	}
	return "", fmt.Errorf("no run %s", id)
}

func runDoc(st fakeGHState, r fakeRun, seen int) map[string]any {
	status, conclusion := "completed", r.Conclusion
	if seen <= r.Pending {
		status, conclusion = "in_progress", ""
	}
	return map[string]any{
		"status": status, "conclusion": conclusion, "databaseId": r.ID, "attempt": 1,
		"url":     fmt.Sprintf("https://github.com/acme/widget/actions/runs/%d", r.ID),
		"headSha": st.HeadSHA, "headBranch": st.Branch, "event": st.Event,
		"name": r.Workflow, "workflowName": r.Workflow, "createdAt": "2026-10-01T00:00:00Z",
	}
}

func apiAnswer(st fakeGHState, path string) (string, error) {
	switch {
	case strings.Contains(path, "/compare/"):
		return asJSON(map[string]any{"status": "ahead", "ahead_by": 1, "behind_by": 0,
			"merge_base_commit": map[string]any{"sha": st.BaseSHA},
			"files":             []map[string]any{{"filename": "go/internal/widget/widget.go"}}})
	case strings.Contains(path, "/commits/"):
		return asJSON(map[string]any{"sha": st.HeadSHA, "parents": []map[string]any{{"sha": st.BaseSHA}}})
	case strings.Contains(path, "/pulls/"):
		return asJSON(map[string]any{"number": 7, "head": map[string]any{"sha": st.HeadSHA, "ref": "feature"},
			"base": map[string]any{"ref": "main"}})
	case strings.Contains(path, "/git/ref"):
		return asJSON(map[string]any{"ref": "refs/tags/" + fixtureTag, "object": map[string]any{"sha": st.HeadSHA, "type": "commit"}})
	}
	return "", fmt.Errorf("unsupported api path %q", path)
}

func asJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func readCount(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(raw)))
}

func bumpCount(path string) (int, error) {
	n, err := readCount(path)
	if err != nil {
		return 0, err
	}
	n++
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(n)), 0o644); err != nil {
		return 0, err
	}
	return n, os.Rename(tmp, path)
}

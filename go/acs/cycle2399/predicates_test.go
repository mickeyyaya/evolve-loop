//go:build acs

package cycle2399

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		_ = os.RemoveAll(evolveBuild.dir)
	}
	os.Exit(code)
}

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func evolveBin(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2399-evolve-")
		if err != nil {
			evolveBuild.failure = err.Error()
			return
		}
		evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
		build.Dir = goDir(t)
		if out, err := build.CombinedOutput(); err != nil {
			evolveBuild.failure = fmt.Sprintf("%v\n%s", err, out)
		}
	})
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

type run struct {
	stdout, stderr string
	code           int
}

func (r run) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func evolve(t *testing.T, root string, args ...string) run {
	t.Helper()
	env := []string{"EVOLVE_PROJECT_ROOT=" + root, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = root, env, 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := run{stdout: stdout.String(), stderr: stderr.String()}
	if exit, ok := err.(*exec.ExitError); ok {
		r.code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("evolve %s: %v", strings.Join(args, " "), err)
	}
	return r
}

const (
	liveSource   = "go/internal/fleet/pool.go"
	liveTest     = "TestPool_AFreeSlotTakesTheNextTodo"
	removedFile  = "go/internal/fleet/legacy.go"
	newFile      = "go/internal/fleet/backfill.go"
	itemTemplate = `{"id":%q,"kind":"feature","priority_class":"performance","weight":0.6,"origin":"console","title":%q,` +
		`"summary":"s","fix":"f","files":[%q],"acceptance":["it is done"],"probes":%s}`
)

func fleetRepo(t *testing.T, policy string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".evolve/inbox/.gitkeep", "")
	repo.Write(liveSource, "package fleet\n")
	repo.Write("go/internal/fleet/pool_test.go", "package fleet\n\nimport \"testing\"\n\nfunc "+liveTest+"(t *testing.T) {}\n")
	if policy != "" {
		repo.Write(".evolve/policy.json", policy)
	}
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	return repo
}

func itemJSON(id, title, file, probes string) string {
	return fmt.Sprintf(itemTemplate, id, title, file, probes)
}

func addFile(t *testing.T, repo *gittest.Repo, body string) run {
	t.Helper()
	path := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return evolve(t, repo.Dir, "inbox", "add", "--file", path)
}

func filed(t *testing.T, repo *gittest.Repo, id string) bool {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repo.Dir, ".evolve", "inbox", "*-"+id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches) > 0
}

func TestC2399_001_InboxAddRefusesAnItemWhoseProbesAlreadyPass(t *testing.T) {
	for name, probes := range map[string]string{
		"a test that exists":          `[{"kind":"test-exists","arg":"` + liveTest + `"}]`,
		"a path that exists":          `[{"kind":"path-exists","arg":"` + liveSource + `"}]`,
		"a path that is already gone": `[{"kind":"path-absent","arg":"` + removedFile + `"}]`,
	} {
		repo := fleetRepo(t, "")
		id := "done-" + strings.ReplaceAll(name, " ", "-")

		r := addFile(t, repo, itemJSON(id, name, liveSource, probes))

		if r.code != 1 || !strings.Contains(r.stderr, "probe") {
			t.Errorf("%s: want exit 1 and a refusal naming the probes, got %s", name, r)
		}
		if filed(t, repo, id) {
			t.Errorf("%s: the refused item was filed", name)
		}
	}
}

func TestC2399_002_InboxAddFilesAnItemWithAProbeThatStillFails(t *testing.T) {
	repo := fleetRepo(t, "")
	probes := `[{"kind":"path-exists","arg":"` + liveSource + `"},{"kind":"path-exists","arg":"` + newFile + `"}]`

	r := addFile(t, repo, itemJSON("todo", "one probe still fails", liveSource, probes))

	if r.code != 0 || !filed(t, repo, "todo") {
		t.Errorf("an item with one failing probe must be filed, got %s", r)
	}
}

func TestC2399_003_InboxAddRefusesAProbeOfAnUnknownKind(t *testing.T) {
	repo := fleetRepo(t, "")

	r := addFile(t, repo, itemJSON("odd", "an unknown probe kind", liveSource, `[{"kind":"grep-matches","arg":"x"}]`))

	if r.code != 1 || !strings.Contains(r.stderr, "grep-matches") || filed(t, repo, "odd") {
		t.Errorf("want exit 1 naming the kind and nothing filed, got %s", r)
	}
}

type proposal struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	Trigger       string `json:"trigger"`
	Deterministic bool   `json:"deterministic"`
	Applied       bool   `json:"applied"`
}

func curate(t *testing.T, repo *gittest.Repo, args ...string) map[string]proposal {
	t.Helper()
	r := evolve(t, repo.Dir, append([]string{"inbox", "curate", "--json"}, args...)...)
	if r.code != 0 {
		t.Fatalf("inbox curate %v: %s", args, r)
	}
	var report struct {
		Proposals []proposal `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &report); err != nil {
		t.Fatalf("the curate report does not decode: %v\n%s", err, r)
	}
	byID := map[string]proposal{}
	for _, p := range report.Proposals {
		byID[p.ID] = p
	}
	return byID
}

func openInbox(t *testing.T, repo *gittest.Repo) {
	t.Helper()
	repo.Write(".evolve/inbox/2026-11-01T00-00-00Z-done.json", itemJSON("done", "the pool already has its file", liveSource, `[{"kind":"path-exists","arg":"`+liveSource+`"}]`))
	repo.Write(".evolve/inbox/2026-11-01T00-00-01Z-gone.json", itemJSON("gone", "a legacy shim to rewrite", removedFile, `[{"kind":"path-exists","arg":"`+newFile+`"}]`))
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "open items")
}

func TestC2399_004_TheBoundarySweepProposesConsumeForPassingProbesAndQuarantineForGonePaths(t *testing.T) {
	repo := fleetRepo(t, `{"inbox":{"curator":{"stage":"shadow"}}}`)
	openInbox(t, repo)

	got := curate(t, repo)

	if p := got["done"]; p.Kind != "consume" || p.Trigger != "sweep" || !p.Deterministic || p.Applied {
		t.Errorf("done = %+v, want a deterministic sweep consume that shadow does not apply", p)
	}
	if p := got["gone"]; p.Kind != "quarantine" || p.Trigger != "sweep" || p.Deterministic || p.Applied {
		t.Errorf("gone = %+v, want a judgment sweep quarantine that is never applied", p)
	}
}

func TestC2399_005_TheEnforceStageAppliesTheProbeConsumeAndLeavesTheQuarantineCandidate(t *testing.T) {
	repo := fleetRepo(t, `{"inbox":{"curator":{"stage":"enforce"}}}`)
	openInbox(t, repo)

	got := curate(t, repo, "--apply")

	if !got["done"].Applied {
		t.Errorf("done = %+v, want the deterministic consume applied in enforce", got["done"])
	}
	if got["gone"].Applied {
		t.Errorf("gone = %+v, want a judgment quarantine that enforce leaves alone", got["gone"])
	}
	inbox := filepath.Join(repo.Dir, ".evolve", "inbox")
	if _, err := os.Stat(filepath.Join(inbox, "2026-11-01T00-00-00Z-done.json")); err == nil {
		t.Errorf("done is still at the inbox root after the consume")
	}
	if _, err := os.Stat(filepath.Join(inbox, "2026-11-01T00-00-01Z-gone.json")); err != nil {
		t.Errorf("gone left the inbox root: %v", err)
	}
}

func TestC2399_010_TheSweepLeavesAnItemWithOneSurvivingDeclaredPathAlone(t *testing.T) {
	repo := fleetRepo(t, `{"inbox":{"curator":{"stage":"shadow"}}}`)
	partial := `{"id":"partial","kind":"feature","priority_class":"performance","weight":0.6,"origin":"console","title":"one path survives",` +
		`"summary":"s","fix":"f","files":["` + removedFile + `","` + liveSource + `"],"acceptance":["it is done"],"probes":[{"kind":"path-exists","arg":"` + newFile + `"}]}`
	repo.Write(".evolve/inbox/2026-11-01T00-00-02Z-partial.json", partial)
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "open partial item")

	got := curate(t, repo)

	if p, ok := got["partial"]; ok {
		t.Errorf("partial = %+v, want no proposal while one declared path still exists", p)
	}
}

func TestC2399_006_TheNamedUnitTestsPassInTheirOwnPackage(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir: goDir(t), Package: "./internal/inboxcurator",
		Pattern: "^(TestAdmit_RefusesAnItemWhoseProbesAlreadyPass|TestSweep_EveryDeclaredPathGoneIsAQuarantine)$",
		Names:   []string{"TestAdmit_RefusesAnItemWhoseProbesAlreadyPass", "TestSweep_EveryDeclaredPathGoneIsAQuarantine"},
	})
}

func coverFuncs(t *testing.T, pkg string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, pkg)
	test.Dir = goDir(t)
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test %s: %v\n%s", pkg, err, acsassert.OutputTail(string(out), 40))
	}
	cover := exec.Command("go", "tool", "cover", "-func="+profile)
	cover.Dir = goDir(t)
	out, err := cover.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, out)
	}
	return string(out)
}

func TestC2399_007_EveryFunctionOfTheCuratorPackageIsFullyCoveredAndTheNewProbeSeamsAreListed(t *testing.T) {
	funcs := coverFuncs(t, "./internal/inboxcurator")
	for _, name := range []string{"Admit", "Sweep", "CheckIntake", "Curate", "ProbesPass"} {
		pct, err := acsassert.FuncCoverage(funcs, name)
		if err != nil || pct != 100 {
			t.Errorf("%s coverage = %.1f%% (%v), want 100%%", name, pct, err)
		}
	}
	for _, line := range strings.Split(funcs, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && !strings.HasPrefix(line, "total:") && fields[len(fields)-1] != "100.0%" {
			t.Errorf("a function of the curator package is below 100%%: %s", line)
		}
	}
	batch := coverFuncs(t, "./internal/inboxbatch")
	if pct, err := acsassert.FuncCoverage(batch, "ProbeKinds"); err != nil || pct != 100 {
		t.Errorf("ProbeKinds coverage = %.1f%% (%v), want 100%%", pct, err)
	}
}

func TestC2399_008_NoCommentEntersTheTwoPackages(t *testing.T) {
	for _, dir := range []string{"internal/inboxcurator", "internal/inboxbatch"} {
		files, err := filepath.Glob(filepath.Join(goDir(t), dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			for _, group := range parsed.Comments {
				if group != parsed.Doc {
					t.Errorf("%s holds a comment that is not the package doc: %q", file, strings.TrimSpace(group.Text()))
				}
			}
		}
	}
}

// acs-predicate: config-check
func TestC2399_009_TheSpecAndBothPackageNotesStateProbesInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := map[string][]string{
		"docs/architecture/inbox-curator.md":                  {"probe", "quarantine", "ProbesPass"},
		"docs/architecture/packages/internal-inboxcurator.md": {"probe", "quarantine", "ErrProbesPass"},
		"docs/architecture/packages/internal-inboxbatch.md":   {"probe", "test-exists", "path-exists", "path-absent"},
	}
	var paths []string
	for rel, needles := range docs {
		paths = append(paths, rel)
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := strings.ToLower(string(raw))
		for _, needle := range needles {
			if !strings.Contains(text, strings.ToLower(needle)) {
				t.Errorf("%s does not state %q", rel, needle)
			}
		}
	}
	r := evolve(t, root, append([]string{"docs", "ste-lint", "--strict", "--project-root", root}, paths...)...)
	if r.code != 0 || !strings.Contains(r.stdout, "0 finding(s)") {
		t.Errorf("ste-lint --strict must report 0 findings on the three documents, got %s", r)
	}
}

// acs-predicate: config-check
func TestC2399_011_TheCuratorPackageNoteStatesSweepOnceWithTheTreeSignature(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs/architecture/packages/internal-inboxcurator.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if n := strings.Count(text, "`Sweep(open, tree, minShare)`"); n != 1 {
		t.Errorf("the package note states Sweep(open, tree, minShare) %d times, want exactly 1", n)
	}
	if strings.Contains(text, "Sweep(open, hasTest, minShare)") {
		t.Errorf("the package note still states the retired Sweep(open, hasTest, minShare) signature")
	}
}

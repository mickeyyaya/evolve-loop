//go:build acs

package cycle2127

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle2127: remove %s: %v\n", evolveBuild.dir, err)
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
	dir, err := os.MkdirTemp("", "cycle2127-evolve-")
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

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func isolatedEnv(root string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "EVOLVE_PROJECT_ROOT="+root, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func evolve(t *testing.T, root string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = root, isolatedEnv(root), 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
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

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func inboxItem(t *testing.T, root, id, created, verified string) {
	t.Helper()
	doc := map[string]any{
		"id": id, "kind": "bug", "weight": 0.5, "priority_class": "stability",
		"title": id, "summary": "s", "files": []string{"go/internal/live/live.go"}, "created_at": created,
	}
	if verified != "" {
		doc["premise_verified_at"] = verified
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, ".evolve/inbox/"+id+".json", string(raw))
}

type rankedTerm struct {
	Factor string  `json:"factor"`
	Weight float64 `json:"weight"`
}

type rankedItem struct {
	ID    string       `json:"id"`
	Score float64      `json:"score"`
	Terms []rankedTerm `json:"terms"`
}

func rankAll(t *testing.T, root string) map[string]rankedItem {
	t.Helper()
	r := evolve(t, root, "inbox", "rank", "--json", "--list", "all")
	if r.code != 0 {
		t.Fatalf("inbox rank --json: %s", r)
	}
	var env struct {
		Data struct {
			Lists []struct {
				Items []rankedItem `json:"items"`
			} `json:"lists"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("inbox rank --json is not JSON: %v\n%s", err, r)
	}
	byID := map[string]rankedItem{}
	for _, l := range env.Data.Lists {
		for _, it := range l.Items {
			byID[it.ID] = it
		}
	}
	return byID
}

func mustOutrank(t *testing.T, ranked map[string]rankedItem, winner, loser string) {
	t.Helper()
	w, wok := ranked[winner]
	l, lok := ranked[loser]
	if !wok || !lok {
		t.Fatalf("ranked ids = %v, want both %s and %s", ranked, winner, loser)
	}
	if w.Score <= l.Score {
		t.Errorf("%s scores %v, %s scores %v: %s must outscore it, not win a tie-break", winner, w.Score, loser, l.Score, winner)
	}
}

func TestC2127_001_AnUnverifiedItemOlderThanTheWindowRanksBelowAFreshOne(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	inboxItem(t, root, "a-old-unverified", now.AddDate(0, 0, -90).Format("2006-01-02"), "")
	inboxItem(t, root, "b-fresh", now.AddDate(0, 0, -1).Format("2006-01-02"), "")

	mustOutrank(t, rankAll(t, root), "b-fresh", "a-old-unverified")
}

func TestC2127_002_APremiseVerifiedInsideTheWindowKeepsAnOldItemFresh(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	filed := now.AddDate(0, 0, -90).Format("2006-01-02")
	inboxItem(t, root, "a-unverified", filed, "")
	inboxItem(t, root, "b-verified", filed, now.AddDate(0, 0, -1).Format(time.RFC3339))

	mustOutrank(t, rankAll(t, root), "b-verified", "a-unverified")
}

func TestC2127_003_TheRankNamesFreshnessAndTheOperatorPolicyKeepsLoading(t *testing.T) {
	for name, key := range map[string]string{"the freshness key": "freshness", "the legacy age key": "age"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, ".evolve/policy.json", `{"inbox_priority":{"factors":{"base":0.4,"class":0.2,"unblocks":0.2,"recurrence":0.1,"`+key+`":0.07,"goal":0.03}}}`)
			inboxItem(t, root, "only", time.Now().UTC().Format("2006-01-02"), "")

			item, ok := rankAll(t, root)["only"]

			if !ok {
				t.Fatalf("the item is not ranked under a policy naming %s", key)
			}
			var freshness *rankedTerm
			for i, term := range item.Terms {
				if term.Factor == "age" {
					t.Errorf("terms still name the retired age factor: %+v", item.Terms)
				}
				if term.Factor == "freshness" {
					freshness = &item.Terms[i]
				}
			}
			if freshness == nil || freshness.Weight != 0.07 {
				t.Errorf("freshness term = %+v, want weight 0.07 read from the policy key %s", freshness, key)
			}
		})
	}
}

func TestC2127_004_InboxLintWarnsOnAPathGoneOnMainAndStillExitsZero(t *testing.T) {
	repo := gittest.Fixture(t)
	write(t, repo.Dir, "go/internal/live/live.go", "package live\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	doc := `{"id":"cites-gone","kind":"bug","weight":0.9,"priority_class":"correctness","title":"t","summary":"s","files":["go/internal/live/live.go","go/internal/gone/gone.go"]}`
	write(t, repo.Dir, ".evolve/inbox/cites-gone.json", doc)

	r := evolve(t, repo.Dir, "inbox", "lint")

	if r.code != 0 {
		t.Errorf("a dead path must be a warning, got %s", r)
	}
	if want := "cites-gone: go/internal/gone/gone.go is gone on main"; !strings.Contains(r.stderr, want) {
		t.Errorf("stderr lacks %q:\n%s", want, r)
	}
	if strings.Contains(r.stderr, "live.go is gone") {
		t.Errorf("a path tracked on main was flagged:\n%s", r)
	}
}

func TestC2127_005_TheRankAndPolicyUnitTestsPassInTheirOwnPackages(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pkg := range []struct{ dir, run string }{
		{"./internal/inboxrank", `^(TestRank_|TestFactors_|TestScore_|TestOrder_)`},
		{"./internal/policy", `^(TestInboxPriority|TestLoad_InboxPriority)`},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", pkg.run, pkg.dir)
		cmd.Dir, cmd.WaitDelay = goDir, 5*time.Second
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Errorf("go test %s: %v\n%s", pkg.dir, err, acsassert.OutputTail(string(out), 40))
		}
	}
}

func TestC2127_006_ThePackageNotesStateTheFreshnessFactorInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	r := evolve(t, root, "docs", "ste-lint", "--strict", "--changed", "refs/remotes/origin/main", "--added", "--project-root", root)
	if r.code != 0 {
		t.Errorf("the added note text breaks ASD-STE100: %s", r)
	}
	for _, note := range []string{"internal-inboxrank.md", "internal-policy.md"} {
		path := filepath.Join(root, "docs", "architecture", "packages", note)
		if !acsassert.FileContains(t, path, "freshness") {
			t.Errorf("%s does not state the freshness factor", note)
		}
	}
}

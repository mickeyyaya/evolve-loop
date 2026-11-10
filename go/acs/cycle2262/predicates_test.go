//go:build acs

// Package cycle2262 encodes the acceptance of inbox-item-writer-keeps-the-filed-format-console.
package cycle2262

import (
	"bytes"
	"context"
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

const (
	inboxPathspec    = ".evolve/inbox"
	itemID           = "writer-format-fixture"
	idCriterion      = "evolve inbox show <id> prints the item & exits 0 when a > b"
	itemWithWeight05 = `{"id":"` + itemID + `","kind":"feature","priority_class":"debuggability","weight":0.5,` +
		`"title":"evolve inbox show prints one item","files":["go/cmd/evolve/cmd_inbox.go"],` +
		`"summary":"No verb prints one item.","fix":"Add evolve inbox show <id>.",` +
		`"acceptance":["` + idCriterion + `"]}`
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs-cycle2262-bin-")
		if err != nil {
			buildErr = err
			return
		}
		goDir, err := acsassert.ModuleDir(".")
		if err != nil {
			buildErr = err
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		bin := filepath.Join(dir, "evolve")
		build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/evolve")
		build.Dir = goDir
		if out, berr := build.CombinedOutput(); berr != nil {
			buildErr = &buildFailure{err: berr, output: string(out)}
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("build evolve: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct {
	err    error
	output string
}

func (b *buildFailure) Error() string { return b.err.Error() + "\n" + b.output }

func runEvolve(t *testing.T, root, stdin string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary(t), append(args, "--project-root", root)...)
	cmd.Dir = root
	cmd.Env = scrubbed(os.Environ())
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run evolve %v: %v", args, err)
	}
	return out.String(), exit.ExitCode()
}

func scrubbed(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			kept = append(kept, kv)
		}
	}
	return kept
}

func filedProject(t *testing.T) (repo *gittest.Repo, itemPath string) {
	t.Helper()
	repo = gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, code := runEvolve(t, repo.Dir, itemWithWeight05, "inbox", "add", "--file", "-"); code != 0 {
		t.Fatalf("inbox add exit=%d\n%s", code, out)
	}
	filed, err := filepath.Glob(filepath.Join(repo.Dir, ".evolve", "inbox", "*.json"))
	if err != nil || len(filed) != 1 {
		t.Fatalf("inbox add must file exactly one item, got %v (%v)", filed, err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "file the item")
	return repo, filed[0]
}

func TestC2262_001_WeightStampChangesOneLine(t *testing.T) {
	repo, itemPath := filedProject(t)
	filed, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if out, code := runEvolve(t, repo.Dir, "", "inbox", "edit", itemID, "--set", "weight=0.4"); code != 0 {
		t.Fatalf("inbox edit exit=%d\n%s", code, out)
	}
	edited, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(edited, []byte(`0.4`)) {
		t.Fatalf("the edit did not set the weight:\n%s", edited)
	}
	numstat := strings.TrimSpace(repo.Git("diff", "--numstat", "--", inboxPathspec))
	if !strings.HasPrefix(numstat, "1\t1\t") || strings.Count(numstat, "\n") != 0 {
		t.Errorf("one field edit must change exactly one line, git diff --numstat says %q\n%s", numstat, repo.Git("diff", "--", inboxPathspec))
	}
	want := strings.Replace(string(filed), `"weight": 0.5`, `"weight": 0.4`, 1)
	if string(edited) != want {
		t.Errorf("the rewritten item must equal the filed bytes with only the weight changed\nwant:\n%s\ngot:\n%s", want, edited)
	}
}

func TestC2262_002_CriterionWithIdPlaceholderSurvivesRewrite(t *testing.T) {
	repo, itemPath := filedProject(t)
	if out, code := runEvolve(t, repo.Dir, "", "inbox", "edit", itemID, "--set", "weight=0.4"); code != 0 {
		t.Fatalf("inbox edit exit=%d\n%s", code, out)
	}
	edited, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(edited, []byte(`"`+idCriterion+`"`)) {
		t.Errorf("the criterion %q must survive a rewrite byte for byte\n%s", idCriterion, edited)
	}
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if bytes.Contains(edited, []byte(escaped)) {
			t.Errorf("the rewrite HTML-escaped the item (%s)\n%s", escaped, edited)
		}
	}
}

func TestC2262_003_EveryEditVerbKeepsTheFiledLayout(t *testing.T) {
	repo, itemPath := filedProject(t)
	filed, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"inbox", "edit", itemID, "--set", "weight=0.4"},
		{"inbox", "edit", itemID, "--set", "weight=0.5"},
	} {
		if out, code := runEvolve(t, repo.Dir, "", args...); code != 0 {
			t.Fatalf("%v exit=%d\n%s", args, code, out)
		}
	}
	back, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, filed) {
		t.Errorf("an edit and its undo must return the filed bytes\nfiled:\n%s\nback:\n%s", filed, back)
	}
	if diff := strings.TrimSpace(repo.Git("diff", "--numstat", "--", inboxPathspec)); diff != "" {
		t.Errorf("an edit and its undo must leave no diff, got %q", diff)
	}
}

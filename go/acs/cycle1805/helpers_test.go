//go:build acs

package cycle1805

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/secretleakscan"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitClean   = 0
	exitFinding = 1
	exitGitIO   = 2
	exitUsage   = 10
	gitIOPrefix = "evolve scan secrets: git diff:"
	passPrefix  = "scan secrets: PASS"
	failPrefix  = "scan secrets: FAIL"
)

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

func TestMain(m *testing.M) {
	code := m.Run()
	for _, b := range []*builtBinary{evolveBuild, commentauditBuild} {
		if b.dir == "" {
			continue
		}
		if err := os.RemoveAll(b.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1805: remove %s: %v\n", b.dir, err)
		}
	}
	os.Exit(code)
}

func (b *builtBinary) get(t *testing.T) string {
	t.Helper()
	b.once.Do(func() { b.failure = b.build(filepath.Join(acsassert.RepoRoot(t), "go")) })
	if b.failure != "" {
		t.Fatalf("go build %s: %s", b.pkg, b.failure)
	}
	return b.path
}

func (b *builtBinary) build(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1805-"+b.name+"-")
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

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func isolatedEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "TMUX") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(append(env, "GIT_TERMINAL_PROMPT=0"), extra...)
}

func runBinary(t *testing.T, dir string, env []string, bin string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
		t.Fatalf("%s %v: %v", bin, args, err)
	}
	return r
}

func evolveIn(t *testing.T, dir string, extraEnv []string, args ...string) result {
	t.Helper()
	return runBinary(t, dir, isolatedEnv(extraEnv...), evolveBuild.get(t), args...)
}

func scan(t *testing.T, dir string, args ...string) result {
	t.Helper()
	return evolveIn(t, dir, nil, append([]string{"scan", "secrets"}, args...)...)
}

func awsKey() string { return "AKIA" + "IOSFODNN7EXAMPLE" }

func maskedAWSKey() string { return "AKIA" + strings.Repeat("*", 16) }

type secretSample struct {
	rule, line, match string
}

func everyRuleSample() []secretSample {
	pem := "-----BEGIN RSA PRIVATE" + " KEY-----"
	github := "gh" + "p_" + "0123456789abcdefghijklmnopqrstuvwxyz"
	slack := "xox" + "b-1234567890-abcdefghij"
	assign := "api" + "_key = \"" + "abcdefghijklmnopqrstuvwxyz0123456789" + "\""
	return []secretSample{
		{"pem-private-key", "const k = \"" + pem + "\"", pem},
		{"aws-access-key-id", "const id = \"" + awsKey() + "\"", awsKey()},
		{"github-token", "token = " + github, github},
		{"slack-token", "t := \"" + slack + "\"", slack},
		{"generic-private-key-assign", assign, assign},
	}
}

func maskOf(match string) string {
	runes := []rune(match)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-4)
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func committedRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	writeFile(t, r.Dir, "README.md", "fixture\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func stageAWSKey(t *testing.T, r *gittest.Repo, rel string) {
	t.Helper()
	writeFile(t, r.Dir, rel, "package config\nconst id = \""+awsKey()+"\"\n")
	r.Git("add", rel)
}

func stdoutLines(stdout string) []string {
	trimmed := strings.TrimRight(stdout, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func summaryLine(t *testing.T, r result) string {
	t.Helper()
	lines := stdoutLines(r.stdout)
	if len(lines) == 0 {
		t.Fatalf("no summary line on stdout:\n%s", r)
	}
	return lines[len(lines)-1]
}

func requireNoRawSecret(t *testing.T, r result, raws ...string) {
	t.Helper()
	for _, raw := range raws {
		if strings.Contains(r.stdout, raw) || strings.Contains(r.stderr, raw) {
			t.Errorf("the raw secret %q (masked %q) leaked to stdout or stderr:\n%s", maskOf(raw), maskOf(raw), r)
		}
	}
}

func findingLocation(f secretleakscan.Finding) (file string, line int, ok bool) {
	v := reflect.ValueOf(f)
	fv, lv := v.FieldByName("File"), v.FieldByName("Line")
	if !fv.IsValid() || !lv.IsValid() || fv.Kind() != reflect.String || lv.Kind() != reflect.Int {
		return "", 0, false
	}
	return fv.String(), int(lv.Int()), true
}

func gitState(t *testing.T, r *gittest.Repo) string {
	t.Helper()
	return strings.Join([]string{
		r.Git("rev-parse", "HEAD"),
		r.Git("ls-files", "-s"),
		r.Git("status", "--porcelain=v1", "--untracked-files=all"),
		r.Git("for-each-ref"),
		r.Git("stash", "list"),
	}, "\n---\n")
}

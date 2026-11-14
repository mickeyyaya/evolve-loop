//go:build acs

package cycle2382

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const verbFile = "cmd/evolve/cmd_inbox_trailercheck.go"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve failed (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	return bin
}

func coverFuncLines(t *testing.T, pkg string, run string) []string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-coverprofile="+profile, "-run", run, pkg)
	if err != nil || code != 0 {
		t.Fatalf("go test %s failed (exit=%d err=%v)\n%s\n%s", pkg, code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	funcs, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover failed (exit=%d err=%v)\n%s", code, err, stderr)
	}
	return strings.Split(funcs, "\n")
}

func requireFullCoverage(t *testing.T, lines []string, fileMarker string) {
	t.Helper()
	seen := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], fileMarker) {
			continue
		}
		seen++
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil || pct != 100 {
			t.Errorf("%s is not at 100%% statement coverage: %s", fileMarker, strings.Join(fields, " "))
		}
	}
	if seen == 0 {
		t.Errorf("the cover output lists no function of %s", fileMarker)
	}
}

func TestC2382_001_TheNamedCmdTestsExistAndPass(t *testing.T) {
	names := []string{
		"TestCmd_InboxTrailerCheck_RefusesARangeWithNoTrailer",
		"TestCmd_InboxTrailerCheck_RefusesAnIdThatIsNotOpen",
		"TestCmd_InboxTrailerCheck_AcceptsNone",
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "^("+strings.Join(names, "|")+")$", "-v", "./cmd/evolve")
	if err != nil || code != 0 {
		t.Errorf("the named tests are not green (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC2382_002_TheEvolveBinaryRunsTheVerbEndToEnd(t *testing.T) {
	bin := buildEvolve(t)
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	rng := base + "..HEAD"
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".evolve", "inbox", "open-item.json"), []byte(`{"id":"open-item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("commit", "--allow-empty", "-m", "feat: a change\n\nIt names no inbox item.")
	run := func() (string, int) {
		stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "inbox", "trailer-check", "--project-root", repo.Dir, "--range", rng)
		if code < 0 {
			t.Fatalf("trailer-check did not run: %v", err)
		}
		return stdout + stderr, code
	}

	out, code := run()
	if code != 0 || !strings.Contains(out, "shadow: would refuse") {
		t.Errorf("default stage: exit=%d out=%q, want exit 0 and the shadow line", code, out)
	}

	policyFile := filepath.Join(repo.Dir, ".evolve", "policy.json")
	if err := os.WriteFile(policyFile, []byte(`{"inbox":{"curator":{"trailer":{"stage":"enforce"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run(); code != 1 || !strings.Contains(out, "no Closes-Inbox") {
		t.Errorf("enforce, no trailer: exit=%d out=%q, want exit 1 and the missing trailer named", code, out)
	}

	repo.Git("commit", "--allow-empty", "-m", "fix: a change\n\nCloses-Inbox: ghost-item")
	if out, code := run(); code != 1 || !strings.Contains(out, "ghost-item is not an open inbox item") {
		t.Errorf("enforce, unknown id: exit=%d out=%q, want exit 1 and ghost-item refused", code, out)
	}

	repo.Git("reset", "-q", "--hard", base)
	repo.Git("commit", "--allow-empty", "-m", "fix: an honest change\n\nCloses-Inbox: open-item")
	if out, code := run(); code != 0 || !strings.Contains(out, "trailer-check: ok") {
		t.Errorf("enforce, open id: exit=%d out=%q, want exit 0 and the ok line", code, out)
	}

	repo.Git("commit", "--allow-empty", "-m", "chore: a change\n\nCloses-Inbox: none")
	if out, code := run(); code != 0 {
		t.Errorf("enforce, none: exit=%d out=%q, want exit 0", code, out)
	}
}

func TestC2382_003_TheTrailerStageKeyDefaultsToShadowAndRefusesOtherWords(t *testing.T) {
	for text, want := range map[string]string{
		`{}`:                                   "shadow",
		`{"inbox":{"curator":{"trailer":{}}}}`: "shadow",
		`{"inbox":{"curator":{"trailer":{"stage":"enforce"}}}}`:                  "enforce",
		`{"inbox":{"curator":{"stage":"enforce","trailer":{"stage":"shadow"}}}}`: "shadow",
	} {
		p, err := policy.Parse([]byte(text))
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := p.InboxTrailerStage(); got != want {
			t.Errorf("%s: InboxTrailerStage() = %q, want %q", text, got, want)
		}
	}
	for _, text := range []string{
		`{"inbox":{"curator":{"trailer":{"stage":"warn"}}}}`,
		`{"inbox":{"curator":{"trailer":{"stage":"ENFORCE"}}}}`,
		`{"inbox":{"curator":{"trailer":{"unknown":1}}}}`,
	} {
		if _, err := policy.Parse([]byte(text)); err == nil {
			t.Errorf("%s: want a refusal at parse", text)
		}
	}
}

func TestC2382_004_EveryFunctionOfTheVerbFileHasFullStatementCoverage(t *testing.T) {
	lines := coverFuncLines(t, "./cmd/evolve", "^TestCmd_InboxTrailerCheck")
	requireFullCoverage(t, lines, "/"+verbFile+":")
}

func TestC2382_005_EveryFunctionOfThePolicyInboxFilesHasFullStatementCoverage(t *testing.T) {
	lines := coverFuncLines(t, "./internal/policy", "")
	requireFullCoverage(t, lines, "/internal/policy/inbox_")
}

func TestC2382_006_TheChangedFilesCarryNoComment(t *testing.T) {
	root := goDir(t)
	files := []string{filepath.Join(root, verbFile)}
	policyFiles, err := filepath.Glob(filepath.Join(root, "internal", "policy", "inbox_*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range policyFiles {
		if !strings.HasSuffix(f, "_test.go") {
			files = append(files, f)
		}
	}
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		for _, group := range parsed.Comments {
			t.Errorf("%s has a comment: %q", path, strings.TrimSpace(group.Text()))
		}
	}
}

// acs-predicate: config-check
func TestC2382_007_TheSpecsAndPackageNotesStateTheChangeAndPassSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	required := map[string][]string{
		"docs/architecture/cli-contract.md":             {"inbox trailer-check"},
		"docs/architecture/policy-config.md":            {"inbox.curator.trailer", "trailer-check"},
		"docs/architecture/packages/internal-policy.md": {"inbox.curator.trailer", "InboxTrailerStage"},
		"docs/architecture/packages/cmd-evolve.md":      {"trailer-check"},
	}
	for rel, tokens := range required {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		for _, token := range tokens {
			if !strings.Contains(string(body), token) {
				t.Errorf("%s does not state %q", rel, token)
			}
		}
	}
	bin := buildEvolve(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "docs", "ste-lint", "--strict", "--changed", "refs/remotes/origin/main", "--added")
	if err != nil || code != 0 || !strings.Contains(stdout+stderr, "0 finding(s)") {
		t.Errorf("ste-lint --strict is not clean (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
}

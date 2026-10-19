//go:build acs

package cycle1976

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	selfcheckPath = "go/internal/core/phase_bindings_selfcheck.go"
	breakerPath   = "go/internal/core/blocker_breaker.go"
	coverAsk      = "TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage"
	guardAsk      = "TestC1950_002_BatchAddsNoProductionEdits"
	acceptOne     = "core-package-coverage-to-100#1"
	acceptTwo     = "core-package-coverage-to-100#2"
	inboxName     = "2026-10-09T08-30-00Z-core-package-coverage-to-100.json"
)

const selfcheckClaim = "BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:138 the json.MarshalIndent error branch needs a seam in a file this phase may not edit"

const selfcheckFailure = "./acs/cycle1946 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1946_001_CoreStatementCoverageIsFull (197.14s)\n" +
	"        github.com/mickeyyaya/evolve-loop/go/internal/core/phase_bindings_selfcheck.go:138: writeBuildSelfCheckArtifact 81.8%\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1946\t197.400s\nFAIL\n"

const conflictClaim = "BLOCKER predicate-conflict " + coverAsk + " " + guardAsk + " go/internal/core/blocker_breaker.go:76-77 001 demands the dead branch covered and 002 forbids the production edit that removes it"

const conflictFailure = "./acs/cycle1950 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: " + coverAsk + " (1.19s)\n" +
	"    predicates_test.go:74: blocker_breaker.go:AppendResolvedFingerprint is at 95.2%; want 100.0%\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1950\t1.471s\nFAIL\n"

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1976: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func repoGoDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := repoGoDir(t)
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1976-evolve-")
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
	return append(env, "EVOLVE_PROJECT_ROOT="+root)
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

type project struct {
	t    *testing.T
	root string
}

func newProject(t *testing.T) project {
	t.Helper()
	return project{t: t, root: t.TempDir()}
}

func (p project) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p project) copyFixture(fixtureRel, rel string) {
	p.t.Helper()
	dir := filepath.Join(repoGoDir(p.t), "internal", "handoff", "request", "testdata")
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(fixtureRel)))
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(rel, string(body))
}

func (p project) cycle1950() handoff.Request {
	p.t.Helper()
	p.copyFixture("cycle1950/predicates.go.fixture", "go/acs/cycle1950/predicates_test.go")
	p.write(breakerPath, "package core\n")
	req, err := request.TDDBuild(filepath.Join(p.root, "go", "acs", "cycle1950"))
	if err != nil {
		p.t.Fatal(err)
	}
	return req
}

func (p project) cycle1946() handoff.Request {
	p.t.Helper()
	p.copyFixture("cycle1946/inbox/core-package-coverage-to-100.json", ".evolve/inbox/"+inboxName)
	p.copyFixture("cycle1946/workspace/triage-decision.json", ".evolve/runs/cycle-1946/triage-decision.json")
	p.write(selfcheckPath, "package core\n")
	p.write("go/.cover-strict", "./internal/core 100\n")
	item, _, err := inboxbatch.LoadFile(filepath.Join(p.root, ".evolve", "inbox", inboxName))
	if err != nil {
		p.t.Fatal(err)
	}
	return request.TriageTDDFromItems([]inboxbatch.Item{item})
}

func (p project) ledger(cycle int, req handoff.Request, entries ...handoff.Entry) {
	p.t.Helper()
	body, err := json.Marshal(handoff.Ledger{
		SchemaVersion: handoff.SchemaVersion,
		Pair:          req.Pair,
		RequestDigest: req.Digest(),
		OutputDigest:  "sha256:output",
		Round:         1,
		Entries:       entries,
	})
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-"+strconv.Itoa(cycle)+"/handoff-"+req.Pair+".json", string(body))
}

func (p project) failures(cycle int, pair string, failures ...string) {
	p.t.Helper()
	body, err := json.Marshal(failures)
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-"+strconv.Itoa(cycle)+"/handoff-"+pair+"-failures.json", string(body))
}

func metPredicate(ask string) handoff.Entry {
	ref := "go/acs/cycle1950/predicates_test.go:" + ask
	return handoff.Entry{Ask: ask, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "predicate", Ref: ref}}}
}

func blockedEntry(ask, claim string) handoff.Entry {
	return handoff.Entry{Ask: ask, Status: handoff.StatusBlocked, Note: claim}
}

func blocked1950(t *testing.T) project {
	t.Helper()
	p := newProject(t)
	p.ledger(1950, p.cycle1950(), blockedEntry(coverAsk, conflictClaim), metPredicate(guardAsk))
	p.failures(1950, "tdd-build", conflictFailure)
	return p
}

func blocked1946(t *testing.T) project {
	t.Helper()
	p := newProject(t)
	met := handoff.Entry{Ask: acceptTwo, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: "path", Ref: "go/.cover-strict:1"}}}
	p.ledger(1946, p.cycle1946(), blockedEntry(acceptOne, selfcheckClaim), met)
	p.failures(1946, "triage-tdd", selfcheckFailure)
	return p
}

func check(t *testing.T, p project, cycle int, pair string, extra ...string) result {
	t.Helper()
	args := append([]string{"handoff", "check", "--cycle", strconv.Itoa(cycle), "--pair", pair, "--project-root", p.root}, extra...)
	return evolve(t, p.root, args...)
}

func TestC1976_001_HandoffCheckCycle1950FixturePrintsBlockedAndExits1(t *testing.T) {
	r := check(t, blocked1950(t), 1950, "tdd-build")
	if r.code != 1 {
		t.Fatalf("evolve handoff check on the cycle 1950 fixture: want exit 1\n%s", r)
	}
	for _, want := range []string{"tdd-build: blocked", coverAsk, guardAsk, breakerPath} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q\n%s", want, r)
		}
	}
}

func TestC1976_002_HandoffCheckCycle1946FixtureNamesTheProtectedPath(t *testing.T) {
	r := check(t, blocked1946(t), 1946, "triage-tdd")
	if r.code != 1 || !strings.Contains(r.stdout, "triage-tdd: blocked") || !strings.Contains(r.stdout, selfcheckPath) {
		t.Fatalf("want exit 1, a blocked line and the protected path %s\n%s", selfcheckPath, r)
	}
}

func TestC1976_003_HandoffCheckBlocksOnlyWhenTheProofHolds(t *testing.T) {
	noFailure := newProject(t)
	noFailure.ledger(1950, noFailure.cycle1950(), blockedEntry(coverAsk, conflictClaim), metPredicate(guardAsk))
	unprotected := newProject(t)
	unprotected.write("go/internal/handoff/types.go", "package handoff\n")
	claim := "BLOCKER protected-path go/internal/handoff/types.go:1 the lane cannot touch this file"
	unprotected.ledger(1950, unprotected.cycle1950(), blockedEntry(coverAsk, claim), metPredicate(guardAsk))
	unprotected.failures(1950, "tdd-build", conflictFailure)
	passing := newProject(t)
	passing.ledger(1950, passing.cycle1950(), metPredicate(coverAsk), metPredicate(guardAsk))
	for name, p := range map[string]project{"no floor failure to explain": noFailure, "an unprotected path": unprotected} {
		r := check(t, p, 1950, "tdd-build")
		if r.code != 1 || strings.Contains(r.stdout, "tdd-build: blocked") || !strings.Contains(r.stdout, "tdd-build: retry") {
			t.Errorf("%s: want exit 1 and retry, not blocked\n%s", name, r)
		}
	}
	if r := check(t, passing, 1950, "tdd-build"); r.code != 0 || !strings.Contains(r.stdout, "tdd-build: pass") {
		t.Errorf("a ledger that meets every ask: want exit 0 and pass\n%s", r)
	}
}

type jsonResult struct {
	Cycle   int      `json:"cycle"`
	Outcome string   `json:"outcome"`
	Exit    int      `json:"exit"`
	Blocked []string `json:"blocked"`
	Paths   []string `json:"paths"`
	Errors  []string `json:"errors"`
}

func decodeOne(t *testing.T, r result) jsonResult {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(r.stdout))
	var got jsonResult
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, r)
	}
	if dec.More() {
		t.Fatalf("stdout holds more than one JSON value\n%s", r)
	}
	if got.Exit != r.code {
		t.Errorf("json exit %d differs from process exit %d", got.Exit, r.code)
	}
	return got
}

func TestC1976_004_HandoffCheckJSONPrintsOneObjectOnEveryExit(t *testing.T) {
	pass := newProject(t)
	pass.ledger(1950, pass.cycle1950(), metPredicate(coverAsk), metPredicate(guardAsk))
	absent := newProject(t)
	absent.cycle1950()
	tests := []struct {
		name    string
		run     func() result
		code    int
		outcome string
	}{
		{"pass", func() result { return check(t, pass, 1950, "tdd-build", "--json") }, 0, "pass"},
		{"blocked", func() result { return check(t, blocked1950(t), 1950, "tdd-build", "--json") }, 1, "blocked"},
		{"io error", func() result { return check(t, absent, 1950, "tdd-build", "--json") }, 2, "error"},
		{"usage", func() result { return check(t, absent, 1950, "bogus", "--json") }, 10, "usage"},
	}
	for _, tc := range tests {
		r := tc.run()
		got := decodeOne(t, r)
		if r.code != tc.code || got.Outcome != tc.outcome {
			t.Errorf("%s: want exit %d outcome %s, got %d %s\n%s", tc.name, tc.code, tc.outcome, r.code, got.Outcome, r)
		}
	}
	got := decodeOne(t, check(t, blocked1946(t), 1946, "triage-tdd", "--json"))
	if len(got.Blocked) != 1 || len(got.Paths) != 1 || got.Paths[0] != selfcheckPath {
		t.Errorf("cycle 1946 json: blocked=%v paths=%v, want one ask and %s", got.Blocked, got.Paths, selfcheckPath)
	}
}

func TestC1976_005_HandoffShowListsTheResultOfEachPair(t *testing.T) {
	p := blocked1950(t)
	r := evolve(t, p.root, "handoff", "show", "--cycle", "1950", "--project-root", p.root)
	if r.code != 0 || !strings.Contains(r.stdout, "triage-tdd: absent") || !strings.Contains(r.stdout, "tdd-build: blocked") {
		t.Fatalf("handoff show: want exit 0, an absent triage-tdd and a blocked tdd-build\n%s", r)
	}
	missing := evolve(t, p.root, "handoff", "show", "--cycle", "7", "--project-root", p.root)
	if missing.code != 2 {
		t.Errorf("handoff show on a cycle with no run directory: want exit 2\n%s", missing)
	}
}

func TestC1976_006_HandoffVerbsAreInTheHelpTextAndTheRuntimeReference(t *testing.T) {
	root := t.TempDir()
	help := evolve(t, root, "help")
	for _, want := range []string{"handoff check --cycle N --pair", "handoff show --cycle N"} {
		if help.code != 0 || !strings.Contains(help.stdout, want) {
			t.Errorf("evolve help lacks %q\n%s", want, help)
		}
	}
	if r := evolve(t, root, "handoff", "bogus"); r.code != 10 || !strings.Contains(r.stderr, "usage: evolve handoff") {
		t.Errorf("an unknown handoff verb: want exit 10 and a usage line\n%s", r)
	}
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	for _, want := range []string{"evolve handoff check", "evolve handoff show"} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("runtime-reference.md lacks %q", want)
		}
	}
}

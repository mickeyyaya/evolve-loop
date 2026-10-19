package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	hcSelfcheckPath = "go/internal/core/phase_bindings_selfcheck.go"
	hcBreakerPath   = "go/internal/core/blocker_breaker.go"
	hcCoverAsk      = "TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage"
	hcGuardAsk      = "TestC1950_002_BatchAddsNoProductionEdits"
	hcAcceptOne     = "core-package-coverage-to-100#1"
	hcAcceptTwo     = "core-package-coverage-to-100#2"
	hcInboxName     = "2026-10-09T08-30-00Z-core-package-coverage-to-100.json"
)

const hcSelfcheckClaim = "BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:138 the json.MarshalIndent error branch needs a seam in a file this phase may not edit"

const hcSelfcheckFailure = "./acs/cycle1946 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1946_001_CoreStatementCoverageIsFull (197.14s)\n" +
	"    predicates_test.go:52: 1 function(s) of ./internal/core below 100.0%:\n" +
	"        github.com/mickeyyaya/evolve-loop/go/internal/core/phase_bindings_selfcheck.go:138: writeBuildSelfCheckArtifact 81.8%\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1946\t197.400s\nFAIL\n"

const hcConflictClaim = "BLOCKER predicate-conflict " + hcCoverAsk + " " + hcGuardAsk + " go/internal/core/blocker_breaker.go:76-77 001 demands the dead branch covered and 002 forbids the production edit that removes it"

const hcConflictFailure = "./acs/cycle1950 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: " + hcCoverAsk + " (1.19s)\n" +
	"    predicates_test.go:74: blocker_breaker.go:AppendResolvedFingerprint is at 95.2% under the ^TestCoreCoverageBatch_ batch tests; want 100.0% (error branches included)\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1950\t1.471s\nFAIL\n"

var hcFixtureDir = filepath.Join("..", "..", "internal", "handoff", "request", "testdata")

type hcProject struct {
	t    *testing.T
	root string
}

func newHCProject(t *testing.T) hcProject {
	t.Helper()
	return hcProject{t: t, root: t.TempDir()}
}

func (p hcProject) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p hcProject) copyFixture(fixtureRel, rel string) {
	p.t.Helper()
	body, err := os.ReadFile(filepath.Join(hcFixtureDir, filepath.FromSlash(fixtureRel)))
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(rel, string(body))
}

func (p hcProject) runDir(cycle int) string {
	return filepath.Join(p.root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
}

func (p hcProject) cycle1950() handoff.Request {
	p.t.Helper()
	p.copyFixture("cycle1950/predicates.go.fixture", "go/acs/cycle1950/predicates_test.go")
	p.write(hcBreakerPath, "package core\n")
	req, err := request.TDDBuild(filepath.Join(p.root, "go", "acs", "cycle1950"))
	if err != nil {
		p.t.Fatal(err)
	}
	return req
}

func (p hcProject) cycle1946() handoff.Request {
	p.t.Helper()
	p.copyFixture("cycle1946/inbox/core-package-coverage-to-100.json", ".evolve/inbox/"+hcInboxName)
	p.copyFixture("cycle1946/workspace/triage-decision.json", ".evolve/runs/cycle-1946/triage-decision.json")
	p.write(hcSelfcheckPath, "package core\n")
	p.write("go/.cover-strict", "./internal/core 100\n")
	item, _, err := inboxbatch.LoadFile(filepath.Join(p.root, ".evolve", "inbox", hcInboxName))
	if err != nil {
		p.t.Fatal(err)
	}
	return request.TriageTDDFromItems([]inboxbatch.Item{item})
}

func (p hcProject) ledger(cycle int, req handoff.Request, entries ...handoff.Entry) {
	p.t.Helper()
	p.ledgerWithDigest(cycle, req, req.Digest(), entries...)
}

func (p hcProject) ledgerWithDigest(cycle int, req handoff.Request, digest string, entries ...handoff.Entry) {
	p.t.Helper()
	body, err := json.Marshal(handoff.Ledger{
		SchemaVersion: handoff.SchemaVersion,
		Pair:          req.Pair,
		RequestDigest: digest,
		OutputDigest:  "sha256:output",
		Round:         1,
		Entries:       entries,
	})
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-"+strconv.Itoa(cycle)+"/handoff-"+req.Pair+".json", string(body))
}

func (p hcProject) failures(cycle int, pair string, failures ...string) {
	p.t.Helper()
	body, err := json.Marshal(failures)
	if err != nil {
		p.t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-"+strconv.Itoa(cycle)+"/handoff-"+pair+"-failures.json", string(body))
}

func hcMet(ask, kind, ref string) handoff.Entry {
	return handoff.Entry{Ask: ask, Status: handoff.StatusMet, Evidence: []handoff.Evidence{{Kind: kind, Ref: ref}}}
}

func hcMetPredicate(ask string) handoff.Entry {
	return hcMet(ask, "predicate", "go/acs/cycle1950/predicates_test.go:"+ask)
}

func hcUnmet(ask string) handoff.Entry {
	return handoff.Entry{Ask: ask, Status: handoff.StatusUnmet, Note: "not done"}
}

func hcBlocked(ask, claim string) handoff.Entry {
	return handoff.Entry{Ask: ask, Status: handoff.StatusBlocked, Note: claim}
}

func runHC(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	cmd := lookupCommand("handoff")
	if cmd == nil {
		t.Fatal("handoff is not in the CLI registry")
	}
	var stdout, stderr bytes.Buffer
	rc := cmd.Run(args, nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

type hcResult struct {
	Cycle   int      `json:"cycle"`
	Pair    string   `json:"pair"`
	Outcome string   `json:"outcome"`
	Exit    int      `json:"exit"`
	Unmet   []string `json:"unmet"`
	Blocked []string `json:"blocked"`
	Paths   []string `json:"paths"`
	Errors  []string `json:"errors"`
}

type hcPairResult struct {
	Pair    string `json:"pair"`
	Outcome string `json:"outcome"`
}

type hcShowResult struct {
	Cycle   int            `json:"cycle"`
	Outcome string         `json:"outcome"`
	Exit    int            `json:"exit"`
	Pairs   []hcPairResult `json:"pairs"`
	Errors  []string       `json:"errors"`
}

func decodeOneObject(t *testing.T, stdout string, into any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	if err := dec.Decode(into); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%q", err, stdout)
	}
	if dec.More() {
		t.Fatalf("stdout holds more than one JSON value: %q", stdout)
	}
}

func checkArgs(p hcProject, cycle int, pair string, extra ...string) []string {
	return append([]string{"check", "--cycle", strconv.Itoa(cycle), "--pair", pair, "--project-root", p.root}, extra...)
}

func blockedProject1950(t *testing.T) hcProject {
	t.Helper()
	p := newHCProject(t)
	req := p.cycle1950()
	p.ledger(1950, req, hcBlocked(hcCoverAsk, hcConflictClaim), hcMetPredicate(hcGuardAsk))
	p.failures(1950, "tdd-build", hcConflictFailure)
	return p
}

func blockedProject1946(t *testing.T) hcProject {
	t.Helper()
	p := newHCProject(t)
	req := p.cycle1946()
	p.ledger(1946, req, hcBlocked(hcAcceptOne, hcSelfcheckClaim), hcMet(hcAcceptTwo, "path", "go/.cover-strict:1"))
	p.failures(1946, "triage-tdd", hcSelfcheckFailure)
	return p
}

func TestHandoffCheck_Cycle1950PrintsBlockedAndExits1(t *testing.T) {
	p := blockedProject1950(t)
	rc, stdout, stderr := runHC(t, checkArgs(p, 1950, "tdd-build")...)
	if rc != 1 {
		t.Fatalf("exit = %d, want 1\nstdout: %s\nstderr: %s", rc, stdout, stderr)
	}
	for _, want := range []string{"tdd-build: blocked", hcCoverAsk, hcGuardAsk, hcBreakerPath} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestHandoffCheck_Cycle1946PrintsBlockedWithTheProtectedPath(t *testing.T) {
	p := blockedProject1946(t)
	rc, stdout, stderr := runHC(t, checkArgs(p, 1946, "triage-tdd")...)
	if rc != 1 {
		t.Fatalf("exit = %d, want 1\nstdout: %s\nstderr: %s", rc, stdout, stderr)
	}
	for _, want := range []string{"triage-tdd: blocked", hcAcceptOne, hcSelfcheckPath} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, hcAcceptTwo) {
		t.Errorf("the met ask %s must not be listed as blocked:\n%s", hcAcceptTwo, stdout)
	}
}

func TestHandoffCheck_JSONNamesTheBlockedAsksAndPaths(t *testing.T) {
	tests := []struct {
		name    string
		project func(*testing.T) hcProject
		cycle   int
		pair    string
		blocked []string
		paths   []string
	}{
		{"cycle 1950 conflict", blockedProject1950, 1950, "tdd-build", []string{hcCoverAsk, hcGuardAsk}, []string{hcBreakerPath}},
		{"cycle 1946 protected path", blockedProject1946, 1946, "triage-tdd", []string{hcAcceptOne}, []string{hcSelfcheckPath}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.project(t)
			rc, stdout, _ := runHC(t, checkArgs(p, tc.cycle, tc.pair, "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != 1 || got.Exit != 1 || got.Outcome != "blocked" {
				t.Fatalf("rc=%d result=%+v, want exit 1 and outcome blocked", rc, got)
			}
			if got.Cycle != tc.cycle || got.Pair != tc.pair {
				t.Errorf("cycle/pair = %d/%q, want %d/%q", got.Cycle, got.Pair, tc.cycle, tc.pair)
			}
			if !reflect.DeepEqual(got.Blocked, tc.blocked) || !reflect.DeepEqual(got.Paths, tc.paths) {
				t.Errorf("blocked=%v paths=%v, want %v %v", got.Blocked, got.Paths, tc.blocked, tc.paths)
			}
		})
	}
}

func TestHandoffCheck_PassExitsZero(t *testing.T) {
	p := newHCProject(t)
	req := p.cycle1950()
	p.ledger(1950, req, hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
	rc, stdout, stderr := runHC(t, checkArgs(p, 1950, "tdd-build")...)
	if rc != 0 || !strings.Contains(stdout, "tdd-build: pass") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want exit 0 and a pass line", rc, stdout, stderr)
	}
}

func TestHandoffCheck_UnmetAsksNameTheIdsToRetry(t *testing.T) {
	tests := map[string]func(p hcProject, req handoff.Request){
		"a required ask is unmet": func(p hcProject, req handoff.Request) {
			p.ledger(1950, req, hcUnmet(hcCoverAsk), hcMetPredicate(hcGuardAsk))
		},
		"a blocked claim has no floor failure to explain": func(p hcProject, req handoff.Request) {
			p.ledger(1950, req, hcBlocked(hcCoverAsk, hcConflictClaim), hcMetPredicate(hcGuardAsk))
		},
		"a blocked claim names a path the lane may edit": func(p hcProject, req handoff.Request) {
			claim := "BLOCKER protected-path go/internal/handoff/types.go:1 the lane cannot touch this file"
			p.write("go/internal/handoff/types.go", "package handoff\n")
			p.ledger(1950, req, hcBlocked(hcCoverAsk, claim), hcMetPredicate(hcGuardAsk))
			p.failures(1950, "tdd-build", hcConflictFailure)
		},
		"a met ask cites a predicate no file declares": func(p hcProject, req handoff.Request) {
			ghost := hcMet(hcCoverAsk, "predicate", "go/acs/cycle1950/predicates_test.go:TestC1950_999_Ghost")
			p.ledger(1950, req, ghost, hcMetPredicate(hcGuardAsk))
		},
		"a met ask cites no evidence": func(p hcProject, req handoff.Request) {
			p.ledger(1950, req, handoff.Entry{Ask: hcCoverAsk, Status: handoff.StatusMet}, hcMetPredicate(hcGuardAsk))
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			p := newHCProject(t)
			setup(p, p.cycle1950())
			rc, stdout, _ := runHC(t, checkArgs(p, 1950, "tdd-build", "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != 1 || got.Outcome != "retry" {
				t.Fatalf("rc=%d result=%+v, want exit 1 and outcome retry", rc, got)
			}
			if !reflect.DeepEqual(got.Unmet, []string{hcCoverAsk}) || len(got.Blocked) != 0 || len(got.Paths) != 0 {
				t.Errorf("unmet=%v blocked=%v paths=%v, want only %s unmet", got.Unmet, got.Blocked, got.Paths, hcCoverAsk)
			}
		})
	}
}

func TestHandoffCheck_AnInvalidLedgerIsNamedAndExits1(t *testing.T) {
	tests := map[string]struct {
		setup func(p hcProject, req handoff.Request)
		want  string
	}{
		"a stale request digest": {func(p hcProject, req handoff.Request) {
			p.ledgerWithDigest(1950, req, "sha256:stale", hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
		}, "stale request digest"},
		"an ask the request does not hold": {func(p hcProject, req handoff.Request) {
			p.ledger(1950, req, hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk), hcMetPredicate("TestC1950_777_Ghost"))
		}, "unknown ask"},
		"an ask with no entry": {func(p hcProject, req handoff.Request) {
			p.ledger(1950, req, hcMetPredicate(hcCoverAsk))
		}, "missing entry"},
		"a ledger that is not JSON": {func(p hcProject, req handoff.Request) {
			p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", "{not json")
		}, "invalid character"},
		"a schema version other than 1": {func(p hcProject, req handoff.Request) {
			p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", `{"schema_version":2,"pair":"tdd-build"}`)
		}, "schema version must be 1"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := newHCProject(t)
			tc.setup(p, p.cycle1950())
			rc, stdout, _ := runHC(t, checkArgs(p, 1950, "tdd-build", "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != 1 || got.Outcome != "invalid" {
				t.Fatalf("rc=%d result=%+v, want exit 1 and outcome invalid", rc, got)
			}
			if !strings.Contains(strings.Join(got.Errors, "\n"), tc.want) {
				t.Errorf("errors = %v, want one containing %q", got.Errors, tc.want)
			}
		})
	}
}

func TestHandoffCheck_AnUnreadableInputExits2(t *testing.T) {
	tests := map[string]struct {
		setup func(p hcProject)
		pair  string
	}{
		"the ledger file is absent": {func(p hcProject) { p.cycle1950() }, "tdd-build"},
		"the cycle has no predicate package": {func(p hcProject) {
			p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", "{}")
		}, "tdd-build"},
		"the cycle has no triage decision": {func(p hcProject) {
			p.write(".evolve/runs/cycle-1950/handoff-triage-tdd.json", "{}")
		}, "triage-tdd"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := newHCProject(t)
			tc.setup(p)
			rc, stdout, stderr := runHC(t, checkArgs(p, 1950, tc.pair)...)
			if rc != 2 {
				t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", rc, stdout, stderr)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("an IO error must say why on stderr")
			}
			rc, stdout, _ = runHC(t, checkArgs(p, 1950, tc.pair, "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != 2 || got.Exit != 2 || got.Outcome != "error" || len(got.Errors) == 0 {
				t.Errorf("rc=%d result=%+v, want exit 2, outcome error and an error text", rc, got)
			}
		})
	}
}

var hcUsageCases = map[string][]string{
	"no verb":                       {},
	"an unknown verb":               {"bogus"},
	"check without flags":           {"check"},
	"check without a pair":          {"check", "--cycle", "1950"},
	"check without a cycle":         {"check", "--pair", "tdd-build"},
	"a cycle of zero":               {"check", "--cycle", "0", "--pair", "tdd-build"},
	"a negative cycle":              {"check", "--cycle", "-3", "--pair", "tdd-build"},
	"a cycle that is not a number":  {"check", "--cycle", "abc", "--pair", "tdd-build"},
	"an unknown pair":               {"check", "--cycle", "1950", "--pair", "bogus"},
	"a pair without a request":      {"check", "--cycle", "1950", "--pair", "scout-triage"},
	"a pair without a request too":  {"check", "--cycle", "1950", "--pair", "build-audit"},
	"a stray argument":              {"check", "--cycle", "1950", "--pair", "tdd-build", "extra"},
	"an unknown flag":               {"check", "--cycle", "1950", "--pair", "tdd-build", "--nope"},
	"show without a cycle":          {"show"},
	"show with a pair":              {"show", "--cycle", "1950", "--pair", "tdd-build"},
	"show with a stray argument":    {"show", "--cycle", "1950", "extra"},
	"show with a cycle that is bad": {"show", "--cycle", "x"},
}

func TestHandoff_UsageErrorsExit10(t *testing.T) {
	for name, args := range hcUsageCases {
		t.Run(name, func(t *testing.T) {
			rc, stdout, stderr := runHC(t, args...)
			if rc != 10 {
				t.Fatalf("exit = %d, want 10\nstdout: %s\nstderr: %s", rc, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("a usage error must leave stdout empty without --json, got %q", stdout)
			}
			if !strings.Contains(stderr, "usage: evolve handoff") {
				t.Errorf("stderr lacks the usage line: %q", stderr)
			}
		})
	}
}

func TestHandoff_JSONPrintsOneObjectOnAUsageError(t *testing.T) {
	for name, args := range hcUsageCases {
		t.Run(name, func(t *testing.T) {
			rc, stdout, _ := runHC(t, append(append([]string(nil), args...), "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != 10 || got.Exit != 10 || got.Outcome != "usage" || len(got.Errors) == 0 {
				t.Errorf("rc=%d result=%+v, want exit 10, outcome usage and an error text", rc, got)
			}
		})
	}
}

func TestHandoffCheck_JSONExitFieldMatchesTheExitCodeOnEveryExit(t *testing.T) {
	pass := newHCProject(t)
	pass.ledger(1950, pass.cycle1950(), hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
	absent := newHCProject(t)
	absent.cycle1950()
	tests := []struct {
		name    string
		project hcProject
		wantRC  int
		outcome string
	}{
		{"pass", pass, 0, "pass"},
		{"blocked", blockedProject1950(t), 1, "blocked"},
		{"error", absent, 2, "error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, stdout, _ := runHC(t, checkArgs(tc.project, 1950, "tdd-build", "--json")...)
			var got hcResult
			decodeOneObject(t, stdout, &got)
			if rc != tc.wantRC || got.Exit != rc || got.Outcome != tc.outcome {
				t.Errorf("rc=%d result=%+v, want exit %d outcome %s", rc, got, tc.wantRC, tc.outcome)
			}
		})
	}
}

func TestHandoffCheck_DefaultsTheProjectRootFromTheEnvironment(t *testing.T) {
	p := blockedProject1950(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", p.root)
	rc, stdout, stderr := runHC(t, "check", "--cycle", "1950", "--pair", "tdd-build")
	if rc != 1 || !strings.Contains(stdout, "tdd-build: blocked") {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want exit 1 and a blocked line", rc, stdout, stderr)
	}
}

func hcSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, rerr := os.ReadFile(path)
		snap[path] = string(body)
		return rerr
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestHandoff_VerbsWriteNothingAndRepeatTheSameAnswer(t *testing.T) {
	p := blockedProject1950(t)
	before := hcSnapshot(t, p.root)
	rc1, out1, _ := runHC(t, checkArgs(p, 1950, "tdd-build", "--json")...)
	rc2, out2, _ := runHC(t, checkArgs(p, 1950, "tdd-build", "--json")...)
	runHC(t, "show", "--cycle", "1950", "--project-root", p.root)
	if rc1 != rc2 || out1 != out2 {
		t.Errorf("two runs differ: %d %q vs %d %q", rc1, out1, rc2, out2)
	}
	if after := hcSnapshot(t, p.root); !reflect.DeepEqual(before, after) {
		t.Errorf("the verbs changed the project tree")
	}
}

func TestHandoffShow_ListsTheResultOfEachPair(t *testing.T) {
	p := blockedProject1950(t)
	rc, stdout, stderr := runHC(t, "show", "--cycle", "1950", "--project-root", p.root)
	if rc != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", rc, stdout, stderr)
	}
	for _, want := range []string{"triage-tdd: absent", "tdd-build: blocked"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Index(stdout, "triage-tdd") > strings.Index(stdout, "tdd-build:") {
		t.Errorf("pairs must list in pipeline order, triage-tdd first:\n%s", stdout)
	}
}

func TestHandoffShow_JSONListsEveryPairInPipelineOrder(t *testing.T) {
	p := blockedProject1950(t)
	rc, stdout, _ := runHC(t, "show", "--cycle", "1950", "--project-root", p.root, "--json")
	var got hcShowResult
	decodeOneObject(t, stdout, &got)
	want := []hcPairResult{{"triage-tdd", "absent"}, {"tdd-build", "blocked"}}
	if rc != 0 || got.Exit != 0 || got.Cycle != 1950 || !reflect.DeepEqual(got.Pairs, want) {
		t.Errorf("rc=%d result=%+v, want exit 0 and pairs %v", rc, got, want)
	}
}

func TestHandoffShow_ReportsAPairWhoseRequestCannotBeBuilt(t *testing.T) {
	p := newHCProject(t)
	p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", "{}")
	rc, stdout, _ := runHC(t, "show", "--cycle", "1950", "--project-root", p.root, "--json")
	var got hcShowResult
	decodeOneObject(t, stdout, &got)
	want := []hcPairResult{{"triage-tdd", "absent"}, {"tdd-build", "error"}}
	if rc != 0 || !reflect.DeepEqual(got.Pairs, want) {
		t.Errorf("rc=%d pairs=%v, want exit 0 and %v", rc, got.Pairs, want)
	}
}

func TestHandoffShow_AMissingRunDirectoryExits2(t *testing.T) {
	p := newHCProject(t)
	rc, stdout, stderr := runHC(t, "show", "--cycle", "1950", "--project-root", p.root)
	if rc != 2 || strings.TrimSpace(stderr) == "" {
		t.Fatalf("rc=%d stdout=%q stderr=%q, want exit 2 with a reason on stderr", rc, stdout, stderr)
	}
	rc, stdout, _ = runHC(t, "show", "--cycle", "1950", "--project-root", p.root, "--json")
	var got hcShowResult
	decodeOneObject(t, stdout, &got)
	if rc != 2 || got.Exit != 2 || got.Outcome != "error" || len(got.Errors) == 0 {
		t.Errorf("rc=%d result=%+v, want exit 2, outcome error and an error text", rc, got)
	}
}

func TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference(t *testing.T) {
	cmd := lookupCommand("handoff")
	if cmd == nil {
		t.Fatal("handoff is not in the CLI registry")
	}
	for _, want := range []string{"check", "show"} {
		if !strings.Contains(cmd.Summary, want) {
			t.Errorf("registry summary lacks %q: %s", want, cmd.Summary)
		}
	}
	for _, want := range []string{"handoff check --cycle N --pair", "handoff show --cycle N"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage text lacks %q", want)
		}
	}
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"evolve handoff check", "evolve handoff show", "handoff-<pair>.json", "handoff-<pair>-failures.json"} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("runtime-reference.md lacks %q", want)
		}
	}
}

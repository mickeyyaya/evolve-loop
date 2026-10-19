//go:build acs

package cycle1969

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	packageNote = "docs/architecture/packages/internal-handoff.md"
	enforceList = "go/.apicover-enforce"
	exitPkg     = "go/internal/handoff/exit"
)

var request = handoff.Request{
	Pair: "build-audit",
	Asks: []handoff.Ask{
		{ID: "a1", Required: true},
		{ID: "a2", Required: true},
		{ID: "a3", Required: true},
		{ID: "o1", Required: false},
	},
}

func met(ask string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusMet, Resolved: true}
}

func unmet(ask string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusUnmet, Resolved: true}
}

func requireOutcome(t *testing.T, label string, got exit.Decision, want exit.Outcome) {
	t.Helper()
	if got.Outcome != want {
		t.Errorf("RED: %s = %q (unmet %v), want %q", label, got.Outcome, got.Unmet, want)
	}
}

func TestC1969_001_RoundThatMeetsNoNewAskCapsBeforeTheRoundLimit(t *testing.T) {
	roomyPolicy := exit.Policy{MaxRedispatch: 5}
	history := []exit.Round{{Met: []string{"a1"}}}
	requireOutcome(t, "no new ask met",
		exit.Decide(request, []exit.Result{met("a1"), unmet("a2"), unmet("a3")}, history, roomyPolicy), exit.Cap)
	requireOutcome(t, "new ask met in the same state",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), unmet("a3")}, history, roomyPolicy), exit.Retry)
	requireOutcome(t, "ask met two rounds ago is not new",
		exit.Decide(request, []exit.Result{met("a2"), unmet("a1"), unmet("a3")}, []exit.Round{{Met: []string{"a1"}}, {Met: []string{"a2"}}}, roomyPolicy), exit.Cap)
	requireOutcome(t, "first round that meets nothing has no earlier round to compare",
		exit.Decide(request, []exit.Result{unmet("a1"), unmet("a2"), unmet("a3")}, nil, roomyPolicy), exit.Retry)
}

func TestC1969_002_RetryCarriesExactlyTheUnmetRequiredAskIds(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 3}
	got := exit.Decide(request, []exit.Result{unmet("a3"), met("a2"), unmet("a1"), unmet("o1")}, nil, policy)
	requireOutcome(t, "two required asks unmet", got, exit.Retry)
	if want := []string{"a1", "a3"}; !reflect.DeepEqual(got.Unmet, want) {
		t.Errorf("RED: Unmet = %v, want %v (request order, no optional ask, no met ask)", got.Unmet, want)
	}
	missing := exit.Decide(request, []exit.Result{met("a1"), met("a2")}, nil, policy)
	if want := []string{"a3"}; !reflect.DeepEqual(missing.Unmet, want) {
		t.Errorf("RED: ask with no result: Unmet = %v, want %v", missing.Unmet, want)
	}
}

func TestC1969_003_PassNeedsEveryRequiredAskMetWithResolvedEvidence(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 3}
	allMet := []exit.Result{met("a1"), met("a2"), met("a3")}
	got := exit.Decide(request, allMet, nil, policy)
	requireOutcome(t, "all required met", got, exit.Pass)
	if len(got.Unmet) != 0 {
		t.Errorf("RED: pass carries Unmet %v", got.Unmet)
	}
	cases := map[string]exit.Result{
		"unresolved evidence": {Ask: "a3", Status: handoff.StatusMet},
		"blocked status":      {Ask: "a3", Status: handoff.StatusBlocked, Resolved: true},
		"not applicable":      {Ask: "a3", Status: handoff.StatusNotApplicable, Resolved: true},
		"unmet status":        unmet("a3"),
	}
	for label, bad := range cases {
		results := []exit.Result{met("a1"), met("a2"), bad}
		requireOutcome(t, "a3 with "+label, exit.Decide(request, results, nil, policy), exit.Retry)
	}
	requireOutcome(t, "met and unmet result for one ask",
		exit.Decide(request, append(append([]exit.Result(nil), allMet...), unmet("a3")), nil, policy), exit.Retry)
	requireOutcome(t, "required ask with no result",
		exit.Decide(request, []exit.Result{met("a1"), met("a2")}, nil, policy), exit.Retry)
}

func TestC1969_004_OptionalUnmetAskDoesNotBlockPass(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 3}
	requireOutcome(t, "optional unmet",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), met("a3"), unmet("o1")}, nil, policy), exit.Pass)
	requireOutcome(t, "optional without a result",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), met("a3")}, nil, policy), exit.Pass)
	optionalOnly := handoff.Request{Asks: []handoff.Ask{{ID: "o1"}}}
	requireOutcome(t, "request of optional asks only",
		exit.Decide(optionalOnly, []exit.Result{unmet("o1")}, nil, policy), exit.Pass)
}

func TestC1969_005_SpentRedispatchCapCapsAndPassStillWins(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 2}
	twoRounds := []exit.Round{{Met: []string{"a1"}}, {Met: []string{"a1", "a2"}}}
	requireOutcome(t, "cap spent, required ask still unmet, round gained",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), met("o1"), unmet("a3")}, twoRounds, policy), exit.Cap)
	requireOutcome(t, "one redispatch left",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), unmet("a3")}, twoRounds[:1], policy), exit.Retry)
	requireOutcome(t, "cap spent but every required ask met",
		exit.Decide(request, []exit.Result{met("a1"), met("a2"), met("a3")}, twoRounds, policy), exit.Pass)
	requireOutcome(t, "zero cap",
		exit.Decide(request, []exit.Result{met("a1"), unmet("a2"), unmet("a3")}, nil, exit.Policy{}), exit.Cap)
}

func TestC1969_006_DecideLeavesItsInputsUntouched(t *testing.T) {
	results := []exit.Result{unmet("a3"), met("a2"), unmet("a1")}
	history := []exit.Round{{Met: []string{"a2"}}}
	wantResults := append([]exit.Result(nil), results...)
	exit.Decide(request, results, history, exit.Policy{MaxRedispatch: 3})
	if !reflect.DeepEqual(results, wantResults) {
		t.Errorf("RED: results mutated: %v", results)
	}
	if !reflect.DeepEqual(history, []exit.Round{{Met: []string{"a2"}}}) {
		t.Errorf("RED: history mutated: %v", history)
	}
}

// acs-predicate: config-check
func TestC1969_007_PackageNoteDocumentsTheExits(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, filepath.FromSlash(packageNote))
	if !acsassert.FileExists(t, note) {
		t.Fatalf("RED: %s missing", packageNote)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", packageNote); code != 0 {
		t.Errorf("RED: %s is not tracked", packageNote)
	}
	body, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"`internal/handoff/exit`", "`Decide`", "`pass`", "`retry`", "`cap`"} {
		if !strings.Contains(string(body), word) {
			t.Errorf("RED: %s does not name %s", packageNote, word)
		}
	}
}

// acs-predicate: config-check
func TestC1969_008_NewPackageGraduatesIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	list, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(enforceList)))
	if err != nil {
		t.Fatal(err)
	}
	enrolled := false
	for _, line := range strings.Split(string(list), "\n") {
		if strings.TrimSpace(line) == "./internal/handoff/exit" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Errorf("RED: ./internal/handoff/exit is not a line of %s", enforceList)
	}
	named := filepath.Join(root, filepath.FromSlash(exitPkg), "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Fatalf("RED: %s missing", named)
	}
	body, err := os.ReadFile(named)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"Decide", "Decision", "Outcome", "Pass", "Retry", "Cap", "Policy", "Result", "Round"} {
		if !strings.Contains(string(body), "exit."+symbol) {
			t.Errorf("RED: apicover_named_test.go does not name exit.%s", symbol)
		}
	}
}

// acs-predicate: config-check
func TestC1969_009_PackageCarriesNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, filepath.FromSlash(exitPkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: %s: %v", exitPkg, err)
	}
	production := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			production++
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.Contains(line, " // ") {
				t.Errorf("RED: %s:%d carries a comment: %q", entry.Name(), i+1, line)
			}
		}
	}
	if production == 0 {
		t.Errorf("RED: %s holds no production Go file", exitPkg)
	}
}

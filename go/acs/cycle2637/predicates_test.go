//go:build acs

package cycle2637

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var callers = codeindex.Request{Query: codeindex.QueryCallers, Arg: "run", Direction: codeindex.DirectionIn}

func settingsOf(t *testing.T, body string) (policy.CodeIndexSettings, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := policy.Load(path)
	if err != nil {
		return policy.CodeIndexSettings{}, err
	}
	return loaded.CodeIndexConfig()
}

func absentGotypesPresentTrail(opts ...codeindex.Option) (codeindex.Backends, error) {
	gotypes := codeindextest.NewAbsentFake("gotypes", codeindex.Capabilities{Callers: true, API: true})
	trail := codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callers: {{File: "a.go", Line: 1, Symbol: "run"}}})
	return codeindex.New([]string{"gotypes", "trail"},
		map[string]codeindex.Factory{
			"gotypes": func() (codeindex.Backend, error) { return gotypes, nil },
			"trail":   func() (codeindex.Backend, error) { return trail, nil },
		}, opts...)
}

func TestC2637_001_PolicyKeyOnAbsentDefaultsToRefuseAcceptsNextAndRejectsAnyOtherWord(t *testing.T) {
	def, err := settingsOf(t, `{}`)
	if err != nil || def.OnAbsent != "refuse" {
		t.Fatalf("default OnAbsent = %q, err = %v, want refuse", def.OnAbsent, err)
	}
	next, err := settingsOf(t, `{"code_index":{"on_absent":"next"}}`)
	if err != nil || next.OnAbsent != "next" {
		t.Errorf("on_absent next: OnAbsent = %q, err = %v", next.OnAbsent, err)
	}
	for _, word := range []string{"failover", "", "Next", "retry"} {
		if _, err := settingsOf(t, `{"code_index":{"on_absent":"`+word+`"}}`); !errors.Is(err, policy.ErrCodeIndexOnAbsent) {
			t.Errorf("on_absent %q: err = %v, want policy.ErrCodeIndexOnAbsent", word, err)
		}
	}
}

func TestC2637_002_FallThroughRoutesAnAbsentBackendToTheNextCapableOneAndNamesIt(t *testing.T) {
	set, err := absentGotypesPresentTrail(codeindex.WithAbsentFallThrough())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	hits, served, err := set.SearchServed(context.Background(), callers)

	if err != nil || served != "trail" || len(hits) != 1 {
		t.Errorf("SearchServed = %v, %q, %v; want the hit of trail", hits, served, err)
	}
}

func TestC2637_003_WithoutTheOptionAnAbsentBackendStillRefusesAndNotFoundNeverFallsThrough(t *testing.T) {
	refusing, err := absentGotypesPresentTrail()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, served, err := refusing.SearchServed(context.Background(), callers)
	if !errors.Is(err, codeindex.ErrAbsent) || served != "gotypes" {
		t.Errorf("refuse mode: served = %q, err = %v, want ErrAbsent from gotypes", served, err)
	}

	missing := codeindextest.NewFake("gotypes", codeindex.Capabilities{Callers: true}, nil)
	trail := codeindextest.NewFake("trail", codeindex.AllCapabilities(), map[codeindex.Request][]codeindex.Hit{callers: {{File: "a.go", Line: 1}}})
	notFound, err := codeindex.New([]string{"gotypes", "trail"},
		map[string]codeindex.Factory{
			"gotypes": func() (codeindex.Backend, error) { return missing, nil },
			"trail":   func() (codeindex.Backend, error) { return trail, nil },
		}, codeindex.WithAbsentFallThrough())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, served, err := notFound.SearchServed(context.Background(), callers); !errors.Is(err, codeindex.ErrNotFound) || served != "gotypes" {
		t.Errorf("not found under next: served = %q, err = %v, want ErrNotFound from gotypes", served, err)
	}
}

func TestC2637_004_TheCommandReachesTheRuleFromThePolicyFileThroughBuildCodeIndex(t *testing.T) {
	names := []string{
		"TestCodeOnAbsentWiring_TheNextPolicyRoutesAnAbsentGotypesIndexToTrail",
		"TestCodeOnAbsentWiring_TheDefaultPolicyRefusesAnAbsentGotypesIndex",
		"TestCodeOnAbsentWiring_TheRefusePolicyWrittenOutRefusesLikeTheDefault",
		"TestCodeOnAbsentWiring_AnUnknownModeFailsTheBuildNamingTheKey",
		"TestCodeOnAbsentWiring_TheCommandAnswersFromTrailAndLogsTheBackendThatAnswered",
		"TestCodeOnAbsentWiring_TheDefaultCommandExitsTwoNamingGotypes",
		"TestCodeOnAbsentWiring_AStaleIndexUnderRefuseStillExitsTwentyOneWhenOnAbsentIsNext",
	}
	out, errOut, code, err := acsassert.SubprocessOutputIn(acsassert.GoModuleDir(t), "go", "test", "-count=1", "-v", "-run=^TestCodeOnAbsentWiring_", "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go test ./cmd/evolve exit=%d err=%v:\n%s", code, err, acsassert.OutputTail(out+errOut, 25))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

// acs-predicate: config-check
func TestC2637_005_SpecAndPlanNameTheOneRuleAndNoLongerClaimThereIsNoFailover(t *testing.T) {
	root := acsassert.RepoRoot(t)
	spec := filepath.Join(root, "docs", "architecture", "code-index-cli.md")
	plan := filepath.Join(root, "docs", "plans", "code-index-backends-2026-10.md")

	acsassert.FileContains(t, spec, "`on_absent`")
	acsassert.FileContains(t, plan, "`on_absent`")
	acsassert.FileNotContains(t, spec, "There is no failover when that backend is absent")
	acsassert.FileNotContains(t, spec, "and does not ask the next backend")
	if !acsassert.LineContainsAll(spec, "| `on_absent` |", "`refuse`", "`next`") {
		t.Errorf("%s has no key-table row naming on_absent with refuse and next", spec)
	}
	if !acsassert.LineContainsAll(plan, "| G4 |", "on_absent") {
		t.Errorf("%s: the G4 row does not name the on_absent rule", plan)
	}
}

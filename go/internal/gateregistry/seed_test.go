package gateregistry

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

func runSeed(t *testing.T, name string, baseline map[string]map[string]string) (gatecontract.Verdict, error) {
	t.Helper()
	fixture := fixtureNamed(name)
	root := fixture.root(t)
	lay(t, root, fixture.tree())
	lay(t, root, baseline[name])
	return entryNamed(t, name).Run(context.Background(), gatecontract.Input{Root: root}, gatecontract.Ports{})
}

func TestSeeds_ATreeAboveItsBaselineIsARedAndNotAnError(t *testing.T) {
	t.Parallel()
	for _, name := range enforcedGates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verdict, err := runSeed(t, name, baselineFiles)
			if err != nil {
				t.Fatalf("Run = error %v, want a verdict with a red", err)
			}
			if len(verdict.Reds) == 0 {
				t.Errorf("Run = %+v, want a red for a tree above its baseline", verdict)
			}
		})
	}
}

func TestSeeds_ATreeWithinItsBaselineHasNoRed(t *testing.T) {
	t.Parallel()
	for _, name := range enforcedGates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verdict, err := runSeed(t, name, greenFiles)
			if err != nil || len(verdict.Reds) != 0 {
				t.Errorf("Run = %+v, %v, want no red and no error", verdict, err)
			}
		})
	}
}

func TestSeeds_ATreeWithNoBaselineFileIsAnErrorAndNotAVerdict(t *testing.T) {
	t.Parallel()
	for _, name := range enforcedGates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verdict, err := runSeed(t, name, nil)
			if err == nil {
				t.Fatalf("Run = %+v, want an error for a failed read", verdict)
			}
			if len(verdict.Reds) != 0 || len(verdict.Notes) != 0 {
				t.Errorf("Run = %+v, want an empty verdict beside the error", verdict)
			}
		})
	}
}

func TestSeeds_AnUnparsableBaselineIsAnErrorAndNotARed(t *testing.T) {
	t.Parallel()
	broken := map[string]map[string]string{
		"testdup-ratchet": {"go/.test-dup-baseline": "count=x\n"},
		"size-ratchet":    {"go/internal/sizeratchet/offenders.json": "["},
		"raw-git-ratchet": {"go/internal/rawgitratchet/baseline.json": "["},
	}
	for _, name := range enforcedGates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if verdict, err := runSeed(t, name, broken); err == nil || len(verdict.Reds) != 0 {
				t.Errorf("Run = %+v, %v, want an error and no red", verdict, err)
			}
		})
	}
}

func TestSeeds_ARootThatDoesNotExistIsAnErrorAndNotAVerdict(t *testing.T) {
	t.Parallel()
	missing := t.TempDir() + "/absent"
	for _, e := range Entries() {
		if verdict, err := e.Run(context.Background(), gatecontract.Input{Root: missing}, gatecontract.Ports{}); err == nil || len(verdict.Reds) != 0 {
			t.Errorf("%s: Run over a missing root = %+v, %v, want an error and no red", e.Name, verdict, err)
		}
	}
}

func TestTestdupRatchet_AnAcsDirectoryCommittedBeforeTheBaselineDateIsNotCounted(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	lay(t, repo.Dir, map[string]string{
		"go/acs/cycle1/a_test.go": strings.Replace(dupHelper, "%s", "cycle1", 1),
		"go/acs/cycle1/b_test.go": strings.Replace(strings.Replace(dupHelper, "%s", "cycle1", 1), "func helper(", "func helper2(", 1),
	})
	repo.Git("add", "-A")
	repo.Git("commit", "-m", "old acs")
	lay(t, repo.Dir, map[string]string{"go/" + testdup.BaselineRelPath: "count=0\ndate=2999-01-01\n"})
	verdict, err := entryNamed(t, "testdup-ratchet").Run(context.Background(), gatecontract.Input{Root: repo.Dir}, gatecontract.Ports{})
	if err != nil || len(verdict.Reds) != 0 {
		t.Errorf("Run = %+v, %v, want no red: the copies are in an acs directory older than the baseline date", verdict, err)
	}
}

func TestRawGitRatchet_TheFallbackNoteGoesToNotes(t *testing.T) {
	t.Parallel()
	verdict, err := runSeed(t, "raw-git-ratchet", greenFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(verdict.Notes) != 1 || !strings.Contains(verdict.Notes[0], "git cannot list tracked files") {
		t.Errorf("Notes = %q, want the fallback note of BoundTestFiles", verdict.Notes)
	}
}

func TestVerdictOf_SplitsAMultiLineErrorIntoOneRedForEachLine(t *testing.T) {
	t.Parallel()
	got := verdictOf(errString("head:\n  one\n  two\n"), "note")
	if strings.Join(got.Reds, "|") != "head:|one|two" || len(got.Notes) != 1 {
		t.Errorf("verdictOf = %+v, want the three lines as reds and one note", got)
	}
	if clean := verdictOf(nil); len(clean.Reds) != 0 || len(clean.Notes) != 0 {
		t.Errorf("verdictOf(nil) = %+v, want an empty verdict", clean)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestSeeds_ASourceFileThatDoesNotParseIsAnErrorAndNotARed(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"testdup-ratchet", "raw-git-ratchet"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := fixtureNamed(name)
			root := fixture.root(t)
			lay(t, root, greenFiles[name])
			lay(t, root, map[string]string{"go/p/broken_test.go": "package p\n\nfunc ("})
			verdict, err := entryNamed(t, name).Run(context.Background(), gatecontract.Input{Root: root}, gatecontract.Ports{})
			if err == nil || len(verdict.Reds) != 0 {
				t.Errorf("Run = %+v, %v, want an error and no red", verdict, err)
			}
		})
	}
}

func fixtureNamed(name string) seedFixture {
	for _, f := range seedFixtures() {
		if f.name == name {
			return f
		}
	}
	return seedFixture{}
}

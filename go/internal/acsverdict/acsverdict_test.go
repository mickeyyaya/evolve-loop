package acsverdict_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

func TestFilename_IsTheArtifactEveryReaderOpens(t *testing.T) {
	if acsverdict.Filename != "acs-verdict.json" {
		t.Errorf("Filename = %q, want acs-verdict.json, the name the audit and ship gates read", acsverdict.Filename)
	}
}

func TestNoPredicatesID_IsAHarnessRedID(t *testing.T) {
	if !strings.HasPrefix(acsverdict.NoPredicatesID, acsverdict.SyntheticRedPrefix) || acsverdict.NoPredicatesID != "egps/no-predicates" {
		t.Errorf("NoPredicatesID = %q with prefix %q, want egps/no-predicates under the harness-red prefix", acsverdict.NoPredicatesID, acsverdict.SyntheticRedPrefix)
	}
}

func TestPath_IsTheCycleVerdictFileUnderAnAbsoluteEvolveDir(t *testing.T) {
	evolveDir := t.TempDir()
	got, err := acsverdict.Path(evolveDir, 12)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(evolveDir, "runs", "cycle-12", acsverdict.Filename); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestPath_RefusesARelativeEvolveDirAndCreatesNothing(t *testing.T) {
	const relative = "acsverdict-relative-evolve-dir-must-not-exist"
	if _, err := acsverdict.Path(filepath.Join(relative, ".evolve"), 12); err == nil {
		t.Fatal("Path accepted a relative evolve dir; both verdict writers rely on it to refuse one")
	}
	if _, err := os.Stat(relative); !os.IsNotExist(err) {
		t.Errorf("a refused path must create nothing (stat err=%v)", err)
	}
}

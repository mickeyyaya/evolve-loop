package core

import (
	"bytes"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
)

func writeScopeDeltaLane(t *testing.T, evolveDir string, cycle int, pinBody, itemID string, files ...string) {
	t.Helper()
	c := strconv.Itoa(cycle)
	item := `{"id":"` + itemID + `","files":["` + strings.Join(files, `","`) + `"]}`
	for dir, file := range map[string][2]string{
		filepath.Join(evolveDir, "runs", "cycle-"+c):                {LaneScopeFile, pinBody},
		filepath.Join(evolveDir, "inbox", "processing", "cycle-"+c): {itemID + ".json", item},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if file[1] == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, file[0]), []byte(file[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClassifyLaneChanges_JudgesEveryChangedPathAgainstOneAssembledScope(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	writeScopeDeltaLane(t, evolveDir, 7, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "go/internal/alpha/alpha.go")
	writeScopeDeltaLane(t, evolveDir, 8, `{"todo_ids":["sib"],"goal_hash":"g"}`, "sib", "go/internal/beta")
	declared := []string{"go/internal/alpha/alpha.go"}
	changed := []string{"go/internal/alpha/alpha.go", "go/internal/alpha/alpha_test.go", "go/internal/beta/beta.go", "docs/stray.md", ".evolve/policy.json"}
	protected := func(p string) bool { return p == ".evolve/policy.json" }

	entries := ClassifyLaneChanges(evolveDir, 7, declared, changed, protected, io.Discard)

	got := map[string]scopedelta.Class{}
	var order []string
	for _, e := range entries {
		got[e.Path] = e.Class
		order = append(order, e.Path)
	}
	want := map[string]scopedelta.Class{
		"go/internal/alpha/alpha.go":      scopedelta.ClassInScope,
		"go/internal/alpha/alpha_test.go": scopedelta.ClassClosure,
		"go/internal/beta/beta.go":        scopedelta.ClassCrossLane,
		"docs/stray.md":                   scopedelta.ClassOpportunistic,
		".evolve/policy.json":             scopedelta.ClassBoundary,
	}
	if !maps.Equal(got, want) || !slices.Equal(order, changed) {
		t.Fatalf("one entry per changed path, in the diff's order, judged against the pin, the wave and the protected member: %+v", entries)
	}
	if unguarded := ClassifyLaneChanges(evolveDir, 7, declared, changed, nil, io.Discard); unguarded[4].Class != scopedelta.ClassOpportunistic {
		t.Errorf("without a protected member no path is boundary: %+v", unguarded[4])
	}
}

func TestSiblingLaneFiles_SameWaveOnlyAndWarnsOnACorruptPin(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	writeScopeDeltaLane(t, evolveDir, 1, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "self.go")
	writeScopeDeltaLane(t, evolveDir, 2, `{"todo_ids":["sib"],"goal_hash":"g"}`, "sib", "sib.go")
	writeScopeDeltaLane(t, evolveDir, 3, `{"todo_ids":["far"],"goal_hash":"other"}`, "far", "far.go")
	writeScopeDeltaLane(t, evolveDir, 4, `{broken`, "bad", "bad.go")
	writeScopeDeltaLane(t, evolveDir, 5, `{"todo_ids":["lone"]}`, "lone", "lone.go")
	writeScopeDeltaLane(t, evolveDir, 6, "", "unpinned", "unpinned.go")
	writeScopeDeltaLane(t, evolveDir, 7, `{"todo_ids":["blocked"],"goal_hash":"g"}`, "blocked", "blocked.go")
	blockedClaims := filepath.Join(evolveDir, "inbox", "processing", "cycle-7")
	if err := os.RemoveAll(blockedClaims); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedClaims, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if got := SiblingLaneFiles(evolveDir, 1, &stderr); !slices.Equal(got, []string{"sib.go"}) {
		t.Errorf("siblings = %v, want only the same-wave lane's sib.go", got)
	}
	if !strings.Contains(stderr.String(), "skip sibling cycle 4") || !strings.Contains(stderr.String(), "skip sibling cycle 7: lane 7 claims:") || strings.Contains(stderr.String(), "cycle 6") {
		t.Errorf("a corrupt sibling pin and unreadable sibling claims are named; a lane without a pin is skipped in silence; stderr = %q", stderr.String())
	}
	if got := SiblingLaneFiles(evolveDir, 5, io.Discard); got != nil {
		t.Errorf("a lane whose pin carries no goal_hash names no wave; got %v", got)
	}
}

func TestLaneDeclaredFiles_FollowsThePinAndCountsEveryClaimWithoutOne(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	writeScopeDeltaLane(t, evolveDir, 1, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "a.go", "b/")
	writeScopeDeltaLane(t, evolveDir, 1, "", "extra", "c.go")
	writeScopeDeltaLane(t, evolveDir, 2, "", "one", "d.go")
	writeScopeDeltaLane(t, evolveDir, 2, "", "two", "e.go")

	pinned, err := LaneDeclaredFiles(evolveDir, 1)
	if err != nil || !slices.Equal(pinned, []string{"a.go", "b/"}) {
		t.Errorf("the pin selects the lane's own claims: got %v, %v", pinned, err)
	}
	unpinned, err := LaneDeclaredFiles(evolveDir, 2)
	slices.Sort(unpinned)
	if err != nil || !slices.Equal(unpinned, []string{"d.go", "e.go"}) {
		t.Errorf("without a pin every claim counts: got %v, %v", unpinned, err)
	}
	if none, err := LaneDeclaredFiles(evolveDir, 3); err != nil || none != nil {
		t.Errorf("a cycle with no claims declares nothing: got %v, %v", none, err)
	}
}

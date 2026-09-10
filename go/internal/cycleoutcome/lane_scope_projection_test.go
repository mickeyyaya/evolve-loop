package cycleoutcome

// lane_scope_projection_test.go — the durable regression pin for AC3 of inbox
// item `multi-slug-lane-scope-reconciliation` (cycle-1620): ONE projection
// source yields the lane's slug set for every consumer.
//
// Why this is the defect's other half. cycle-1480's audit FAIL H1 reads "TDD
// minted a cycle-wide predicate suite covering both slugs while the Builder
// contract bound only the first slug; nothing reconciles the two scopes." Two
// scopes exist to disagree only because two parsers of lane-scope.json exist:
// go/internal/core/lanescope.go and this package. A coherence gate on top of
// two derivations treats the symptom; collapsing the derivations removes the
// class (never_duplicate_centralize_via_design_patterns).
//
// Import directions, compiler-probed 2026-09-09 (cycle-644 obligation — never
// freeze a pin whose import shape is unproven):
//
//	core -> cycleoutcome  BLOCKED: cycleoutcome -> inboxmover -> adapters/ledger
//	                      -> core is an import cycle ("import cycle not allowed").
//	cycleoutcome -> core  BUILDABLE (`go build ./internal/cycleoutcome` clean),
//	                      despite the stale "cannot import core" comment on
//	                      cycleoutcome.go's LaneScopeIDs.
//
// So the collapse has two legal shapes — a leaf package both import, or this
// package delegating to core — and these tests pin the BEHAVIOR and the
// declaration COUNT, never which shape was chosen.

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// laneScopeWireTag is the struct tag that declares the lane-scope wire shape.
// Exactly one non-test Go file in the module may carry it.
const laneScopeWireTag = `json:"todo_ids"`

func writeLaneScopePin(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "lane-scope.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lane-scope.json: %v", err)
	}
	return dir
}

// TestLaneScopeProjection_OrderedAndFailOpen pins the projection contract every
// consumer inherits: the pinned members come back IN ORDER, and every degraded
// pin fails OPEN to the empty set. A projection that returned a partial set on a
// malformed pin would hand two consumers two different truths — the exact
// divergence this AC removes — and one that aborted would recreate the
// cycle-760..762 false-abort destruction class.
func TestLaneScopeProjection_OrderedAndFailOpen(t *testing.T) {
	pinned := writeLaneScopePin(t, t.TempDir(), `{"todo_ids":["alpha","beta","gamma"],"goal_hash":"h"}`)
	if got := LaneScopeIDs(pinned); !slices.Equal(got, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("pinned members must project in order; got %v", got)
	}
	malformed := writeLaneScopePin(t, t.TempDir(), "{not json")
	if got := LaneScopeIDs(malformed); len(got) != 0 {
		t.Errorf("a malformed pin must fail open to the empty projection, never a partial set; got %v", got)
	}
	if got := LaneScopeIDs(t.TempDir()); len(got) != 0 {
		t.Errorf("an absent pin must fail open to the empty projection; got %v", got)
	}
	if got := LaneScopeIDs(""); len(got) != 0 {
		t.Errorf("an empty workspace path must fail open to the empty projection; got %v", got)
	}
}

// TestLaneScopeProjection_SingleWireShapeDeclaration is the AC's literal
// grep-proof: "no second parser of lane-scope slugs". Two declarations of the
// wire shape are two parsers, and two parsers are what let the TDD scope and the
// Builder contract scope diverge in cycle-1480.
func TestLaneScopeProjection_SingleWireShapeDeclaration(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	var decls []string
	err = filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "acs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), laneScopeWireTag) {
			rel, relErr := filepath.Rel(moduleRoot, path)
			if relErr != nil {
				rel = path
			}
			decls = append(decls, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}
	slices.Sort(decls)
	if len(decls) != 1 {
		t.Errorf("the lane-scope slug set must have exactly ONE parser; %d non-test files declare %s: %v",
			len(decls), laneScopeWireTag, decls)
	}
}

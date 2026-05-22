// Package profiles loads .evolve/profiles/*.json agent permission
// profiles. The shape is pinned by the existing 16 profile files in
// the repo; this loader must round-trip every one of them.
package profiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

// sampleProfile mirrors .evolve/profiles/scout.json's load-bearing
// fields (the orchestrator and sandbox adapter consult these).
const sampleProfile = `{
  "name": "scout",
  "role": "scout",
  "cli": "claude",
  "model_tier_default": "sonnet",
  "model_tier_overrides": {
    "cycle_1_or_low_goal": "opus",
    "cycle_4_plus_mature": "haiku"
  },
  "allowed_tools": ["Read", "Grep", "Bash(git status:*)"],
  "disallowed_tools": ["Agent"],
  "max_turns": 30,
  "max_budget_usd": 1.50,
  "budget_tiers": {"low": 0.5, "high": 2.0},
  "parallel_eligible": true,
  "output_artifact": ".evolve/runs/cycle-*/scout-report.md",
  "sandbox": {
    "enabled": true,
    "read_only_repo": false,
    "write_subpaths": [".evolve/runs/cycle-*"],
    "deny_subpaths": [".git", "scripts"],
    "allow_network": true
  },
  "research_quota": {"web_search": 3, "web_fetch": 5, "kb_search": 20},
  "_comment": "informational only, must not break parsing"
}`

// minimalProfile — smallest valid profile; verifies the loader doesn't
// require optional fields.
const minimalProfile = `{"name": "tiny", "role": "tiny", "cli": "claude", "model_tier_default": "haiku"}`

func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"scout.json":   &fstest.MapFile{Data: []byte(sampleProfile)},
		"tiny.json":    &fstest.MapFile{Data: []byte(minimalProfile)},
		"AGENTS.md":    &fstest.MapFile{Data: []byte("non-JSON")}, // must NOT appear in List
		"README.txt":   &fstest.MapFile{Data: []byte("ignored")},
	}
}

// TestGet_HappyPath_TypedFields verifies every load-bearing typed field
// populates as expected.
func TestGet_HappyPath_TypedFields(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, err := l.Get("scout")
	if err != nil {
		t.Fatalf("Get(scout): %v", err)
	}
	if p.Name != "scout" || p.Role != "scout" {
		t.Errorf("Name/Role=%q/%q, want scout/scout", p.Name, p.Role)
	}
	if p.CLI != "claude" {
		t.Errorf("CLI=%q, want claude", p.CLI)
	}
	if p.ModelTierDefault != "sonnet" {
		t.Errorf("ModelTierDefault=%q, want sonnet", p.ModelTierDefault)
	}
	if got := p.ModelTierOverrides["cycle_1_or_low_goal"]; got != "opus" {
		t.Errorf("override[cycle_1_or_low_goal]=%q, want opus", got)
	}
	if p.MaxTurns != 30 || p.MaxBudgetUSD != 1.50 {
		t.Errorf("MaxTurns=%d MaxBudgetUSD=%g, want 30 1.5", p.MaxTurns, p.MaxBudgetUSD)
	}
	if !p.ParallelEligible {
		t.Error("ParallelEligible=false, want true")
	}
	if got := p.BudgetTiers["high"]; got != 2.0 {
		t.Errorf("BudgetTiers[high]=%g, want 2.0", got)
	}
	if p.ResearchQuota["web_search"] != 3 || p.ResearchQuota["kb_search"] != 20 {
		t.Errorf("ResearchQuota=%v missing expected entries", p.ResearchQuota)
	}
	if !reflect.DeepEqual(p.AllowedTools, []string{"Read", "Grep", "Bash(git status:*)"}) {
		t.Errorf("AllowedTools=%v", p.AllowedTools)
	}
	if !reflect.DeepEqual(p.DisallowedTools, []string{"Agent"}) {
		t.Errorf("DisallowedTools=%v", p.DisallowedTools)
	}
}

// TestGet_SandboxConfig — nested object parses into typed struct.
func TestGet_SandboxConfig(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, _ := l.Get("scout")
	if p.Sandbox == nil {
		t.Fatal("Sandbox=nil")
	}
	if !p.Sandbox.Enabled {
		t.Error("Sandbox.Enabled=false")
	}
	if !p.Sandbox.AllowNetwork {
		t.Error("Sandbox.AllowNetwork=false")
	}
	if !reflect.DeepEqual(p.Sandbox.WriteSubpaths, []string{".evolve/runs/cycle-*"}) {
		t.Errorf("WriteSubpaths=%v", p.Sandbox.WriteSubpaths)
	}
	if !reflect.DeepEqual(p.Sandbox.DenySubpaths, []string{".git", "scripts"}) {
		t.Errorf("DenySubpaths=%v", p.Sandbox.DenySubpaths)
	}
}

// TestGet_ReadOnlyRepo_AuditorPattern — auditor.json sets
// read_only_repo:true; verifies this critical sandboxing flag survives
// the round-trip.
func TestGet_ReadOnlyRepo_AuditorPattern(t *testing.T) {
	fsys := fstest.MapFS{
		"auditor.json": &fstest.MapFile{Data: []byte(`{"name":"auditor","role":"auditor","cli":"claude","model_tier_default":"opus","sandbox":{"read_only_repo":true,"deny_subpaths":[".evolve/state.json"]}}`)},
	}
	p, err := NewFromFS(fsys).Get("auditor")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !p.Sandbox.ReadOnlyRepo {
		t.Error("auditor sandbox.read_only_repo=false (critical: would let auditor mutate repo)")
	}
}

// TestGet_MinimalProfile_NoOptionalFields — caller doesn't need to
// configure optional fields.
func TestGet_MinimalProfile_NoOptionalFields(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, err := l.Get("tiny")
	if err != nil {
		t.Fatalf("Get(tiny): %v", err)
	}
	if p.Name != "tiny" {
		t.Errorf("Name=%q, want tiny", p.Name)
	}
	if p.Sandbox != nil {
		t.Errorf("Sandbox=%v, want nil on minimal profile", p.Sandbox)
	}
	if p.MaxTurns != 0 {
		t.Errorf("MaxTurns=%d, want 0 default", p.MaxTurns)
	}
}

// TestGet_NotFound — fs.ErrNotExist propagates.
func TestGet_NotFound(t *testing.T) {
	l := NewFromFS(fixtureFS())
	_, err := l.Get("nonexistent")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want fs.ErrNotExist", err)
	}
}

// TestGet_MalformedJSON_Surfaces — corrupt profile must return an
// error with file context, not panic.
func TestGet_MalformedJSON_Surfaces(t *testing.T) {
	fsys := fstest.MapFS{"bad.json": &fstest.MapFile{Data: []byte(`{not json`)}}
	_, err := NewFromFS(fsys).Get("bad")
	if err == nil {
		t.Fatal("want JSON parse error")
	}
}

// TestGet_RawJSONPreserved — Raw must contain the original bytes so
// callers can extract un-typed fields (e.g., `_comment`, parallel_subtasks).
func TestGet_RawJSONPreserved(t *testing.T) {
	l := NewFromFS(fixtureFS())
	p, _ := l.Get("scout")
	if len(p.Raw) == 0 {
		t.Error("Raw empty; callers can't extract un-typed fields")
	}
	// Verify _comment survives the round-trip via Raw (typed field ignored).
	if !containsBytes(p.Raw, []byte("informational only")) {
		t.Errorf("Raw missing _comment payload")
	}
}

// TestList_SortedAndJSONOnly — only *.json files appear; sorted alphabetically.
func TestList_SortedAndJSONOnly(t *testing.T) {
	names, err := NewFromFS(fixtureFS()).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"scout", "tiny"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("List()=%v, want %v (AGENTS.md and .txt excluded)", names, want)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(names, want) {
		t.Errorf("List() not sorted: %v", names)
	}
}

// TestZeroLoader_GetReturnsErrNotExist — zero loader contract.
func TestZeroLoader_GetReturnsErrNotExist(t *testing.T) {
	l := NewFromFS(nil)
	_, err := l.Get("any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want fs.ErrNotExist", err)
	}
}

// TestNewFromDir_ReadsRealFile — disk round-trip via os.DirFS.
func TestNewFromDir_ReadsRealFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "x.json"), []byte(minimalProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := NewFromDir(tmp).Get("x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "tiny" {
		t.Errorf("Name=%q, want tiny (from JSON)", p.Name)
	}
}

// TestNewFromDir_Empty_ReturnsZeroLoader — empty path → zero loader.
func TestNewFromDir_Empty_ReturnsZeroLoader(t *testing.T) {
	l := NewFromDir("")
	_, err := l.Get("any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err=%v, want fs.ErrNotExist", err)
	}
}

// TestSmoke_RealProfiles — load every profile under .evolve/profiles/
// and verify each has Name + Role + CLI. Skipped if dir absent. This
// is the canary for any schema drift between bash JSON and Go types.
func TestSmoke_RealProfiles(t *testing.T) {
	root := "../../../.evolve/profiles"
	if _, err := os.Stat(root); err != nil {
		t.Skipf("profiles dir not reachable: %v", err)
	}
	l := NewFromDir(root)
	names, err := l.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no profiles found")
	}
	for _, name := range names {
		p, err := l.Get(name)
		if err != nil {
			t.Errorf("Get(%s): %v", name, err)
			continue
		}
		if p.Name == "" || p.Role == "" || p.CLI == "" {
			t.Errorf("profile %s missing required name/role/cli (got %+v)", name,
				struct{ Name, Role, CLI string }{p.Name, p.Role, p.CLI})
		}
	}
}

// containsBytes — local helper (no strings import in test file).
func containsBytes(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

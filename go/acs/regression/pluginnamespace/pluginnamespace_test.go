//go:build acs

// Package pluginnamespace is the durable regression guard that locks in the
// 2026-06-24 plugin/command namespace rename evolve-loop → evo.
//
// In Claude Code the slash-command namespace IS the plugin's `name` field, so
// .claude-plugin/plugin.json and .claude-plugin/marketplace.json are the single
// source of truth for whether commands surface as /evo:loop, /evo:tdd, … (the
// /ecc:prune pattern). If anyone reverts the name in either manifest — or lets
// the two disagree — the /evo:* namespace silently breaks at install time. The
// rename itself touched 87 files, but only these two fields actually drive the
// namespace; everything else is consistency. This gate pins the field that
// matters and the agreement between the two manifests.
//
// acs-tagged like every go/acs/regression predicate; CI runs it via
//
//	go test -count=1 -tags acs ./acs/regression/...
//
// It needs no .apicover-enforce / completeness enrollment: a test-only package
// outside ./internal/... (exactly like acs/regression/noorphan, flagreaders).
package pluginnamespace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// wantNamespace is the command namespace the rename established. Editing it here
// is a deliberate re-namespace decision; an accidental manifest revert is not,
// and that is exactly what these tests are here to catch.
const wantNamespace = "evo"

type pluginManifest struct {
	Name string `json:"name"`
}

type marketplaceManifest struct {
	Name    string `json:"name"`
	Plugins []struct {
		Name string `json:"name"`
	} `json:"plugins"`
}

// loadManifest reads + parses a repo-relative JSON manifest, failing loudly.
func loadManifest(t *testing.T, rel string, dst any) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

// TestPluginManifest_NamespaceIsEvo — plugin.json `name` is THE command
// namespace; reverting it to "evolve-loop" silently breaks every /evo:* command.
func TestPluginManifest_NamespaceIsEvo(t *testing.T) {
	var pj pluginManifest
	loadManifest(t, ".claude-plugin/plugin.json", &pj)
	if pj.Name != wantNamespace {
		t.Errorf("plugin.json name = %q, want %q — the /%s:* slash-command namespace would break", pj.Name, wantNamespace, wantNamespace)
	}
}

// TestMarketplaceManifest_NamespaceIsEvo — the marketplace top-level name and
// its first plugin entry both carry the namespace (the /ecc-style mirror).
func TestMarketplaceManifest_NamespaceIsEvo(t *testing.T) {
	var mp marketplaceManifest
	loadManifest(t, ".claude-plugin/marketplace.json", &mp)
	if mp.Name != wantNamespace {
		t.Errorf("marketplace.json name = %q, want %q", mp.Name, wantNamespace)
	}
	if len(mp.Plugins) == 0 {
		t.Fatal("marketplace.json has no plugins[] entry")
	}
	if got := mp.Plugins[0].Name; got != wantNamespace {
		t.Errorf("marketplace.json plugins[0].name = %q, want %q", got, wantNamespace)
	}
}

// TestManifests_NamespaceConsistent pins the resolve invariant itself
// (value-independent): plugin.json name MUST equal marketplace plugins[].name.
// A disagreement is the exact failure mode that makes Claude Code install a
// plugin whose namespace differs from the marketplace entry, so /evo:* never
// resolves — even if each file is internally "valid".
func TestManifests_NamespaceConsistent(t *testing.T) {
	var pj pluginManifest
	var mp marketplaceManifest
	loadManifest(t, ".claude-plugin/plugin.json", &pj)
	loadManifest(t, ".claude-plugin/marketplace.json", &mp)
	if len(mp.Plugins) == 0 {
		t.Fatal("marketplace.json has no plugins[] entry")
	}
	if pj.Name != mp.Plugins[0].Name {
		t.Errorf("namespace disagreement: plugin.json name = %q but marketplace plugins[0].name = %q", pj.Name, mp.Plugins[0].Name)
	}
}

// TestNoDeadInstallToken locks the regression class a reviewer caught during
// the rename: operator-facing install/upgrade instructions that still printed
// the dead "<old>@<old>" plugin handle (a copy-paste yields a "plugin not
// found" error post-rename). It must survive in no tracked text source.
// CHANGELOG keeps verbatim history; golden testdata and the prebuilt go/evolve
// binary (rebuilt at release) are excluded; git grep -I skips binary. The
// search handle is assembled at runtime so this guard never matches itself.
func TestNoDeadInstallToken(t *testing.T) {
	root := acsassert.RepoRoot(t)
	deadHandle := "evolve-loop" + "@" + "evolve-loop" // assembled — avoids a self-match
	stdout, _, code, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "grep", "-I", "-n", deadHandle, "--",
		":!CHANGELOG.md", ":!*/testdata/*", ":!go/evolve",
	)
	switch code {
	case 0:
		t.Errorf("dead install handle still present (run: git grep %q):\n%s", deadHandle, stdout)
	case 1:
		// no matches — the invariant holds
	default:
		t.Fatalf("git grep failed (code=%d)", code)
	}
}

// TestLoopSkill_DescribesEvoCommand catches a half-done rename where the
// manifests flipped to evo but the canonical loop skill prose still advertises
// the dead /evolve-loop command to users.
func TestLoopSkill_DescribesEvoCommand(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "skills", "loop", "SKILL.md")
	// Read explicitly so a missing/unreadable file is a FATAL setup error, not
	// conflated with the substring-absent assertion below (acsassert.FileContains
	// reports both via the same non-fatal Errorf — undesirable in a CI gate).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := "/" + wantNamespace + ":loop"
	if !strings.Contains(string(raw), want) {
		t.Errorf("skills/loop/SKILL.md does not advertise %q — half-done rename?", want)
	}
}

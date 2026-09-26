package bridge

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The recovery agent (ADR-0106) may write the cycle's run directory and nothing else: the change's worktree,
// the inbox and the ledger stay out of reach, so a repair can restate the logic but never alter it.
func TestDeliverableRecoveryProfile_WritesOnlyTheRunDir(t *testing.T) {
	prof, err := LoadProfile(filepath.Join(realProfilesDir(t), "deliverable-recovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	grants, err := resolveSandboxWriteGrants(prof.Sandbox.WriteSubpaths, root, "")
	if err != nil {
		t.Fatal(err)
	}
	denies, err := resolveSandboxDenials(prof.Sandbox.DenySubpaths, root, "", true)
	if err != nil {
		t.Fatalf("the launcher would refuse this profile: %v", err)
	}
	canonicalRoot, err := canonicalSandboxPath(root)
	if err != nil {
		t.Fatal(err)
	}
	writable := func(rel string) bool {
		p := filepath.Join(canonicalRoot, filepath.FromSlash(rel))
		return coveredByAGrant(p, grants) && !coveredByAGrant(p, denies)
	}
	if !writable(".evolve/runs/cycle-1707/build-report.md") {
		t.Errorf("the owed deliverable must be writable (grants %v)", grants)
	}
	for _, rel := range []string{
		".evolve/worktrees/cycle-cd3ae73e-1707/go/fix.go",
		".evolve/inbox/item.json",
		".evolve/ledger.jsonl",
		".evolve/policy.json",
		".evolve/dossiers-pending/cycle-1707.json",
		"go/internal/core/fix.go",
	} {
		if writable(rel) {
			t.Errorf("%s must be out of the recovery agent's reach (grants %v)", rel, grants)
		}
	}
	if !prof.Sandbox.ReadOnlyRepo || !prof.Sandbox.Enabled {
		t.Error("the recovery agent's profile must enable the sandbox over a read-only repository: without it a helper launched with no worktree is not wrapped at all")
	}
}

// A helper is launched without a worktree. Inside the repository the wrapper may grant only the workspace and
// the profile's declared paths; the CLI's own state directories outside it are not the change.
func TestDeliverableRecoveryLaunch_GrantsNothingOfTheRepositoryButTheRunDir(t *testing.T) {
	prof, err := LoadProfile(filepath.Join(realProfilesDir(t), "deliverable-recovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	root, ws := t.TempDir(), t.TempDir()
	sbpl := renderSBPL(t, SandboxWrapRequest{
		Phase: "deliverable-recovery", RepoRoot: root, Workspace: ws, WriteSubpaths: prof.Sandbox.WriteSubpaths,
	})
	grants, err := resolveSandboxWriteGrants(prof.Sandbox.WriteSubpaths, root, "")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := canonicalSandboxPath(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalWS, err := canonicalSandboxPath(ws)
	if err != nil {
		t.Fatal(err)
	}
	allowed := regexp.MustCompile(`\(allow file-write\* \(subpath "([^"]+)"\)\)`).FindAllStringSubmatch(sbpl, -1)
	granted := map[string]bool{}
	for _, m := range allowed {
		granted[m[1]] = true
	}
	if !granted[canonicalWS] && !granted[ws] {
		t.Fatalf("the run dir must be writable:\n%s", sbpl)
	}
	for p := range granted {
		inRepo := p == canonicalRoot || p == root || coveredByAGrant(p, []string{canonicalRoot, root})
		if inRepo && !coveredByAGrant(p, grants) {
			t.Errorf("write grant %q reaches the repository beyond the declared paths", p)
		}
		if strings.Contains(p, "worktrees") {
			t.Errorf("write grant %q reaches a worktree", p)
		}
	}
}

// The profile declares no network, and the launch path forces the network on for every dispatch today with a
// WARN (inbox: sandbox-wrapper-forces-network-on), so the filesystem grant is the boundary that holds. This
// pins both facts through the real launch path; it flips when the wrapper honours the declaration.
func TestDeliverableRecoveryLaunch_DeclaresNoNetworkAndTheWrapperStillForcesIt(t *testing.T) {
	prof, err := LoadProfile(filepath.Join(realProfilesDir(t), "deliverable-recovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	if prof.Sandbox.AllowNetwork {
		t.Fatal("a repair from local evidence needs no network")
	}
	fw := &fakeWrap{prefix: []string{"sandbox-exec", "-p", "x"}, available: true}
	var stderr strings.Builder
	cfg := &Config{RequireSandbox: prof.Sandbox.Enabled, AllowNetwork: prof.Sandbox.AllowNetwork, Workspace: "/ws", ProjectRoot: "/repo", Agent: "deliverable-recovery"}

	prefix, ok := sandboxPrefixForLaunch(Deps{SandboxWrap: fw.wrap(), Stderr: &stderr}, cfg, "")

	if !ok || len(prefix) == 0 || len(fw.calls) != 1 {
		t.Fatalf("a helper without a worktree must still be confined: prefix=%v ok=%v calls=%d", prefix, ok, len(fw.calls))
	}
	if !fw.calls[0].AllowNetwork || !strings.Contains(stderr.String(), "forcing true") {
		t.Fatalf("the wrapper no longer forces the network on: update this pin and the inbox item; calls=%+v stderr=%q", fw.calls, stderr.String())
	}
}

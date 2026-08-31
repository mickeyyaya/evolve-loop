package core

// phase_bindings_staging_scope_test.go — cycle-1594 RED contract for
// `gitignore-staging-sweep`.
//
// The defect (reproduced in .evolve/runs/cycle-1594/bug-reproduction-report.md):
// worktreeContentSHA stages the whole worktree with `git add -A` purely to
// compute a content identity. Any unrelated untracked file that happens to sit
// in the lane — a bug-reproduction reproducer, a regenerated coverage artifact,
// a minted phase stub, another agent's scratch file — is therefore adopted into
// the tree the audit binding records (phase_bindings.go:129) and into the
// ADR-0048 verdict-cache key. The binding then attests a tree the auditor never
// reviewed, and the cache is keyed on foreign content. This is the mechanism
// behind the wave-3 cycle-1572/1574 staging rejections amortised into the
// `gitignore-staging-sweep` inbox record.
//
// The contract these tests pin is a SELECTION boundary, deliberately expressed
// in observable git terms so the Builder keeps design freedom (no new exported
// symbol, parameter, or carrier is frozen here — the current signature
// `worktreeContentSHA(ctx, worktree)` stays valid):
//
//	declared     = content already in the lane's git index (tracked files, plus
//	               anything the builder explicitly `git add`ed) + tracked
//	               modifications on disk
//	NOT declared = untracked files nobody staged
//
// Adversarial axes:
//   - negative  : ExcludesUnrelatedUntrackedResidue / ResidueOnlyWorktreeKeepsBaseIdentity
//                 (residue must NOT ride) and CapturesUnstagedTrackedModification
//                 (the anti-degenerate guard — a fix that just returns HEAD^{tree}
//                 passes the two exclusion pins and dies here, because a binding
//                 identical to the base is exactly the INTEGRITY_TREE_DRIFT /
//                 fresh-base cache collision this identity exists to avoid).
//   - edge      : the residue-ONLY worktree, where no declared work exists at
//                 all and the empty selection must not silently adopt residue.
//   - semantic  : exclusion, retention, tracked-modification capture and
//                 production reachability are four distinct behaviors.
//
// Every pin drives real git against a real repository and asserts on the real
// tree object; none greps source.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stagingScopeRepo builds an ephemeral lane worktree: one commit holding
// base.txt and the .gitignore the provisioner plants (so .evolve/ dirt is never
// part of the question), with the index clean at HEAD.
func stagingScopeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitOut(t, repo, "init", "-q")
	gitOut(t, repo, "config", "user.email", "test@test")
	gitOut(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, repo, "add", "base.txt", ".gitignore")
	gitOut(t, repo, "commit", "-q", "-m", "base")
	return repo
}

// treeNames lists every path recorded in a tree object.
func treeNames(t *testing.T, repo, tree string) []string {
	t.Helper()
	out := gitOut(t, repo, "ls-tree", "-r", "--name-only", tree)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func treeHas(t *testing.T, repo, tree, name string) bool {
	t.Helper()
	for _, got := range treeNames(t, repo, tree) {
		if got == name {
			return true
		}
	}
	return false
}

// writeFile is the fixture's disk mutator; a helper so each pin reads as the
// scenario it encodes rather than as error plumbing.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AC1 (RED). An untracked file nobody declared must be ABSENT from the binding
// tree. This is the reproduced defect verbatim: `git add -A` adopts it.
func TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	// A declared change, so the lane is genuinely dirty and the identity is not
	// trivially the base tree.
	writeFile(t, filepath.Join(repo, "base.txt"), "base\ndeclared edit\n")
	// Residue: an unrelated untracked file. Not gitignored — that is the whole
	// point; .gitignore already excludes the families it knows about, and the
	// classes that burned cycles 1572/1574 were precisely the ones it did not.
	writeFile(t, filepath.Join(repo, "foreign-residue.txt"), "must not enter the audit binding\n")

	tree := worktreeContentSHA(context.Background(), repo)
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree — the identity must still resolve")
	}
	if treeHas(t, repo, tree, "foreign-residue.txt") {
		t.Errorf("unrelated untracked residue entered binding tree %s (contents: %v) — "+
			"the audit binding and the verdict-cache key would attest content the auditor never reviewed",
			tree, treeNames(t, repo, tree))
	}
}

// AC2 (regression guard). A NEW file the builder declared by staging it must be
// RETAINED. Expected to be GREEN before the fix; it is the over-correction pin —
// a fix that simply stops staging anything, or that binds HEAD^{tree}, drops
// legitimate new source from the tree ship will commit.
func TestWorktreeContentSHA_StagesDeclaredNewFile(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	declared := filepath.Join("go", "internal", "newpkg", "newpkg.go")
	writeFile(t, filepath.Join(repo, declared), "package newpkg\n")
	// Declared = the builder put it in the lane's index.
	gitOut(t, repo, "add", declared)

	tree := worktreeContentSHA(context.Background(), repo)
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree")
	}
	if !treeHas(t, repo, tree, filepath.ToSlash(declared)) {
		t.Errorf("declared new file %s missing from binding tree %s (contents: %v) — "+
			"scoping staging must not drop the builder's own output",
			declared, tree, treeNames(t, repo, tree))
	}
}

// AC3 (regression guard / anti-degenerate). A modification to a TRACKED file
// that was never `git add`ed must still be captured. Without this, the cheapest
// way to pass AC1 and AC4 — return the base tree, or stage nothing at all —
// would look correct while making every binding identical to its base: the
// INTEGRITY_TREE_DRIFT class (cycle-152) and the verdict-cache fresh-base
// collision ADR-0048's ProbeEligible guard exists to prevent.
func TestWorktreeContentSHA_CapturesUnstagedTrackedModification(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "base.txt"), "base\nunstaged tracked edit\n")

	tree := worktreeContentSHA(context.Background(), repo)
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree")
	}
	baseTree := gitOut(t, repo, "rev-parse", "HEAD^{tree}")
	if tree == baseTree {
		t.Fatalf("binding tree %s equals the base tree — an unstaged tracked modification was dropped, "+
			"so every dirty lane would bind a base-identical (and cross-lane shared) identity", tree)
	}
	if got := gitOut(t, repo, "show", tree+":base.txt"); !strings.Contains(got, "unstaged tracked edit") {
		t.Errorf("binding tree %s carries stale base.txt content %q — tracked worktree edits must be captured", tree, got)
	}
}

// AC4 (RED, edge). The empty-selection case: a lane whose ONLY difference from
// its base is unrelated residue must keep its base identity exactly. Adopting
// residue here is worse than cosmetic — it manufactures a "changed" identity for
// an untouched worktree, which is precisely what ProbeEligible reads to decide
// whether a verdict may be cached or reused.
func TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "coverage.core476.func.txt"), "regenerated build output\n")
	writeFile(t, filepath.Join(repo, ".evolve-scratch-residue.txt"), "another lane's scratch\n")

	tree := worktreeContentSHA(context.Background(), repo)
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree — residue-only must still resolve an identity")
	}
	if baseTree := gitOut(t, repo, "rev-parse", "HEAD^{tree}"); tree != baseTree {
		t.Errorf("residue-only worktree resolved identity %s, want the base tree %s (contents: %v) — "+
			"an empty declared selection must not silently adopt residue",
			tree, baseTree, treeNames(t, repo, tree))
	}
}

// AC5 (RED, wiring proof). The seam must be reached from the PRODUCTION caller,
// not merely be correct in isolation: emitPhaseBindings → recordAuditBinding →
// worktreeContentSHA is the path that writes WorktreeTreeSHA into the auditor
// ledger entry ship verifies (phase_bindings.go:129). A predicate that only
// called the helper would pass on dead code. This drives the real orchestrator
// method against a real repository and reads the identity back off the real
// ledger entry.
func TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	base := gitOut(t, repo, "rev-parse", "HEAD")
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-1594")
	writeFile(t, filepath.Join(ws, "audit-report.md"), "## Verdict\n**PASS**\n")

	declared := filepath.Join("go", "internal", "newpkg", "newpkg.go")
	writeFile(t, filepath.Join(repo, declared), "package newpkg\n")
	gitOut(t, repo, "add", declared)
	writeFile(t, filepath.Join(repo, "foreign-residue.txt"), "must not enter the audit binding\n")

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }
	cs := CycleState{CycleID: 1594, WorkspacePath: ws, ActiveWorktree: repo, WorktreeBaseSHA: base}
	o.emitPhaseBindings(context.Background(), 1594, repo, cs, PhaseAudit, VerdictPASS)

	if len(led.entries) != 1 {
		t.Fatalf("want exactly 1 auditor binding entry, got %d (%+v)", len(led.entries), led.entries)
	}
	tree := led.entries[0].WorktreeTreeSHA
	if tree == "" {
		t.Fatal("audit binding recorded an empty worktree_tree_sha — ship cannot bind the audited content")
	}
	if treeHas(t, repo, tree, "foreign-residue.txt") {
		t.Errorf("audit binding %s attests unrelated untracked residue (contents: %v) — "+
			"the scoped selection is not wired into the production binding path",
			tree, treeNames(t, repo, tree))
	}
	if !treeHas(t, repo, tree, filepath.ToSlash(declared)) {
		t.Errorf("audit binding %s dropped declared builder output %s (contents: %v)",
			tree, declared, treeNames(t, repo, tree))
	}
}

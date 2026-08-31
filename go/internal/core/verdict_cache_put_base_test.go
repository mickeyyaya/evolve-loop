//go:build integration

package core

// verdict_cache_put_base_test.go — RED contract for the salvage-review HIGH-1
// (salvage/verdict-cache-land): the Put-site fresh-base guard must compare the
// audited worktree against the WORKTREE'S OWN base commit (CycleState.
// WorktreeBaseSHA), never against projectRoot HEAD at audit time. Under fleet
// concurrency a sibling ship advances main mid-cycle; resolving the base from
// the advanced HEAD (a commit the lane's worktree may not even contain) either
// diverges the operands or fails open — and an UNCHANGED worktree's shared
// fresh-base identity gets WRITTEN into the verdict cache, the exact entry
// class ADR-0048's guard exists to prevent on the read side.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// putBaseRepo initializes a repo with one committed file and the .gitignore
// the provisioner would plant (so worktreeContentSHA never sees .evolve dirt).
func putBaseRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitOut(t, root, "init", "-q")
	gitOut(t, root, "config", "user.email", "test@test")
	gitOut(t, root, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(root, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "base.txt", ".gitignore")
	gitOut(t, root, "commit", "-q", "-m", "base")
	return root
}

// putBaseClone clones src at its current tip into a fresh dir — the lane
// "worktree" frozen at the base commit.
func putBaseClone(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "wt")
	gitOut(t, src, "clone", "-q", src, dst)
	return dst
}

// putBaseAdvance lands a sibling commit on the repo's main line.
func putBaseAdvance(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "sibling.txt"), []byte("sibling ship"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "sibling.txt")
	gitOut(t, root, "commit", "-q", "-m", "sibling ship")
}

// putBaseWorkspace mints a workspace holding the audit artifact the recorder
// reads fatally.
func putBaseWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("# Audit\nPASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// TestAuditBindingPut_FreshBaseSuppressedWhenMainAdvances: projectRoot gains a
// commit after the lane worktree was cut; the worktree is UNCHANGED from its
// base. The audit-binding cache projection must NOT record the worktree's
// (shared, fresh-base) tree identity — regardless of where main's HEAD sits.
func TestAuditBindingPut_FreshBaseSuppressedWhenMainAdvances(t *testing.T) {
	ctx := context.Background()
	projectRoot := putBaseRepo(t)
	baseSHA := gitOut(t, projectRoot, "rev-parse", "HEAD")
	worktree := putBaseClone(t, projectRoot)
	putBaseAdvance(t, projectRoot)
	ws := putBaseWorkspace(t)

	now := func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.now = now

	cs := CycleState{
		WorkspacePath:   ws,
		ActiveWorktree:  worktree,
		WorktreeBaseSHA: baseSHA,
	}
	o.emitPhaseBindings(ctx, 7, projectRoot, cs, PhaseAudit, VerdictPASS)

	sha := worktreeContentSHA(ctx, worktree)
	if sha == "" {
		t.Fatal("worktree content SHA is empty")
	}
	if entry, ok := verdictcache.NewStore(projectRoot, now).Lookup(sha); ok {
		t.Fatalf("fresh-base verdict was WRITTEN to the cache despite main advancing (entry=%+v) — the Put guard resolved its base from projectRoot HEAD instead of the worktree's own base", entry)
	}
}

// TestAuditBindingPut_ChangedWorktreeStillRecords: the twin control — a lane
// with real changes must keep its cache projection even when main advanced, or
// the fix trades collision-pollution for silently lost legitimate entries.
func TestAuditBindingPut_ChangedWorktreeStillRecords(t *testing.T) {
	ctx := context.Background()
	projectRoot := putBaseRepo(t)
	baseSHA := gitOut(t, projectRoot, "rev-parse", "HEAD")
	worktree := putBaseClone(t, projectRoot)
	if err := os.WriteFile(filepath.Join(worktree, "feature.go"), []byte("package feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, worktree, "add", "feature.go")
	putBaseAdvance(t, projectRoot)
	ws := putBaseWorkspace(t)

	now := func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.now = now

	cs := CycleState{
		WorkspacePath:   ws,
		ActiveWorktree:  worktree,
		WorktreeBaseSHA: baseSHA,
	}
	o.emitPhaseBindings(ctx, 8, projectRoot, cs, PhaseAudit, VerdictPASS)

	sha := worktreeContentSHA(ctx, worktree)
	if sha == "" {
		t.Fatal("worktree content SHA is empty")
	}
	if _, ok := verdictcache.NewStore(projectRoot, now).Lookup(sha); !ok {
		t.Fatal("changed worktree's verdict was NOT recorded — the base fix over-suppressed legitimate entries")
	}
}

// TestAuditBindingPut_NoBaseIdentityFailsClosed: with no recorded base the Put
// side cannot prove the worktree is not fresh — it must SKIP the cache write
// (write-side fail-closed; the read side's fail-open stays, its worst case is
// only a shadow log line).
func TestAuditBindingPut_NoBaseIdentityFailsClosed(t *testing.T) {
	ctx := context.Background()
	projectRoot := putBaseRepo(t)
	worktree := putBaseClone(t, projectRoot)
	ws := putBaseWorkspace(t)
	now := func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.now = now

	cs := CycleState{WorkspacePath: ws, ActiveWorktree: worktree} // WorktreeBaseSHA absent
	o.emitPhaseBindings(ctx, 9, projectRoot, cs, PhaseAudit, VerdictPASS)

	sha := worktreeContentSHA(ctx, worktree)
	if _, ok := verdictcache.NewStore(projectRoot, now).Lookup(sha); ok {
		t.Fatal("cache write happened with NO base identity — the Put side must fail closed")
	}
}

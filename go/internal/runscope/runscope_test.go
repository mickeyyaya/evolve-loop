package runscope_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/projecthash"
	"github.com/mickeyyaya/evolveloop/go/internal/runscope"
	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

// TestResolveLane_DistinctRootsDistinctLanes is the multi-stream-bug precondition:
// two concurrent sibling worktrees (distinct roots) must get distinct lanes so
// their cycle branches never collide on the shared object store.
func TestResolveLane_DistinctRootsDistinctLanes(t *testing.T) {
	a := runscope.ResolveLane("", "/Users/x/ai/evolve-loop-campaign", noEnv)
	b := runscope.ResolveLane("", "/Users/x/ai/evolve-loop-dossier", noEnv)
	if a == "" || b == "" {
		t.Fatalf("lane must be non-empty: a=%q b=%q", a, b)
	}
	if a == b {
		t.Fatalf("distinct roots must yield distinct lanes; both = %q (would collide)", a)
	}
}

// TestResolveLane_StableAcrossResume: one root resolves to the SAME lane on every
// call (so a resumed cycle reuses its branch + warm worktree), and path variants
// of one dir (trailing slash) normalize to the same lane.
func TestResolveLane_StableAcrossResume(t *testing.T) {
	root := "/Users/x/ai/evolve-loop-campaign"
	first := runscope.ResolveLane("", root, noEnv)
	if again := runscope.ResolveLane("", root, noEnv); again != first {
		t.Fatalf("lane not stable: %q then %q", first, again)
	}
	if slash := runscope.ResolveLane("", root+"/", noEnv); slash != first {
		t.Fatalf("trailing-slash variant lane = %q, want %q (same dir)", slash, first)
	}
}

// TestResolveLane_OverridePrecedence: explicit --lane beats EVOLVE_LANE beats the
// hash default; all overrides are sanitized to a git-ref-safe token.
func TestResolveLane_OverridePrecedence(t *testing.T) {
	env := func(k string) string {
		if k == runscope.EnvLane {
			return "from-env"
		}
		return ""
	}
	if got := runscope.ResolveLane("explicit", "/root", env); got != "explicit" {
		t.Errorf("explicit should win: got %q", got)
	}
	if got := runscope.ResolveLane("", "/root", env); got != "from-env" {
		t.Errorf("env should win over hash: got %q", got)
	}
	// An all-unsafe explicit value sanitizes to empty → falls through to env.
	if got := runscope.ResolveLane("///", "/root", env); got != "from-env" {
		t.Errorf("unsafe explicit must fall through to env: got %q", got)
	}
	// Unsafe chars in an override are stripped to a ref-safe token.
	if got := runscope.ResolveLane("camp aign:2", "/root", noEnv); strings.ContainsAny(string(got), "/ :~^?*[\\\t") {
		t.Errorf("override not sanitized: %q", got)
	}
	// Dot-bearing overrides must NOT yield git-ref-illegal forms (.., trailing
	// .lock, leading dot) — the sanitizer drops '.' entirely so the resulting
	// cycle-<lane>-N branch is always a valid ref.
	for _, in := range []string{"a..b", "a.lock", ".hidden", "x...y"} {
		got := string(runscope.ResolveLane(in, "/root", noEnv))
		branch := runscope.New(runscope.Lane(got), "", 1).CycleBranch()
		if strings.Contains(branch, "..") || strings.HasSuffix(branch, ".lock") {
			t.Errorf("override %q produced ref-illegal branch %q", in, branch)
		}
	}
}

// TestRunScope_BranchNamesRefSafe: every branch projection is a valid git ref
// fragment (no chars git rejects).
func TestRunScope_BranchNamesRefSafe(t *testing.T) {
	rs := runscope.New(runscope.LaneFromRoot("/Users/x/ai/evolve-loop-campaign"), "", 7)
	for _, name := range []string{rs.CycleBranch(), rs.IntegrationBranch(), rs.WorkerBranch("w0")} {
		if strings.ContainsAny(name, "/ \t\n~^:?*[\\") {
			t.Errorf("branch %q is not a valid git-ref fragment", name)
		}
	}
}

// TestRunScope_NamingProjections pins the exact name shapes.
func TestRunScope_NamingProjections(t *testing.T) {
	var rs runscope.RunScope = runscope.New(runscope.Lane("camp"), "01ABCDEF99", 12)
	if got, want := rs.CycleBranch(), "cycle-camp-12"; got != want {
		t.Errorf("CycleBranch = %q, want %q", got, want)
	}
	if got, want := rs.IntegrationBranch(), "cycle-camp-12-integration"; got != want {
		t.Errorf("IntegrationBranch = %q, want %q", got, want)
	}
	if got, want := rs.WorkerBranch("w0"), "cycle-camp-12-w0"; got != want {
		t.Errorf("WorkerBranch = %q, want %q", got, want)
	}
	if got, want := rs.WorktreeDir("/base"), filepath.Join("/base", "cycle-camp-12"); got != want {
		t.Errorf("WorktreeDir = %q, want %q", got, want)
	}
	if got, want := rs.LaneToken(), "camp"; got != want {
		t.Errorf("LaneToken = %q, want %q", got, want)
	}
	if got, want := rs.Cycle(), 12; got != want {
		t.Errorf("Cycle = %d, want %d", got, want)
	}
	if got, want := rs.RunID(), "01ABCDEF99"; got != want {
		t.Errorf("RunID = %q, want %q", got, want)
	}
}

// TestRunScope_RunTokenMatchesLegacy: the session run-token projection is
// byte-identical to sessionrecord.RunScopeToken (preserves the CB.6 observer
// assertion), and SessionPrefix is empty when RunID is unset (legacy name).
func TestRunScope_RunTokenMatchesLegacy(t *testing.T) {
	for _, id := range []string{"01ABCDEF99XYZ", "short", "", "12345678"} {
		want := sessionrecord.RunScopeToken(id)
		if got := runscope.New("", id, 0).RunToken(); got != want {
			t.Errorf("RunToken(%q) = %q, want legacy %q", id, got, want)
		}
	}
	if got := runscope.New("", "", 3).SessionPrefix(); got != "" {
		t.Errorf("SessionPrefix with empty RunID = %q, want \"\" (legacy name)", got)
	}
	if got, want := runscope.New("", "01ABCDEF99", 3).SessionPrefix(), "r01ABCDEF-"; got != want {
		t.Errorf("SessionPrefix = %q, want %q", got, want)
	}
}

// TestRunScope_WorkspacePathStableNoLane: the run-workspace path must stay the
// bare .evolve/runs/cycle-<N> form (CB.4/CB.5 contract) — NO lane token, or the
// run.json symlink + tmux registry path would not match across the worktree.
func TestRunScope_WorkspacePathStableNoLane(t *testing.T) {
	rs := runscope.New(runscope.LaneFromRoot("/some/root"), "rid", 9)
	got := rs.WorkspacePath("/some/root")
	want := filepath.Join("/some/root", ".evolve", "runs", "cycle-9")
	if got != want {
		t.Fatalf("WorkspacePath = %q, want %q", got, want)
	}
	if strings.Contains(got, string(rs.LaneToken())) {
		t.Fatalf("WorkspacePath %q must NOT embed the lane token %q", got, rs.LaneToken())
	}
}

// TestLaneFromRoot_EqualsHotfixToken pins back-compat: the lane for an absolute
// root must byte-equal the shipped hotfix gitexec.WorktreeToken formula
// (first 4 bytes of sha256(absClean(root)) in hex == projecthash.Compute), so
// the running loops' existing cycle-<token>-N worktrees stay valid after this
// supersedes the hotfix.
func TestLaneFromRoot_EqualsHotfixToken(t *testing.T) {
	root := "/Users/x/ai/evolve-loop-flagreduce" // absolute ⇒ absClean == Clean(root)
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	wantHotfix := hex.EncodeToString(sum[:4]) // exactly what WorktreeToken returned
	if got := string(runscope.LaneFromRoot(root)); got != wantHotfix {
		t.Fatalf("LaneFromRoot = %q, want hotfix token %q (worktree back-compat broken)", got, wantHotfix)
	}
	// And it equals the projecthash primitive it is built on (no drift).
	if got, want := string(runscope.LaneFromRoot(root)), projecthash.Compute(filepath.Clean(root)); got != want {
		t.Fatalf("LaneFromRoot = %q, want projecthash %q", got, want)
	}
}

func noEnv(string) string { return "" }

package runscope_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/projecthash"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

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
	if got := runscope.ResolveLane("///", "/root", env); got != "from-env" {
		t.Errorf("unsafe explicit must fall through to env: got %q", got)
	}
	if got := runscope.ResolveLane("camp aign:2", "/root", noEnv); strings.ContainsAny(string(got), "/ :~^?*[\\\t") {
		t.Errorf("override not sanitized: %q", got)
	}
	for _, in := range []string{"a..b", "a.lock", ".hidden", "x...y"} {
		got := string(runscope.ResolveLane(in, "/root", noEnv))
		branch := runscope.New(runscope.Lane(got), "", 1).CycleBranch()
		if strings.Contains(branch, "..") || strings.HasSuffix(branch, ".lock") {
			t.Errorf("override %q produced ref-illegal branch %q", in, branch)
		}
	}
}

func TestRunScope_BranchNamesRefSafe(t *testing.T) {
	rs := runscope.New(runscope.LaneFromRoot("/Users/x/ai/evolve-loop-campaign"), "", 7)
	for _, name := range []string{rs.CycleBranch(), rs.IntegrationBranch(), rs.WorkerBranch("w0")} {
		if strings.ContainsAny(name, "/ \t\n~^:?*[\\") {
			t.Errorf("branch %q is not a valid git-ref fragment", name)
		}
	}
}

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

func TestLaneFromRoot_EqualsHotfixToken(t *testing.T) {
	root := "/Users/x/ai/evolve-loop-flagreduce"
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	wantHotfix := hex.EncodeToString(sum[:4])
	if got := string(runscope.LaneFromRoot(root)); got != wantHotfix {
		t.Fatalf("LaneFromRoot = %q, want hotfix token %q (worktree back-compat broken)", got, wantHotfix)
	}
	if got, want := string(runscope.LaneFromRoot(root)), projecthash.Compute(filepath.Clean(root)); got != want {
		t.Fatalf("LaneFromRoot = %q, want projecthash %q", got, want)
	}
}

func noEnv(string) string { return "" }

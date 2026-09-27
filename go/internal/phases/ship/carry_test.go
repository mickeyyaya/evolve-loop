package ship

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

func testGit(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = filteredEnv()
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode(), nil
	}
	return string(out), 0, err
}

type carriedLane struct {
	repo, base0, tree0, base1, tree1 string
}

// rebasedLane is a lane whose audited change (tree0 on base0) is pended byte for byte on a peer's later base (tree1 on base1).
func rebasedLane(t *testing.T) carriedLane {
	t.Helper()
	repo := makeRepo(t)
	rev := func(args ...string) string { return strings.TrimSpace(runGitOut(t, repo, args...)) }
	base0 := rev("rev-parse", "HEAD")
	mustWrite(t, filepath.Join(repo, "lane.txt"), "the lane's audited change\n")
	runGit(t, repo, "add", "lane.txt")
	tree0 := rev("write-tree")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane")
	lane := rev("rev-parse", "HEAD")
	runGit(t, repo, "reset", "-q", "--hard", base0)
	mustWrite(t, filepath.Join(repo, "peer.txt"), "a peer landing\n")
	runGit(t, repo, "add", "peer.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "peer")
	base1 := rev("rev-parse", "HEAD")
	runGit(t, repo, "-c", "commit.gpgsign=false", "cherry-pick", lane)
	runGit(t, repo, "reset", "-q", "--soft", base1)
	return carriedLane{repo: repo, base0: base0, tree0: tree0, base1: base1, tree1: rev("write-tree")}
}

func writeCarry(t *testing.T, l carriedLane, ref, auditedTree string) {
	t.Helper()
	writeCarryStating(t, l, ref, auditedTree, l.tree1)
}

// writeCarryStating writes a chained carry record that names auditedTree and treeState, whatever the truth.
func writeCarryStating(t *testing.T, l carriedLane, ref, auditedTree, treeState string) {
	t.Helper()
	audited, composed, _, err := treedelta.Identical(context.Background(), testGit, l.repo, l.base0, l.tree0, l.base1, l.tree1)
	if err != nil {
		t.Fatal(err)
	}
	writeCarryOf(t, l, ref, auditedTree, treeState, audited, composed)
}

// writeCarryOf writes a chained carry record over the given diffs; the writer binds the record's patch-id to them.
func writeCarryOf(t *testing.T, l carriedLane, ref, auditedTree, treeState string, audited, composed []byte) {
	t.Helper()
	patchID, err := ledger.PatchID(audited)
	if err != nil {
		t.Fatal(err)
	}
	gates := map[string]string{}
	for _, g := range ciparity.RequiredComposedGates {
		gates[g] = "pass"
	}
	if err := ledger.WriteCompositionVerdict(filepath.Join(l.repo, ".evolve", "ledger.jsonl"), ledger.CompositionVerdictInput{
		Cycle: 1715, Method: ledger.IdenticalRebaseMethod, LaneAuditRef: ref, PatchID: patchID,
		AuditedBase: l.base0, GitHead: l.base1, TreeStateSHA: treeState, AuditedTreeSHA: auditedTree, GateResults: gates,
		AuditedDiff: audited, ComposedDiff: composed, ArtifactDir: filepath.Join(l.repo, ".evolve", "composition-artifacts"),
	}); err != nil {
		t.Fatal(err)
	}
}

func boundTo(t *testing.T, l carriedLane, tree, ref string) *Options {
	t.Helper()
	opts := auditOpts(t, l.repo)
	opts.internalAuditBoundTreeSHA = tree
	opts.internalAuditArtifactSHA = ref
	return opts
}

func TestAuditBindingSatisfied_AcceptsAReProvenCarry(t *testing.T) {
	l := rebasedLane(t)
	writeCarry(t, l, "audit-ref", l.tree0)

	ok, detail := auditBindingSatisfied(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1)

	if !ok || !strings.Contains(detail, "carry of cycle 1715, re-proven") {
		t.Fatalf("auditBindingSatisfied = (%v, %q); the audited tree carried byte for byte onto the peer's base is bound", ok, detail)
	}
	if ok, _ := auditBindingSatisfied(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), "", l.tree1); !ok {
		t.Error("the direct path applies the same rule from the project root")
	}
}

func TestAuditBindingSatisfied_DeclinesACarryItCannotReProve(t *testing.T) {
	l := rebasedLane(t)
	writeCarry(t, l, "audit-ref", l.tree0)
	ctx := context.Background()
	if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, "another-audit"), l.repo, l.tree1); ok {
		t.Error("a carry of another audit never binds this one")
	}
	if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.base0+"^{tree}", "audit-ref"), l.repo, l.tree1); ok {
		t.Error("a carry whose audited tree is not the bound one never binds")
	}
	mustWrite(t, filepath.Join(l.repo, "lane.txt"), "the lane's audited change, edited after the carry\n")
	runGit(t, l.repo, "add", "lane.txt")
	edited := strings.TrimSpace(runGitOut(t, l.repo, "write-tree"))
	if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, edited); ok {
		t.Error("a byte changed after the carry is tree drift, not the carry")
	}
	if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, ""), l.repo, l.tree1); ok {
		t.Error("a binding that names no audit artifact has no carry to look up")
	}
}

func TestAuditBindingSatisfied_DeclinesACarryWhoseRecordLies(t *testing.T) {
	l := rebasedLane(t)
	writeCarry(t, l, "audit-ref", l.tree0)
	runGit(t, l.repo, "reset", "-q", "--hard", l.base0)
	mustWrite(t, filepath.Join(l.repo, "other.txt"), "an unrelated base\n")
	runGit(t, l.repo, "add", "other.txt")
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "elsewhere")
	mustWrite(t, filepath.Join(l.repo, "lane.txt"), "the lane's audited change\n")
	runGit(t, l.repo, "add", "lane.txt")

	ok, _ := auditBindingSatisfied(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1)

	if ok {
		t.Fatal("the record's base is not an ancestor of HEAD: the carry is re-proven, never trusted")
	}
}

func TestAuditBindingSatisfied_ReProvesTheRecordsClaimsIndependently(t *testing.T) {
	ctx := context.Background()
	t.Run("the record names the right tree but the bytes are not the audited change", func(t *testing.T) {
		l := rebasedLane(t)
		emptyAudit := strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", l.base0+"^{tree}"))
		writeCarryStating(t, l, "audit-ref", emptyAudit, l.tree1)
		if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, emptyAudit, "audit-ref"), l.repo, l.tree1); ok {
			t.Error("an audited tree with no change in it carries nothing, whatever the record says")
		}
	})
	t.Run("the bytes are the audited change but the record names another tree", func(t *testing.T) {
		l := rebasedLane(t)
		writeCarryStating(t, l, "audit-ref", l.tree0, strings.Repeat("0", 40))
		if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1); ok {
			t.Error("a record that does not name the tree ship holds is not this ship's carry")
		}
	})
	t.Run("a line outside the ledger chain never carries", func(t *testing.T) {
		l := rebasedLane(t)
		writeCarry(t, l, "audit-ref", l.tree0)
		ledgerPath := filepath.Join(l.repo, ".evolve", "ledger.jsonl")
		body, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		forged := strings.Replace(lines[len(lines)-1], `"cycle":1715`, `"cycle":1716`, 1)
		if forged == lines[len(lines)-1] {
			t.Fatal("fixture: the record's cycle is not where this test expects")
		}
		if err := os.WriteFile(ledgerPath, []byte(string(body)+forged+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1); ok {
			t.Error("a record appended outside the chain, even a copy of a true one, is not evidence")
		}
	})
	t.Run("the record's patch-id is not the proven bytes'", func(t *testing.T) {
		l := rebasedLane(t)
		other := []byte("diff --git a/other.txt b/other.txt\nnew file mode 100644\nindex 0000000000000000000000000000000000000000..a1b2c3d4e5f60718293a4b5c6d7e8f9012345678\n--- /dev/null\n+++ b/other.txt\n@@ -0,0 +1 @@\n+another change\n")
		writeCarryOf(t, l, "audit-ref", l.tree0, l.tree1, other, other)
		if ok, _ := auditBindingSatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1); ok {
			t.Error("a chained record whose diffs are another change is not this change's carry")
		}
	})
}

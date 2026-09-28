package guards

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// activeCycle is the storage a lane's guard reads: only the cycle identity matters to docdelete.
type activeCycle struct {
	core.Storage
	cs core.CycleState
}

func (a activeCycle) ReadCycleState(context.Context) (core.CycleState, error) { return a.cs, nil }

// cycle 2 of run "draft" owns docs/explain/builds/cycle-2-draft.md (explanationdocs.DocumentPath).
var draftCycle = activeCycle{cs: core.CycleState{CycleID: 2, RunID: "draft"}}

const ownDraft = "docs/explain/builds/cycle-2-draft.md"

func gitRepo(t *testing.T, committed map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
	for rel, body := range committed {
		writeFile(t, dir, rel, body)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	return dir
}

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// laneWithADraft is a lane worktree whose HEAD holds committed documentation, plus the active cycle's
// explanation draft, written by a Build and never committed.
func laneWithADraft(t *testing.T) string {
	t.Helper()
	dir := gitRepo(t, map[string]string{
		"docs/explain/builds/cycle-1-run.md": "a published record\n",
		"docs/architecture/README.md":        "a committed index\n",
		"knowledge-base/cycles/cycle-1.md":   "a committed dossier\n",
	})
	writeFile(t, dir, ownDraft, "the active cycle's draft\n")
	return dir
}

func decide(storage core.Storage, dir, cmd string) core.GuardDecision {
	return NewDocDelete(false, storage).Decide(context.Background(), core.GuardInput{
		ToolName: "Bash", ToolInput: map[string]any{"command": cmd}, CWD: dir,
	})
}

func TestDocDelete_ABuildMayRetractItsOwnDraftThatWasNeverCommitted(t *testing.T) {
	dir := laneWithADraft(t)
	for _, cmd := range []string{
		"rm " + ownDraft,
		"git rm -f -q " + ownDraft,
		"git rm -f -q " + ownDraft + "; echo rc=$?; git status --short; ls docs/explain/builds/ | grep cycle-2",
		"rm mydocs.txt",
	} {
		if dec := decide(draftCycle, dir, cmd); !dec.Allow {
			t.Errorf("%q denied: %s", cmd, dec.Reason)
		}
	}
}

func TestDocDelete_EverythingElseUnderTheDocRootsStaysProtected(t *testing.T) {
	dir := laneWithADraft(t)
	for _, cmd := range []string{
		"rm docs/explain/builds/cycle-1-run.md",
		"rm " + ownDraft + " docs/explain/builds/cycle-1-run.md",
		"rm docs/explain/builds/cycle-3-other.md",
		"rm ./" + ownDraft,
		"rm " + filepath.Join(dir, ownDraft),
		"rm DOCS/explain/builds/cycle-1-run.md",
		"git --no-pager rm -f -q " + ownDraft,
		"git -C docs rm -f -q " + ownDraft,
		"GIT_WORK_TREE=docs git rm -f -q " + ownDraft,
		"/bin/rm " + ownDraft,
		"rm docs/architecture/readme.md",
		"rm -rf docs",
		"rm -rf knowledge-base",
		"rm -rf docs/explain",
		"rm $(echo docs/explain/builds/cycle-1-run.md)",
		"rm `echo docs/explain/builds/cycle-1-run.md`",
		"rm {docs,x}/explain/builds/cycle-1-run.md",
		"git rm ':(top)docs/explain/builds/cycle-1-run.md'",
		"git -C docs rm explain/builds/cycle-1-run.md",
		"git -C docs mv explain/builds/cycle-1-run.md /tmp/x.md",
		"cd docs/explain/builds && rm cycle-1-run.md",
		"cd docs && mv explain/builds/cycle-1-run.md /tmp/x.md",
		"xargs rm docs/explain/builds/cycle-1-run.md",
		"sudo rm " + ownDraft,
		"rm build.log; ls docs/explain/builds/",
		"mv " + ownDraft + " /tmp/draft.md",
		"mv docs/explain/builds/cycle-1-run.md /tmp/x.md",
	} {
		if dec := decide(draftCycle, dir, cmd); dec.Allow {
			t.Errorf("%q allowed", cmd)
		}
	}
}

func TestDocDelete_TheDraftExceptionNeedsTheActiveCycleAndANeverCommittedPath(t *testing.T) {
	dir := laneWithADraft(t)
	if dec := decide(nil, dir, "rm "+ownDraft); dec.Allow {
		t.Error("allowed a draft retraction with no active cycle to name the draft")
	}
	if dec := decide(draftCycle, t.TempDir(), "rm "+ownDraft); dec.Allow {
		t.Error("allowed a draft retraction outside a repository")
	}
	published := activeCycle{cs: core.CycleState{CycleID: 1, RunID: "run"}}
	if dec := decide(published, dir, "rm docs/explain/builds/cycle-1-run.md"); dec.Allow {
		t.Error("allowed removing the cycle's document although HEAD holds it")
	}
	for _, cmd := range []string{"rm explain/builds/cycle-2-draft.md", "rm explain/builds/cycle-1-run.md", "mv explain/builds/cycle-1-run.md /tmp/x.md"} {
		if dec := decide(draftCycle, filepath.Join(dir, "docs"), cmd); dec.Allow {
			t.Errorf("%q from inside docs/ allowed: its operands name no doc root", cmd)
		}
	}
}

func TestDocDelete_ADraftPathThroughASymlinkedDirectoryIsNotTheDraft(t *testing.T) {
	dir := gitRepo(t, map[string]string{"docs/architecture/cycle-2-draft.md": "committed under another name\n"})
	if err := os.MkdirAll(filepath.Join(dir, "docs", "explain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../architecture", filepath.Join(dir, "docs", "explain", "builds")); err != nil {
		t.Fatal(err)
	}
	if dec := decide(draftCycle, dir, "rm "+ownDraft); dec.Allow {
		t.Fatal("allowed an rm that resolves through a symlinked directory to committed documentation")
	}
}

func TestDocDelete_TheArchiveAdviceKeepsTheArchiveStaged(t *testing.T) {
	dec := decide(draftCycle, laneWithADraft(t), "rm docs/explain/builds/cycle-1-run.md")
	if dec.Allow || !strings.Contains(dec.Reason, "git mv") {
		t.Fatalf("deny reason %q must advise git mv, so an archived copy is staged rather than left untracked", dec.Reason)
	}
}

func TestDocDelete_TheDraftNameMustNotAliasACommittedFile(t *testing.T) {
	caseVariant := gitRepo(t, map[string]string{"docs/explain/builds/Cycle-2-Draft.md": "committed, differing only in case\n"})
	if dec := decide(draftCycle, caseVariant, "rm "+ownDraft); dec.Allow {
		t.Error("allowed an rm that a case-insensitive filesystem resolves to a committed file")
	}
	nested := gitRepo(t, map[string]string{"sub/" + ownDraft: "committed under a nested docs root\n"})
	if dec := decide(draftCycle, filepath.Join(nested, "sub"), "rm "+ownDraft); dec.Allow {
		t.Error("allowed an rm whose operand is relative to a subdirectory, where it names committed documentation")
	}
}

func TestDocDelete_EvasionsThatBashStillResolvesToTheDocRootsAreDenied(t *testing.T) {
	dir := laneWithADraft(t)
	for _, cmd := range []string{
		"rm doc''s/architecture/README.md",
		`rm do\cs/architecture/README.md`,
		`rm "docs"/architecture/README.md`,
		"unlink docs/architecture/README.md",
		"find docs/architecture -name README.md -delete",
		`find docs -name README.md -exec rm {} \;`,
		"git clean -fdx docs",
		"mv doc''s/architecture/README.md /tmp/x.md",
		"git mv doc''s/architecture/README.md /tmp/x.md",
		"git mv docs/architecture/README.md /tmp/x.md",
		"git --no-pager mv doc''s/architecture/README.md /tmp/x.md",
		"mv -t /tmp docs/architecture/README.md",
		"mv --target-directory=/tmp docs/architecture/README.md",
		"mv --target=/tmp docs/architecture/README.md",
		"mv -T doc''s/architecture/README.md /tmp/x.md",
		"mv -- doc''s/architecture/README.md /tmp/x.md",
		"mv -- -x/../doc''s/architecture/README.md /tmp/x.md",
		`find docs -name README.md -exec /bin/rm {} \;`,
		`find docs -name README.md -execdir rm {} +`,
		"find $'docs' -name README.md -delete",
		`find $'\x64ocs' -name README.md -delete`,
		`find $'\144ocs' -name README.md -delete`,
		`find $'\x44OCS' -name README.md -delete`,
		"mv $'docs'/architecture/README.md /tmp/x.md",
		`cd $'\x44ocs' && rm architecture/README.md`,
		"git rm -f -q " + ownDraft + "; unlink docs/architecture/README.md",
		"/usr/bin/unlink docs/architecture/README.md",
		"FOO=1 unlink docs/architecture/README.md",
		"/usr/bin/find docs -name README.md -delete",
		"/usr/bin/git clean -fdx docs",
		"/bin/rm doc''s/architecture/README.md",
		"/bin/mv doc''s/architecture/README.md /tmp/x.md",
		"command rm doc''s/architecture/README.md",
		"env rm doc''s/architecture/README.md",
		"MV docs/architecture/README.md /tmp/x.md",
		"Unlink docs/architecture/README.md",
		`find docs -name README.md -ok rm {} \;`,
		`find docs -name README.md -okdir rm {} \;`,
		`find docs -name README.md -exec unlink {} \;`,
		`find docs -name README.md -exec env rm {} \;`,
		"mv -t/tmp docs/architecture/README.md",
		"mv -S.t docs/architecture/README.md /tmp/x.md",
		"mv -fS .t docs/architecture/README.md /tmp/x.md",
		`rm $'\U00000064'ocs/architecture/README.md`,
		`find $'\U00000064ocs' -name README.md -delete`,
		`mv $'\U00000064ocs'/architecture/README.md /tmp/x.md`,
		`git -C $'\x64ocs' rm -q architecture/README.md`,
		`find $"docs" -name README.md -exec rm {} \;`,
		`git clean -fdx $"docs"`,
		`unlink $"docs"/architecture/README.md`,
		"mv -St docs/architecture/README.md /tmp/x.md",
		"env -u FOO unlink docs/architecture/README.md",
		"env -u PATH rm docs/architecture/README.md",
		"env --unset PATH rm doc''s/architecture/README.md",
		"exec -a fakename rm doc''s/architecture/README.md",
		"exec -a x unlink docs/architecture/README.md",
		"env -u FOO find docs -delete",
		"sudo unlink docs/architecture/README.md",
		"sudo -u root rm doc''s/architecture/README.md",
		"timeout 5 unlink docs/architecture/README.md",
		"nice -n 5 find docs -delete",
		"xargs rm docs/architecture/README.md",
		"xargs unlink docs/architecture/README.md",
		"sudo git -C docs mv architecture/README.md /tmp/x.md",
		"sudo mv doc''s/architecture/README.md /tmp/x.md",
		"sudo -u mv -t docs/x mv doc''s/architecture/README.md /tmp/y.md",
		"caffeinate unlink docs/architecture/README.md",
		"arch -arm64 find docs -delete",
	} {
		if dec := decide(draftCycle, dir, cmd); dec.Allow {
			t.Errorf("%q allowed: bash resolves it to a removal under a doc root", cmd)
		}
	}
	for _, cmd := range []string{
		"find . -name '*.tmp' -delete",
		"unlink /tmp/lane.sock",
		"mv docs/architecture/README.md docs/private/research/archived-2026-09-26/README.md",
		"mv -t docs/private/research/archived-2026-09-26 docs/architecture/README.md",
		"find docs -name rm",
		`find . -name '*.tmp' -exec /bin/rm {} \;`,
		`printf $'docs\n'`,
		"grep -rn unlink docs/architecture",
		"mv -S.bak docs/architecture/README.md docs/private/research/archived-2026-09-26/README.md",
		"git -c core.quotepath=off status docs",
		"mv --suffix docs README.md /tmp/x.md",
		"mv --su docs README.md /tmp/x.md",
		"find . -exec grep -l docs {} +",
		"time go test ./internal/guards/",
		"xargs grep -l unlink",
	} {
		if dec := decide(draftCycle, dir, cmd); !dec.Allow {
			t.Errorf("%q denied: it removes nothing under a doc root: %s", cmd, dec.Reason)
		}
	}
}

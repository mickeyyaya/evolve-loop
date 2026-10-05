package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type handoffReplay struct {
	repo      *gittest.Repo
	root      string
	worktree  string
	workspace string
	base      string
	cycle     int
	runID     string
}

func newHandoffReplay(t *testing.T, cycle int, baseFiles map[string]string) handoffReplay {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := gittest.Fixture(t)
	r := handoffReplay{repo: repo, root: t.TempDir(), worktree: repo.Dir, cycle: cycle, runID: fmt.Sprintf("run-%d", cycle)}
	r.workspace = filepath.Join(r.root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	r.write(t, ".gitignore", ".evolve/\n")
	r.write(t, "go/go.mod", "module floorfixture\n\ngo 1.23\n")
	for rel, body := range baseFiles {
		r.write(t, rel, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	r.base = repo.Git("rev-parse", "HEAD")
	r.bindCycle(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", r.root)
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	return r
}

func (r handoffReplay) writeRunState(t *testing.T, contractVersion int) {
	t.Helper()
	state, err := json.Marshal(map[string]any{
		"cycle_id": r.cycle, "phase": "build", "run_id": r.runID, "workspace_path": r.workspace,
		"active_worktree": r.worktree, "worktree_base_sha": r.base,
		"explanation_documentation_version": contractVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.writeWorkspace(t, "run.json", string(state))
}

func (r handoffReplay) bindCycle(t *testing.T) {
	t.Helper()
	r.writeRunState(t, explanationdocs.CurrentContractVersion)
	if err := os.MkdirAll(filepath.Join(r.worktree, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(r.workspace, "run.json"), filepath.Join(r.worktree, ".evolve", "cycle-state.json")); err != nil {
		t.Fatal(err)
	}
	binding := explanationdocs.CycleBinding{
		ProjectRoot: r.root, Workspace: r.workspace, Cycle: r.cycle, RunID: r.runID,
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	binding.Worktree, binding.BaseSHA = r.worktree, r.base
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatalf("SealBuild: %v", err)
	}
}

func (r handoffReplay) write(t *testing.T, rel, body string) {
	t.Helper()
	writeReplayFile(t, filepath.Join(r.worktree, filepath.FromSlash(rel)), body)
}

func (r handoffReplay) writeWorkspace(t *testing.T, rel, body string) {
	t.Helper()
	writeReplayFile(t, filepath.Join(r.workspace, filepath.FromSlash(rel)), body)
}

func writeReplayFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r handoffReplay) documentPath(t *testing.T) string {
	t.Helper()
	p, err := explanationdocs.DocumentPath(r.cycle, r.runID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (r handoffReplay) explanation(changedAreas ...string) string {
	return fmt.Sprintf("# Build Explanation — Cycle %d\n\n", r.cycle) +
		fmt.Sprintf("## Build Binding\n- Cycle: %d\n- Base SHA: %s\n\n", r.cycle, r.base) +
		"## Summary\nAdds the unsatisfiability lint the pinned inbox item asks for.\n\n" +
		"## Rationale\nA predicate that no tree can pass wastes a whole lane, so the lint names it before the build starts.\n\n" +
		"## Changed Areas\n" + strings.Join(changedAreas, "\n") + "\n\n" +
		"## Design Decisions\nThe lint reads the predicate AST and never runs the predicate itself.\n\n" +
		"## Verification\nThe package tests drive every flagged shape and every clean shape.\n\n" +
		"## Compatibility\nNo existing command changes its output.\n\n" +
		"## Limitations\nOnly Go predicates are linted.\n"
}

func (r handoffReplay) commitBuild(t *testing.T, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		r.write(t, rel, body)
	}
	r.repo.Git("add", "-A")
	r.repo.Git("commit", "-q", "-m", "builder work")
}

func (r handoffReplay) reportWithSection(t *testing.T) string {
	t.Helper()
	return "# Build Report — cycle " + fmt.Sprint(r.cycle) + "\n\n## Changes\n- the lint and its tests\n\n" +
		"## Explanation Documentation\n- Status: REQUIRED\n- Document: " + r.documentPath(t) + "\n"
}

func failsWithOnlyTheFloorsLine(code int, out, floorLine string) bool {
	return code == 1 && strings.Contains(out, floorLine) && strings.Count(out, "Explanation Documentation: ") == 1
}

func runProbe(args ...string) (int, string) {
	var out, errb bytes.Buffer
	code := dispatch(args, nil, &out, &errb)
	return code, out.String() + errb.String()
}

func (r handoffReplay) phaseVerify() (int, string) {
	return runProbe("phase", "verify", "build", "--workspace", r.workspace)
}

func (r handoffReplay) selfcheck() (int, string) {
	return runProbe("selfcheck", "build", "--worktree", r.worktree)
}

const (
	replayLint     = "go/internal/evalqualitycheck/unsatisfiable.go"
	replayLintBody = "package evalqualitycheck\n\nfunc Unsatisfiable(src string) bool { return src == \"\" }\n"
)

func TestPhaseVerifyBuild_Replay1788_ReportWithoutTheExplanationSectionFailsWithTheFloorsMessage(t *testing.T) {
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{
		replayLint:        replayLintBody,
		r.documentPath(t): r.explanation("- `" + replayLint + "` — the AST lint that flags a positive assertion used to prove absence."),
	})
	r.writeWorkspace(t, "build-report.md", "# Build Report — cycle 1788\n\n## Changes\n- `"+replayLint+"`: the lint.\n\n## Verification\n- package tests ok.\n")
	code, out := r.phaseVerify()
	const floorLine = "Explanation Documentation: build-report.md is missing the required ## Explanation Documentation section"
	if !failsWithOnlyTheFloorsLine(code, out, floorLine) {
		t.Fatalf("cycle 1788: `phase verify build` printed OK while the floor rejected the report; the probe must fail with the floor's own line.\nexit=%d\n%s", code, out)
	}
}

func TestBuildHandoffProbes_Replay1788_CitedPathNotInTheDiffFailsBothProbes(t *testing.T) {
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{
		replayLint: replayLintBody,
		r.documentPath(t): r.explanation(
			"- `"+replayLint+"` — the AST lint that flags a positive assertion used to prove absence.",
			"- `go/internal/cli/guardcmd/eval.go` — wires the lint into `evolve eval quality-check -predicates`.",
		),
	})
	r.writeWorkspace(t, "build-report.md", r.reportWithSection(t))
	const floorLine = "Explanation Documentation: cited path go/internal/cli/guardcmd/eval.go is not in the Build diff"
	for name, probe := range map[string]func() (int, string){"phase verify build": r.phaseVerify, "selfcheck build": r.selfcheck} {
		if code, out := probe(); !failsWithOnlyTheFloorsLine(code, out, floorLine) {
			t.Errorf("cycle 1788 correction 1: `%s` must fail with the floor's line %q\nexit=%d\n%s", name, floorLine, code, out)
		}
	}
}

func TestBuildHandoffProbes_Replay1763_MaterialPathWithoutAChangedAreasBulletFailsBothProbes(t *testing.T) {
	const tagged = "go/internal/core/phase_bindings_selfcheck.go"
	const fence = "go/internal/treefence/fence.go"
	r := newHandoffReplay(t, 1763, map[string]string{
		tagged: "package core\n\nconst taggedTimeout = \"120s\"\n",
		fence:  "package treefence\n\nfunc seedIndex() int { return 0 }\n",
	})
	r.commitBuild(t, map[string]string{
		fence:             "package treefence\n\nfunc seedIndex() int { return 1 }\n",
		tagged:            "package core\n\nconst taggedTimeout = \"480s\"\n",
		r.documentPath(t): r.explanation("- `" + fence + "` — seeds the fence index from the base, so a later lane cannot inherit a stale entry."),
	})
	r.writeWorkspace(t, "build-report.md", r.reportWithSection(t))
	const floorLine = "Explanation Documentation: Changed Areas does not explain material path " + tagged
	for name, probe := range map[string]func() (int, string){"phase verify build": r.phaseVerify, "selfcheck build": r.selfcheck} {
		if code, out := probe(); !failsWithOnlyTheFloorsLine(code, out, floorLine) {
			t.Errorf("cycle 1763: `%s` said GREEN while the floor rejected a material path with no Changed Areas bullet; want the floor's line %q\nexit=%d\n%s", name, floorLine, code, out)
		}
	}
}

func TestBuildHandoffProbes_AnExplainedBuildIsGreenOnBothProbes(t *testing.T) {
	r := newHandoffReplay(t, 1788, nil)
	r.commitBuild(t, map[string]string{
		replayLint:        replayLintBody,
		r.documentPath(t): r.explanation("- `" + replayLint + "` — the AST lint that flags a positive assertion used to prove absence."),
	})
	r.writeWorkspace(t, "build-report.md", r.reportWithSection(t))
	if code, out := r.phaseVerify(); code != 0 {
		t.Errorf("an explained build must pass `phase verify build`; exit=%d\n%s", code, out)
	}
	if code, out := r.selfcheck(); code != 0 || !strings.Contains(out, "safe to hand off") {
		t.Errorf("an explained build must pass `selfcheck build`; exit=%d\n%s", code, out)
	}
}

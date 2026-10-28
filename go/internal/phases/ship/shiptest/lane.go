package shiptest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	GithubServerError = "! [remote rejected] main -> main (Internal Server Error)\n"
	GithubPolicyError = "! [remote rejected] main -> main (protected branch hook declined)\n"
	GithubPushRace    = "! [rejected]        main -> main (fetch first)\n"
	LandingCycle      = 1830
	LandingRunID      = "run-1830"
	LandingBranch     = "cycle-1830-branch"
	LandingItemID     = "cli-phase-cycle-request"
	LandingItem       = ".evolve/inbox/2026-09-30T10-51-14Z-cli-phase-cycle-request.json"
	landingGitignore  = ".evolve/\n!.evolve/\n.evolve/*\n!.evolve/inbox/\n.evolve/inbox/processed/\n.evolve/inbox/processing/\n.evolve/inbox/rejected/\n"
)

type Lane struct {
	Repo, Bare, WT, WS, Base, Audited string
	View                              *phaseio.ExplanationView
}

type PushOutage struct {
	Failures int
	Stderr   string
	Pushes   int
	OnPush   func()
}

func (o *PushOutage) Runner() sysexec.RunFunc {
	return func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "push" {
			o.Pushes++
			if o.OnPush != nil {
				o.OnPush()
			}
			if o.Pushes <= o.Failures {
				_, _ = io.WriteString(stderr, o.Stderr)
				return 1, nil
			}
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
}

type SleepLog []time.Duration

func (s *SleepLog) Sleep(d time.Duration) { *s = append(*s, d) }

func LaneReadyToLand(tb testing.TB) Lane {
	tb.Helper()
	l := Lane{Repo: MakeRepo(tb)}
	l.Bare = AddRemote(tb, l.Repo)
	MustWrite(tb, filepath.Join(l.Repo, ".gitignore"), landingGitignore)
	MustWrite(tb, filepath.Join(l.Repo, filepath.FromSlash(LandingItem)), `{"id":"`+LandingItemID+`","title":"cli phase cycle request"}`+"\n")
	RunGit(tb, l.Repo, "add", ".gitignore", LandingItem)
	RunGit(tb, l.Repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "the lane's inbox item")
	RunGit(tb, l.Repo, "push", "-q", "origin", "main")
	l.Base = HeadSHA(tb, l.Repo)
	l.WT = MakeWorktree(tb, l.Repo, LandingBranch)
	l.WS = filepath.Join(l.Repo, ".evolve", "runs", fmt.Sprintf("cycle-%d", LandingCycle))
	MustWrite(tb, filepath.Join(l.Repo, ".evolve", "cycle-state.json"), fmt.Sprintf(
		`{"cycle_id":%d,"run_id":%q,"audit_dispatches":1,"phase":"ship","active_worktree":%q,"workspace_path":%q,"worktree_base_sha":%q,"explanation_documentation_version":%d}`,
		LandingCycle, LandingRunID, l.WT, l.WS, l.Base, explanationdocs.CurrentContractVersion))
	MustWrite(tb, filepath.Join(l.WS, "triage-decision.json"), `{"schema_version":1,"top_n":[{"id":"`+LandingItemID+`"}],"deferred":[],"dropped":[]}`)
	l.View = sealTheBuildExplanation(tb, l)
	SeedAudit(tb, l.Repo, "PASS")
	l.Audited = strings.TrimSpace(RunGitOut(tb, l.WT, "write-tree"))
	return l
}

func (l Lane) binding() explanationdocs.CycleBinding {
	return explanationdocs.CycleBinding{ProjectRoot: l.Repo, Worktree: l.WT, Workspace: l.WS, BaseSHA: l.Base,
		Cycle: LandingCycle, RunID: LandingRunID, ContractVersion: explanationdocs.CurrentContractVersion}
}

func sealTheBuildExplanation(tb testing.TB, l Lane) *phaseio.ExplanationView {
	tb.Helper()
	activation := l.binding()
	activation.Worktree, activation.BaseSHA = "", ""
	check(tb, explanationdocs.Activate(activation), "activate the explanation contract")
	check(tb, explanationdocs.SealBuild(l.binding()), "seal the build")
	document, err := explanationdocs.DocumentPath(LandingCycle, LandingRunID)
	check(tb, err, "name the explanation document")
	MustWrite(tb, filepath.Join(l.WT, "fixture.txt"), "fixture line 1\nthe audited cycle work\n")
	MustWrite(tb, filepath.Join(l.WT, filepath.FromSlash(document)), buildExplanation(l.Base))
	MustWrite(tb, filepath.Join(l.WS, "build-report.md"), "## Explanation Documentation\n- Status: REQUIRED\n- Document: "+document+"\n")
	return l.loadAndSealTheResult(tb)
}

func buildExplanation(base string) string {
	return "# Build Explanation — Cycle 1830\n\n" +
		"## Build Binding\n- Cycle: 1830\n- Base SHA: " + base + "\n\n" +
		"## Summary\nThe fixture records the audited cycle work.\n\n" +
		"## Rationale\nThe tracked fixture is the smallest material change a landing can carry.\n\n" +
		"## Changed Areas\n- `fixture.txt` — adds the audited line the landing publishes.\n\n" +
		"## Design Decisions\nThe lane lands through the real worktree ship.\n\n" +
		"## Verification\nShip lands the audited commit across a failed push.\n\n" +
		"## Compatibility\nNo public interface changes.\n\n" +
		"## Limitations\nCovers the worktree landing only.\n"
}

func (l Lane) loadAndSealTheResult(tb testing.TB) *phaseio.ExplanationView {
	tb.Helper()
	check(tb, joinFailures(explanationdocs.CheckBuild(context.Background(), l.binding())), "CheckBuild")
	view, err := explanationdocs.Load(l.WS)
	check(tb, err, "load the explanation view")
	check(tb, explanationdocs.SealResult(context.Background(), l.binding()), "seal the explanation result")
	return view
}

func (l Lane) Tip(tb testing.TB) string {
	tb.Helper()
	return strings.TrimSpace(RunGitOut(tb, l.Repo, "rev-parse", LandingBranch))
}

func (l Lane) IntentFile() string {
	return filepath.Join(l.Repo, ".evolve", "landing", fmt.Sprintf("cycle-%d.json", LandingCycle))
}

func (l Lane) LockTheLandingDir(tb testing.TB) {
	tb.Helper()
	dir := filepath.Dir(l.IntentFile())
	MustMkdir(tb, dir)
	check(tb, os.Chmod(dir, 0o555), "lock %s", dir)
	tb.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func (l Lane) WriteIntentMap(tb testing.TB, in map[string]any) {
	tb.Helper()
	body, err := json.MarshalIndent(in, "", "  ")
	check(tb, err, "encode the landing intent")
	MustWrite(tb, l.IntentFile(), string(body)+"\n")
}

func ReadIntentAt(tb testing.TB, path string) (map[string]any, bool) {
	tb.Helper()
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false
	}
	check(tb, err, "read %s", path)
	var in map[string]any
	check(tb, json.Unmarshal(body, &in), "decode %s", path)
	return in, true
}

func (l Lane) Intent(tb testing.TB) map[string]any {
	tb.Helper()
	in, found := ReadIntentAt(tb, l.IntentFile())
	if !found {
		tb.Fatalf("no landing intent at %s", l.IntentFile())
	}
	return in
}

func joinFailures(failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

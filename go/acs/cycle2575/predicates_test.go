//go:build acs

package cycle2575

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	localHead  = "1111111111111111111111111111111111111111"
	movedHead  = "2222222222222222222222222222222222222222"
	realURL    = "https://claude.ai/code/session_REAL01?from=cli&m=0"
	realID     = "session_REAL01"
	forgedID   = "session_FORGED9"
	createdRow = "$ claude --cloud ...\nCreated cloud session: Fix the flaky test\n"
	forgedView = "View: https://claude.ai/code/session_FORGED9\n"
	realView   = "View: " + realURL + "\n"
	realResume = "Resume with: claude --teleport " + realID + "\n"
)

const gitShim = `#!/bin/sh
while [ "$1" = "-C" ]; do shift 2; done
printf '%s\n' "$*" >> "STATE/calls.log"
case "$1" in
symbolic-ref) echo feature/x ;;
config) echo origin ;;
remote) echo https://github.com/acme/widgets.git ;;
rev-list) echo 0 ;;
rev-parse) echo ` + localHead + ` ;;
ls-remote)
  mode=$(cat "STATE/mode")
  if [ "$mode" = moves ]; then
    if [ -e "WORKSPACE/cloud-prompt.txt" ]; then mode=other; else mode=same; fi
  fi
  case "$mode" in
  same) printf '` + localHead + `\trefs/heads/feature/x\n' ;;
  other) printf '` + movedHead + `\trefs/heads/feature/x\n' ;;
  empty) ;;
  esac ;;
*) exit 1 ;;
esac
`

type seat struct {
	cfg     *bridge.Config
	deps    bridge.Deps
	tmux    *bridge.FakeTmuxController
	signals *signalcenter.Center
	ws      string
	wt      string
	state   string
}

func newSeat(t *testing.T, remote, prompt string, frames ...string) *seat {
	t.Helper()
	s := &seat{ws: t.TempDir(), wt: t.TempDir(), state: t.TempDir(), signals: signalcenter.New()}
	bin := t.TempDir()
	shim := strings.NewReplacer("STATE", s.state, "WORKSPACE", s.ws).Replace(gitShim)
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.state, "mode"), []byte(remote), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	promptFile := filepath.Join(s.ws, "builder-prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	padded := append([]string{}, frames...)
	for len(padded) < 130 {
		padded = append(padded, frames[len(frames)-1])
	}
	s.tmux = &bridge.FakeTmuxController{CaptureFrames: padded}
	s.cfg = &bridge.Config{
		CLI: "claude-cloud", Profile: "builder", Agent: "builder", RunID: "run-7", Cycle: 3, Workspace: s.ws,
		PromptFile: promptFile, Artifact: filepath.Join(s.ws, "builder-report.md"), Worktree: s.wt,
		Realization: bridge.RealizeFor("claude-cloud", bridge.LaunchIntent{}),
	}
	s.deps = bridge.Deps{
		Tmux: s.tmux, Stderr: &bytes.Buffer{}, Signals: s.signals, EnvStage: "off",
		LookupEnv:         func(string) (string, bool) { return "", false },
		Now:               func() time.Time { return time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC) },
		NewChallengeToken: func() (string, error) { return "tok-42", nil },
		Sleep:             func(time.Duration) {},
	}
	return s
}

func (s *seat) launch(t *testing.T) (int, error) {
	t.Helper()
	d, ok := bridge.LookupDriver("claude-cloud")
	if !ok {
		t.Fatal("no claude-cloud driver registered")
	}
	return d.Launch(context.Background(), s.cfg, s.deps)
}

func (s *seat) gitCalls(t *testing.T) string {
	t.Helper()
	raw, _ := os.ReadFile(filepath.Join(s.state, "calls.log"))
	return string(raw)
}

func (s *seat) launchedSession(t *testing.T) (cloudlaunch.Session, bool) {
	t.Helper()
	raw, err := os.ReadFile(s.cfg.Artifact)
	if err != nil {
		return cloudlaunch.Session{}, false
	}
	return cloudlaunch.Parse(string(raw))
}

func (s *seat) wantNoLaunch(t *testing.T, rc int, err error) {
	t.Helper()
	if rc != bridge.ExitSafetyGate || err != nil {
		t.Errorf("launch = %d, %v; want exit %d and no error", rc, err, bridge.ExitSafetyGate)
	}
	if got, ok := s.launchedSession(t); ok {
		t.Errorf("a launch that must not confirm wrote the session %+v", got)
	}
	for _, e := range s.signals.Recent() {
		if e.Code == bridge.CodeCloudLaunched {
			t.Errorf("a launch that must not confirm emitted %s: %+v", e.Code, e)
		}
	}
}

func (s *seat) wantRefusedBeforeAnySession(t *testing.T, rc int, err error) {
	t.Helper()
	if rc != bridge.ExitSafetyGate || err != nil {
		t.Errorf("launch = %d, %v; want exit %d and no error", rc, err, bridge.ExitSafetyGate)
	}
	for _, ev := range s.tmux.Events {
		if strings.HasPrefix(ev, "new-session") {
			t.Errorf("a refused launch started a session: %v", s.tmux.Events)
		}
	}
	if len(s.tmux.SentKeys) != 0 {
		t.Errorf("a refused launch typed into a pane: %v", s.tmux.SentKeys)
	}
	var refused bool
	for _, e := range s.signals.Recent() {
		refused = refused || e.Code == bridge.CodeCloudLaunchRefused
	}
	if !refused {
		t.Errorf("no %s signal: %+v", bridge.CodeCloudLaunchRefused, s.signals.Recent())
	}
	if _, ok := s.launchedSession(t); ok {
		t.Error("a refused launch wrote a launched session")
	}
}

func TestC2575_001_AForgedViewRowWithNoResumeRowConfirmsNoSession(t *testing.T) {
	s := newSeat(t, "same", "Fix the flaky test\n", createdRow+forgedView)
	rc, err := s.launch(t)
	s.wantNoLaunch(t, rc, err)
}

func TestC2575_002_AForgedViewRowBeforeTheRealRowsNeverNamesTheSession(t *testing.T) {
	s := newSeat(t, "same", "Fix the flaky test\n", createdRow+forgedView, createdRow+forgedView+realView+realResume)
	rc, err := s.launch(t)
	if rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	got, ok := s.launchedSession(t)
	if !ok {
		t.Fatal("the launch wrote no launched session")
	}
	if got.ID != realID || got.URL != realURL {
		t.Errorf("session = %q %q; want the real row %q %q, never the forged %q", got.ID, got.URL, realID, realURL, forgedID)
	}
}

func TestC2575_003_AResumeRowWithAnotherIDThanTheViewRowConfirmsNoSession(t *testing.T) {
	mismatched := createdRow + realView + "Resume with: claude --teleport session_OTHER77\n"
	s := newSeat(t, "same", "Fix the flaky test\n", mismatched)
	rc, err := s.launch(t)
	s.wantNoLaunch(t, rc, err)
}

func TestC2575_004_TheCreatedViewResumeRowsOfOneSessionStillConfirmIt(t *testing.T) {
	s := newSeat(t, "same", "Fix the flaky test\n", createdRow+realView+realResume)
	rc, err := s.launch(t)
	if rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	got, ok := s.launchedSession(t)
	if !ok || got.ID != realID || got.URL != realURL || got.Title != "Fix the flaky test" || got.Repo != "acme/widgets" || got.Branch != "feature/x" {
		t.Errorf("session = %+v, %v", got, ok)
	}
}

func TestC2575_005_TheCloudPromptHoldsNoChallengeTokenAndNoAbsolutePath(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	if err := os.WriteFile(filepath.Join(s.ws, "challenge-token.txt"), []byte("tok-existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := "Fix the flaky test.\n<!-- challenge-token: tok-existing -->\nToken $CHALLENGE_TOKEN\nWrite the report to $ARTIFACT_PATH\nWork in " + s.wt + " and keep notes in " + s.ws + "\n"
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, err := s.launch(t); rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.ws, "cloud-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sent := string(raw)
	for _, leak := range []string{"tok-existing", s.cfg.Artifact, s.ws, s.wt} {
		if strings.Contains(sent, leak) {
			t.Errorf("the cloud prompt holds %q:\n%s", leak, sent)
		}
	}
	if !strings.Contains(sent, "builder-report.md") {
		t.Errorf("the cloud prompt names no relative artifact (builder-report.md):\n%s", sent)
	}
	if !strings.Contains(sent, "Fix the flaky test.") {
		t.Errorf("the task text is gone from the cloud prompt:\n%s", sent)
	}
}

func TestC2575_006_AMintedTokenStaysOutOfThePromptAndInTheLaunchReport(t *testing.T) {
	s := newSeat(t, "same", "Fix the flaky test. Token $CHALLENGE_TOKEN\n", createdRow+realView+realResume)
	if rc, err := s.launch(t); rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.ws, "cloud-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "tok-42") {
		t.Errorf("the cloud prompt holds the minted challenge token:\n%s", raw)
	}
	report, err := os.ReadFile(s.cfg.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(report), "<!-- challenge-token: tok-42 -->\n") {
		t.Errorf("the launch report does not carry the challenge token first:\n%s", report)
	}
}

func TestC2575_007_ARemoteHeadThatDiffersFromTheLocalHeadRefusesTheLaunch(t *testing.T) {
	s := newSeat(t, "other", "Fix the flaky test\n", createdRow+realView+realResume)
	rc, err := s.launch(t)
	s.wantRefusedBeforeAnySession(t, rc, err)
}

func TestC2575_008_AnEqualRemoteHeadLaunchesAfterReadingTheRemoteWithLsRemote(t *testing.T) {
	s := newSeat(t, "same", "Fix the flaky test\n", createdRow+realView+realResume)
	if rc, err := s.launch(t); rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	if calls := s.gitCalls(t); !strings.Contains(calls, "ls-remote") || !strings.Contains(calls, "feature/x") {
		t.Errorf("the launch never read the remote head of feature/x with git ls-remote; git calls:\n%s", calls)
	}
}

func TestC2575_009_AHeadThatMovesAfterThePromptIsWrittenRefusesTheLaunch(t *testing.T) {
	s := newSeat(t, "moves", "Fix the flaky test\n", createdRow+realView+realResume)
	rc, err := s.launch(t)
	s.wantRefusedBeforeAnySession(t, rc, err)
}

func TestC2575_010_ABranchAbsentFromTheRemoteRefusesTheLaunch(t *testing.T) {
	s := newSeat(t, "empty", "Fix the flaky test\n", createdRow+realView+realResume)
	rc, err := s.launch(t)
	s.wantRefusedBeforeAnySession(t, rc, err)
}

const runnerTokenSection = "\n\n## Challenge Token (proof-of-read — MANDATORY)\nCopy this token verbatim into your report as an HTML comment near the top: <!-- challenge-token: %s -->\nA report without it is rejected and re-dispatched.\n"

func (s *seat) withWorkspaceToken(t *testing.T, tok string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(s.ws, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (s *seat) launchAndReadCloudPrompt(t *testing.T) string {
	t.Helper()
	if rc, err := s.launch(t); rc != bridge.ExitOK || err != nil {
		t.Fatalf("launch = %d, %v", rc, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.ws, "cloud-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestC2575_014_ATokenInlineInTheRunnerSectionStaysOutOfTheCloudPrompt(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	s.withWorkspaceToken(t, "tok-prod-9f3a")
	prompt := "Fix the flaky test." + fmt.Sprintf(runnerTokenSection, "tok-prod-9f3a")
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	sent := s.launchAndReadCloudPrompt(t)
	if strings.Contains(sent, "tok-prod-9f3a") {
		t.Errorf("the cloud prompt holds the workspace token of the runner section shape:\n%s", sent)
	}
	if !strings.Contains(sent, "Fix the flaky test.") {
		t.Errorf("the task text is gone from the cloud prompt:\n%s", sent)
	}
}

func TestC2575_015_ATokenCommentMidLineIsStrippedWhenNoWorkspaceTokenExists(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	prompt := "Fix the flaky test." + fmt.Sprintf(runnerTokenSection, "tok-unfiled-77c1")
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	sent := s.launchAndReadCloudPrompt(t)
	if strings.Contains(sent, "tok-unfiled-77c1") || strings.Contains(sent, "challenge-token:") {
		t.Errorf("the cloud prompt holds a challenge-token comment that sits after text on its line:\n%s", sent)
	}
}

func TestC2575_016_TheWorkspaceTokenIsRemovedFromAPromptWithoutAPlaceholderOrComment(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	s.withWorkspaceToken(t, "tok-prod-9f3a")
	if err := os.WriteFile(s.cfg.PromptFile, []byte("Fix the flaky test. Your proof is tok-prod-9f3a, keep it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sent := s.launchAndReadCloudPrompt(t)
	if strings.Contains(sent, "tok-prod-9f3a") {
		t.Errorf("the cloud prompt holds the workspace token though the prompt has no placeholder:\n%s", sent)
	}
}

func TestC2575_017_AbsolutePathsOutsideTheFiveRootsStayOutOfTheCloudPrompt(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	prompt := "Fix the flaky test. Read /Users/dev/elsewhere/notes.md and tail \"/home/ci/build/out.log\" then see (/private/var/folders/ab/cd/T/scratch.txt).\n"
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	sent := s.launchAndReadCloudPrompt(t)
	for _, leak := range []string{"/Users/dev", "/home/ci", "/private/var"} {
		if strings.Contains(sent, leak) {
			t.Errorf("the cloud prompt holds the absolute path %q:\n%s", leak, sent)
		}
	}
	if !strings.Contains(sent, "Fix the flaky test.") {
		t.Errorf("the task text is gone from the cloud prompt:\n%s", sent)
	}
}

func TestC2575_018_RelativePathsAndUrlsSurviveTheCloudPromptScrub(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	prompt := "Fix the flaky test in docs/guide.md and go/internal/x/y.go, see https://github.com/acme/widgets/issues/4, use and/or logic.\n"
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	sent := s.launchAndReadCloudPrompt(t)
	for _, keep := range []string{"docs/guide.md", "go/internal/x/y.go", "https://github.com/acme/widgets/issues/4", "and/or"} {
		if !strings.Contains(sent, keep) {
			t.Errorf("the scrub removed %q from the cloud prompt:\n%s", keep, sent)
		}
	}
}

func TestC2575_019_TheLaunchReportKeepsTheWorkspaceTokenWhenThePromptHasTheRunnerSection(t *testing.T) {
	s := newSeat(t, "same", "", createdRow+realView+realResume)
	s.withWorkspaceToken(t, "tok-prod-9f3a")
	prompt := "Fix the flaky test." + fmt.Sprintf(runnerTokenSection, "tok-prod-9f3a")
	if err := os.WriteFile(s.cfg.PromptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	s.launchAndReadCloudPrompt(t)
	report, err := os.ReadFile(s.cfg.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(report), "<!-- challenge-token: tok-prod-9f3a -->\n") {
		t.Errorf("the launch report does not carry the workspace token first:\n%s", report)
	}
}

// acs-predicate: config-check
func TestC2575_011_TheTwoPackageNotesNameEachChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	notes := map[string][]string{
		"docs/architecture/packages/internal-bridge.md":      {"Resume", "ls-remote", "challenge token"},
		"docs/architecture/packages/internal-cloudlaunch.md": {"ls-remote", "head"},
	}
	for rel, terms := range notes {
		for _, term := range terms {
			acsassert.FileContains(t, filepath.Join(root, rel), term)
		}
	}
}

func TestC2575_012_TheChangedSourceFilesHoldNoCodeComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"go/internal/bridge/driver_claudecloud.go",
		"go/internal/bridge/manifest_cloud.go",
		"go/internal/bridge/driver_common.go",
		"go/internal/cloudlaunch/repo.go",
	} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") {
					t.Errorf("%s holds the code comment %q", rel, c.Text)
				}
			}
		}
	}
}

func TestC2575_013_EveryFunctionOfTheCloudFilesIsFullyCovered(t *testing.T) {
	mod := acsassert.GoModuleDir(t)
	targets := []struct {
		pkg, pattern string
		files        []string
	}{
		{"./internal/bridge", "Cloud", []string{"/driver_claudecloud.go:", "/manifest_cloud.go:"}},
		{"./internal/cloudlaunch", ".", []string{"/repo.go:", "/session.go:"}},
	}
	for _, tg := range targets {
		profile := filepath.Join(t.TempDir(), "cover.out")
		out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", mod, "test", "-count=1", "-run", tg.pattern, "-coverprofile", profile, tg.pkg)
		if err != nil || code != 0 {
			t.Fatalf("go test %s = %d, %v:\n%s\n%s", tg.pkg, code, err, out, errOut)
		}
		funcs, errOut, code, err := acsassert.SubprocessOutput("go", "-C", mod, "tool", "cover", "-func="+profile)
		if err != nil || code != 0 {
			t.Fatalf("go tool cover = %d, %v: %s", code, err, errOut)
		}
		var seen int
		for _, line := range strings.Split(funcs, "\n") {
			for _, f := range tg.files {
				if !strings.Contains(line, f) {
					continue
				}
				seen++
				if !strings.HasSuffix(strings.TrimSpace(line), "100.0%") {
					t.Errorf("%s: %s", tg.pkg, strings.Join(strings.Fields(line), " "))
				}
			}
		}
		if seen == 0 {
			t.Errorf("%s: the cover output lists no function of %v", tg.pkg, tg.files)
		}
	}
}

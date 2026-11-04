package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	CodeCloudLaunched      signalcenter.Code = "BRIDGE_CLOUD_LAUNCHED"
	CodeCloudLaunchRefused signalcenter.Code = "BRIDGE_CLOUD_LAUNCH_REFUSED"
)

const (
	cloudSessionPrefix  = "evolve-cloud-"
	cloudPromptFile     = "cloud-prompt.txt"
	cloudPaneFile       = "cloud-final-pane.txt"
	cloudScrollback     = 200
	cloudLaunchOrigin   = "launchCloud"
	cloudMaxPromptBytes = 120 << 10
	cloudTaskPrefix     = "Task:\n"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeCloudLaunched, "a cloud seat started a cloud session: the CLI printed the session line and the view line within the deadline of its manifest; the dispatch outcome is launched and the deliverable is a later pull request of that session; fields cli, session_id, url, title, repo, branch, profile")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeCloudLaunchRefused, "a cloud seat refused a launch before it started a session: the working tree is on no branch, the branch has no upstream or has commits that are not pushed, or the remote is not on a host that the manifest names; the reason names the cause; fields cli, profile, dir")
}

type claudeCloudDriver struct{}

func (claudeCloudDriver) Name() string { return "claude-cloud" }

func (d claudeCloudDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, refused := claudeCredentialGate(cfg, deps, "["+d.Name()+"]"); refused {
		return rc, nil
	}
	m, err := LoadManifest(d.Name())
	if err != nil {
		return ExitBadFlags, err
	}
	return launchCloud(ctx, cfg, deps, m)
}

type cloudLaunch struct {
	cfg      *Config
	deps     Deps
	m        Manifest
	patterns cloudPatterns
	prefix   string
	dir      string
}

func launchCloud(ctx context.Context, cfg *Config, deps Deps, m Manifest) (int, error) {
	l := cloudLaunch{cfg: cfg, deps: deps, m: m, prefix: "[" + m.CLI + "]", dir: cloudWorkingDir(cfg)}
	if m.CloudLaunch == nil {
		return ExitBadFlags, fmt.Errorf("%s the manifest declares no cloud_launch", l.prefix)
	}
	var err error
	if l.patterns, err = m.CloudLaunch.patterns(m.CLI); err != nil {
		return ExitBadFlags, err
	}
	if err := ensureDirs(cfg); err != nil {
		return ExitBadFlags, fmt.Errorf("%s %w", l.prefix, err)
	}
	if err := os.Remove(cfg.Artifact); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ExitBadFlags, fmt.Errorf("%s clear the artifact of an earlier seat: %w", l.prefix, err)
	}
	repo, err := deps.cloudRepo(ctx, l.dir)
	if err == nil {
		err = repo.Check(m.CloudLaunch.RemoteHosts)
	}
	var refusal *cloudlaunch.Refusal
	if errors.As(err, &refusal) {
		return l.refuse(refusal), nil
	}
	if err != nil {
		return ExitBadFlags, fmt.Errorf("%s read the repository of %s: %w", l.prefix, l.dir, err)
	}
	promptFile, err := l.writePrompt()
	if err != nil {
		return ExitBadFlags, err
	}
	confirmed, rc, err := l.start(ctx, promptFile)
	if rc != ExitOK {
		return rc, err
	}
	return l.record(repo, confirmed)
}

func cloudWorkingDir(cfg *Config) string {
	if cfg.Worktree != "" {
		return cfg.Worktree
	}
	if cfg.ProjectRoot != "" {
		return cfg.ProjectRoot
	}
	cwd, _ := os.Getwd()
	return cwd
}

func (deps Deps) cloudRepo(ctx context.Context, dir string) (cloudlaunch.Repo, error) {
	if deps.CloudRepo != nil {
		return deps.CloudRepo(ctx, dir)
	}
	return cloudlaunch.ReadRepo(ctx, sysexec.DefaultRunner, dir)
}

func (l cloudLaunch) refuse(err *cloudlaunch.Refusal) int {
	fmt.Fprintf(l.deps.Stderr, "%s safety gate: launch refused: %v\n", l.prefix, err)
	l.emit(signalcenter.Event{
		Code: CodeCloudLaunchRefused, Kind: signalcenter.KindBridgeWarning, Severity: signalcenter.SeverityWarn, Reason: err.Error(),
		Fields: map[string]string{"cli": l.m.CLI, "profile": l.cfg.Profile, "dir": l.dir},
	})
	return ExitSafetyGate
}

func (l cloudLaunch) writePrompt() (string, error) {
	prompt, err := resolvePromptText(l.cfg, l.deps)
	if err != nil {
		return "", fmt.Errorf("%s %w", l.prefix, err)
	}
	if len(prompt) > cloudMaxPromptBytes {
		return "", fmt.Errorf("%s the prompt has %d bytes, over the argument limit of %d: the pane passes it as one argument of the CLI", l.prefix, len(prompt), cloudMaxPromptBytes)
	}
	path := filepath.Join(l.cfg.Workspace, cloudPromptFile)
	if err := os.WriteFile(path, []byte(cloudTaskArgument(prompt)), 0o600); err != nil {
		return "", fmt.Errorf("%s write the cloud prompt: %w", l.prefix, err)
	}
	return path, nil
}

func cloudTaskArgument(prompt string) string {
	if strings.HasPrefix(strings.TrimSpace(prompt), "-") {
		return cloudTaskPrefix + prompt
	}
	return prompt
}

func (l cloudLaunch) start(ctx context.Context, promptFile string) (cloudConfirmation, int, error) {
	session, _ := resolveSession(&Config{Agent: l.cfg.Agent, RunID: l.cfg.RunID, Cycle: l.cfg.Cycle}, l.deps, cloudSessionPrefix)
	if err := l.deps.startSession(ctx, session, l.dir, l.deps.paneEnv(l.cfg)); err != nil {
		return cloudConfirmation{}, ExitBadFlags, fmt.Errorf("%s new-session: %w", l.prefix, err)
	}
	defer tmuxCleanup(ctx, l.deps, l.m.CLI, session, paneCapture{shared: filepath.Join(l.cfg.Workspace, cloudPaneFile), lines: cloudScrollback}, false)
	if err := primePaneShell(ctx, l.deps, l.cfg, session, l.dir); err != nil {
		return cloudConfirmation{}, ExitBadFlags, fmt.Errorf("%s %w", l.prefix, err)
	}
	if err := l.deps.Tmux.SendKeys(ctx, session, l.command(promptFile), true); err != nil {
		return cloudConfirmation{}, ExitBadFlags, fmt.Errorf("%s send the launch to session %s: %w", l.prefix, session, err)
	}
	fmt.Fprintf(l.deps.Stderr, "%s launching in the private session %s\n", l.prefix, session)
	return l.await(ctx, session)
}

func (l cloudLaunch) command(promptFile string) string {
	flags := append(slices.Clone(l.cfg.Realization.LaunchFlags), l.m.CloudLaunch.Args...)
	return launchCmdLine(resolveBinary(l.deps, l.m.Binary), flags) + ` "$(cat ` + shellQuotePOSIX(promptFile) + `)"`
}

func (l cloudLaunch) await(ctx context.Context, session string) (cloudConfirmation, int, error) {
	deadline := l.m.CloudLaunch.DeadlineS
	for range deadline {
		l.deps.Sleep(time.Second)
		if err := ctx.Err(); err != nil {
			return cloudConfirmation{}, ExitSafetyGate, err
		}
		pane, err := l.capture(ctx, session)
		if err != nil && !l.deps.Tmux.HasSession(ctx, session) {
			fmt.Fprintf(l.deps.Stderr, "%s FAIL: the pane of session %s is gone: %v; the state of the launch is unknown, so read the cloud sessions of the account before a new launch\n", l.prefix, session, err)
			return cloudConfirmation{}, ExitSafetyGate, nil
		}
		if err != nil {
			continue
		}
		pane = stripANSI(pane)
		if c, ok := l.patterns.confirmation(pane); ok {
			return c, ExitOK, nil
		}
		if reason, failed := l.patterns.failure(pane); failed {
			fmt.Fprintf(l.deps.Stderr, "%s FAIL: the CLI refused the launch: %s\n", l.prefix, reason)
			return cloudConfirmation{}, ExitBadFlags, nil
		}
	}
	fmt.Fprintf(l.deps.Stderr, "%s FAIL: no session confirmation within %ds; the state of the launch is unknown, so read the cloud sessions of the account before a new launch\n", l.prefix, deadline)
	return cloudConfirmation{}, ExitSafetyGate, nil
}

func (l cloudLaunch) capture(ctx context.Context, session string) (string, error) {
	if j, ok := l.deps.Tmux.(JoinedPaneCapturer); ok {
		return j.CapturePaneJoined(ctx, session, cloudScrollback)
	}
	return l.deps.Tmux.CapturePane(ctx, session, cloudScrollback)
}

func (l cloudLaunch) record(repo cloudlaunch.Repo, c cloudConfirmation) (int, error) {
	s := cloudlaunch.Session{
		Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: c.id, URL: c.url, Title: c.title,
		Repo: cloudlaunch.RepoSlug(repo.URL), Branch: repo.Branch, Profile: l.cfg.Profile, CLI: l.m.CLI,
		TS: l.deps.Now().UTC().Format(time.RFC3339),
	}
	fmt.Fprintf(l.deps.Stderr, "%s launched cloud session %s: %s\n", l.prefix, s.ID, s.URL)
	l.emit(signalcenter.Event{
		Code: CodeCloudLaunched, Kind: signalcenter.KindPhaseDispatched, Severity: signalcenter.SeverityInfo,
		Reason: fmt.Sprintf("cloud session %s launched on %s, branch %s; its deliverable is a later pull request", s.ID, s.Repo, s.Branch),
		Fields: map[string]string{"cli": s.CLI, "session_id": s.ID, "url": s.URL, "title": s.Title, "repo": s.Repo, "branch": s.Branch, "profile": s.Profile},
	})
	challenge, _ := phasecontract.ChallengeToken(l.cfg.Workspace)
	if err := os.WriteFile(l.cfg.Artifact, []byte(cloudlaunch.Render(s, challenge)), 0o644); err != nil {
		return ExitBadFlags, fmt.Errorf("%s cloud session %s (%s) is launched, but its report was not written: %w", l.prefix, s.ID, s.URL, err)
	}
	return ExitOK, nil
}

func (l cloudLaunch) emit(e signalcenter.Event) {
	id := configIdentity(l.cfg)
	e.Cycle, e.RunID, e.Phase = id.cycle, id.runID, id.phase
	e.Module, e.Origin = signalcenter.ModuleBridge, cloudLaunchOrigin
	l.deps.Signals.Emit(e)
}

func init() { Register(claudeCloudDriver{}) }

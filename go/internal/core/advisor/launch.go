package advisor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// launch captures before the caller parses, so an unparseable response is still recorded and replayable.
func (a *Advisor) launch(in router.RouteInput, d decision, prompt string) (LaunchResponse, error) {
	if err := a.preflight(in, d); err != nil {
		a.warn(in, d, CodeLaunchFailed, err.Error(), map[string]string{"step": stepPreflight})
		return LaunchResponse{}, err
	}
	resp, err := a.dispatch(in, d, a.profilePath(in), prompt)
	if err != nil {
		return LaunchResponse{}, err
	}
	a.capture(in, d, prompt, resp)
	return resp, nil
}

// preflight's depth check is defense in depth; the primary recursion guard is the mint denylist.
func (a *Advisor) preflight(in router.RouteInput, d decision) error {
	if a.launcher == nil {
		return fmt.Errorf("%s: nil bridge", d.errPfx())
	}
	if in.Workspace == "" {
		return fmt.Errorf("%s: empty workspace", d.errPfx())
	}
	if a.checkDepth != nil && a.checkDepth(in.Env) {
		return fmt.Errorf("%s: recursion guard: depth check failed", d.errPfx())
	}
	return nil
}

func (a *Advisor) profilePath(in router.RouteInput) string {
	profile := a.identity.Profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(paths.EvolveDirOf(in.ProjectRoot), "profiles", "router.json")
	}
	return profile
}

// dispatch makes identity.CLI, not the profile's cli, the primary so the composition root's bench-aware swap holds.
func (a *Advisor) dispatch(in router.RouteInput, d decision, profile, prompt string) (LaunchResponse, error) {
	plan := llmroute.ChainFor(a.identity.CLI, a.profileFor(in, d, profile))
	var resp LaunchResponse
	dispatched := llmroute.Dispatch(plan, func(cli string) (int, error) {
		var launchErr error
		resp, launchErr = a.launchOnce(in, d, cli, profile, prompt)
		return resp.ExitCode, launchErr
	})
	if dispatched.Err != nil {
		err := fmt.Errorf("%s: bridge launch: %w", d.errPfx(), dispatched.Err)
		a.warn(in, d, CodeLaunchFailed, err.Error(), map[string]string{
			"step": stepDispatch, "cli": a.identity.CLI, "chain": strings.Join(dispatched.Attempts, ","),
			"exit_code": strconv.Itoa(resp.ExitCode), "profile": profile,
		})
		return LaunchResponse{}, err
	}
	return resp, nil
}

// profileFor keeps an absent profile silent; any other fault is reported because the fallback chain is then not in force.
func (a *Advisor) profileFor(in router.RouteInput, d decision, path string) *profiles.Profile {
	if path == "" {
		return nil
	}
	prof, err := a.loadProfile(path)
	if err == nil {
		return prof
	}
	if !errors.Is(err, fs.ErrNotExist) {
		a.warn(in, d, CodeProfileLoadFailed, err.Error(), map[string]string{"step": stepDispatch, "path": path})
	}
	return nil
}

func defaultProfileLoader(path string) (*profiles.Profile, error) {
	dir := filepath.Dir(path)
	name := strings.TrimSuffix(filepath.Base(path), ".json")
	prof, err := profiles.NewFromDir(dir).Get(name)
	if err != nil {
		return nil, err
	}
	return &prof, nil
}

func (a *Advisor) launchOnce(in router.RouteInput, d decision, cli, profile, prompt string) (LaunchResponse, error) {
	return a.launcher.Launch(context.Background(), a.launchRequestFor(in, d, cli, profile, prompt))
}

// launchRequestFor falls back to the workspace as the worktree: the tmux driver refuses an empty one under EVOLVE_FLEET.
func (a *Advisor) launchRequestFor(in router.RouteInput, d decision, cli, profile, prompt string) LaunchRequest {
	worktree := in.ActiveWorktree
	if worktree == "" {
		worktree = in.Workspace
	}
	return LaunchRequest{
		CLI:          cli,
		Profile:      profile,
		Model:        a.identity.Model,
		Skills:       a.resolveOverlays(cli),
		Prompt:       prompt,
		Workspace:    in.Workspace,
		Worktree:     worktree,
		ProjectRoot:  in.ProjectRoot,
		ArtifactPath: filepath.Join(in.Workspace, d.artifactFile()),
		Completion:   d.completion(),
		Agent:        a.identity.AgentLabel,
		Contract:     d.contractID(),
		Cycle:        in.Cycle,
		Env:          in.Env,
	}
}

// resolveOverlays passes the tier-shaped Model as both Model and Tier. The default uses the zero
// policy, so the operator's policy.json overlays do not govern the advisor's own dispatch.
func (a *Advisor) resolveOverlays(cli string) []string {
	if a.overlays != nil {
		return a.overlays(cli)
	}
	return policy.Policy{}.ResolveOverlays(
		policy.DispatchFromPhaseRequest(a.identity.AgentLabel, cli, a.identity.Model, a.identity.Model))
}

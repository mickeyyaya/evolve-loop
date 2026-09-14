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

// launch is the shared wiring for the three decisions: the preflight guards
// (nil launcher → empty workspace → depth, in that order), the router
// profile path, the fallback-chain dispatch, then the redacted capture
// BEFORE the caller parses — so the decision is debuggable and replayable
// even when the response is unparseable. A preflight or dispatch failure is
// one ADVISOR_LAUNCH_FAILED and the error the caller degrades on.
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

// preflight refuses a launch that cannot proceed: no launcher, no workspace,
// or the WS1-S2 recursion guard (defense-in-depth, ADR-0052 §4.3 — the
// PRIMARY guard is the mint denylist in MintConfigsFrom) signalling a nested
// invocation, degrading the cycle to the static path rather than nesting
// brains.
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

// profilePath is the router profile the launch names: the identity's
// explicit profile, else <root>/.evolve/profiles/router.json projected from
// the ONE spelling of the .evolve layout, else "" (no project root).
func (a *Advisor) profilePath(in router.RouteInput) string {
	profile := a.identity.Profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(paths.EvolveDirOf(in.ProjectRoot), "profiles", "router.json")
	}
	return profile
}

// dispatch walks [identity.CLI]+profile.cli_fallback via llmroute.Dispatch —
// the SAME chain-walk the runner uses for every ordinary phase — instead of a
// single un-fallback-able launch. identity.CLI (not the profile's cli) is the
// explicit primary so the composition root's bench-aware CLI swap is honored.
// Chain exhaustion is one ADVISOR_LAUNCH_FAILED naming the primary, every CLI
// tried, the last exit code and the profile.
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

// profileFor reads the router profile the chain walk needs. An empty path
// (no explicit profile, no project root) reads nothing; an absent file is
// the documented single-candidate degrade and stays silent; any other read
// or parse fault is ADVISOR_PROFILE_LOAD_FAILED — the chain still degrades to
// the single primary exactly as before, but the operator learns their
// fallback chain is not in force.
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

// defaultProfileLoader parses the router profile at path into a
// profiles.Profile so llmroute.ChainFor can read its cli_fallback chain.
func defaultProfileLoader(path string) (*profiles.Profile, error) {
	dir := filepath.Dir(path)
	name := strings.TrimSuffix(filepath.Base(path), ".json")
	prof, err := profiles.NewFromDir(dir).Get(name)
	if err != nil {
		return nil, err
	}
	return &prof, nil
}

// launchOnce is one attempt of the chain walk through the Launcher port.
func (a *Advisor) launchOnce(in router.RouteInput, d decision, cli, profile, prompt string) (LaunchResponse, error) {
	return a.launcher.Launch(context.Background(), a.launchRequestFor(in, d, cli, profile, prompt))
}

// launchRequestFor builds the fourteen-field request for one attempt. The
// worktree is the cycle's active worktree, else the workspace (the tmux
// driver refuses an empty worktree under EVOLVE_FLEET); the artifact path is
// the ABSOLUTE workspace artifact the prompt also names; the skill overlays
// are resolved per attempt for the attempted CLI.
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

// resolveOverlays names the skill overlays for one attempted CLI on the SAME
// contract as an ordinary phase dispatch (phases/runner): the identity's
// Model is a tier-shaped string ("deep"/"top" under the composition root's
// WithProposerModel), so it is passed as both the Model and Tier selector. The
// default resolves the compiled policy default (deep/top→fable) over the zero
// policy — the operator's policy.json overlay block does not govern the
// advisor's own dispatch (preserved; the resolver is the seam to change it).
func (a *Advisor) resolveOverlays(cli string) []string {
	if a.overlays != nil {
		return a.overlays(cli)
	}
	return policy.Policy{}.ResolveOverlays(
		policy.DispatchFromPhaseRequest(a.identity.AgentLabel, cli, a.identity.Model, a.identity.Model))
}

package bridge

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
)

const seatTokenFileNotIsolated = "bridge: --seat-token-file is set, but this CLI does not run an isolated seat, so the flag cannot apply. Next: remove --seat-token-file, or route the profile to a claude seat"

const (
	seatDirMode       = 0o700
	seatConfigDirEnv  = "CLAUDE_CONFIG_DIR"
	seatOAuthTokenEnv = "CLAUDE_CODE_OAUTH_TOKEN"
	claudeBinary      = "claude"
	seatSettingsName  = paths.SeatSettingsName
)

var seatLoginFallback = seatlogin.OS

func (deps Deps) seatLoginDeps() seatlogin.Deps {
	if deps.SeatLogin.Stat == nil || deps.SeatLogin.ReadFile == nil {
		return seatLoginFallback()
	}
	return deps.SeatLogin
}

func observeSeatLogin(deps Deps, login string) {
	if deps.onSeatLogin != nil {
		deps.onSeatLogin(login)
	}
}

func (deps Deps) resolveSeat(cfg *Config) (int, error) {
	m, err := loadManifestRaw(cfg.CLI)
	if err != nil {
		return notIsolatedSeat(cfg)
	}
	home, _ := os.UserHomeDir()
	if m.SeatLogin != SeatLoginIsolated {
		if cfg.SeatTokenFile != "" {
			return ExitSeatTokenRefused, errors.New(seatTokenFileNotIsolated)
		}
		d := seatlogin.Resolve(home, "", deps.seatLoginDeps())
		cfg.Decision = seatlogin.Decision{HostIsolated: d.HostIsolated}
		if m.Binary == claudeBinary {
			observeSeatLogin(deps, SeatLoginStored)
		}
		return ExitOK, nil
	}
	d := seatlogin.Resolve(home, cfg.SeatTokenFile, deps.seatLoginDeps())
	if d.Refusal == "" && d.SeatToken != "" {
		d.Refusal = deps.seatConfinementRefusal()
	}
	if d.Refusal == "" && d.SeatToken != "" {
		if err := provisionSeatSettings(home, d.Dir); err != nil {
			d.Refusal = fmt.Sprintf("bridge: the seat directory %s cannot be provisioned: %v, so no claude seat can start.", d.Dir, err)
		}
	}
	if d.Refusal != "" {
		return ExitSeatTokenRefused, errors.New(d.Refusal)
	}
	cfg.Decision = d
	if d.SeatToken == "" {
		observeSeatLogin(deps, SeatLoginStored)
		return ExitOK, nil
	}
	cfg.RequireSandbox = true
	observeSeatLogin(deps, SeatLoginIsolated)
	return ExitOK, nil
}

func notIsolatedSeat(cfg *Config) (int, error) {
	if cfg.SeatTokenFile != "" {
		return ExitSeatTokenRefused, errors.New(seatTokenFileNotIsolated)
	}
	return ExitOK, nil
}

func (deps Deps) seatConfinementRefusal() string {
	switch {
	case strings.TrimSpace(deps.SandboxMode) == config.SandboxModeOff:
		return "bridge: a seat token is set, so a claude seat cannot start with --sandbox-off: the token must not reach an unconfined pane. Next: run without --sandbox-off, or run evolve auth seat-token clear to go back to the stored login."
	case deps.envStage() == envStageOff:
		return "bridge: a seat token is set and isolation.i2 is off, so no claude seat can start: the token travels in the hermetic environment only. Next: set isolation.i2 to shadow or enforce, or run evolve auth seat-token clear to go back to the stored login."
	}
	return ""
}

func seatEnv(cliEnv map[string]string, d seatlogin.Decision) map[string]string {
	out := make(map[string]string, len(cliEnv)+2)
	for k, v := range cliEnv {
		out[k] = v
	}
	out[seatConfigDirEnv] = d.Dir
	out[seatOAuthTokenEnv] = d.SeatToken
	return out
}

func markSeatLogin(run launchRun) func(*llmcalls.Record) {
	return func(rec *llmcalls.Record) { rec.SeatLogin = run.seatLogin }
}

func (deps Deps) recipeSeatRefusal(cfg *Config) error {
	if cfg.Worktree == "" {
		return nil
	}
	home, _ := os.UserHomeDir()
	if d := seatlogin.Resolve(home, "", deps.seatLoginDeps()); d.HostIsolated {
		return errors.New("bridge: a recipe with --worktree cannot run while the claude seat is isolated: the recipe pane would use the stored login. Next: run the recipe without --worktree, or run evolve auth seat-token clear to go back to the stored login")
	}
	return nil
}

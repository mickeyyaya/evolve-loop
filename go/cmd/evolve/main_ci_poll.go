package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	errOriginMainLSRemote = errors.New("ls-remote origin main")
	errNoOriginMainRef    = errors.New("no refs/heads/main on origin")
)

type mainCIPoll struct {
	projectRoot string
	every       time.Duration
	originMain  func(ctx context.Context) (string, error)
	ci          func(ctx context.Context, projectRoot, sha string) (red bool, failed []string, err error)
	last        time.Time
}

func newMainCIPoll(projectRoot string, every time.Duration) *mainCIPoll {
	return &mainCIPoll{projectRoot: projectRoot, every: every, originMain: originMainTip(projectRoot), ci: mainCIRedForSHA}
}

func newLoopMainCIPoll(projectRoot string) *mainCIPoll {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		pol = policy.Policy{}
	}
	return newMainCIPoll(projectRoot, pol.GatesConfig().MainCIPoll)
}

func originMainTip(projectRoot string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		g := gitexec.Git{Dir: projectRoot, Exec: sysexec.DefaultRunner}
		out, err := g.Output(ctx, "ls-remote", "origin", "refs/heads/main")
		if err != nil {
			return "", fmt.Errorf("%w: %w", errOriginMainLSRemote, err)
		}
		sha, _, _ := strings.Cut(strings.TrimSpace(out), "\t")
		if sha == "" {
			return "", fmt.Errorf("%w: %w", errOriginMainLSRemote, errNoOriginMainRef)
		}
		return sha, nil
	}
}

func (p *mainCIPoll) tick(now time.Time, signals *signalcenter.Center, warn io.Writer) {
	if p == nil || (!p.last.IsZero() && now.Sub(p.last) < p.every) {
		return
	}
	p.last = now
	ctx, cancel := context.WithTimeout(context.Background(), waveSyncTimeout)
	defer cancel()
	sha, err := p.originMain(ctx)
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: main CI poll: %v — no stop is opened or closed on an unknown state\n", err)
		return
	}
	red, failed, err := p.ci(ctx, p.projectRoot, sha)
	switch {
	case err != nil:
		fmt.Fprintf(warn, "[loop] WARN: main CI poll: main CI status unavailable (%v) — no stop is opened or closed on an unknown state\n", err)
	case red:
		p.open(core.MainCIRed{Main: sha, Failed: failed}, signals, warn)
	default:
		p.closeOnGreen(sha, now, signals, warn)
	}
}

func (p *mainCIPoll) open(red core.MainCIRed, signals *signalcenter.Center, warn io.Writer) {
	opened, err := core.OpenMainCIRedStop(p.projectRoot, red)
	if opened {
		signals.Emit(red.Incident("mainCIPoll.open", err))
		return
	}
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: main CI poll: %v\n", err)
	}
}

func (p *mainCIPoll) closeOnGreen(sha string, now time.Time, signals *signalcenter.Center, warn io.Writer) {
	evolveDir := filepath.Join(p.projectRoot, ".evolve")
	stop, holds := selfheal.ReadStop(evolveDir, warn)
	if !holds {
		return
	}
	mover := inboxmover.Options{ProjectRoot: p.projectRoot, Stderr: warn, Signals: signals}
	release := selfheal.StopRelease{
		EvolveDir: evolveDir, Now: now, Emit: signals.Emit,
		Event:   signalcenter.Event{Origin: "mainCIPoll.closeOnGreen"},
		Consume: func(id string) error { return inboxmover.ConsumeFixItem(mover, id, inboxmover.PromoteOpts{}) },
	}
	if err := release.Keys(core.StopCIKeys(stop), "origin/main CI green at "+sha); err != nil {
		fmt.Fprintf(warn, "[loop] WARN: main CI poll: release the stop: %v\n", err)
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func runLanesThenPublish(ctx context.Context, sup *fleet.Supervisor, specs []fleet.CycleSpec, projectRoot string, warn io.Writer) []fleet.Result {
	results := sup.Run(ctx, specs)
	publishPendingDossiers(paths.AbsoluteRoot("the project root", projectRoot, nil), warn)
	return results
}

func publishPendingDossiers(projectRoot string, warn io.Writer) {
	if !hasPendingDossiers(projectRoot, warn) {
		return
	}
	if _, err := plane.Classify(projectRoot); err != nil {
		return
	}
	if busy := anotherRunLive(projectRoot); busy != "" {
		fmt.Fprintf(warn, "[dossier] WARN: pending dossiers: %s — they stay pending\n", busy)
		return
	}
	release, err := flock.Lock(flock.ShipLockPath(projectRoot))
	if err != nil {
		fmt.Fprintf(warn, "[dossier] WARN: pending dossiers: git-mutation lock: %v — they stay pending\n", err)
		return
	}
	defer release()
	if hold := publishHold(projectRoot); hold != "" {
		fmt.Fprintf(warn, "[dossier] WARN: pending dossiers: %s — they stay pending\n", hold)
		return
	}
	res, err := dossier.PublishPending(projectRoot, warn)
	if err != nil {
		fmt.Fprintf(warn, "[dossier] WARN: pending dossiers: %v — they stay pending\n", err)
		return
	}
	if len(res.Published) > 0 || len(res.Skipped) > 0 || len(res.Failed) > 0 {
		fmt.Fprintf(warn, "[dossier] pending dossiers: published cycles %v; half pairs %v; refused or failed %d\n", res.Published, res.Skipped, len(res.Failed))
	}
}

func anotherRunLive(projectRoot string) string {
	run, active, err := loopchain.LiveSiblingRun(paths.EvolveDirOf(projectRoot))
	if err != nil {
		return fmt.Sprintf("cannot prove that no other run is live (%v)", err)
	}
	if active {
		return fmt.Sprintf("another run is live on this plane: %s (%s); let it finish, or clear a stale one with `evolve cycle reset`", run.Dir, run.Reason)
	}
	return ""
}

func hasPendingDossiers(projectRoot string, warn io.Writer) bool {
	entries, err := os.ReadDir(dossier.PendingDir(projectRoot))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(warn, "[dossier] WARN: pending dossiers: cannot check: %v — nothing is published this pass\n", err)
	}
	return len(entries) > 0
}

func publishHold(projectRoot string) string {
	ctx, cancel := context.WithTimeout(context.Background(), waveSyncTimeout)
	defer cancel()
	g := gitexec.Default(projectRoot)
	if _, _, code, err := g.Capture(ctx, "remote", "get-url", "origin"); err != nil || code != 0 {
		return ""
	}
	rel, err := g.RelationToRemote(ctx, "origin/main")
	if err != nil {
		return err.Error()
	}
	if rel.Kind != gitexec.RelationCurrent && rel.Kind != gitexec.RelationAhead {
		return rel.String()
	}
	return ""
}

package ship

// pushonly.go — the sanctioned completion for attested-but-stranded commits
// (inbox ship-push-only-recovery; 3 live instances). The strand: a ship's
// push hits GIT_PUSH_REJECTED with true divergence (a console PR merged to
// origin mid-batch), `evolve sync-main` reconciles merge-only (never
// pushes), and re-running ship reports "nothing to ship" while the plane
// sits ahead>0 — with bare `git push` correctly guard-denied, the recovery
// the error prescribed could not complete through any sanctioned command
// (the 2026-08-02 dead end; #408 stranded 15 commits the same way).
//
// Two halves close it:
//   - every successful ship appends its minted commit to the durable
//     provenance journal (.evolve/ship-journal.jsonl) — finalize's success
//     path, all classes, never dry-run;
//   - `evolve ship --push-only` pushes the ahead set after verifying EVERY
//     ahead commit's provenance: recorded in the journal, or a merge commit
//     with an origin-ancestor parent (the sync-main reconcile shape — the
//     merge is minted by the sanctioned command, so its identity is
//     structural). Any other commit refuses BY NAME — push-only must never
//     become the guard bypass for hand-made commits; commits predating the
//     journal refuse the same way, with the remediation named.
//
// TRUST POSTURE (stated, review MEDIUM): the journal is plane-local and
// unguarded-writable — its trust is equivalent in KIND to
// .commit-gate/attestation.json (which --class manual already pushes on),
// though weaker in degree (no tree-SHA binding, consulted unboundedly
// later); a Write-capable actor who can forge either can already ship. The
// sync-main reconcile merge's TREE is likewise trusted, not verified — a
// conflicted reconcile concluded by hand can smuggle content, the same
// plane-local-mutation trust class. rev-list enumerates every ahead commit
// INDIVIDUALLY, so a merge can never wrap unprovenanced commits past the
// check — each wrapped commit is tested and refused by name.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// shipJournalName is the durable per-commit ship provenance record.
const shipJournalName = "ship-journal.jsonl"

func shipJournalPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", shipJournalName)
}

// shipJournalEntry is one minted ship commit.
type shipJournalEntry struct {
	SHA   string `json:"sha"`
	Class string `json:"class"`
	TS    string `json:"ts"`
}

// appendShipJournal records a successfully shipped commit. Best-effort by
// design (a journal write failure must never fail a ship that already
// pushed); O_APPEND single-write keeps concurrent lanes line-atomic.
func appendShipJournal(projectRoot string, sha string, class Class) {
	if projectRoot == "" || sha == "" {
		return
	}
	line, err := json.Marshal(shipJournalEntry{SHA: sha, Class: string(class), TS: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	f, err := os.OpenFile(shipJournalPath(projectRoot), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(line, '\n'))
}

// journalHasSHA reports whether sha is recorded in the ship journal.
func journalHasSHA(projectRoot, sha string) bool {
	f, err := os.Open(shipJournalPath(projectRoot))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e shipJournalEntry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.SHA == sha {
			return true
		}
	}
	return false
}

// runPushOnly pushes the current branch's attested ahead set. It never
// commits, never stages, never releases — push is the ONLY mutation.
func runPushOnly(ctx context.Context, opts *Options, res *RunResult) error {
	// A push-only invocation with staged work is a category error — the
	// operator wants a normal ship (commit-gate, class checks, the works).
	if exit, err := opts.run(ctx, "git", []string{"diff", "--cached", "--quiet"}, opts.Stdout, opts.Stderr); err != nil || exit != 0 {
		return fmt.Errorf("ship --push-only: staged changes present — push-only completes an already-committed strand; run a normal `evolve ship` for new work")
	}
	branchOut, err := captureGitOutput(ctx, opts, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("ship --push-only: resolve branch: %w", err)
	}
	branch := strings.TrimSpace(branchOut)
	// Honest ahead set: refresh origin first (best-effort — an offline push
	// fails loudly below anyway).
	_, _ = opts.run(ctx, "git", []string{"fetch", "origin", branch}, opts.Stdout, opts.Stderr)
	aheadOut, err := captureGitOutput(ctx, opts, "rev-list", "origin/"+branch+"..HEAD")
	if err != nil {
		return fmt.Errorf("ship --push-only: no origin/%s to compare against: %w", branch, err)
	}
	ahead := strings.Fields(aheadOut)
	if len(ahead) == 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] PUSH-ONLY: origin/%s already carries HEAD — nothing to push", branch))
		return nil
	}
	var unprovenanced []string
	for _, sha := range ahead {
		if journalHasSHA(opts.ProjectRoot, sha) || isSyncMainMerge(ctx, opts, sha, branch) {
			continue
		}
		unprovenanced = append(unprovenanced, sha[:min(12, len(sha))])
	}
	if len(unprovenanced) > 0 {
		return fmt.Errorf("ship --push-only: REFUSED — %d ahead commit(s) lack ship provenance (not in %s, not a sync-main reconcile merge): [%s]. Push-only is a recovery for attested strands, never a guard bypass; land un-provenanced work through a normal `evolve ship`",
			len(unprovenanced), shipJournalName, strings.Join(unprovenanced, ", "))
	}
	// Same push + inline reject-repair policy as the ordinary ship.
	exit, perr := opts.run(ctx, "git", []string{"push", "origin", branch}, opts.Stdout, opts.Stderr)
	if perr != nil || exit != 0 {
		origErr := shipErr(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship --push-only: git push failed (rc=%d): %v", exit, perr),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(perr), "branch", branch)
		if rerr := repairPushRace(ctx, opts, res, branch, origErr); rerr != nil {
			return rerr
		}
	}
	head, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	res.CommitSHA = strings.TrimSpace(head)
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] PUSH-ONLY: pushed %d attested commit(s) to origin/%s", len(ahead), branch))
	return nil
}

// isSyncMainMerge reports whether sha is a merge commit with at least one
// parent already on origin/<branch> — the shape `evolve sync-main` mints when
// reconciling a diverged plane (merge-only, never pushed). The identity is
// structural: only a reconcile merge has an origin-side parent while sitting
// in the ahead set.
func isSyncMainMerge(ctx context.Context, opts *Options, sha, branch string) bool {
	out, err := captureGitOutput(ctx, opts, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return false
	}
	fields := strings.Fields(out)
	if len(fields) < 3 { // [self parent1 parent2 ...] — a merge has ≥2 parents
		return false
	}
	for _, parent := range fields[1:] {
		if exit, err := opts.run(ctx, "git", []string{"merge-base", "--is-ancestor", parent, "origin/" + branch}, opts.Stdout, opts.Stderr); err == nil && exit == 0 {
			return true
		}
	}
	return false
}

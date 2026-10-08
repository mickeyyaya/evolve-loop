package ship

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
)

const shipJournalName = "ship-journal.jsonl"

func shipJournalPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", shipJournalName)
}

type shipJournalEntry struct {
	SHA   string `json:"sha"`
	Class string `json:"class"`
	TS    string `json:"ts"`
	Cycle int    `json:"cycle,omitempty"`
}

func appendShipJournal(projectRoot string, e shipJournalEntry) error {
	e.TS = time.Now().UTC().Format(time.RFC3339)
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := shipJournalPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	return errors.Join(werr, f.Close())
}

func readShipJournal(projectRoot string) map[string]shipJournalEntry {
	entries := map[string]shipJournalEntry{}
	f, err := os.Open(shipJournalPath(projectRoot))
	if err != nil {
		return entries
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e shipJournalEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.SHA != "" {
			entries[e.SHA] = e
		}
	}
	return entries
}

func journalHasSHA(projectRoot, sha string) bool {
	_, found := readShipJournal(projectRoot)[sha]
	return found
}

// runPushOnly never commits, stages, or releases — push is its only mutation.
func runPushOnly(ctx context.Context, opts *Options, res *RunResult) error {
	if exit, err := opts.run(ctx, "git", []string{"diff", "--cached", "--quiet"}, opts.Stdout, opts.Stderr); err != nil || exit != 0 {
		return fmt.Errorf("ship --push-only: staged changes present — push-only completes an already-committed strand; run a normal `evolve ship` for new work")
	}
	branchOut, err := captureGitOutput(ctx, opts, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("ship --push-only: resolve branch: %w", err)
	}
	branch := strings.TrimSpace(branchOut)
	// Best-effort refresh: an offline push still fails loudly below.
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
	// Same push + reject-repair policy as an ordinary ship (gitops_landing.go).
	if err := pushWithRepair(ctx, opts, res, landing.PushRequest{Branch: branch, Site: landing.SitePushOnly}); err != nil {
		return err
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] PUSH-ONLY: pushed %d attested commit(s) to origin/%s", len(ahead), branch))
	return nil
}

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

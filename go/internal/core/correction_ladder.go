package core

// correction_ladder.go — ADR-0045 I2: the orchestrator-side executors for the
// graduated correction ladder (the DECISION lives in interaction.NextCorrection,
// a pure leaf). Rung 1 (salvage) turns the cycle-265 class — a valid
// deliverable at the wrong path, burned through two full re-dispatches — into
// an atomic relocate + breaker-neutral verify, no agent involved. Rung 3's
// directive is enriched with kernel-verified evidence so correction attempts
// stop retrying blind (cycle-265: attempt 2 carried only the violation text).
//
// Salvage safety posture (threat S2 — salvage as a smuggling vector):
//   - candidates are constructed by the KERNEL from the phase's own roots
//     (worktree, workspace, cwd) + the CONTRACTED basename — never from agent
//     input, so traversal cannot be steered;
//   - lstat gate: only regular files move (a planted symlink at the stray
//     location must not be followed into the contracted path);
//   - size + staleness caps bound what a hostile or ancient stray can inject;
//   - relocate FIRST, verify the DESTINATION after (TOCTOU: the pre-move copy
//     is never the trusted artifact — what landed at the contracted path is
//     what gets verified and gated);
//   - salvage NEVER upgrades a verdict: a verified relocation still faces the
//     review gate (the breaker-touching FINAL outcome) like any native artifact.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// salvageMaxBytes caps what salvage will relocate: a contracted report is
	// tens of KB; anything over 1 MiB is not a deliverable we should move.
	salvageMaxBytes = 1 << 20
	// salvageMaxAge rejects ancient strays: worktree + workspace are
	// per-cycle fresh, so this mostly guards the cwd candidate from
	// resurrecting a days-old leftover as this cycle's deliverable.
	salvageMaxAge = 6 * time.Hour
	// evidenceStatusMaxNames caps the worktree git-status names embedded in
	// the rung-3 directive (the ACI compact-feedback principle: ≤20-line
	// digests, never log dumps).
	evidenceStatusMaxNames = 15
)

// salvageResult describes one salvage attempt for the ladder driver.
type salvageResult struct {
	// Relocated: a candidate was atomically moved to the contracted path.
	Relocated bool
	// From is the candidate path that was moved (kernel evidence for the
	// rung-3 directive when the destination then failed verification).
	From string
	// Verified: the DESTINATION passed the breaker-neutral re-check after
	// the move.
	Verified bool
	// Reason is the human trail when nothing was relocated.
	Reason string
}

// salvageDeliverable implements rung 1: locate the contracted basename under
// {worktree, workspace, cwd}, relocate the first safe candidate ATOMICALLY to
// the contracted path, then verify the DESTINATION breaker-neutrally.
func (o *Orchestrator) salvageDeliverable(ctx context.Context, in ReviewInput) salvageResult {
	if o.contractVerifier == nil {
		return salvageResult{Reason: "no breaker-neutral verifier wired"}
	}
	v, err := o.contractVerifier.VerifyDeliverable(ctx, in)
	if err != nil {
		// Ambiguity (unknown phase) — fail open by NOT acting (never
		// relocate blind), mirroring the gate's posture.
		return salvageResult{Reason: fmt.Sprintf("verifier ambiguity: %v", err)}
	}
	if v.OK {
		// The reject was not artifact-shaped (e.g. an evalgate reject) —
		// there is nothing for salvage to fix.
		return salvageResult{Reason: "destination already well-formed (reject was not artifact-shaped)"}
	}
	dest := v.ArtifactPath
	if dest == "" {
		return salvageResult{Reason: "contract resolves no artifact path"}
	}
	base := filepath.Base(dest)
	if base == "." || base == string(os.PathSeparator) || !filepath.IsLocal(base) {
		return salvageResult{Reason: fmt.Sprintf("contracted basename %q is not salvage-safe", base)}
	}

	destClean := filepath.Clean(dest)
	// S2 defense-in-depth: relocate ONLY to a contracted path inside the two
	// legitimate roots phasecontract.ArtifactPath can resolve to — the workspace
	// (TargetWorkspace) or <ProjectRoot>/.evolve (TargetEvolveDir). The basename
	// check above bounds the filename; this bounds the DIRECTORY, so a future or
	// compromised ContractVerifier returning an out-of-tree absolute path can
	// never make salvage write outside the run's own roots. Current production
	// dests always satisfy this (ArtifactName is a single component), so it is a
	// guard, not a behavior change.
	if !withinRoot(destClean, in.Workspace) && !withinRoot(destClean, filepath.Join(in.ProjectRoot, ".evolve")) {
		return salvageResult{Reason: fmt.Sprintf("contracted dest %q is outside the workspace/.evolve roots", destClean)}
	}
	now := o.now()
	var skips []string
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		skips = append(skips, "cwd: unreadable: "+cwdErr.Error())
	}
	// Search order is load-bearing (worktree first): a source-writing phase
	// most plausibly misplaced its report into the tree it was editing.
	for _, root := range []string{in.Worktree, in.Workspace, cwd} {
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, base)
		if filepath.Clean(candidate) == destClean {
			skips = append(skips, candidate+": is the contracted path itself")
			continue
		}
		if ok, why := fileSalvageable(candidate, now); !ok {
			if why != "absent" {
				skips = append(skips, candidate+": "+why)
			}
			continue
		}
		// Relocate FIRST (atomic rename — same filesystem for repo-local
		// roots), THEN verify what actually landed at the contracted path.
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			skips = append(skips, candidate+": mkdir dest: "+err.Error())
			continue
		}
		if err := os.Rename(candidate, dest); err != nil {
			skips = append(skips, candidate+": rename: "+err.Error())
			continue
		}
		v2, verr := o.contractVerifier.VerifyDeliverable(ctx, in)
		return salvageResult{
			Relocated: true,
			From:      candidate,
			Verified:  verr == nil && v2.OK,
		}
	}
	reason := "no candidate found under {worktree, workspace, cwd}"
	if len(skips) > 0 {
		reason += " (" + strings.Join(skips, "; ") + ")"
	}
	return salvageResult{Reason: reason}
}

// withinRoot reports whether path is root itself or lives under it, without
// following symlinks or admitting `..` traversal. An empty root never matches
// (so a missing ProjectRoot can't accidentally whitelist the whole tree).
func withinRoot(path, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

// fileSalvageable applies the S2 candidate gates: regular file (lstat — a
// symlink is never followed), non-empty, size-capped, fresh enough.
func fileSalvageable(path string, now time.Time) (ok bool, reason string) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, "absent"
	}
	if !fi.Mode().IsRegular() {
		return false, fmt.Sprintf("not a regular file (mode %s)", fi.Mode())
	}
	if fi.Size() == 0 {
		return false, "empty"
	}
	if fi.Size() > salvageMaxBytes {
		return false, fmt.Sprintf("size %d exceeds salvage cap %d", fi.Size(), salvageMaxBytes)
	}
	if age := now.Sub(fi.ModTime()); age > salvageMaxAge {
		return false, fmt.Sprintf("stale (mtime %s old, cap %s)", age.Round(time.Minute), salvageMaxAge)
	}
	return true, ""
}

// kernelEvidenceDigest composes the rung-3 directive's evidence block from
// kernel-verified facts only (design principle #1: external, unfakeable —
// never agent self-assessment): the worktree's git-status NAMES and, when
// rung 1 found-but-couldn't-validate a stray, its original path. Compact by
// construction (ACI): name-level only, capped, never file contents.
func kernelEvidenceDigest(worktree, foundButInvalid string) string {
	var b strings.Builder
	if foundButInvalid != "" {
		fmt.Fprintf(&b, "- a file with the contracted name was found at %s and relocated to the contracted path, but it FAILS the contract — fix it in place (do not write elsewhere)\n", foundButInvalid)
	}
	if names := gitStatusNames(worktree, evidenceStatusMaxNames); len(names) > 0 {
		fmt.Fprintf(&b, "- worktree `git status --porcelain` (what your previous attempt actually touched):\n")
		for _, n := range names {
			fmt.Fprintf(&b, "    %s\n", n)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "## Evidence (kernel-verified)\n" + b.String()
}

// gitStatusNames returns up to max porcelain-status names for the worktree,
// best-effort (an unreadable tree degrades to nothing, never an error — the
// digest is enrichment, not a gate).
func gitStatusNames(worktree string, max int) []string {
	if worktree == "" {
		return nil
	}
	out, err := exec.Command("git", "-C", worktree, "status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if ln == "" {
			continue
		}
		names = append(names, ln)
		if len(names) >= max {
			break
		}
	}
	return names
}

package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	salvageMaxBytes        = 1 << 20
	salvageMaxAge          = 6 * time.Hour
	evidenceStatusMaxNames = 15
)

var workingDir = os.Getwd

type salvageResult struct {
	Relocated bool
	From      string
	Verified  bool
	Reason    string
}

func (o *Orchestrator) salvageDeliverable(ctx context.Context, in ReviewInput) salvageResult {
	if o.contractVerifier == nil {
		return salvageResult{Reason: "no breaker-neutral verifier wired"}
	}
	v, err := o.contractVerifier.VerifyDeliverable(ctx, in)
	if err != nil {
		return salvageResult{Reason: fmt.Sprintf("verifier ambiguity: %v", err)}
	}
	if v.OK {
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
	destInsideRunRoots := withinRoot(destClean, in.Workspace) || withinRoot(destClean, filepath.Join(in.ProjectRoot, ".evolve"))
	if !destInsideRunRoots {
		return salvageResult{Reason: fmt.Sprintf("contracted dest %q is outside the workspace/.evolve roots", destClean)}
	}
	now := o.now()
	var skips []string
	cwd, cwdErr := workingDir()
	if cwdErr != nil {
		skips = append(skips, "cwd: unreadable: "+cwdErr.Error())
	}
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

func gitStatusNames(worktree string, max int) []string {
	if worktree == "" {
		return nil
	}
	out, _, code, err := gitexec.Git{Dir: worktree, Exec: gitRunner}.Capture(context.Background(), "status", "--porcelain")
	if err != nil || code != 0 {
		return nil
	}
	var names []string
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
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

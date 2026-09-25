package triage

// premise_drift.go — F40 (cycle 1691). Lane 1691 committed an item filed
// 2026-08-16 whose premise #535 had made unreachable on 2026-09-09; the builder
// "fixed" a non-bug, opened a fail-open, and the audit caught it a full cycle
// later. Six commits had touched the item's own declared files after it was
// filed and #535 touched its package — evidence nothing put in front of
// triage. The host gathers that drift here (Core Rule 5: the evidence is
// mechanical, the judgment stays triage's), so a stale premise is dropped at
// triage — where a drop ends a lane as planned no-work and hands the item to
// the console (F30) — instead of costing a build, an audit and a repair round.
// Drift is evidence to re-verify, not a verdict: most drifted items still hold.

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// The section is bounded like the carry-forward sweep: git work on triage's
// critical path must never stall prompt composition.
const (
	premiseDriftMaxItems   = 5               // a lane's scope (one item, or a small menu); the cap guards a runaway scope
	premiseDriftMaxCommits = 5               // declared-path commits listed per item (newest first)
	premiseDriftMaxOther   = 3               // id-naming and package-only commits listed per item
	premiseDriftTextLen    = 100             // runes kept of each id, subject and path list
	premiseDriftTimeout    = 5 * time.Second // one deadline for the whole section
)

// premiseDriftUnavailable is the visible line for an item whose evidence could
// not be gathered — silence would read as "no drift".
const premiseDriftUnavailable = "drift unavailable (git failed or timed out)\n"

// premiseDriftSection renders, for each fleet-scoped inbox item, the evidence
// a premise re-check needs: the commits since the item was filed that name its
// id (a ship that consumed it lists the item's consumed file in its "## Actual
// diff" footer — phases/ship/gitops.go), that touched its declared
// paths, and — by subject — that touched only their packages, plus the
// declared paths no longer at HEAD. The item is found wherever triage's claim
// left it (the inbox root or a processing/ claim dir — inboxmover.Locate), so a
// re-dispatched triage sees the same evidence. Fail-open like the sibling
// sections: no scope, an unknown item or one with no filing date renders
// nothing; a git failure renders a visible "drift unavailable" line.
func premiseDriftSection(ctx context.Context, projectRoot, scope string) string {
	ids := premiseDriftScope(scope)
	if projectRoot == "" || len(ids) == 0 {
		return ""
	}
	inboxDir := filepath.Join(projectRoot, ".evolve", "inbox")
	ctx, cancel := context.WithTimeout(ctx, premiseDriftTimeout)
	defer cancel()
	var lines strings.Builder
	for _, id := range ids {
		if it, ok := locateItem(inboxDir, id); ok {
			lines.WriteString(itemDrift(ctx, projectRoot, it))
		}
	}
	if lines.Len() == 0 {
		return ""
	}
	return "- premise_drift: evidence to re-verify, not a verdict — the code these scoped items name has changed since they were filed. Most drifted items still hold: re-check each premise at HEAD before claiming it (Step 0b), and drop one only with cited evidence (`stale: <sha or file:line>`):\n" + lines.String()
}

// premiseDriftScope splits the lane's comma-separated scope into ids.
func premiseDriftScope(scope string) []string {
	var ids []string
	for _, id := range strings.Split(scope, ",") {
		if id = strings.TrimSpace(id); id != "" && len(ids) < premiseDriftMaxItems {
			ids = append(ids, id)
		}
	}
	return ids
}

// locateItem loads an inbox item wherever it is — the root or any claim dir.
func locateItem(inboxDir, id string) (inboxbatch.Item, bool) {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return inboxbatch.Item{}, false
	}
	it, _, err := inboxbatch.LoadFile(loc.Path)
	return it, err == nil
}

// itemDrift is one item's drift bullet, or "" when it has none.
func itemDrift(ctx context.Context, root string, it inboxbatch.Item) string {
	since := it.FiledAt()
	if since.IsZero() {
		return ""
	}
	head := fmt.Sprintf("  - %s (filed %s): ", promptSafe(it.ID), since.UTC().Format("2006-01-02"))
	naming, ok := commitsSince(ctx, root, since, []string{"-E", "--grep=" + idMentionPattern(it.ID)}, nil)
	if !ok {
		return head + premiseDriftUnavailable
	}
	var parts []string
	if n := len(naming); n > 0 {
		parts = append(parts, fmt.Sprintf("%d commit(s) since filing name its id: %s", n, listCommits(naming, premiseDriftMaxOther)))
	}
	inRepo, outside := repoPaths(it.DeclaredPaths())
	if len(inRepo) > 0 {
		declaredParts, ok := declaredDrift(ctx, root, since, inRepo)
		if !ok {
			return head + premiseDriftUnavailable
		}
		parts = append(parts, declaredParts...)
	}
	if len(outside) > 0 {
		parts = append(parts, "declared outside the repository: "+promptSafe(strings.Join(outside, ", ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return head + strings.Join(parts, " | ") + "\n"
}

// declaredDrift is the declared-surface half of an item's drift: commits on
// its declared paths, package-only commits beside them, and the declared paths
// no longer at HEAD. ok is false when git failed or the deadline ran out.
func declaredDrift(ctx context.Context, root string, since time.Time, declared []string) (parts []string, ok bool) {
	onDeclared, ok := commitsSince(ctx, root, since, nil, declared)
	if !ok {
		return nil, false
	}
	if n := len(onDeclared); n > 0 {
		parts = append(parts, fmt.Sprintf("%d commit(s) since filing touched its declared paths: %s", n, listCommits(onDeclared, premiseDriftMaxCommits)))
	}
	if packages := packageDirs(declared); len(packages) > 0 {
		inPackages, ok := commitsSince(ctx, root, since, nil, packages)
		if !ok {
			return nil, false
		}
		if only := without(inPackages, onDeclared); len(only) > 0 {
			parts = append(parts, fmt.Sprintf("%d more commit(s) since filing touched only its packages (%s): %s", len(only), promptSafe(strings.Join(packages, ", ")), listCommits(only, premiseDriftMaxOther)))
		}
	}
	gone, ok := vanishedAtHEAD(ctx, root, declared)
	if !ok {
		return nil, false
	}
	if len(gone) > 0 {
		parts = append(parts, "not at HEAD: "+promptSafe(strings.Join(gone, ", ")))
	}
	return parts, true
}

type driftCommit struct{ hash, date, subject string }

// commitsSince lists the commits since `since` (committer date — the one git's
// --since filters on, and the one displayed) matching extra log arguments
// and/or touching paths, newest first. ok is false on a git failure.
func commitsSince(ctx context.Context, root string, since time.Time, extra, paths []string) ([]driftCommit, bool) {
	args := append([]string{"log", "--since=" + since.UTC().Format(time.RFC3339), "--format=%h%x09%cd%x09%s", "--date=short"}, extra...)
	if len(paths) > 0 {
		args = append(append(args, "--"), paths...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var commits []driftCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f := strings.SplitN(line, "\t", 3); len(f) == 3 {
			commits = append(commits, driftCommit{hash: f[0], date: f[1], subject: promptSafe(f[2])})
		}
	}
	return commits, true
}

// listCommits renders up to limit commits, noting how many more there are.
func listCommits(commits []driftCommit, limit int) string {
	shown := make([]string, 0, limit+1)
	for _, c := range commits[:min(len(commits), limit)] {
		shown = append(shown, c.hash+" "+c.date+" "+c.subject)
	}
	if extra := len(commits) - limit; extra > 0 {
		shown = append(shown, fmt.Sprintf("+%d more", extra))
	}
	return strings.Join(shown, "; ")
}

// without returns the commits in all that are not in listed.
func without(all, listed []driftCommit) []driftCommit {
	seen := make(map[string]bool, len(listed))
	for _, c := range listed {
		seen[c.hash] = true
	}
	var out []driftCommit
	for _, c := range all {
		if !seen[c.hash] {
			out = append(out, c)
		}
	}
	return out
}

// repoPaths splits declared paths into clean repo-relative ones (queried) and
// ones that escape the repository — absolute, or leading ".." once cleaned —
// which are reported, never queried: git rejects an out-of-repo pathspec,
// which would blank the whole item's evidence, and nothing outside the
// repository is ever examined (go-review MAJOR).
func repoPaths(declared []string) (inRepo, outside []string) {
	for _, p := range declared {
		c := path.Clean(p)
		if path.IsAbs(c) || c == ".." || strings.HasPrefix(c, "../") {
			outside = append(outside, p)
			continue
		}
		if strings.HasSuffix(p, "/") {
			c += "/" // keep the directory marker packageDirs reads
		}
		inRepo = append(inRepo, c)
	}
	return inRepo, outside
}

// idMentionPattern is the extended regex a commit message must match to NAME
// the id: the id not glued to a longer word on either side, so "warn-gap"
// never counts "warn-gap-v2" or "xwarn-gap" — while a stamped filename
// ("...00Z-warn-gap.json") still matches. Ids are kebab-case; QuoteMeta keeps
// any other character literal.
func idMentionPattern(id string) string {
	return `(^|[^a-z0-9])` + regexp.QuoteMeta(id) + `([^a-z0-9-]|$)`
}

// packageDirs are the directories holding the declared FILES — the recall aid
// for an invalidating change beside the named file. A declared directory is
// already queried as itself; a parent shallower than two segments ("go/",
// "docs/") would put the whole tree's churn in front of triage, so it is
// skipped.
func packageDirs(declared []string) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, p := range declared {
		if strings.HasSuffix(p, "/") {
			continue
		}
		dir := path.Dir(p) + "/"
		if strings.Count(dir, "/") < 2 || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

// vanishedAtHEAD are the (repo-relative) declared paths with no object in
// HEAD's tree. It asks git (`cat-file -e HEAD:<path>`) — the same source the
// log queries read — never the working tree's filesystem. ok is false when the
// deadline cut the check short, so a timeout is never reported as a vanished
// path.
func vanishedAtHEAD(ctx context.Context, root string, declared []string) (gone []string, ok bool) {
	head := exec.CommandContext(ctx, "git", "rev-parse", "--quiet", "--verify", "HEAD^{tree}")
	head.Dir = root
	if head.Run() != nil {
		return nil, false // no HEAD tree to judge against: unavailable, never "everything vanished"
	}
	for _, p := range declared {
		cmd := exec.CommandContext(ctx, "git", "cat-file", "-e", "HEAD:"+strings.TrimSuffix(p, "/"))
		cmd.Dir = root
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return nil, false
			}
			gone = append(gone, p)
		}
	}
	return gone, true
}

// promptSafe applies the one control-character rule (inboxbatch.StripControl)
// and caps the text in runes.
func promptSafe(s string) string {
	s = inboxbatch.StripControl(s)
	if r := []rune(s); len(r) > premiseDriftTextLen {
		return string(r[:premiseDriftTextLen]) + "…"
	}
	return s
}

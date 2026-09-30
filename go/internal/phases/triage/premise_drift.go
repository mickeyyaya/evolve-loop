package triage

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

// Bounded like the carry-forward sweep: git work here must never stall
// prompt composition.
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

func premiseDriftScope(scope string) []string {
	var ids []string
	for _, id := range strings.Split(scope, ",") {
		if id = strings.TrimSpace(id); id != "" && len(ids) < premiseDriftMaxItems {
			ids = append(ids, id)
		}
	}
	return ids
}

func locateItem(inboxDir, id string) (inboxbatch.Item, bool) {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return inboxbatch.Item{}, false
	}
	it, _, err := inboxbatch.LoadFile(loc.Path)
	return it, err == nil
}

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

func idMentionPattern(id string) string {
	return `(^|[^a-z0-9])` + regexp.QuoteMeta(id) + `([^a-z0-9-]|$)`
}

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

func promptSafe(s string) string {
	s = inboxbatch.StripControl(s)
	if r := []rune(s); len(r) > premiseDriftTextLen {
		return string(r[:premiseDriftTextLen]) + "…"
	}
	return s
}

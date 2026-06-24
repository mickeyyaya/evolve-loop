// Package naminguard is the config-driven guard against dead naming tokens a
// plugin/skill/repo rename leaves behind.
//
// A rename (the evolve-loop -> evo command namespace; the hyphen-less GitHub
// slug -> evolve-loop) updates the obvious files but routinely strands references
// in prose, manifests, and docs — a dead /plugin install handle, a 404 repo
// slug, an old /command: namespace. Historically each was caught (if at all) by
// a hand-written one-off regression test per token. This package replaces that
// with one scanner driven by a single manifest (.evolve/naming.json): the SSOT
// lists every forbidden token and its replacement, and both enforcement
// surfaces — the legacynames acs gate and the `evolve release` preflight — call
// Scan against it. `evolve names fix` calls Fix to rewrite the stale tokens.
//
// Scanning is restricted to git-tracked files (via `git grep`), so build
// artifacts and untracked scratch never trip the guard, and the manifest's
// exclude[] (git pathspecs) drives both Scan and Fix so they cannot drift.
package naminguard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DefaultManifestPath is the repo-relative SSOT location.
const DefaultManifestPath = ".evolve/naming.json"

// Canonical documents the current names (intent). It is not enforced directly —
// forbidden[] is — but it keeps the SSOT self-describing.
type Canonical struct {
	PluginNamespace string `json:"pluginNamespace"`
	RepoSlug        string `json:"repoSlug"`
	CommandPrefix   string `json:"commandPrefix"`
}

// Forbidden is one dead token: it must not appear in any tracked file, and Fix
// rewrites it to Replacement.
//
// Match selects how Token is interpreted: "" or "fixed" (default) treats Token
// as a literal substring; "regex" treats it as a pattern that must be valid in
// BOTH git grep -E and Go's regexp (keep regex patterns simple). Regex mode
// exists for tokens that collide as substrings — e.g. the dead command
// namespace token also appears inside repo-ref URLs of the form
// <owner>/<repo>:<ref>, so it is boundary-anchored as a regex and Replacement
// uses $1 to preserve the matched boundary character.
type Forbidden struct {
	Token       string `json:"token"`
	Replacement string `json:"replacement"`
	Reason      string `json:"reason"`
	Match       string `json:"match,omitempty"`
}

// isRegex reports whether this entry is matched as a regex (vs fixed string).
func (f Forbidden) isRegex() bool { return f.Match == "regex" }

// grepMatchArgs returns the git-grep flags that select this entry's matches.
func (f Forbidden) grepMatchArgs() []string {
	if f.isRegex() {
		return []string{"-E", "-e", f.Token}
	}
	return []string{"--fixed-strings", "-e", f.Token}
}

// apply rewrites every match of this entry in content. Fixed entries use a
// literal replacement; regex entries use Go regexp and Replacement may reference
// $1.. capture groups (e.g. to preserve a matched boundary character).
func (f Forbidden) apply(content string) (string, error) {
	if f.isRegex() {
		re, err := regexp.Compile(f.Token)
		if err != nil {
			return "", fmt.Errorf("naminguard: compile regex %q: %w", f.Token, err)
		}
		return re.ReplaceAllString(content, f.Replacement), nil
	}
	return strings.ReplaceAll(content, f.Token, f.Replacement), nil
}

// Manifest is the parsed .evolve/naming.json SSOT.
type Manifest struct {
	Canonical Canonical   `json:"canonical"`
	Forbidden []Forbidden `json:"forbidden"`
	Exclude   []string    `json:"exclude"`
}

// Violation is one tracked-file line that contains a forbidden token.
type Violation struct {
	File  string
	Line  int
	Token string
	Text  string
}

// String renders a violation as `file:line: token (reason-free, one line)`.
func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: contains %q — %s", v.File, v.Line, v.Token, v.Text)
}

// Load reads and validates the manifest at path.
func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("naminguard: read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("naminguard: parse manifest %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate fails loudly on a manifest that cannot enforce or converge under Fix.
func (m *Manifest) Validate() error {
	if len(m.Forbidden) == 0 {
		return fmt.Errorf("naminguard: manifest has no forbidden[] tokens")
	}
	for i, f := range m.Forbidden {
		if strings.TrimSpace(f.Token) == "" {
			return fmt.Errorf("naminguard: forbidden[%d] has empty token", i)
		}
		if f.Replacement == "" {
			return fmt.Errorf("naminguard: forbidden[%d] (%q) has empty replacement", i, f.Token)
		}
		if f.Match != "" && f.Match != "fixed" && f.Match != "regex" {
			return fmt.Errorf("naminguard: forbidden[%d] (%q) match=%q — want \"fixed\" or \"regex\"", i, f.Token, f.Match)
		}
		if f.isRegex() {
			if _, err := regexp.Compile(f.Token); err != nil {
				return fmt.Errorf("naminguard: forbidden[%d] regex %q: %w", i, f.Token, err)
			}
			// Skip the fixed-token convergence check below: a regex replacement
			// legitimately contains capture-group back-refs ($1) that look like a
			// substring of the pattern, so Contains() can't reason about it. The
			// boundary-anchored patterns in use are idempotent under Fix (proven by
			// TestApplyRegex_BoundaryAnchored).
			continue
		}
		// Fixed only: if the replacement still contains the token, a Fix pass
		// would leave a match behind — Scan after Fix would never reach zero.
		if strings.Contains(f.Replacement, f.Token) {
			return fmt.Errorf("naminguard: forbidden[%d] replacement %q still contains token %q — fix would not converge", i, f.Replacement, f.Token)
		}
	}
	return nil
}

// excludePathspecs turns exclude entries into git ':!<entry>' pathspecs.
func (m *Manifest) excludePathspecs() []string {
	out := make([]string, 0, len(m.Exclude))
	for _, e := range m.Exclude {
		out = append(out, ":!"+e)
	}
	return out
}

// gitGrep runs `git -C root grep <args>` and returns stdout and the exit code
// (git grep: 0 = matches, 1 = none, >1 = error). It is a package var so tests
// can inject a fake without a real repository.
var gitGrep = func(root string, args ...string) (stdout string, code int, err error) {
	full := append([]string{"-C", root, "grep"}, args...)
	cmd := exec.Command("git", full...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	if runErr == nil {
		return out.String(), 0, nil
	}
	if ee, ok := runErr.(*exec.ExitError); ok {
		return out.String(), ee.ExitCode(), nil
	}
	return out.String(), -1, fmt.Errorf("naminguard: git grep: %w (%s)", runErr, strings.TrimSpace(errb.String()))
}

// Scan returns every tracked-file line containing a forbidden token. root must
// be a git work tree. Results are sorted by (file, line) for stable output.
func Scan(root string, m *Manifest) ([]Violation, error) {
	var all []Violation
	excludes := m.excludePathspecs()
	for _, f := range m.Forbidden {
		args := append([]string{"-I", "-n", "--no-color"}, f.grepMatchArgs()...)
		args = append(args, "--")
		args = append(args, excludes...)
		out, code, err := gitGrep(root, args...)
		if err != nil {
			return nil, err
		}
		switch code {
		case 0:
			vs, perr := parseGrepOutput(out, f.Token)
			if perr != nil {
				return nil, perr
			}
			all = append(all, vs...)
		case 1: // no matches for this token
		default:
			return nil, fmt.Errorf("naminguard: git grep %q failed (exit %d): %s", f.Token, code, out)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Line < all[j].Line
	})
	return all, nil
}

// parseGrepOutput turns `path:line:text` grep output into Violations.
func parseGrepOutput(out, token string) ([]Violation, error) {
	var vs []Violation
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" {
			continue
		}
		first := strings.IndexByte(ln, ':')
		if first < 0 {
			return nil, fmt.Errorf("naminguard: unparseable grep line: %q", ln)
		}
		rest := ln[first+1:]
		second := strings.IndexByte(rest, ':')
		if second < 0 {
			return nil, fmt.Errorf("naminguard: unparseable grep line: %q", ln)
		}
		lineNo, err := strconv.Atoi(rest[:second])
		if err != nil {
			return nil, fmt.Errorf("naminguard: bad line number in %q: %w", ln, err)
		}
		vs = append(vs, Violation{
			File:  ln[:first],
			Line:  lineNo,
			Token: token,
			Text:  strings.TrimSpace(rest[second+1:]),
		})
	}
	return vs, nil
}

// Fix rewrites every forbidden token to its replacement across tracked files
// (honoring exclude[]), in manifest order. It returns the sorted repo-relative
// paths it changed. It stages nothing and commits nothing — the caller reviews.
func Fix(root string, m *Manifest) ([]string, error) {
	changed := map[string]bool{}
	for _, f := range m.Forbidden {
		args := append([]string{"-I", "-l", "--no-color"}, f.grepMatchArgs()...)
		args = append(args, "--")
		args = append(args, m.excludePathspecs()...)
		out, code, err := gitGrep(root, args...)
		if err != nil {
			return nil, err
		}
		switch code {
		case 1:
			continue // none
		case 0:
		default:
			return nil, fmt.Errorf("naminguard: git grep -l %q failed (exit %d): %s", f.Token, code, out)
		}
		for _, rel := range strings.Split(strings.TrimSpace(out), "\n") {
			if rel == "" {
				continue
			}
			abs := filepath.Join(root, filepath.FromSlash(rel))
			b, err := os.ReadFile(abs)
			if err != nil {
				return nil, fmt.Errorf("naminguard: read %s: %w", rel, err)
			}
			nb, err := f.apply(string(b))
			if err != nil {
				return nil, err
			}
			if nb == string(b) {
				continue
			}
			// Atomic, mode-preserving replace: Fix rewrites tracked source files,
			// so a truncate-in-place (os.WriteFile) could leave one half-written if
			// interrupted. Write a sibling temp + rename instead (atomic on POSIX),
			// carrying the original's permission bits so executable scripts keep
			// their bit (atomicwrite.Bytes would force 0644).
			if err := writeFilePreservingMode(abs, []byte(nb)); err != nil {
				return nil, fmt.Errorf("naminguard: write %s: %w", rel, err)
			}
			changed[rel] = true
		}
	}
	out := make([]string, 0, len(changed))
	for k := range changed {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// writeFilePreservingMode atomically replaces abs with data: write a sibling
// temp file, copy abs's existing permission bits onto it, then rename into place.
// The rename is atomic on POSIX, so an interrupt mid-Fix can never leave a tracked
// source file truncated. Copying the mode keeps an executable script's bit — the
// reason a plain atomicwrite.Bytes (which forces 0644) is not used here.
func writeFilePreservingMode(abs string, data []byte) error {
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, abs)
}

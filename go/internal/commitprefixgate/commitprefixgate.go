// Package commitprefixgate ports legacy/scripts/guards/commit-prefix-gate.sh.
//
// Layer 1 of the Reward-Hacking Defense System (ADR-0012). Verifies that
// the commit-message prefix matches the diff scope declared in
// .evolve/commit-prefix-scope.json. Rejects mislabeled commits with rc=2.
//
// Closes the cycle-70/72/75 mislabeling pattern (feat() commits that were
// actually role-gate fixes or pure-docs).
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0 = prefix matches scope (or unknown prefix = pass-through)
//	2 = scope violation (ErrScopeViolation)
//	3 = bad arguments (ErrBadArgs)
//	4 = manifest missing or malformed (ErrBadManifest)
//
// Bash 3.2 glob semantics are reproduced — case-statement style matching
// where `*` matches arbitrary characters including `/`.
package commitprefixgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Sentinel errors.
var (
	ErrScopeViolation = errors.New("commitprefixgate: scope violation")
	ErrBadArgs        = errors.New("commitprefixgate: bad arguments")
	ErrBadManifest    = errors.New("commitprefixgate: manifest missing or malformed")
)

// Mode for gathering diff paths.
type Mode int

const (
	ModeStaged Mode = iota // git diff --cached --name-only
	ModeRef                // git diff <ref>..HEAD --name-only
)

// Options drives a Run() invocation.
type Options struct {
	CommitMsg    string
	RepoDir      string
	Mode         Mode
	DiffRef      string // required when Mode == ModeRef
	ManifestPath string // defaulted to <RepoDir>/.evolve/commit-prefix-scope.json
	GuardsLog    string // defaulted to <RepoDir>/.evolve/guards.log
	Stderr       io.Writer

	// Explicit command inputs.
	Bypass    bool
	ShipClass string // SHIP_CLASS

	// Seams for testing.
	Now          func() time.Time
	GetDiffPaths func(repoDir string, mode Mode, diffRef string) ([]string, error)
}

// Result describes what happened.
type Result struct {
	Prefix     string
	Allowed    bool
	DiffPaths  []string
	Manifest   *PrefixManifest
	PrefixRule *PrefixRule
	Reason     string // human-readable allow/deny reason
}

// PrefixManifest is the shape of .evolve/commit-prefix-scope.json.
type PrefixManifest struct {
	Prefixes map[string]PrefixRule `json:"prefixes"`
}

// PrefixRule is one prefix's scope declaration.
type PrefixRule struct {
	AnyPath            bool     `json:"any_path"`
	RequiredPaths      []string `json:"required_paths"`
	ForbiddenOnlyPaths []string `json:"forbidden_only_paths"`
	DiffMustBeSubset   bool     `json:"diff_must_be_subset"`
}

// prefixRE matches conventional-commits prefix: type[(scope)][!]:
// e.g. "feat:", "fix(scope):", "feat(token-opt)!:"
var prefixRE = regexp.MustCompile(`^([a-z][a-z-]*(\([a-z0-9-]+\))?)!?:`)

// Run executes the gate logic.
func Run(opts Options) (Result, error) {
	res := Result{}

	if opts.CommitMsg == "" {
		return res, fmt.Errorf("%w: CommitMsg required", ErrBadArgs)
	}
	if opts.RepoDir == "" {
		return res, fmt.Errorf("%w: RepoDir required", ErrBadArgs)
	}
	if opts.ManifestPath == "" {
		opts.ManifestPath = filepath.Join(opts.RepoDir, ".evolve", "commit-prefix-scope.json")
	}
	if opts.GuardsLog == "" {
		opts.GuardsLog = filepath.Join(opts.RepoDir, ".evolve", "guards.log")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.GetDiffPaths == nil {
		opts.GetDiffPaths = defaultGetDiffPaths
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		appendGuardsLog(opts.GuardsLog, opts.Now(), msg)
	}
	stderrf := func(format string, args ...any) {
		fmt.Fprintf(opts.Stderr, "[commit-prefix-gate] "+format+"\n", args...)
	}

	// Bypass check.
	if opts.Bypass {
		shipClass := opts.ShipClass
		if shipClass == "" {
			shipClass = "cycle"
		}
		if shipClass == "manual" {
			logf("WARN: --bypass + SHIP_CLASS=manual — bypass allowed")
			stderrf("WARN: bypass active (manual class); gate not enforcing")
			res.Allowed = true
			res.Reason = "bypass-manual"
			return res, nil
		}
		stderrf("DENY: bypass requested but SHIP_CLASS='%s' (only 'manual' permits bypass)", shipClass)
		return res, fmt.Errorf("%w: bypass-with-non-manual-class", ErrScopeViolation)
	}

	// Manifest check.
	body, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
		// Missing manifest = pass-through (gate not provisioned yet).
		logf("WARN: manifest missing at %s — pass-through (gate not yet provisioned)", opts.ManifestPath)
		stderrf("WARN: manifest missing at %s — pass-through", opts.ManifestPath)
		res.Allowed = true
		res.Reason = "manifest-missing"
		return res, nil
	}
	var manifest PrefixManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		stderrf("ERROR: manifest is not valid JSON: %s", opts.ManifestPath)
		return res, fmt.Errorf("%w: %v", ErrBadManifest, err)
	}
	res.Manifest = &manifest

	// Extract prefix from first line.
	firstLine := strings.TrimLeft(strings.SplitN(opts.CommitMsg, "\n", 2)[0], " \t")
	m := prefixRE.FindStringSubmatch(firstLine)
	prefix := ""
	if len(m) >= 2 {
		prefix = m[1]
	}
	res.Prefix = prefix
	if prefix == "" {
		logf("WARN: no conventional-commit prefix in '%s' — pass-through", firstLine)
		stderrf("WARN: commit message lacks conventional prefix; pass-through. Use 'type(scope): message' for gate coverage.")
		res.Allowed = true
		res.Reason = "no-prefix"
		return res, nil
	}

	logf("prefix='%s' commit_msg='%s' repo_dir='%s' mode=%v", prefix, firstLine, opts.RepoDir, opts.Mode)

	rule, ok := manifest.Prefixes[prefix]
	if !ok {
		logf("unknown prefix '%s' — pass-through", prefix)
		stderrf("INFO: unknown prefix '%s' — pass-through. Add to %s if regulation needed.", prefix, opts.ManifestPath)
		res.Allowed = true
		res.Reason = "unknown-prefix"
		return res, nil
	}
	res.PrefixRule = &rule

	// Permissive escape hatch.
	if rule.AnyPath {
		logf("ALLOW: prefix '%s' has any_path=true (permissive)", prefix)
		res.Allowed = true
		res.Reason = "any-path"
		return res, nil
	}

	// Gather diff paths.
	diffPaths, err := opts.GetDiffPaths(opts.RepoDir, opts.Mode, opts.DiffRef)
	if err != nil {
		logf("WARN: GetDiffPaths failed: %v — pass-through", err)
		stderrf("WARN: diff path lookup failed: %v — pass-through", err)
		res.Allowed = true
		res.Reason = "diff-lookup-failed"
		return res, nil
	}
	res.DiffPaths = diffPaths
	if len(diffPaths) == 0 {
		logf("WARN: no diff paths found in %v mode — pass-through", opts.Mode)
		stderrf("WARN: no diff (empty staged or empty ref-diff); pass-through")
		res.Allowed = true
		res.Reason = "no-diff"
		return res, nil
	}

	// Rule 1: required_paths — at least one diff path must match.
	if len(rule.RequiredPaths) > 0 {
		matched := false
		for _, p := range diffPaths {
			for _, pat := range rule.RequiredPaths {
				if matchPath(pat, p) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			stderrf("DENY: prefix '%s' requires at least one diff path under %v, but diff contains only: %v",
				prefix, rule.RequiredPaths, diffPaths)
			stderrf("To bypass (emergency, manual class only): pass --bypass with SHIP_CLASS=manual")
			logf("DENY: required_paths failed")
			return res, fmt.Errorf("%w: required_paths", ErrScopeViolation)
		}
		logf("required_paths check passed")
	}

	// Rule 2: forbidden_only_paths — diff must NOT be entirely under forbidden patterns.
	if len(rule.ForbiddenOnlyPaths) > 0 {
		allForbidden := true
		for _, p := range diffPaths {
			matchedAny := false
			for _, pat := range rule.ForbiddenOnlyPaths {
				if matchPath(pat, p) {
					matchedAny = true
					break
				}
			}
			if !matchedAny {
				allForbidden = false
				break
			}
		}
		if allForbidden {
			stderrf("DENY: prefix '%s' diff is entirely under forbidden_only_paths %v. This commit looks like docs/lessons/test work mislabeled as a feature. Use a different prefix (docs:, chore:, test:).",
				prefix, rule.ForbiddenOnlyPaths)
			stderrf("To bypass (emergency, manual class only): pass --bypass with SHIP_CLASS=manual")
			logf("DENY: forbidden_only_paths")
			return res, fmt.Errorf("%w: forbidden_only_paths", ErrScopeViolation)
		}
		logf("forbidden_only_paths check passed")
	}

	// Rule 3: diff_must_be_subset — every diff path must match at least one required pattern.
	if rule.DiffMustBeSubset && len(rule.RequiredPaths) > 0 {
		violators := []string{}
		for _, p := range diffPaths {
			matched := false
			for _, pat := range rule.RequiredPaths {
				if matchPath(pat, p) {
					matched = true
					break
				}
			}
			if !matched {
				violators = append(violators, p)
			}
		}
		if len(violators) > 0 {
			stderrf("DENY: prefix '%s' requires diff to be a subset of %v, but these paths violate: %v",
				prefix, rule.RequiredPaths, violators)
			stderrf("To bypass (emergency, manual class only): pass --bypass with SHIP_CLASS=manual")
			logf("DENY: diff_must_be_subset")
			return res, fmt.Errorf("%w: diff_must_be_subset", ErrScopeViolation)
		}
		logf("diff_must_be_subset check passed")
	}

	logf("ALLOW: prefix '%s' matches scope", prefix)
	res.Allowed = true
	res.Reason = "scope-matched"
	return res, nil
}

// matchPath reproduces bash case-statement glob semantics + the bash port's
// three matching strategies:
//
//  1. direct glob (`*` matches anything including `/`)
//  2. `**` collapsed to `*` and retried
//  3. `prefix/**` matches `prefix/...anything`
//  4. `**/suffix` matches `...anything/suffix` or just `suffix`
func matchPath(pattern, path string) bool {
	if globMatch(pattern, path) {
		return true
	}
	// ** → * collapse.
	flat := strings.ReplaceAll(pattern, "**", "*")
	if flat != pattern && globMatch(flat, path) {
		return true
	}
	// prefix/** form.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	// **/suffix form.
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if path == suffix || strings.HasSuffix(path, "/"+suffix) {
			return true
		}
	}
	return false
}

// globMatch implements bash case-statement glob semantics where:
//   - `*` matches arbitrary chars including `/`
//   - `?` matches one char
//   - `[abc]` matches a char in the class
//   - literal chars match themselves
//
// path/filepath.Match is NOT used because it treats `*` as
// non-separator-crossing. Bash's case-glob crosses `/`.
func globMatch(pattern, path string) bool {
	return globMatchRec(pattern, 0, path, 0)
}

func globMatchRec(pat string, pi int, path string, si int) bool {
	for pi < len(pat) {
		switch pat[pi] {
		case '*':
			// Greedy: try matching 0..N chars.
			if pi == len(pat)-1 {
				return true
			}
			for k := si; k <= len(path); k++ {
				if globMatchRec(pat, pi+1, path, k) {
					return true
				}
			}
			return false
		case '?':
			if si >= len(path) {
				return false
			}
			pi++
			si++
		case '[':
			// Class. Find ']'.
			end := pi + 1
			for end < len(pat) && pat[end] != ']' {
				end++
			}
			if end >= len(pat) || si >= len(path) {
				return false
			}
			class := pat[pi+1 : end]
			matched := false
			for i := 0; i < len(class); i++ {
				if i+2 < len(class) && class[i+1] == '-' {
					if path[si] >= class[i] && path[si] <= class[i+2] {
						matched = true
						break
					}
					i += 2
				} else if class[i] == path[si] {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
			pi = end + 1
			si++
		default:
			if si >= len(path) || pat[pi] != path[si] {
				return false
			}
			pi++
			si++
		}
	}
	return si == len(path)
}

// defaultGetDiffPaths shells out to git for the production code path.
func defaultGetDiffPaths(repoDir string, mode Mode, diffRef string) ([]string, error) {
	var args []string
	switch mode {
	case ModeStaged:
		args = []string{"-C", repoDir, "diff", "--cached", "--name-only"}
	case ModeRef:
		if diffRef == "" {
			return nil, errors.New("diff ref required for ModeRef")
		}
		args = []string{"-C", repoDir, "diff", diffRef + "..HEAD", "--name-only"}
	}
	cmd := execGit(args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result, nil
}

// appendGuardsLog is a best-effort NDJSON-ish append.
func appendGuardsLog(path string, ts time.Time, msg string) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s] [commit-prefix-gate] %s\n",
		ts.UTC().Format(time.RFC3339), msg)
}

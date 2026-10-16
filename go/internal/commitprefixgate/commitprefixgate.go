// Package commitprefixgate refuses a commit whose conventional prefix does not match its diff scope.
// See docs/architecture/packages/internal-commitprefixgate.md.
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

	"github.com/mickeyyaya/evolve-loop/go/internal/guardslog"
)

var (
	ErrScopeViolation = errors.New("commitprefixgate: scope violation")
	ErrBadArgs        = errors.New("commitprefixgate: bad arguments")
	ErrBadManifest    = errors.New("commitprefixgate: manifest missing or malformed")
)

type Mode int

const (
	ModeStaged Mode = iota
	ModeRef
)

type Options struct {
	CommitMsg    string
	RepoDir      string
	Mode         Mode
	DiffRef      string
	ManifestPath string
	GuardsLog    string
	Stderr       io.Writer

	Bypass    bool
	ShipClass string

	Now          func() time.Time
	GetDiffPaths func(repoDir string, mode Mode, diffRef string) ([]string, error)
}

type Result struct {
	Prefix     string
	Allowed    bool
	DiffPaths  []string
	Manifest   *PrefixManifest
	PrefixRule *PrefixRule
	Reason     string
}

type PrefixManifest struct {
	Prefixes map[string]PrefixRule `json:"prefixes"`
}

type PrefixRule struct {
	AnyPath            bool     `json:"any_path"`
	RequiredPaths      []string `json:"required_paths"`
	ForbiddenOnlyPaths []string `json:"forbidden_only_paths"`
	DiffMustBeSubset   bool     `json:"diff_must_be_subset"`
}

var prefixRE = regexp.MustCompile(`^([a-z][a-z-]*(\([a-z0-9-]+\))?)!?:`)

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

	body, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
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

	if rule.AnyPath {
		logf("ALLOW: prefix '%s' has any_path=true (permissive)", prefix)
		res.Allowed = true
		res.Reason = "any-path"
		return res, nil
	}

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

func matchPath(pattern, path string) bool {
	if globMatch(pattern, path) {
		return true
	}
	flat := strings.ReplaceAll(pattern, "**", "*")
	if flat != pattern && globMatch(flat, path) {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if path == suffix || strings.HasSuffix(path, "/"+suffix) {
			return true
		}
	}
	return false
}

func globMatch(pattern, path string) bool {
	return globMatchRec(pattern, 0, path, 0)
}

func globMatchRec(pat string, pi int, path string, si int) bool {
	for pi < len(pat) {
		switch pat[pi] {
		case '*':
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

func appendGuardsLog(path string, ts time.Time, msg string) {
	_ = guardslog.Append(path, "commit-prefix-gate", msg, ts)
}

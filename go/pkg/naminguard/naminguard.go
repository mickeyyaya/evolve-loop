// Package naminguard is the config-driven guard against dead naming tokens a
// plugin/skill/repo rename leaves behind.
// See docs/architecture/packages/pkg-naminguard.md.
package naminguard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const DefaultManifestPath = ".evolve/naming.json"

type Canonical struct {
	PluginNamespace string `json:"pluginNamespace"`
	RepoSlug        string `json:"repoSlug"`
	CommandPrefix   string `json:"commandPrefix"`
}

type Forbidden struct {
	Token       string `json:"token"`
	Replacement string `json:"replacement"`
	Reason      string `json:"reason"`
	Match       string `json:"match,omitempty"`
}

func (f Forbidden) isRegex() bool { return f.Match == "regex" }

func (f Forbidden) grepMatchArgs() []string {
	if f.isRegex() {
		return []string{"-E", "-e", f.Token}
	}
	return []string{"--fixed-strings", "-e", f.Token}
}

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

type Manifest struct {
	Canonical Canonical   `json:"canonical"`
	Forbidden []Forbidden `json:"forbidden"`
	Exclude   []string    `json:"exclude"`
}

type Violation struct {
	File  string
	Line  int
	Token string
	Text  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: contains %q — %s", v.File, v.Line, v.Token, v.Text)
}

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
			continue
		}
		if strings.Contains(f.Replacement, f.Token) {
			return fmt.Errorf("naminguard: forbidden[%d] replacement %q still contains token %q — fix would not converge", i, f.Replacement, f.Token)
		}
	}
	return nil
}

func (m *Manifest) excludePathspecs() []string {
	out := make([]string, 0, len(m.Exclude))
	for _, e := range m.Exclude {
		out = append(out, ":!"+e)
	}
	return out
}

var gitGrep = func(root string, args ...string) (stdout string, code int, err error) {
	full := append([]string{"-C", root, "grep"}, args...)
	cmd := sysexec.Command(context.Background(), "git", full...)
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
		case 1:
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
			continue
		case 0:
		default:
			return nil, fmt.Errorf("naminguard: git grep -l %q failed (exit %d): %s", f.Token, code, out)
		}
		for _, rel := range strings.Split(strings.TrimSpace(out), "\n") {
			if rel == "" {
				continue
			}
			rewritten, err := rewriteFile(root, rel, f)
			if err != nil {
				return nil, err
			}
			if rewritten {
				changed[rel] = true
			}
		}
	}
	out := make([]string, 0, len(changed))
	for k := range changed {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func rewriteFile(root, rel string, f Forbidden) (bool, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(abs)
	if err != nil {
		return false, fmt.Errorf("naminguard: read %s: %w", rel, err)
	}
	nb, err := f.apply(string(b))
	if err != nil {
		return false, err
	}
	if nb == string(b) {
		return false, nil
	}
	if err := writeFilePreservingMode(abs, []byte(nb)); err != nil {
		return false, fmt.Errorf("naminguard: write %s: %w", rel, err)
	}
	return true, nil
}

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

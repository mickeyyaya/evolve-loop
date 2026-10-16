// Package doctor implements `evolve doctor probe <tool>` — the Go port of
// scripts/utility/probe-tool.sh.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Result struct {
	Tool    string   `json:"tool"`
	Found   bool     `json:"found"`
	Path    string   `json:"-"`
	Method  string   `json:"method,omitempty"`
	Checked []string `json:"checked"`
}

type resultJSON struct {
	Tool    string   `json:"tool"`
	Found   bool     `json:"found"`
	Path    *string  `json:"path"`
	Method  string   `json:"method,omitempty"`
	Checked []string `json:"checked"`
}

func (r Result) toWire() resultJSON {
	out := resultJSON{Tool: r.Tool, Found: r.Found, Method: r.Method, Checked: r.Checked}
	if r.Path != "" {
		p := r.Path
		out.Path = &p
	}
	return out
}

type probeHooks struct {
	lookPath   func(name string) (string, error)
	homeDir    func() (string, error)
	candidates func(tool, home string) []string
	marshal    func(any) ([]byte, error)
}

var hooks = probeHooks{
	lookPath:   exec.LookPath,
	homeDir:    os.UserHomeDir,
	candidates: defaultCandidates,
	marshal:    func(v any) ([]byte, error) { return json.Marshal(v) },
}

func withHooks(replacement probeHooks, fn func()) {
	prev := hooks
	if replacement.lookPath != nil {
		hooks.lookPath = replacement.lookPath
	}
	if replacement.homeDir != nil {
		hooks.homeDir = replacement.homeDir
	}
	if replacement.candidates != nil {
		hooks.candidates = replacement.candidates
	}
	if replacement.marshal != nil {
		hooks.marshal = replacement.marshal
	}
	defer func() { hooks = prev }()
	fn()
}

func defaultCandidates(tool, home string) []string {
	out := []string{
		filepath.Join("/usr/local/bin", tool),
		filepath.Join("/opt/homebrew/bin", tool),
	}
	if home != "" {
		out = append(out, filepath.Join(home, ".local", "bin", tool))
		out = append(out, filepath.Join(home, "bin", tool))
	}
	out = append(out, filepath.Join("/usr/bin", tool))
	return out
}

func Probe(tool string) (Result, error) {
	r := Result{Tool: tool}
	if path, err := hooks.lookPath(tool); err == nil && path != "" {
		r.Found = true
		r.Path = path
		r.Method = "path"
		r.Checked = append(r.Checked, fmt.Sprintf("exec.LookPath(%s) → %s", tool, path))
		return r, nil
	}
	r.Checked = append(r.Checked, fmt.Sprintf("exec.LookPath(%s) → not found", tool))

	home, homeErr := hooks.homeDir()
	if homeErr != nil {
		r.Checked = append(r.Checked, fmt.Sprintf("os.UserHomeDir → error: %v (home-prefixed paths skipped)", homeErr))
		home = ""
	}
	for _, p := range hooks.candidates(tool, home) {
		if isExecutable(p) {
			r.Checked = append(r.Checked, fmt.Sprintf("%s → executable", p))
			r.Found = true
			r.Path = p
			r.Method = "fallback"
			return r, nil
		}
		r.Checked = append(r.Checked, fmt.Sprintf("%s → not present", p))
	}
	return r, nil
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := st.Mode()
	if mode.IsDir() {
		return false
	}
	return mode.Perm()&0o111 != 0
}

func EmitJSON(r Result) ([]byte, error) {
	buf, err := hooks.marshal(r.toWire())
	if err != nil {
		return nil, fmt.Errorf("doctor: marshal: %w", err)
	}
	return buf, nil
}

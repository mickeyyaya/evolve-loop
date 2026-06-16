// Package doctor implements `evolve doctor probe <tool>` — the Go
// port of scripts/utility/probe-tool.sh.
//
// /insights documented the "Claude declares a tool unavailable based
// on a single check" failure pattern. The probe walks PATH first via
// exec.LookPath, then explicit fallback locations (Homebrew Intel +
// Apple-silicon, ~/.local/bin, ~/bin, /usr/bin), and surfaces the
// full checked-list so the operator sees the diagnostic trail.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Result is the structured payload Probe returns and EmitJSON serializes.
type Result struct {
	Tool    string   `json:"tool"`
	Found   bool     `json:"found"`
	Path    string   `json:"-"`                // serialized via custom JSON to allow null
	Method  string   `json:"method,omitempty"` // "path" (LookPath) | "fallback" (explicit dirs)
	Checked []string `json:"checked"`
}

// resultJSON is the wire shape so Path serializes as null when empty.
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

// probeHooks holds injectable seams so unit tests can drive
// PATH-miss + fallback-hit + home-error branches deterministically.
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

// defaultCandidates returns the explicit fallback paths probe-tool.sh
// walks after a PATH miss. Order matches the bash script verbatim:
// /usr/local/bin → /opt/homebrew/bin → ~/.local/bin → ~/bin → /usr/bin.
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

// Probe locates a CLI tool by walking PATH, then explicit fallback
// directories. Returns a populated Result; the error channel is
// reserved for unexpected I/O failures and is currently never set —
// missing-tool is communicated via Found=false.
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

// isExecutable returns true if path is a file and any of its mode-bits
// include the executable flag. On Windows the check degrades to "is
// a regular file" since the executable bit isn't carried in NTFS ACLs
// the same way.
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

// EmitJSON serializes a Result to the bash-contract wire shape:
// {tool, found, path, method, checked:[...]}. Path is null when empty.
func EmitJSON(r Result) ([]byte, error) {
	buf, err := hooks.marshal(r.toWire())
	if err != nil {
		return nil, fmt.Errorf("doctor: marshal: %w", err)
	}
	return buf, nil
}

// Package posteditvalidate ports legacy/scripts/verification/postedit-validate.sh.
//
// PostToolUse hook for Edit|Write tool calls. Validates the file that was
// just edited or written by extension:
//
//	.json → jq empty (parse check)
//	.sh   → bash -n  (syntax check)
//	.py   → python3 -m py_compile
//	other → silent no-op
//
// This hook NEVER blocks (PostToolUse fires AFTER the tool ran). On a
// detected issue, it emits a stderr WARN that Claude Code surfaces to the
// LLM, prompting an immediate re-edit.
//
// Bypass: Options.Bypass, wired from the command's --bypass flag.
//
// Always returns nil error — exit codes are non-blocking by design. The
// cmd layer always exits 0.
package posteditvalidate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options drives a Run() invocation.
type Options struct {
	Payload     []byte    // raw JSON tool_use_input payload from stdin
	ProjectRoot string    // for guards.log path; required
	LLMStderr   io.Writer // visible to the LLM via Claude Code's reminder mechanism
	GuardsLog   string    // optional override; defaults to <ProjectRoot>/.evolve/guards.log
	Bypass      bool      // explicit emergency bypass
	Now         func() time.Time

	// Validator seams: each takes the file path, returns (ok, errMsg).
	// Defaults shell out to jq / bash / python3 respectively.
	ValidateJSON func(filePath string) (bool, string)
	ValidateBash func(filePath string) (bool, string)
	ValidatePy   func(filePath string) (bool, string)
}

// Result describes what happened (mostly for tests + observability).
type Result struct {
	Kind        string // "skip" | "json" | "sh" | "py" | "noop" | "bypass"
	FilePath    string
	OK          bool
	WarnEmitted bool
}

// Run validates the just-edited file. Always returns nil error to keep
// the cmd layer's exit code at 0 (PostToolUse cannot block).
func Run(opts Options) Result {
	res := Result{}

	logw := opts.LLMStderr
	if logw == nil {
		logw = io.Discard
	}
	warnLLM := func(format string, args ...any) {
		res.WarnEmitted = true
		fmt.Fprintf(logw, "[postedit-validate] "+format+"\n", args...)
	}

	// Resolve guards.log path.
	guardsLog := opts.GuardsLog
	if guardsLog == "" && opts.ProjectRoot != "" {
		guardsLog = filepath.Join(opts.ProjectRoot, ".evolve", "guards.log")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logf := func(format string, args ...any) {
		appendGuardsLog(guardsLog, now(), fmt.Sprintf(format, args...))
	}

	if len(opts.Payload) == 0 {
		logf("no-payload; skip")
		res.Kind = "skip"
		return res
	}

	if opts.Bypass {
		logf("WARN: --bypass active; bypassing")
		res.Kind = "bypass"
		return res
	}

	filePath := extractFilePath(opts.Payload)
	if filePath == "" {
		logf("no file_path in payload; skip")
		res.Kind = "skip"
		return res
	}
	res.FilePath = filePath

	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		logf("file does not exist (deleted?): %s; skip", filePath)
		res.Kind = "skip"
		return res
	}

	// Resolve validator defaults.
	vJSON := opts.ValidateJSON
	if vJSON == nil {
		vJSON = defaultValidateJSON
	}
	vBash := opts.ValidateBash
	if vBash == nil {
		vBash = defaultValidateBash
	}
	vPy := opts.ValidatePy
	if vPy == nil {
		vPy = defaultValidatePy
	}

	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".json":
		res.Kind = "json"
		ok, errMsg := vJSON(filePath)
		res.OK = ok
		if ok {
			logf("OK: %s (json)", filePath)
		} else {
			logf("WARN: invalid JSON in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s does NOT parse as valid JSON: %s", filePath, errMsg)
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
	case ".sh":
		res.Kind = "sh"
		ok, errMsg := vBash(filePath)
		res.OK = ok
		if ok {
			logf("OK: %s (bash syntax)", filePath)
		} else {
			logf("WARN: bash syntax error in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s has a bash syntax error: %s", filePath, errMsg)
			warnLLM("  Common causes: bash 4+ features (declare -A, mapfile) on a 3.2 target; unbalanced quotes; missing fi/done.")
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
	case ".py":
		res.Kind = "py"
		ok, errMsg := vPy(filePath)
		res.OK = ok
		if ok {
			logf("OK: %s (py_compile)", filePath)
			// Clean up __pycache__ left behind by py_compile.
			cleanPyCache(filePath)
		} else {
			logf("WARN: python compile error in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s has a Python compile error: %s", filePath, errMsg)
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
	default:
		res.Kind = "noop"
		res.OK = true
	}
	return res
}

// --- Payload parsing -------------------------------------------------------

type payloadShape struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

func extractFilePath(payload []byte) string {
	var p payloadShape
	if err := json.Unmarshal(payload, &p); err == nil && p.ToolInput.FilePath != "" {
		return p.ToolInput.FilePath
	}
	return ""
}

// --- Default validators ----------------------------------------------------

func defaultValidateJSON(filePath string) (bool, string) {
	// Prefer jq for byte-parity with bash; fall back to encoding/json if jq is missing.
	if _, err := exec.LookPath("jq"); err == nil {
		cmd := exec.Command("jq", "empty", filePath)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return true, ""
		}
		return false, strings.TrimSpace(string(out))
	}
	body, err := os.ReadFile(filePath)
	if err != nil {
		return false, err.Error()
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func defaultValidateBash(filePath string) (bool, string) {
	if _, err := exec.LookPath("bash"); err != nil {
		return true, "" // bash unavailable: don't false-positive
	}
	cmd := exec.Command("bash", "-n", filePath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, ""
	}
	return false, strings.TrimSpace(string(out))
}

func defaultValidatePy(filePath string) (bool, string) {
	py := ""
	if _, err := exec.LookPath("python3"); err == nil {
		py = "python3"
	} else if _, err := exec.LookPath("python"); err == nil {
		py = "python"
	}
	if py == "" {
		return true, "" // no python: don't false-positive
	}
	cmd := exec.Command(py, "-m", "py_compile", filePath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, ""
	}
	return false, strings.TrimSpace(string(out))
}

// cleanPyCache removes py_compile's stamped pyc files for the just-checked
// file. Mirrors bash: rm -rf "$(dirname)/__pycache__/$(basename .py)".*.pyc
func cleanPyCache(filePath string) {
	dir := filepath.Dir(filePath)
	base := strings.TrimSuffix(filepath.Base(filePath), ".py")
	cacheDir := filepath.Join(dir, "__pycache__")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	prefix := base + "."
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".pyc") {
			_ = os.Remove(filepath.Join(cacheDir, name))
		}
	}
}

// --- Guards-log append (best-effort) ---------------------------------------

func appendGuardsLog(path string, ts time.Time, msg string) {
	if path == "" {
		return
	}
	// Best-effort: ensure parent dir, ignore failures (read-only sandboxes
	// are expected to fail here; never surface to stderr).
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s] [postedit-validate] %s\n",
		ts.UTC().Format(time.RFC3339), msg)
}

// Package posteditvalidate checks a just-edited json, shell or python file and warns the model without blocking.
// See docs/architecture/packages/internal-posteditvalidate.md.
package posteditvalidate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guardslog"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Options struct {
	Payload     []byte
	ProjectRoot string
	LLMStderr   io.Writer
	GuardsLog   string
	Bypass      bool
	Now         func() time.Time

	ValidateJSON func(filePath string) (bool, string)
	ValidateBash func(filePath string) (bool, string)
	ValidatePy   func(filePath string) (bool, string)
}

type Result struct {
	Kind        string
	FilePath    string
	OK          bool
	WarnEmitted bool
}

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
	logf := guardsLogger(opts)

	filePath, skipKind := resolveTarget(opts, logf)
	res.FilePath = filePath
	if skipKind != "" {
		res.Kind = skipKind
		return res
	}

	res.Kind, res.OK = validateFile(opts, filePath, logf, warnLLM)
	return res
}

func guardsLogger(opts Options) func(format string, args ...any) {
	guardsLog := opts.GuardsLog
	if guardsLog == "" && opts.ProjectRoot != "" {
		guardsLog = filepath.Join(opts.ProjectRoot, ".evolve", "guards.log")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return func(format string, args ...any) {
		appendGuardsLog(guardsLog, now(), fmt.Sprintf(format, args...))
	}
}

func resolveTarget(opts Options, logf func(string, ...any)) (filePath, skipKind string) {
	if len(opts.Payload) == 0 {
		logf("no-payload; skip")
		return "", "skip"
	}

	if opts.Bypass {
		logf("WARN: --bypass active; bypassing")
		return "", "bypass"
	}

	filePath = extractFilePath(opts.Payload)
	if filePath == "" {
		logf("no file_path in payload; skip")
		return "", "skip"
	}

	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		logf("file does not exist (deleted?): %s; skip", filePath)
		return filePath, "skip"
	}
	return filePath, ""
}

func resolveValidators(opts Options) (vJSON, vBash, vPy func(string) (bool, string)) {
	vJSON = opts.ValidateJSON
	if vJSON == nil {
		vJSON = defaultValidateJSON
	}
	vBash = opts.ValidateBash
	if vBash == nil {
		vBash = defaultValidateBash
	}
	vPy = opts.ValidatePy
	if vPy == nil {
		vPy = defaultValidatePy
	}
	return vJSON, vBash, vPy
}

func validateFile(opts Options, filePath string, logf, warnLLM func(string, ...any)) (kind string, ok bool) {
	vJSON, vBash, vPy := resolveValidators(opts)
	var errMsg string
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".json":
		ok, errMsg = vJSON(filePath)
		if ok {
			logf("OK: %s (json)", filePath)
		} else {
			logf("WARN: invalid JSON in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s does NOT parse as valid JSON: %s", filePath, errMsg)
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
		return "json", ok
	case ".sh":
		ok, errMsg = vBash(filePath)
		if ok {
			logf("OK: %s (bash syntax)", filePath)
		} else {
			logf("WARN: bash syntax error in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s has a bash syntax error: %s", filePath, errMsg)
			warnLLM("  Common causes: bash 4+ features (declare -A, mapfile) on a 3.2 target; unbalanced quotes; missing fi/done.")
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
		return "sh", ok
	case ".py":
		ok, errMsg = vPy(filePath)
		if ok {
			logf("OK: %s (py_compile)", filePath)
			cleanPyCache(filePath)
		} else {
			logf("WARN: python compile error in %s: %s", filePath, errMsg)
			warnLLM("WARN: just-edited file %s has a Python compile error: %s", filePath, errMsg)
			warnLLM("  Re-read and fix before continuing. Emergency bypass: --bypass.")
		}
		return "py", ok
	default:
		return "noop", true
	}
}

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

func defaultValidateJSON(filePath string) (bool, string) {
	if _, err := exec.LookPath("jq"); err == nil {
		cmd := sysexec.Command(context.Background(), "jq", "empty", filePath)
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
		return true, ""
	}
	cmd := sysexec.Command(context.Background(), "bash", "-n", filePath)
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
		return true, ""
	}
	cmd := sysexec.Command(context.Background(), py, "-m", "py_compile", filePath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, ""
	}
	return false, strings.TrimSpace(string(out))
}

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

func appendGuardsLog(path string, ts time.Time, msg string) {
	_ = guardslog.Append(path, "postedit-validate", msg, ts)
}

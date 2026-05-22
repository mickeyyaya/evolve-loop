// Package acsrunner drives `go test -json ./acs/cycle-N/...` and
// aggregates per-test verdicts into the acs-verdict.json schema the
// EGPS gate consumes (red_count == 0 → ship-eligible).
package acsrunner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Verdict is the schema written to acs-verdict.json.
type Verdict struct {
	Cycle      int         `json:"cycle"`
	Total      int         `json:"total"`
	RedCount   int         `json:"red_count"`
	Predicates []Predicate `json:"predicates"`
}

// Predicate captures one test's outcome.
type Predicate struct {
	Name       string `json:"name"`
	Verdict    string `json:"verdict"` // PASS | FAIL | SKIP
	DurationMS int    `json:"duration_ms"`
	Output     string `json:"output,omitempty"`
}

// testJSONLine matches the schema emitted by `go test -json`.
type testJSONLine struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// ParseTestJSON walks NDJSON test events and aggregates them into a
// Verdict for the given cycle number. Lines without a Test field
// (package-level events) are ignored. Invalid JSON lines are skipped.
func ParseTestJSON(r io.Reader, cycle int) (Verdict, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	preds := map[string]*Predicate{}
	order := []string{}
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var line testJSONLine
		if err := json.Unmarshal(raw, &line); err != nil {
			continue // tolerate garbage lines (e.g. build output prelude)
		}
		if line.Test == "" {
			continue
		}
		p, ok := preds[line.Test]
		if !ok {
			p = &Predicate{Name: line.Test}
			preds[line.Test] = p
			order = append(order, line.Test)
		}
		switch line.Action {
		case "output":
			p.Output += line.Output
		case "pass":
			p.Verdict = "PASS"
			p.DurationMS = int(line.Elapsed * 1000)
		case "fail":
			p.Verdict = "FAIL"
			p.DurationMS = int(line.Elapsed * 1000)
		case "skip":
			p.Verdict = "SKIP"
			p.DurationMS = int(line.Elapsed * 1000)
		}
	}
	if err := scanner.Err(); err != nil {
		return Verdict{}, fmt.Errorf("acsrunner scan: %w", err)
	}
	v := Verdict{Cycle: cycle}
	for _, name := range order {
		p := preds[name]
		v.Total++
		if p.Verdict == "FAIL" {
			v.RedCount++
		}
		v.Predicates = append(v.Predicates, *p)
	}
	return v, nil
}

// runCommander is a testable seam over `go test -json` invocation.
// It returns a stdout reader plus a wait() func; tests inject canned
// outputs and forced errors without spinning up an actual `go test`.
type runCommander func(ctx context.Context, args ...string) (stdout io.ReadCloser, wait func() error, err error)

var execCommand runCommander = func(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start: %w", err)
	}
	return stdout, cmd.Wait, nil
}

func withCommander(c runCommander, fn func()) {
	prev := execCommand
	execCommand = c
	defer func() { execCommand = prev }()
	fn()
}

// Run executes `go test -json <pkg>` and returns the parsed Verdict.
// pkg is typically "./acs/cycle-N/..." or an explicit module-qualified
// import path.
func Run(ctx context.Context, cycle int, pkg string) (Verdict, error) {
	stdout, wait, err := execCommand(ctx, "go", "test", "-json", "-count=1", pkg)
	if err != nil {
		return Verdict{}, fmt.Errorf("acsrunner: %w", err)
	}
	v, parseErr := ParseTestJSON(stdout, cycle)
	waitErr := wait()
	if parseErr != nil {
		return v, parseErr
	}
	// `go test` exits non-zero when any test fails — that's the
	// reportable case, not a runner failure. We propagate the parsed
	// verdict; the exit error itself is informational.
	if waitErr != nil && v.RedCount == 0 {
		return v, fmt.Errorf("acsrunner go test: %w", waitErr)
	}
	return v, nil
}

// writeHooks holds testable seams for the verdict file write.
type writeHooks struct {
	marshal func(any) ([]byte, error)
	write   func(f *os.File, b []byte) (int, error)
	closeF  func(f *os.File) error
	rename  func(oldpath, newpath string) error
}

var whooks = writeHooks{
	marshal: func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") },
	write:   func(f *os.File, b []byte) (int, error) { return f.Write(b) },
	closeF:  func(f *os.File) error { return f.Close() },
	rename:  os.Rename,
}

func withWriteHooks(replacement writeHooks, fn func()) {
	prev := whooks
	if replacement.marshal != nil {
		whooks.marshal = replacement.marshal
	}
	if replacement.write != nil {
		whooks.write = replacement.write
	}
	if replacement.closeF != nil {
		whooks.closeF = replacement.closeF
	}
	if replacement.rename != nil {
		whooks.rename = replacement.rename
	}
	defer func() { whooks = prev }()
	fn()
}

// WriteVerdict serializes v to <evolveDir>/runs/cycle-<N>/acs-verdict.json
// atomically (tmp + rename).
func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", v.Cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir verdict dir: %w", err)
	}
	dst := filepath.Join(dir, "acs-verdict.json")
	buf, err := whooks.marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal verdict: %w", err)
	}
	buf = append(buf, '\n')
	tmp, err := os.CreateTemp(dir, "acs-verdict.*.tmp")
	if err != nil {
		return "", fmt.Errorf("verdict tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := whooks.write(tmp, buf); err != nil {
		_ = whooks.closeF(tmp)
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write verdict: %w", err)
	}
	if err := whooks.closeF(tmp); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close verdict: %w", err)
	}
	if err := whooks.rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename verdict: %w", err)
	}
	return dst, nil
}

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
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Verdict struct {
	SchemaVersion   string               `json:"schema_version"`
	Cycle           int                  `json:"cycle"`
	Total           int                  `json:"total"`
	GreenCount      int                  `json:"green_count"`
	RedCount        int                  `json:"red_count"`
	SkipCount       int                  `json:"skip_count"`
	RedIDs          []string             `json:"red_ids,omitempty"`
	IncompleteCount int                  `json:"incomplete_count,omitempty"`
	IncompleteIDs   []string             `json:"incomplete_ids,omitempty"`
	Verdict         string               `json:"verdict"`
	ShipEligible    bool                 `json:"ship_eligible"`
	PredicateSuite  PredicateSuiteCounts `json:"predicate_suite"`
	Predicates      []Predicate          `json:"predicates"`
}

type PredicateSuiteCounts struct {
	Total int `json:"total"`
}

const verdictSchemaVersion = "1.0"

type Predicate struct {
	Name       string `json:"name"`
	Verdict    string `json:"verdict"`
	DurationMS int    `json:"duration_ms"`
	Output     string `json:"output,omitempty"`
}

type testJSONLine struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

func ParseTestJSON(r io.Reader, cycle int) (Verdict, error) {
	preds, order, err := scanPredicates(r)
	if err != nil {
		return Verdict{}, err
	}
	v := Verdict{SchemaVersion: verdictSchemaVersion, Cycle: cycle}
	for _, name := range order {
		v.record(*preds[name])
	}
	if v.Total == 0 {
		v.record(noPredicatesRed())
	}
	v.ShipEligible = v.RedCount == 0 && v.IncompleteCount == 0
	v.Verdict = "FAIL"
	if v.ShipEligible {
		v.Verdict = "PASS"
	}
	v.PredicateSuite.Total = v.Total
	return v, nil
}

func scanPredicates(r io.Reader) (map[string]*Predicate, []string, error) {
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
			continue
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
		return nil, nil, fmt.Errorf("acsrunner scan: %w", err)
	}
	return preds, order, nil
}

func (v *Verdict) record(p Predicate) {
	v.Total++
	switch p.Verdict {
	case "PASS":
		v.GreenCount++
	case "FAIL":
		v.RedCount++
		v.RedIDs = append(v.RedIDs, p.Name)
	case "SKIP":
		v.SkipCount++
	default:
		v.IncompleteCount++
		v.IncompleteIDs = append(v.IncompleteIDs, p.Name)
	}
	v.Predicates = append(v.Predicates, p)
}

func noPredicatesRed() Predicate {
	return Predicate{Name: acsverdict.NoPredicatesID, Verdict: "FAIL", Output: "go test reported no predicate: the package has no test, does not exist, or did not compile"}
}

func (v Verdict) hasPredicateRed() bool {
	for _, id := range v.RedIDs {
		if !strings.HasPrefix(id, acsverdict.SyntheticRedPrefix) {
			return true
		}
	}
	return false
}

type runCommander func(ctx context.Context, args ...string) (stdout io.ReadCloser, wait func() error, err error)

var execCommand runCommander = func(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	cmd := sysexec.Command(ctx, args[0], args[1:]...)
	cmd.Env = ipcenv.Scrub(os.Environ())
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
	if waitErr != nil && !v.hasPredicateRed() {
		return v, fmt.Errorf("acsrunner go test: %w", waitErr)
	}
	return v, nil
}

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

func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dst, err := acsverdict.Path(evolveDir, v.Cycle)
	if err != nil {
		return "", fmt.Errorf("verdict path: %w", err)
	}
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir verdict dir: %w", err)
	}
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

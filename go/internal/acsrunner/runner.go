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
//
// LOCKSTEP WITH acssuite.Verdict, and now asserted rather than asserted-about:
// this comment used to claim the two producers stayed in step while this type
// omitted `ship_eligible`, `verdict`, `green_count`, `red_ids` and
// `schema_version`. The audit's EGPS reader takes ship_eligible as a POINTER so
// that pre-stamp verdicts stay back-compatible — absent means "no opinion", and
// the gate honours that by NOT blocking. A verdict written through this path
// therefore SILENTLY DISABLED the ship-eligibility gate.
//
// The fields below are a PROJECTION of the same facts the counters already
// carry (verdict and ship-eligibility are both functions of RedCount), computed
// once at construction so a reader can never see a set of counters and a
// disagreeing headline. schema_lockstep_test.go decodes this type through the
// gate's own view and fails if any of them goes missing again.
type Verdict struct {
	SchemaVersion string   `json:"schema_version"`
	Cycle         int      `json:"cycle"`
	Total         int      `json:"total"`
	GreenCount    int      `json:"green_count"`
	RedCount      int      `json:"red_count"`
	SkipCount     int      `json:"skip_count"`
	RedIDs        []string `json:"red_ids,omitempty"`
	// IncompleteCount counts predicates whose stream never reported an
	// outcome — the shape a SIGKILLed, timed-out or panic-aborted run leaves
	// behind. Counted separately from red because it is a different fact
	// (nothing was learned about them), and surfaced because a suite that did
	// not finish must never read as one that passed.
	IncompleteCount int      `json:"incomplete_count,omitempty"`
	IncompleteIDs   []string `json:"incomplete_ids,omitempty"`
	Verdict         string   `json:"verdict"`
	ShipEligible    bool     `json:"ship_eligible"`
	// PredicateSuite mirrors acssuite's nested shape. The ship gate reads
	// predicate_suite.total for the message it prints while an operator is
	// holding a block; acsrunner wrote only the flat `total`, so that message
	// said "total=0" on every verdict this producer wrote (review MEDIUM).
	PredicateSuite PredicateSuiteCounts `json:"predicate_suite"`
	Predicates     []Predicate          `json:"predicates"`
}

// PredicateSuiteCounts is the nested counter block acssuite emits and the ship
// gate reads. Projected from the same totals as the flat fields, so the two
// views cannot disagree.
type PredicateSuiteCounts struct {
	Total int `json:"total"`
}

// verdictSchemaVersion matches acssuite's, because the two files are read by
// the same consumers and a version that differed per producer would tell a
// reader nothing.
const verdictSchemaVersion = "1.0"

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
	v := Verdict{SchemaVersion: verdictSchemaVersion, Cycle: cycle}
	for _, name := range order {
		p := preds[name]
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
			// Empty verdict = the stream ended before this predicate reported.
			// The default arm used to fall through to GREEN, so a killed suite
			// aggregated to red_count=0 and then claimed ship-eligibility
			// (review BLOCK). Nothing was learned about this predicate; the
			// honest counter is its own.
			v.IncompleteCount++
			v.IncompleteIDs = append(v.IncompleteIDs, p.Name)
		}
		v.Predicates = append(v.Predicates, *p)
	}
	// Derived once, here, from the counters above — never by a caller, so the
	// headline and the counts cannot disagree. Same rule as acssuite: a cycle
	// ships only when nothing is red, and a SKIP is neither red nor green.
	// A run ships only when nothing failed AND nothing was left unfinished. A
	// SKIP is a declared non-result and stays neutral; an INCOMPLETE is an
	// undeclared one and must not be read as consent.
	v.ShipEligible = v.RedCount == 0 && v.IncompleteCount == 0
	v.Verdict = "FAIL"
	if v.ShipEligible {
		v.Verdict = "PASS"
	}
	v.PredicateSuite.Total = v.Total
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

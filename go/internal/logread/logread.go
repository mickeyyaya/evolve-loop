// Package logread is the bounded read path for the logs of one cycle. Each verb
// has a hard line budget, ends with a truncation line that names a narrower
// query, and never returns a whole file. See docs/architecture/packages/internal-logread.md.
package logread

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	BudgetD0     = 40
	BudgetD1     = 200
	BudgetD2     = 200
	BudgetD3     = 150
	MaxLineBytes = 1024
)

type Layer string

const (
	D0 Layer = "D0"
	D1 Layer = "D1"
	D2 Layer = "D2"
	D3 Layer = "D3"
)

func ParseLayer(s string) (Layer, bool) {
	l := Layer(strings.ToUpper(s))
	switch l {
	case D0, D1, D2, D3:
		return l, true
	}
	return "", false
}

type Reader struct {
	RunDir  string
	Cycle   int
	Signals *signalcenter.Center
}

type Query struct {
	Phase string
	Kind  string
	Code  string
	Level signalcenter.Severity
	From  int
}

var ErrNoLog = errors.New("logread: no log for the query")

func (r Reader) Digest() ([]string, error) {
	events, err := r.events()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	lines := []string{}
	failed := 0
	for _, e := range events {
		if e.Kind != signalcenter.KindPhaseOutcome {
			continue
		}
		verdict := e.Fields["verdict"]
		line := e.Phase + " " + orUnknown(verdict)
		if ms := e.Fields["duration_ms"]; ms != "" {
			line += " " + ms + "ms"
		}
		if verdict != "PASS" && verdict != "SKIPPED" {
			failed++
			line += " -> " + r.command("events", Query{Phase: e.Phase})
		}
		lines = append(lines, line)
	}
	head := fmt.Sprintf("cycle %d: %d phase outcomes, %d failed", r.Cycle, len(lines), failed)
	narrower := r.command("events", Query{Kind: string(signalcenter.KindPhaseOutcome)})
	return bound(append([]string{head}, lines...), BudgetD0, narrower), nil
}

func (r Reader) Events(q Query) ([]string, error) {
	return r.query("events", q, BudgetD1)
}

func (r Reader) Failures(q Query) ([]string, error) {
	q.Level = signalcenter.SeverityWarn
	return r.query("failures", q, BudgetD1)
}

func (r Reader) query(verb string, q Query, budget int) ([]string, error) {
	events, err := r.events()
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, e := range events {
		if q.matches(e) {
			matched = append(matched, signalcenter.FormatLine(e))
		}
	}
	skip := max(q.From-1, 0)
	if skip > len(matched) {
		skip = len(matched)
	}
	window := matched[skip:]
	next := q
	next.From = skip + budget
	return bound(window, budget, r.command(verb, next)), nil
}

func (q Query) matches(e signalcenter.Event) bool {
	return (q.Phase == "" || e.Phase == q.Phase) &&
		(q.Kind == "" || string(e.Kind) == q.Kind) &&
		(q.Code == "" || string(e.Code) == q.Code) &&
		(q.Level == "" || e.Severity.AtLeast(q.Level))
}

func (r Reader) Index(from int) ([]string, error) {
	entries, err := os.ReadDir(r.RunDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoLog, err)
	}
	var rows []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		size := int64(-1)
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		rows = append(rows, e.Name()+" "+strconv.FormatInt(size, 10)+" bytes")
	}
	sort.Strings(rows)
	skip := min(max(from-1, 0), len(rows))
	return bound(rows[skip:], BudgetD1, fmt.Sprintf("evolve logs index --cycle %d --from %d", r.Cycle, skip+BudgetD1)), nil
}

func (r Reader) Show(phase string, layer Layer, from int) ([]string, error) {
	budget, names := BudgetD2, []string{phase + "-report.md"}
	if layer == D3 {
		budget, names = BudgetD3, []string{phase + "-stdout.clean.txt", phase + "-stdout.log"}
	}
	path := ""
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(r.RunDir, n)); err == nil {
			path = filepath.Join(r.RunDir, n)
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("%w: cycle %d has no %s file for phase %s (try evolve logs index --cycle %d)", ErrNoLog, r.Cycle, strings.ToLower(string(layer)), phase, r.Cycle)
	}
	start := max(from, 1)
	window, more, err := readWindow(path, start, budget-1)
	if err != nil {
		return nil, err
	}
	if len(window) == 0 && more == 0 {
		return []string{fmt.Sprintf("no line at or after %d in the %s file of phase %s", start, strings.ToLower(string(layer)), phase)}, nil
	}
	if more > 0 {
		narrower := fmt.Sprintf("evolve logs show --cycle %d --phase %s --layer %s --from %d", r.Cycle, phase, strings.ToLower(string(layer)), start+len(window))
		window = append(window, truncationLine("more lines", narrower))
	}
	return window, nil
}

func readWindow(path string, from, count int) (window []string, more int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrNoLog, err)
	}
	defer func() { _ = f.Close() }()
	br := bufio.NewReader(f)
	for n := 1; ; n++ {
		line, readErr := readBoundedLine(br)
		if readErr == io.EOF && line == nil {
			return window, more, nil
		}
		if readErr != nil && readErr != io.EOF {
			return nil, 0, readErr
		}
		switch {
		case n < from:
		case len(window) < count:
			window = append(window, *line)
		default:
			return window, 1, nil
		}
		if readErr == io.EOF {
			return window, more, nil
		}
	}
}

func readBoundedLine(br *bufio.Reader) (*string, error) {
	var buf []byte
	cut := false
	for {
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			if len(buf) == 0 && !cut {
				return nil, err
			}
			s := finishLine(buf, cut)
			return &s, err
		}
		if room := MaxLineBytes - len(buf); len(chunk) > room {
			buf = append(buf, chunk[:room]...)
			cut = true
		} else {
			buf = append(buf, chunk...)
		}
		if !isPrefix {
			s := finishLine(buf, cut)
			return &s, nil
		}
	}
}

func finishLine(buf []byte, cut bool) string {
	s := string(buf)
	if cut {
		s += "…"
	}
	return s
}

func (r Reader) events() ([]signalcenter.Event, error) {
	chunk, err := signalcenter.ReadStream(filepath.Join(r.RunDir, signalcenter.StreamFileName), 0)
	if err != nil {
		return nil, err
	}
	return chunk.Events, nil
}

func (r Reader) command(verb string, q Query) string {
	parts := []string{"evolve logs", verb, "--cycle", strconv.Itoa(r.Cycle)}
	for _, f := range []struct{ flag, value string }{
		{"--phase", q.Phase}, {"--kind", q.Kind}, {"--code", q.Code},
	} {
		if f.value != "" {
			parts = append(parts, f.flag, f.value)
		}
	}
	if verb != "failures" && q.Level != "" {
		parts = append(parts, "--level", strings.ToLower(string(q.Level)))
	}
	if q.From > 1 {
		parts = append(parts, "--from", strconv.Itoa(q.From))
	}
	return strings.Join(parts, " ")
}

func bound(lines []string, budget int, narrower string) []string {
	if len(lines) <= budget {
		return capLines(lines)
	}
	kept := capLines(lines[:budget-1])
	return append(kept, truncationLine(fmt.Sprintf("%d more lines", len(lines)-(budget-1)), narrower))
}

func truncationLine(what, narrower string) string {
	return "truncated: " + what + "; narrower query: " + narrower
}

func capLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = capLine(l)
	}
	return out
}

func capLine(l string) string {
	if len(l) <= MaxLineBytes {
		return l
	}
	return l[:MaxLineBytes] + "…"
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	warnRatio   = 1.5
	warnMargin  = 30.0
	warnHeading = "## Packages above their recorded p90"
)

var classCeilings = map[string]float64{"small": 60, "medium": 300, "large": 600}

type record struct {
	Class string  `json:"class"`
	P90   float64 `json:"p90"`
}

type ratchetFile struct {
	Packages map[string]record `json:"packages"`
}

func loadRatchet(path string) (*ratchetFile, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ratchet %s: %w", path, err)
	}
	var rf ratchetFile
	if err := json.Unmarshal(body, &rf); err != nil {
		return nil, fmt.Errorf("ratchet %s: %w", path, err)
	}
	for name, rec := range rf.Packages {
		if _, ok := classCeilings[rec.Class]; !ok {
			return nil, fmt.Errorf("ratchet %s: package %s has unknown class %q", path, name, rec.Class)
		}
		if rec.P90 <= 0 {
			return nil, fmt.Errorf("ratchet %s: package %s has p90 %v, want a positive number of seconds", path, name, rec.P90)
		}
	}
	return &rf, nil
}

func (rf *ratchetFile) limit(pkg string, timeout float64) float64 {
	rec, ok := rf.Packages[shortPkg(pkg)]
	if !ok {
		return timeout
	}
	return min(2*classCeilings[rec.Class], timeout)
}

func (rf *ratchetFile) failures(rep *Report, timeout float64) []string {
	var out []string
	for _, p := range rep.Packages {
		if limit := rf.limit(p.Pkg, timeout); p.Wall > limit {
			out = append(out, fmt.Sprintf("%s ran %.2fs, above its limit of %.2fs", shortPkg(p.Pkg), p.Wall, limit))
		}
	}
	return out
}

func (rf *ratchetFile) regressed(rep *Report) []pkgStat {
	var out []pkgStat
	for _, p := range rep.Packages {
		rec, ok := rf.Packages[shortPkg(p.Pkg)]
		if ok && p.Wall > rec.P90*warnRatio && p.Wall-rec.P90 > warnMargin {
			out = append(out, p)
		}
	}
	return out
}

func (rf *ratchetFile) writeWarnings(w io.Writer, rep *Report) {
	fmt.Fprintf(w, "\n%s\n\n", warnHeading)
	regressed := rf.regressed(rep)
	if len(regressed) == 0 {
		fmt.Fprint(w, "_(none)_\n")
		return
	}
	fmt.Fprint(w, "| Package | Recorded p90 (s) | Measured wall (s) |\n|---|--:|--:|\n")
	for _, p := range regressed {
		fmt.Fprintf(w, "| %s | %.2f | %.2f |\n", shortPkg(p.Pkg), rf.Packages[shortPkg(p.Pkg)].P90, p.Wall)
	}
}

func (rf *ratchetFile) tighten(rep *Report) bool {
	changed := false
	for _, p := range rep.Packages {
		name := shortPkg(p.Pkg)
		rec, ok := rf.Packages[name]
		if ok && p.Status != "" && p.Wall > 0 && p.Wall < rec.P90 {
			rec.P90 = p.Wall
			rf.Packages[name] = rec
			changed = true
		}
	}
	return changed
}

func (rf *ratchetFile) save(path string) error {
	body, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func runTighten(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := newFlagSet("testlatency tighten", stderr)
	path := fs.String("ratchet", "", "path of the ratchet file to lower")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		fmt.Fprintln(stderr, "testlatency: tighten needs -ratchet")
		return 2
	}
	rf, err := loadRatchet(*path)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 2
	}
	rep, err := Parse(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return 1
	}
	if rf.tighten(rep) {
		if err := rf.save(*path); err != nil {
			fmt.Fprintf(stderr, "testlatency: %v\n", err)
			return 2
		}
	}
	return 0
}

func loadGate(path, rawTimeout string, stderr io.Writer) (*ratchetFile, float64, bool) {
	if path == "" {
		return nil, 0, true
	}
	timeout, err := parseTimeout(rawTimeout)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return nil, 0, false
	}
	rf, err := loadRatchet(path)
	if err != nil {
		fmt.Fprintf(stderr, "testlatency: %v\n", err)
		return nil, 0, false
	}
	return rf, timeout, true
}

func (rf *ratchetFile) gate(stdout, stderr io.Writer, rep *Report, timeout float64) bool {
	if rf == nil {
		return false
	}
	rf.writeWarnings(stdout, rep)
	failures := rf.failures(rep, timeout)
	if len(failures) > 0 {
		fmt.Fprint(stderr, sortedFailures(failures))
	}
	return len(failures) > 0
}

func parseTimeout(raw string) (float64, error) {
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("-test-timeout %q: want a positive duration", raw)
	}
	return d.Seconds(), nil
}

func sortedFailures(failures []string) string {
	sort.Strings(failures)
	return strings.Join(failures, "\n") + "\n"
}

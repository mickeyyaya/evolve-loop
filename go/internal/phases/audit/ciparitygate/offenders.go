package ciparitygate

import (
	"regexp"
	"strings"
)

// goCompilerDiagRe matches a Go compiler/vet diagnostic line ("file.go:12:34: …"
// or "file.go:12: …") — the line shape that names a build/vet offender.
var goCompilerDiagRe = regexp.MustCompile(`^\S+\.go:\d+(:\d+)?:`)

// offenderMarkerLine is the ONE home of "this line is a real failure marker"
// — shared by offenderLines (which additionally falls back to the last lines
// when nothing matches) and hasOffenderMarker (which must NOT inherit that
// fallback: the deadline-kill path degrades to WARN precisely when no marker
// exists, and the fallback would make every non-empty truncation look judged).
// Matching is LINE-ANCHORED on real failure markers — the old substring
// heuristics ("error"/"FAIL" anywhere in the line) kept PASSING tests' verbose
// chatter while the last-12 cap pushed the real `--- FAIL` lines out, so
// cycles 930/931/932 recorded verdicts citing 12 lines of noise with the true
// offender unknowable.
func offenderMarkerLine(ln string) bool {
	return strings.HasPrefix(ln, "--- FAIL") || // test failure header
		strings.HasPrefix(ln, "FAIL") || // go test package summary ("FAIL\tpkg…")
		strings.HasPrefix(ln, "panic:") || // runtime panic
		strings.HasPrefix(ln, "# ") || // build-failure package header
		strings.Contains(ln, "import cycle") ||
		strings.Contains(ln, "UNCOVERED") || // apicover offender lines
		strings.Contains(ln, "measurement error") || // apicover's synthesized infra line
		goCompilerDiagRe.MatchString(ln) // compiler/vet diagnostics
}

// hasOffenderMarker reports whether any line of out carries a real failure
// marker — the fallback-free projection of offenderMarkerLine.
func hasOffenderMarker(out string) bool {
	for _, ln := range strings.Split(out, "\n") {
		if offenderMarkerLine(strings.TrimSpace(ln)) {
			return true
		}
	}
	return false
}

// offenderLines extracts the lines that IDENTIFY a failure from a failing
// command's output, bounded so a runaway log cannot bloat the verdict: the
// marker lines, else the last six non-empty lines; at most the last twelve.
func offenderLines(out string) []string {
	all := strings.Split(out, "\n")
	var keep []string
	for _, ln := range all {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if offenderMarkerLine(ln) {
			keep = append(keep, ln)
		}
	}
	if len(keep) == 0 { // no recognizable marker — fall back to the last few lines
		start := len(all) - 6
		if start < 0 {
			start = 0
		}
		for _, ln := range all[start:] {
			if ln = strings.TrimSpace(ln); ln != "" {
				keep = append(keep, ln)
			}
		}
	}
	if len(keep) > 12 {
		keep = keep[len(keep)-12:]
	}
	return keep
}

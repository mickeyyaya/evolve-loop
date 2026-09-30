package ciparitygate

import (
	"regexp"
	"strings"
)

var goCompilerDiagRe = regexp.MustCompile(`^\S+\.go:\d+(:\d+)?:`)

const (
	goBuildFailurePackageHeader    = "# "
	apicoverUncoveredMarker        = "UNCOVERED"
	apicoverMeasurementErrorMarker = "measurement error"
)

const (
	markerlessFallbackTailLines = 6
	maxOffenderLines            = 12
)

func offenderMarkerLine(ln string) bool {
	return strings.HasPrefix(ln, "--- FAIL") ||
		strings.HasPrefix(ln, "FAIL") ||
		strings.HasPrefix(ln, "panic:") ||
		strings.HasPrefix(ln, goBuildFailurePackageHeader) ||
		strings.Contains(ln, "import cycle") ||
		strings.Contains(ln, apicoverUncoveredMarker) ||
		strings.Contains(ln, apicoverMeasurementErrorMarker) ||
		goCompilerDiagRe.MatchString(ln)
}

func hasOffenderMarker(out string) bool {
	for _, ln := range strings.Split(out, "\n") {
		if offenderMarkerLine(strings.TrimSpace(ln)) {
			return true
		}
	}
	return false
}

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
	if len(keep) == 0 {
		start := len(all) - markerlessFallbackTailLines
		if start < 0 {
			start = 0
		}
		for _, ln := range all[start:] {
			if ln = strings.TrimSpace(ln); ln != "" {
				keep = append(keep, ln)
			}
		}
	}
	if len(keep) > maxOffenderLines {
		keep = keep[len(keep)-maxOffenderLines:]
	}
	return keep
}

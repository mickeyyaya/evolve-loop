package builddiff

import (
	"fmt"
	"strings"
)

const (
	posthocMarker   = "<!-- POSTHOC:"
	filesBullet     = "- **Files changed:**"
	changesHeading  = "## Changes"
	notDescribed    = "(not described by the build)"
	shortBaseLength = 12
)

type metric int

const (
	noMetric metric = iota
	filesMetric
	addedMetric
	deletedMetric
	linesMetric
)

func Apply(report string, s Summary, base string, deriveErr error) (string, []string) {
	short := base[:min(len(base), shortBaseLength)]
	values := hostValues{s: s, comment: "<!-- host-derived: git diff --numstat " + short + " -->", short: short}
	var warnings []string
	if deriveErr != nil {
		cause := commentSafe.Replace(deriveErr.Error())
		values = hostValues{err: deriveErr, comment: "<!-- host could not derive the diff: " + cause + " -->"}
		warnings = append(warnings, "build-diff-summary: the host could not derive the diff of the cycle, so the report states absent: "+deriveErr.Error())
	}
	lines := strings.Split(report, "\n")
	for i, line := range lines {
		lines[i] = values.substitute(line)
	}
	out := strings.Join(lines, "\n")
	if deriveErr == nil {
		out = replaceChanges(out, s, short)
	}
	return out, warnings
}

var commentSafe = strings.NewReplacer("-->", "->", "\n", " ")

type hostValues struct {
	s       Summary
	err     error
	comment string
	short   string
}

func (v hostValues) value(m metric) string {
	if v.err != nil {
		return "absent"
	}
	switch m {
	case filesMetric:
		return fmt.Sprint(v.s.Files)
	case addedMetric:
		return fmt.Sprint(v.s.Added)
	case deletedMetric:
		return fmt.Sprint(v.s.Deleted)
	}
	return fmt.Sprintf("%d / %d", v.s.Added, v.s.Deleted)
}

func (v hostValues) substitute(line string) string {
	if strings.HasPrefix(line, filesBullet) {
		if v.err != nil {
			return filesBullet + " absent " + v.comment
		}
		return fmt.Sprintf("%s %d (host-derived from the cycle base %s)", filesBullet, v.s.Files, v.short)
	}
	at := strings.Index(line, posthocMarker)
	if at < 0 {
		return line
	}
	cellStart := strings.LastIndex(line[:at], "|")
	m := metricOf(line[:max(cellStart, 0)])
	end := strings.Index(line[at:], "-->")
	if m == noMetric || cellStart < 0 || end < 0 {
		return line
	}
	return line[:cellStart+1] + " " + v.value(m) + " " + v.comment + line[at+end+len("-->"):]
}

func metricOf(cell string) metric {
	name := strings.ToLower(strings.ReplaceAll(cell, "_", " "))
	switch {
	case strings.Contains(name, "files changed"):
		return filesMetric
	case strings.Contains(name, "lines added") && strings.Contains(name, "lines removed"):
		return linesMetric
	case strings.Contains(name, "lines added"):
		return addedMetric
	case strings.Contains(name, "lines removed"):
		return deletedMetric
	}
	return noMetric
}

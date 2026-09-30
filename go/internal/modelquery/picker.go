package modelquery

import (
	"regexp"
	"strings"
)

type PickerParser func(pane string) []string

var pickerParsers = map[string]PickerParser{
	"codex":  parseCodexPicker,
	"agy":    parseAgyPicker,
	"claude": parseClaudePicker,
}

var numberedRowRE = regexp.MustCompile(`^[^\w]*\d+\.\s+(\S+)`)

func parseCodexPicker(pane string) []string {
	var ids []string
	for _, line := range strings.Split(pane, "\n") {
		if m := numberedRowRE.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids
}

var (
	claudeRowRE    = regexp.MustCompile(`^[^\w]*\d+\.`)
	claudeFamilyRE = regexp.MustCompile(`(?i)\b(opus|sonnet|haiku)\b`)
)

func parseClaudePicker(pane string) []string {
	var ids []string
	for _, line := range strings.Split(pane, "\n") {
		if !claudeRowRE.MatchString(line) {
			continue
		}
		if m := claudeFamilyRE.FindStringSubmatch(line); m != nil {
			ids = append(ids, strings.ToLower(m[1]))
		}
	}
	return ids
}

const (
	agyRegionStart      = "Switch Model"
	agyCurrentMark      = "(current)"
	agySelectionMarkers = " \t>❯›•▸●◆"
)

var agyRegionEndPrefixes = []string{"Keyboard:", "esc to cancel"}

func parseAgyPicker(pane string) []string {
	var ids []string
	inRegion := false
	for _, line := range strings.Split(pane, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inRegion {
			if trimmed == agyRegionStart {
				inRegion = true
			}
			continue
		}
		if hasAnyPrefix(trimmed, agyRegionEndPrefixes) {
			break
		}
		name := strings.TrimLeft(line, agySelectionMarkers)
		name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), agyCurrentMark))
		if name != "" {
			ids = append(ids, name)
		}
	}
	return ids
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

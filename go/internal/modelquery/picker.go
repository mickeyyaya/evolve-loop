package modelquery

import (
	"regexp"
	"strings"
)

// PickerParser extracts the offered model ids from a captured /model picker
// pane for one CLI. Each CLI renders its picker differently, so parsing is a
// per-CLI Strategy.
type PickerParser func(pane string) []string

// pickerParsers is the per-CLI Strategy registry, keyed by base CLI name.
// ollama is absent — it has a non-interactive `ollama list` (OllamaLister) and
// no /model picker.
var pickerParsers = map[string]PickerParser{
	"codex":  parseCodexPicker,
	"agy":    parseAgyPicker,
	"claude": parseClaudePicker,
}

// numberedRowRE matches a picker list row "<chrome> N. <token> …", capturing
// the first token after the list number. Leading non-word chrome (selection
// markers ›/❯/>, box glyphs, spaces) is skipped; the digit-dot-space shape
// discriminates list rows from prose and banner lines (e.g. "gpt-5.5 medium"
// has no "N. " so it never matches).
var numberedRowRE = regexp.MustCompile(`^[^\w]*\d+\.\s+(\S+)`)

// parseCodexPicker reads codex's numbered picker rows
// ("N. <api-id>  <description>"), taking the api id (first token).
func parseCodexPicker(pane string) []string {
	var ids []string
	for _, line := range strings.Split(pane, "\n") {
		if m := numberedRowRE.FindStringSubmatch(line); m != nil {
			ids = append(ids, m[1])
		}
	}
	return ids
}

// claudeRowRE detects a claude numbered picker row; claudeFamilyRE pulls the
// dispatch-usable model family from it.
var (
	claudeRowRE    = regexp.MustCompile(`^[^\w]*\d+\.`)
	claudeFamilyRE = regexp.MustCompile(`(?i)\b(opus|sonnet|haiku)\b`)
)

// parseClaudePicker reads claude's numbered picker. The picker labels are
// aliases (Default/Sonnet/Haiku) but the dispatch-usable identifier is the
// model family — `claude --model` accepts opus|sonnet|haiku — so each row's
// family word is extracted and lower-cased.
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
	agyRegionStart = "Switch Model"
	agyCurrentMark = "(current)"
	// agySelectionMarkers are the leading runes (whitespace + common TUI cursor
	// glyphs) stripped from a picker row before the model name. agy model names
	// begin with a letter (Gemini/Claude/GPT-OSS), so trimming these is safe.
	agySelectionMarkers = " \t>❯›•▸●◆"
)

// agyRegionEndPrefixes mark the end of agy's model-list region.
var agyRegionEndPrefixes = []string{"Keyboard:", "esc to cancel"}

// parseAgyPicker reads agy's flat picker. agy rows are human display names with
// the effort baked in (e.g. "Gemini 3.5 Flash (Medium)") and no list numbers,
// so the model region is bounded by the "Switch Model" header and the keyboard
// hint footer. The selection marker and trailing "(current)" tag are stripped;
// the parenthetical effort is part of the name and kept.
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

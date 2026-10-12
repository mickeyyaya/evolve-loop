package practicewatch

import (
	"html"
	"regexp"
	"strings"
)

const monthName = `(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|June?|July?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)`

var (
	scriptOrStyle = regexp.MustCompile(`(?is)<(script|style)\b.*?</(?:script|style)\s*>`)
	markupComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	markupTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	isoDate       = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}(?:[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})?)?`)
	monthFirst    = regexp.MustCompile(`(?i)\b` + monthName + `\.? \d{1,2}(?:st|nd|rd|th)?(?:,? \d{4})?\b`)
	dayFirst      = regexp.MustCompile(`(?i)\b\d{1,2}(?:st|nd|rd|th)? ` + monthName + `\.?(?:,? \d{4})?\b`)
	relativeTime  = regexp.MustCompile(`(?i)\b(?:\d+|an?) (?:second|minute|hour|day|week|month|year)s? ago\b`)
	blankRun      = regexp.MustCompile(`[ \t\f\v\x{a0}]+`)
)

func Normalize(kind Kind, text string) string {
	if kind == KindPage || kind == KindGithubReleases {
		text = pageText(text)
	}
	return trimLines(text)
}

func pageText(markup string) string {
	text := scriptOrStyle.ReplaceAllString(markup, "\n")
	text = markupComment.ReplaceAllString(text, "\n")
	text = markupTag.ReplaceAllString(text, "\n")
	text = html.UnescapeString(text)
	text = markupTag.ReplaceAllString(text, "\n")
	text = relativeTime.ReplaceAllString(text, "")
	text = isoDate.ReplaceAllString(text, "")
	text = monthFirst.ReplaceAllString(text, "")
	return dayFirst.ReplaceAllString(text, "")
}

func trimLines(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = strings.TrimSpace(blankRun.ReplaceAllString(line, " ")); line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

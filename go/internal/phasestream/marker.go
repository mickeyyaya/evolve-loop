package phasestream

import "regexp"

// infraMarker is the system-wide infrastructure-failure vocabulary.
// Defined ONCE here (ADR-0020): the normalizer owns detection, and
// cycleclassify consumes the typed kind==infra_failure events rather
// than re-scanning raw text. Mirrors the markers cycleclassify's
// reInfrastructure regex caught (classify.go:57).
type infraMarker struct {
	re     *regexp.Regexp
	marker string
}

// infraMarkers is ordered: the first match wins, so more specific /
// higher-signal markers come before broad ones (e.g. api_529 before a
// generic overloaded match).
var infraMarkers = []infraMarker{
	{regexp.MustCompile(`(?i)operation not permitted|sandbox_apply|sandbox-exec.*not permitted|\bEPERM\b`), "eperm"},
	{regexp.MustCompile(`(?i)\b529\b|overloaded`), "api_529"},
	{regexp.MustCompile(`(?i)\b429\b|too many requests`), "api_429"},
	{regexp.MustCompile(`(?i)rate.?limit`), "rate_limit"},
	{regexp.MustCompile(`(?i)operation timed out|\bETIMEDOUT\b|timed out`), "timeout"},
	{regexp.MustCompile(`(?i)connection refused|\bECONNREFUSED\b`), "conn_refused"},
}

// detectInfraMarker returns the first matching marker name, or "" when
// the text carries no infrastructure-failure signal.
func detectInfraMarker(text string) string {
	for _, m := range infraMarkers {
		if m.re.MatchString(text) {
			return m.marker
		}
	}
	return ""
}

package phasestream

import "regexp"

type infraMarker struct {
	re     *regexp.Regexp
	marker string
}

var infraMarkers = []infraMarker{
	{regexp.MustCompile(`(?i)operation not permitted|sandbox_apply|sandbox-exec.*not permitted|\bEPERM\b`), "eperm"},
	{regexp.MustCompile(`(?i)\b529\b|overloaded`), "api_529"},
	{regexp.MustCompile(`(?i)\b429\b|too many requests`), "api_429"},
	{regexp.MustCompile(`(?i)rate.?limit`), "rate_limit"},
	{regexp.MustCompile(`(?i)operation timed out|\bETIMEDOUT\b|timed out`), "timeout"},
	{regexp.MustCompile(`(?i)connection refused|\bECONNREFUSED\b`), "conn_refused"},
}

var paneNoise = regexp.MustCompile(`(?m)^.*(?:compinit:\d+: operation not permitted|verifylock: open \S*evolve-verify\.lock: operation not permitted).*$`)

func detectInfraMarker(text string) string {
	text = paneNoise.ReplaceAllString(text, "")
	for _, m := range infraMarkers {
		if m.re.MatchString(text) {
			return m.marker
		}
	}
	return ""
}

func InfraMarker(text string) string { return detectInfraMarker(text) }

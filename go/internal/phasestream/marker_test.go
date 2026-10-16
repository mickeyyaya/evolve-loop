package phasestream

import "testing"

func TestDetectInfraMarker_SpecificMarkerBeatsBroadOne(t *testing.T) {
	cases := map[string]string{
		"429 rate limit exceeded":  "api_429",
		"rate limit reached":       "rate_limit",
		"API Error 529 overloaded": "api_529",
		"plain output":             "",
	}
	for text, want := range cases {
		if got := detectInfraMarker(text); got != want {
			t.Errorf("detectInfraMarker(%q) = %q, want %q", text, got, want)
		}
	}
}

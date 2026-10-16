package opscmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func releaseServer(t *testing.T, status int, body string) map[string]string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return map[string]string{"claude": srv.URL}
}

func TestLatestRelease_ReadsTheVersionFromTheRegistry(t *testing.T) {
	t.Parallel()
	urls := releaseServer(t, http.StatusOK, `{"version":"2.1.0"}`)
	got, err := latestRelease(http.DefaultClient, urls, "claude")
	if err != nil || got != "2.1.0" {
		t.Fatalf("latestRelease = %q, %v; want 2.1.0", got, err)
	}
}

func TestLatestRelease_RefusesWhatTheRegistryCannotAnswer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"non-200 status", http.StatusNotFound, `{}`, "HTTP 404"},
		{"undecodable body", http.StatusOK, `not json`, "invalid character"},
		{"empty version", http.StatusOK, `{"version":""}`, "empty version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := latestRelease(http.DefaultClient, releaseServer(t, tc.status, tc.body), "claude")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestLatestRelease_ReportsAnUnreachableRegistry(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	if _, err := latestRelease(http.DefaultClient, map[string]string{"claude": url}, "claude"); err == nil {
		t.Fatal("a closed registry must be reported")
	}
}

func TestLatestRelease_ReportsAMalformedSourceURL(t *testing.T) {
	t.Parallel()
	if _, err := latestRelease(http.DefaultClient, map[string]string{"claude": "http://bad host"}, "claude"); err == nil {
		t.Fatal("a malformed source URL must be reported")
	}
}

func TestDefaultLatestRelease_RefusesABinaryWithNoReleaseSource(t *testing.T) {
	t.Parallel()
	_, err := defaultLatestRelease("gemini")
	if err == nil || !strings.Contains(err.Error(), "no release source configured for gemini") {
		t.Fatalf("err = %v, want the no-source refusal", err)
	}
	for _, bin := range []string{"claude", "codex"} {
		if !strings.HasPrefix(npmLatestReleaseURLs[bin], "https://registry.npmjs.org/") {
			t.Fatalf("%s release source = %q, want the npm registry", bin, npmLatestReleaseURLs[bin])
		}
	}
}

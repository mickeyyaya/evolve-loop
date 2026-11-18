package inboxcmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func stubOpenPRRunner(stdout string, exitCode int, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		if stdout != "" {
			_, _ = io.WriteString(out, stdout)
		}
		return exitCode, err
	}
}

func TestOpenPRMatcher_MatchesExactTokenBoundaryAndRejectsSubstrings(t *testing.T) {
	t.Parallel()
	payload := `[{"number":42,"title":"fix: ad2-70 regression","headRefName":"lane/fix-other","body":"Closes ad2-7."}]`
	var stderr bytes.Buffer

	match := openPRMatcherWith(context.Background(), stubOpenPRRunner(payload, 0, nil), t.TempDir(), &stderr)

	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if !match("ad2-70") {
		t.Error("match(ad2-70) = false, want true from title")
	}
	if !match("ad2-7") {
		t.Error("match(ad2-7) = false, want true from body exact token")
	}
	if match("ad2") {
		t.Error("match(ad2) = true, want false (prefix substring only)")
	}
	if match("ad2-700") {
		t.Error("match(ad2-700) = true, want false")
	}
	if match("") {
		t.Error("match(\"\") = true, want false")
	}
}

func TestOpenPRMatcher_DoesNotConfuseShortIDWithLongerPrefixInTitle(t *testing.T) {
	t.Parallel()
	payload := `[{"number":99,"title":"fix: ad2-70 only","headRefName":"evolve/ad2-70","body":"touches ad2-70"}]`
	var stderr bytes.Buffer

	match := openPRMatcherWith(context.Background(), stubOpenPRRunner(payload, 0, nil), t.TempDir(), &stderr)

	if match("ad2-7") {
		t.Error("match(ad2-7) = true, want false when PR only names ad2-70")
	}
}

func TestOpenPRMatcher_LogsStderrAndDegradesToFalseOnGhOrJSONFailure(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]sysexec.RunFunc{
		"gh error":     stubOpenPRRunner("", -1, errors.New("gh not found")),
		"gh non-zero":  stubOpenPRRunner("", 1, nil),
		"invalid json": stubOpenPRRunner("{not-json", 0, nil),
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			match := openPRMatcherWith(context.Background(), run, t.TempDir(), &stderr)
			if match("ad2-7") {
				t.Errorf("%s: match = true, want false on failure", name)
			}
			if !strings.Contains(stderr.String(), "inbox curate: WARN open-pr") {
				t.Errorf("%s: stderr = %q, want WARN open-pr line", name, stderr.String())
			}
		})
	}
}

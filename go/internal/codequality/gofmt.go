// Package codequality runs deterministic source-format gates that mirror the
// CI `vet + fmt` step, so the autonomous cycle's audit can catch gofmt
// regressions before ship instead of leaking them to CI. Cycles 339-341
// shipped to main CI-red because the cycle-scoped audit never ran gofmt over
// the generated go/acs/cycle<N>/*.go predicate files; this package closes that
// gap in-process.
package codequality

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// UnformattedGoFiles returns the .go files under dir that are not gofmt-clean
// with simplification, matching the CI gate (`gofmt -d -s .`) exactly. It
// shells out to the same gofmt binary CI uses — so the gate and CI can never
// disagree — passing -l to list offending files and -s to apply the same
// simplification CI requires. Non-.go files are ignored by gofmt. An empty
// result means the tree is clean. A missing gofmt binary or a gofmt failure is
// returned as an error (never silently treated as clean).
func UnformattedGoFiles(dir string) ([]string, error) {
	out, err := sysexec.Command(context.Background(), "gofmt", "-l", "-s", dir).Output()
	files := nonEmptyLines(out)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// gofmt binary missing / OS-level failure: the gate cannot run.
			// Returned as an error so the caller fails OPEN (warns, never
			// silently treats the tree as clean).
			return nil, fmt.Errorf("gofmt -l -s %s: %w", dir, err)
		}
		detail := strings.TrimSpace(string(exitErr.Stderr))
		if failure, isNotParse := nonParseFailure(detail); isNotParse {
			if len(files) > 0 {
				sort.Strings(files)
				return files, nil
			}
			return nil, fmt.Errorf("gofmt -l -s %s could not read the tree: %w: %s", dir, err, failure)
		}
		files = append(files, "gofmt parse error: "+firstLine(detail))
		sort.Strings(files)
		return files, nil
	}
	sort.Strings(files)
	return files, nil
}

var gofmtParseDiagnostic = regexp.MustCompile(`^.+:\d+(:\d+)?: `)

func nonParseFailure(stderr string) (string, bool) {
	lines := nonEmptyLines([]byte(stderr))
	if len(lines) == 0 {
		return "no diagnostic on stderr", true
	}
	for _, line := range lines {
		if !gofmtParseDiagnostic.MatchString(line) {
			return line, true
		}
	}
	return "", false
}

func nonEmptyLines(b []byte) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

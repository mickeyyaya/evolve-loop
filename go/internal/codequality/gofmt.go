// Package codequality runs deterministic source-format gates that mirror the
// CI `vet + fmt` step, so the autonomous cycle's audit can catch gofmt
// regressions before ship instead of leaking them to CI.
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

func UnformattedGoFiles(dir string) ([]string, error) {
	out, err := sysexec.Command(context.Background(), "gofmt", "-l", "-s", dir).Output()
	files := nonEmptyLines(out)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
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

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var exit = os.Exit

func main() {
	exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Environ(), time.Now().UTC()))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, environ []string, now time.Time) int {
	fs := flag.NewFlagSet("vulnfilter", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allowlistPath := fs.String("allowlist", "vulncheck-allowlist.json", "path to allowlist JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	f, err := os.Open(*allowlistPath)
	if err != nil {
		fmt.Fprintf(stderr, "failed to open allowlist %s: %v\n", *allowlistPath, err)
		return 1
	}
	defer func() { _ = f.Close() }()

	entries, baseRes, err := LoadAllowlist(f, now)
	if err != nil {
		fmt.Fprintf(stderr, "failed to parse allowlist %s: %v\n", *allowlistPath, err)
		return 1
	}

	res, filterErr := FilterStream(stdin, entries, now, baseRes)

	if summaryPath := stepSummaryPath(environ); summaryPath != "" {
		_ = AppendStepSummary(summaryPath, res, entries)
	}

	if filterErr != nil {
		fmt.Fprintf(stderr, "stream decode error: %v\n", filterErr)
		return 1
	}

	for _, w := range res.WaivedFindings {
		fmt.Fprintf(stdout, "WAIVED %s (expires: %s, reason: %s)\n", w.ID, w.Expires, w.Reason)
	}
	for _, u := range res.UnlistedFindings {
		fmt.Fprintf(stderr, "UNLISTED VULNERABILITY: %s\n", u)
	}
	for _, e := range res.ExpiredEntries {
		fmt.Fprintf(stderr, "EXPIRED ALLOWLIST ENTRY: %s (expired %s)\n", e.ID, e.Expires)
	}
	for _, inv := range res.InvalidEntries {
		fmt.Fprintf(stderr, "INVALID ALLOWLIST ENTRY: %s\n", inv)
	}

	if !res.Passed {
		return 1
	}
	return 0
}

func stepSummaryPath(environ []string) string {
	for _, kv := range environ {
		if strings.HasPrefix(kv, "GITHUB_STEP_SUMMARY=") {
			return strings.TrimPrefix(kv, "GITHUB_STEP_SUMMARY=")
		}
	}
	return ""
}

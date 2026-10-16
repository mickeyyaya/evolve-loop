package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	allowlistPath := flag.String("allowlist", "vulncheck-allowlist.json", "path to allowlist JSON")
	flag.Parse()

	f, err := os.Open(*allowlistPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open allowlist %s: %v\n", *allowlistPath, err)
		os.Exit(1)
	}
	defer f.Close()

	now := time.Now().UTC()
	entries, baseRes, err := LoadAllowlist(f, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse allowlist %s: %v\n", *allowlistPath, err)
		os.Exit(1)
	}

	res, filterErr := FilterStream(os.Stdin, entries, now, baseRes)

	if summaryPath := stepSummaryPath(); summaryPath != "" {
		_ = AppendStepSummary(summaryPath, res, entries)
	}

	if filterErr != nil {
		fmt.Fprintf(os.Stderr, "stream decode error: %v\n", filterErr)
		os.Exit(1)
	}

	for _, w := range res.WaivedFindings {
		fmt.Printf("WAIVED %s (expires: %s, reason: %s)\n", w.ID, w.Expires, w.Reason)
	}
	for _, u := range res.UnlistedFindings {
		fmt.Fprintf(os.Stderr, "UNLISTED VULNERABILITY: %s\n", u)
	}
	for _, e := range res.ExpiredEntries {
		fmt.Fprintf(os.Stderr, "EXPIRED ALLOWLIST ENTRY: %s (expired %s)\n", e.ID, e.Expires)
	}
	for _, inv := range res.InvalidEntries {
		fmt.Fprintf(os.Stderr, "INVALID ALLOWLIST ENTRY: %s\n", inv)
	}

	if !res.Passed {
		os.Exit(1)
	}
}

func stepSummaryPath() string {
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GITHUB_STEP_SUMMARY=") {
			return strings.TrimPrefix(kv, "GITHUB_STEP_SUMMARY=")
		}
	}
	return ""
}

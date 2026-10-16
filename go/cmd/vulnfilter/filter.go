package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type AllowlistEntry struct {
	ID      string `json:"id"`
	Expires string `json:"expires"`
	Reason  string `json:"reason"`
}

type StreamMessage struct {
	Config   *json.RawMessage `json:"config,omitempty"`
	Progress *json.RawMessage `json:"progress,omitempty"`
	OSV      *OSV             `json:"osv,omitempty"`
	Finding  *Finding         `json:"finding,omitempty"`
}

type OSV struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

type Finding struct {
	OSV          string       `json:"osv"`
	FixedVersion string       `json:"fixed_version"`
	Trace        []TraceFrame `json:"trace"`
}

type TraceFrame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Package  string `json:"package"`
	Function string `json:"function"`
}

type Result struct {
	Passed           bool
	WaivedFindings   []AllowlistEntry
	UnlistedFindings []string
	ExpiredEntries   []AllowlistEntry
	InvalidEntries   []string
	Errors           []string
}

func parseExpiry(s string, now time.Time) (bool, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			return false, err
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
	}
	return now.After(t), nil
}

func LoadAllowlist(r io.Reader, now time.Time) ([]AllowlistEntry, *Result, error) {
	var entries []AllowlistEntry
	dec := json.NewDecoder(r)
	if err := dec.Decode(&entries); err != nil {
		return nil, nil, err
	}
	res := &Result{Passed: true}
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		reason := strings.TrimSpace(entry.Reason)
		if id == "" || reason == "" {
			res.Passed = false
			res.InvalidEntries = append(res.InvalidEntries, fmt.Sprintf("%s (reason missing or empty)", entry.ID))
			continue
		}
		expired, err := parseExpiry(strings.TrimSpace(entry.Expires), now)
		if err != nil {
			res.Passed = false
			res.InvalidEntries = append(res.InvalidEntries, fmt.Sprintf("%s (invalid expiry format: %s)", entry.ID, entry.Expires))
			continue
		}
		if expired {
			res.Passed = false
			res.ExpiredEntries = append(res.ExpiredEntries, entry)
		}
	}
	return entries, res, nil
}

func isReachable(f *Finding) bool {
	if f == nil {
		return false
	}
	for _, frame := range f.Trace {
		if strings.TrimSpace(frame.Function) != "" {
			return true
		}
	}
	return false
}

func FilterStream(stream io.Reader, entries []AllowlistEntry, now time.Time, baseResult *Result) (*Result, error) {
	res := baseResult
	if res == nil {
		res = &Result{Passed: true}
	}
	allowMap := make(map[string]AllowlistEntry)
	for _, e := range entries {
		allowMap[e.ID] = e
	}
	seenReachable := make(map[string]bool)
	dec := json.NewDecoder(stream)
	for {
		var msg StreamMessage
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			res.Passed = false
			res.Errors = append(res.Errors, err.Error())
			return res, err
		}
		if msg.Finding != nil && isReachable(msg.Finding) {
			osvID := strings.TrimSpace(msg.Finding.OSV)
			if osvID != "" && !seenReachable[osvID] {
				seenReachable[osvID] = true
				if entry, ok := allowMap[osvID]; ok {
					res.WaivedFindings = append(res.WaivedFindings, entry)
				} else {
					res.Passed = false
					res.UnlistedFindings = append(res.UnlistedFindings, osvID)
				}
			}
		}
	}
	if len(res.ExpiredEntries) > 0 || len(res.InvalidEntries) > 0 || len(res.UnlistedFindings) > 0 || len(res.Errors) > 0 {
		res.Passed = false
	}
	return res, nil
}

func WriteSummary(w io.Writer, res *Result, entries []AllowlistEntry) error {
	var sb strings.Builder
	sb.WriteString("### Vulnerability Scan Summary\n\n")
	if len(entries) > 0 {
		sb.WriteString("#### Configured Allowlist Entries\n\n")
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (expires: %s)\n", e.ID, e.Reason, e.Expires))
		}
		sb.WriteString("\n")
	}
	if len(res.WaivedFindings) > 0 {
		sb.WriteString("#### Waived Vulnerabilities\n\n")
		for _, w := range res.WaivedFindings {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (expires: %s)\n", w.ID, w.Reason, w.Expires))
		}
		sb.WriteString("\n")
	}
	if len(res.UnlistedFindings) > 0 {
		sb.WriteString("#### Unlisted Vulnerabilities\n\n")
		for _, u := range res.UnlistedFindings {
			sb.WriteString(fmt.Sprintf("- **%s**: unlisted vulnerability detected\n", u))
		}
		sb.WriteString("\n")
	}
	if len(res.ExpiredEntries) > 0 {
		sb.WriteString("#### Expired Allowlist Entries\n\n")
		for _, e := range res.ExpiredEntries {
			sb.WriteString(fmt.Sprintf("- **%s**: expired on %s\n", e.ID, e.Expires))
		}
		sb.WriteString("\n")
	}
	if len(res.InvalidEntries) > 0 {
		sb.WriteString("#### Invalid Allowlist Entries\n\n")
		for _, inv := range res.InvalidEntries {
			sb.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		sb.WriteString("\n")
	}
	if res.Passed && len(res.WaivedFindings) == 0 {
		sb.WriteString("No vulnerabilities found.\n\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func AppendStepSummary(path string, res *Result, entries []AllowlistEntry) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteSummary(f, res, entries)
}

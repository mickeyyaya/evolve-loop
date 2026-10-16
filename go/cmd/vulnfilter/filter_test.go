package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	expired, err := parseExpiry("2026-10-15T12:00:00Z", now)
	if err != nil || !expired {
		t.Fatalf("expected expired RFC3339 date: expired=%v err=%v", expired, err)
	}

	expired, err = parseExpiry("2026-10-17T12:00:00Z", now)
	if err != nil || expired {
		t.Fatalf("expected unexpired RFC3339 date: expired=%v err=%v", expired, err)
	}

	expired, err = parseExpiry("2026-10-15", now)
	if err != nil || !expired {
		t.Fatalf("expected expired DateOnly date: expired=%v err=%v", expired, err)
	}

	expired, err = parseExpiry("2026-10-17", now)
	if err != nil || expired {
		t.Fatalf("expected unexpired DateOnly date: expired=%v err=%v", expired, err)
	}

	_, err = parseExpiry("invalid-date", now)
	if err == nil {
		t.Fatal("expected error on invalid date format")
	}
}

func TestLoadAllowlist(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	validJSON := `[{"id":"GO-2026-0001","expires":"2026-11-01","reason":"mitigated"}]`
	entries, res, err := LoadAllowlist(strings.NewReader(validJSON), now)
	if err != nil || !res.Passed || len(entries) != 1 {
		t.Fatalf("valid allowlist failed: err=%v passed=%v len=%d", err, res.Passed, len(entries))
	}

	emptyReasonJSON := `[{"id":"GO-2026-0002","expires":"2026-11-01","reason":"  "}]`
	_, res, err = LoadAllowlist(strings.NewReader(emptyReasonJSON), now)
	if err != nil || res.Passed || len(res.InvalidEntries) != 1 {
		t.Fatalf("empty reason should fail: passed=%v invalid=%d", res.Passed, len(res.InvalidEntries))
	}

	expiredJSON := `[{"id":"GO-2026-0003","expires":"2026-09-01","reason":"expired waiver"}]`
	_, res, err = LoadAllowlist(strings.NewReader(expiredJSON), now)
	if err != nil || res.Passed || len(res.ExpiredEntries) != 1 {
		t.Fatalf("expired entry should fail: passed=%v expired=%d", res.Passed, len(res.ExpiredEntries))
	}

	badJSON := `[not-json`
	_, _, err = LoadAllowlist(strings.NewReader(badJSON), now)
	if err == nil {
		t.Fatal("expected error for malformed json")
	}
}

func TestFilterStream(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	entries := []AllowlistEntry{
		{ID: "GO-2026-0001", Expires: "2026-11-01", Reason: "waived"},
	}

	cleanStream := `{"config":{}}` + "\n"
	res, err := FilterStream(strings.NewReader(cleanStream), entries, now, nil)
	if err != nil || !res.Passed {
		t.Fatalf("clean stream should pass: err=%v passed=%v", err, res.Passed)
	}

	waivedStream := `{"finding":{"osv":"GO-2026-0001","trace":[{"function":"VulnFunc"}]}}` + "\n"
	res, err = FilterStream(strings.NewReader(waivedStream), entries, now, nil)
	if err != nil || !res.Passed || len(res.WaivedFindings) != 1 {
		t.Fatalf("waived stream should pass: err=%v passed=%v waived=%d", err, res.Passed, len(res.WaivedFindings))
	}

	unlistedStream := `{"finding":{"osv":"GO-2026-9999","trace":[{"function":"OtherFunc"}]}}` + "\n"
	res, err = FilterStream(strings.NewReader(unlistedStream), entries, now, nil)
	if err != nil || res.Passed || len(res.UnlistedFindings) != 1 {
		t.Fatalf("unlisted stream should fail: err=%v passed=%v unlisted=%d", err, res.Passed, len(res.UnlistedFindings))
	}

	unreachableStream := `{"finding":{"osv":"GO-2026-8888","trace":[{"module":"pkg","package":"pkg"}]}}` + "\n"
	res, err = FilterStream(strings.NewReader(unreachableStream), entries, now, nil)
	if err != nil || !res.Passed || len(res.UnlistedFindings) != 0 {
		t.Fatalf("unreachable stream without function should be ignored: passed=%v unlisted=%d", res.Passed, len(res.UnlistedFindings))
	}

	malformedStream := `{"finding":{` + "\n"
	_, err = FilterStream(strings.NewReader(malformedStream), entries, now, nil)
	if err == nil {
		t.Fatal("expected error on malformed stream")
	}
}

func TestWriteSummaryAndAppend(t *testing.T) {
	entries := []AllowlistEntry{
		{ID: "GO-2026-0001", Expires: "2026-11-01", Reason: "mitigated"},
	}
	res := &Result{
		Passed:           false,
		WaivedFindings:   entries,
		UnlistedFindings: []string{"GO-2026-9999"},
		ExpiredEntries:   []AllowlistEntry{{ID: "GO-2026-0002", Expires: "2026-01-01", Reason: "old"}},
		InvalidEntries:   []string{"GO-2026-0003 (reason missing)"},
	}

	var buf bytes.Buffer
	if err := WriteSummary(&buf, res, entries); err != nil {
		t.Fatalf("WriteSummary failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "GO-2026-0001") || !strings.Contains(out, "GO-2026-9999") {
		t.Fatalf("summary missing expected contents: %s", out)
	}

	tmpDir := t.TempDir()
	summaryFile := filepath.Join(tmpDir, "summary.md")
	if err := os.WriteFile(summaryFile, []byte("INITIAL\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AppendStepSummary(summaryFile, res, entries); err != nil {
		t.Fatalf("AppendStepSummary failed: %v", err)
	}

	content, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "INITIAL\n") || !strings.Contains(string(content), "GO-2026-0001") {
		t.Fatalf("append summary did not preserve prefix or add content: %s", string(content))
	}

	if err := AppendStepSummary("", res, entries); err != nil {
		t.Fatalf("empty summary path should succeed without error: %v", err)
	}
}

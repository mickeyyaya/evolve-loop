package failurelog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecord_StateUnreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), `{}`)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644)

	_, err := Record(path, "", RecordRequest{Cycle: 1, Classification: "audit-fail"})
	if err == nil {
		t.Fatalf("expected error from unreadable state.json")
	}
	if errors.Is(err, ErrStateMissing) {
		t.Fatalf("unreadable should NOT be ErrStateMissing (file exists, just no perms); got %v", err)
	}
}

func TestExtractSummary_CapsAtMaxLines(t *testing.T) {
	t.Parallel()
	body := "## Failure Root Cause\n"
	for i := 0; i < 12; i++ {
		body += fmt.Sprintf("line%d\n", i)
	}
	path := mustWrite(t, filepath.Join(t.TempDir(), "report.md"), body)
	s := extractSummary(path)
	if !strings.Contains(s, "line7") {
		t.Fatalf("summary should include line7: %q", s)
	}
	if strings.Contains(s, "line8") || strings.Contains(s, "line11") {
		t.Fatalf("summary should cap at 8 lines: %q", s)
	}
}

func TestMustMarshalToAny_Defensive(t *testing.T) {
	t.Parallel()
	got := mustMarshalToAny(make(chan int))
	if got == nil {
		t.Fatalf("must not return nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map for unmarshalable input; got %v", got)
	}
}

func TestPruneExpired_StateUnreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), `{}`)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644)

	_, err := PruneExpired(path, time.Now())
	if err == nil {
		t.Fatalf("expected read error from unreadable state.json")
	}
}

func TestPruneExpired_ZeroNow(t *testing.T) {
	t.Parallel()
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": yesterday},
	})
	res, err := PruneExpired(path, time.Time{})
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 1 {
		t.Fatalf("removed=%d want 1 (zero now → time.Now)", res.Removed)
	}
}

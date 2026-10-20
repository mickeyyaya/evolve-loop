package shipwindow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tryAcquireFixture(t *testing.T) (dir, queueDir, ticket string) {
	t.Helper()
	dir = t.TempDir()
	queueDir = filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ticket = "00000000000000000001-000000001-1"
	if err := os.WriteFile(filepath.Join(queueDir, ticket), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, queueDir, ticket
}

func TestTryAcquire_AMissingOwnTicketIsAnError(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := tryAcquire(dir, queueDir, "00000000000000000009-000000009-1", Options{}.normalized()); err == nil || !strings.Contains(err.Error(), "missing from queue") {
		t.Fatalf("err = %v, want the missing own ticket", err)
	}
}

func TestTryAcquire_AnUnreadableLeaseIsAnError(t *testing.T) {
	dir, queueDir, ticket := tryAcquireFixture(t)
	if err := os.Mkdir(PathIn(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := tryAcquire(dir, queueDir, ticket, Options{}.normalized()); err == nil || !strings.Contains(err.Error(), "read lease") {
		t.Fatalf("err = %v, want the read failure", err)
	}
}

func TestTryAcquire_AStaleLeaseThatCannotBeRenamedIsAnError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file modes")
	}
	dir, queueDir, ticket := tryAcquireFixture(t)
	if err := os.WriteFile(PathIn(dir), []byte(`{"owner_pid":0,"heartbeat_at":"","token":"dead"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := tryAcquire(dir, queueDir, ticket, Options{}.normalized()); err == nil || !strings.Contains(err.Error(), "break stale lease") {
		t.Fatalf("err = %v, want the break failure", err)
	}
}

func TestTryAcquire_AMarshalFailureIsAnError(t *testing.T) {
	prev := marshalRecord
	t.Cleanup(func() { marshalRecord = prev })
	marshalRecord = func(any) ([]byte, error) { return nil, errors.New("boom") }
	dir, queueDir, ticket := tryAcquireFixture(t)
	if _, err := tryAcquire(dir, queueDir, ticket, Options{}.normalized()); err == nil || !strings.Contains(err.Error(), "marshal: boom") {
		t.Fatalf("err = %v, want the marshal failure", err)
	}
}

func TestTryAcquire_ALinkRaceWaitsAndALinkFailureIsAnError(t *testing.T) {
	prev := linkLease
	t.Cleanup(func() { linkLease = prev })
	for name, tc := range map[string]struct {
		linkErr error
		wantErr string
	}{
		"a sibling linked first": {linkErr: os.ErrExist},
		"the link fails":         {linkErr: os.ErrPermission, wantErr: "create lease"},
	} {
		t.Run(name, func(t *testing.T) {
			linkLease = func(string, string) error { return &os.LinkError{Op: "link", Err: tc.linkErr} }
			dir, queueDir, ticket := tryAcquireFixture(t)
			l, err := tryAcquire(dir, queueDir, ticket, Options{Now: func() time.Time { return time.Unix(0, 0) }}.normalized())
			if l != nil {
				t.Fatal("a failed link returned a lease")
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if _, err := os.Stat(PathIn(dir) + ".tmp." + ticket); !os.IsNotExist(err) {
				t.Fatalf("the staged lease was left behind (stat err=%v)", err)
			}
		})
	}
}

//go:build acs

package cycle1932

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1932_004_InboxWriteScanEnrollsNoRecurrenceWriter(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1",
		"-run", "TestInboxWriters_EveryAutofiledItemCarriesAKnownClass/every_production_write_into_the_inbox_is_enrolled",
		"./cmd/evolve")
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the inbox write-site scan fails: %v\n%s", err, out)
	}
	contract := filepath.Join(root, "go", "cmd", "evolve", "inbox_writers_contract_test.go")
	if !acsassert.FileNotContains(t, contract, `"internal/recurrence`) {
		t.Errorf("RED: inboxWritersThatFileNothing still enrolls a recurrence writer: every inbox item write must go through internal/inboxmover")
	}
}

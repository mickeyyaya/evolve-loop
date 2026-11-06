package integration

import (
	"errors"
	"strings"
	"testing"
)

func TestEpochHolder_ReportsAWriteThatCannotBeMade(t *testing.T) {
	t.Parallel()
	h := NewEpochHolder(t.TempDir())
	h.write = func(string, []byte) error { return errors.New("disk full") }

	_, err := h.Claim()

	if err == nil || !strings.Contains(err.Error(), "write the epoch file") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("Claim = %v, want the write error", err)
	}
}

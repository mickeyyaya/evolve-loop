package inboxfile

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestAPI_EveryExportIsNamed(t *testing.T) {
	var (
		_ func(string, []byte, func() time.Time) (Result, error)                     = Mint
		_ func(string, []byte, []string, time.Time) (string, error)                  = Publish
		_ func(string, []byte, []string, time.Time) (string, error)                  = File
		_ func(string, map[string]json.RawMessage) error                             = PublishNewItem
		_ func(string) (map[string]bool, error)                                      = FiledIDs
		_ func(string) (string, bool)                                                = ReadID
		_ func(string) bool                                                          = IsLifecycleOwned
		_ func(map[string]json.RawMessage, inboxbatch.Item, func(string) bool) error = CheckFieldRules
		_ Result                                                                     = Result{Path: "p", Skipped: true}
		_ string                                                                     = LockName
		_ time.Duration                                                              = LockWait
		_ error                                                                      = ErrInvalidItem
		_ error                                                                      = ErrAlreadyFiled
	)
	if LockName != ".bump.lock" || LockWait <= 0 {
		t.Fatalf("lock constants drifted: %q %v", LockName, LockWait)
	}
}

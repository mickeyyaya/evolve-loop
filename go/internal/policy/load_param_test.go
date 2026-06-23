package policy_test

// policy.Load — the single public entry that turns a policy.json path into the
// typed parameters. Asymmetric by contract: a MISSING file is not an error
// (all built-in defaults apply), a MALFORMED file IS (prevents a silent policy
// bypass). Unknown fields are ignored (lenient json.Unmarshal). All inputs are
// explicit paths — no environment.

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	pol, err := policy.Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	// A zero Policy resolves every accessor to its built-in defaults.
	if got := pol.FanoutConfig().Concurrency; got != 2 {
		t.Errorf("missing file → FanoutConfig().Concurrency = %d, want default 2", got)
	}
}

func TestLoad_MalformedJSONErrors(t *testing.T) {
	_, err := policy.Load(writeTempPolicy(t, `{"fanout": this is not json}`))
	if err == nil {
		t.Fatal("malformed policy.json must return an error (no silent bypass)")
	}
}

func TestLoad_UnknownFieldsIgnored(t *testing.T) {
	// Lenient parse: an unrecognized top-level key must not break the known ones.
	pol, err := policy.Load(writeTempPolicy(t, `{"totally_unknown_key":123,"quota_reset":{"reset_at":"X"}}`))
	if err != nil {
		t.Fatalf("unknown fields must be ignored, got %v", err)
	}
	if got := pol.QuotaResetConfig().ResetAt; got != "X" {
		t.Errorf("known field after unknown = %q, want X", got)
	}
}

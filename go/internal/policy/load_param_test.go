package policy_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	pol, err := policy.Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
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

func TestLoad_AnUnknownFieldFailsTheLoadAndNamesTheFile(t *testing.T) {
	path := writeTempPolicy(t, `{"totally_unknown_key":123,"quota_reset":{"reset_at":"X"}}`)
	pol, err := policy.Load(path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), `"totally_unknown_key"`) {
		t.Fatalf("an unknown key fails the load and names the file and the key, got %v", err)
	}
	if got := pol.QuotaResetConfig().ResetAt; got != "" {
		t.Errorf("a refused policy yields no settings, got reset_at %q", got)
	}
}

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclefacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestFailureDigest_EachFieldThatCyclefactsReadsSurvivesTheWriterSchema(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	written := FailureDigest{
		Cycle:            41,
		Fingerprint:      "fp-1",
		PreClass:         "gate-block",
		Recurrence:       3,
		Checks:           []string{"composed-floor", "repo-contract"},
		ClassFingerprint: "class-fp",
		ClassLayer:       "gate",
		ClassRecurrence:  7,
		ClassRoute:       "console",
		ClassRouteReason: "recurs",
	}
	raw, err := json.Marshal(written)
	if err != nil {
		t.Fatal(err)
	}
	path := paths.FailureDigestRecordPath(evolveDir, 41)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got := cyclefacts.ReadDigestRecord(evolveDir, 41)

	want := cyclefacts.DigestRecord{
		Fingerprint:      written.Fingerprint,
		PreClass:         written.PreClass,
		Checks:           written.Checks,
		ClassFingerprint: written.ClassFingerprint,
		ClassLayer:       written.ClassLayer,
		ClassRecurrence:  written.ClassRecurrence,
		ClassRoute:       written.ClassRoute,
		ClassRouteReason: written.ClassRouteReason,
	}
	if got.State != cyclefacts.StatePresent || !reflect.DeepEqual(got.Value, want) {
		t.Errorf("read = %+v, want present with %+v", got, want)
	}
}

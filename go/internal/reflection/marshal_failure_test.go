package reflection

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWrite_ReportsAnEncoderFailureAndLeavesNoFile(t *testing.T) {
	original := marshalYAML
	t.Cleanup(func() { marshalYAML = original })
	boom := errors.New("encoder down")
	marshalYAML = func(any) ([]byte, error) { return nil, boom }
	out := filepath.Join(t.TempDir(), "r.yaml")
	in := &Reflection{
		SchemaVersion: 1, Cycle: 7, Phase: "build", Agent: "evolve-builder",
		Slowdowns:            []Slowdown{{Category: "tool-batching", Evidence: "stdout.log:1", Severity: "high"}},
		ReflectionConfidence: 0.7,
	}

	err := Write(out, in)

	if !errors.Is(err, boom) {
		t.Fatalf("Write error = %v, want it to wrap the encoder error", err)
	}
	if _, statErr := os.Stat(out + ".tmp"); !os.IsNotExist(statErr) {
		t.Errorf("a failed encode left %s.tmp behind: %v", out, statErr)
	}
}

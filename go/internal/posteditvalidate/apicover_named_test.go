package posteditvalidate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ResultContract(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte("{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		ValidateJSON: func(string) (bool, string) { return false, "unexpected EOF" },
	})

	want := Result{Kind: "json", FilePath: p, OK: false, WarnEmitted: true}
	if got != want {
		t.Errorf("Run() = %+v, want %+v", got, want)
	}
}

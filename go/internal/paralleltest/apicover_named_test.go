package paralleltest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAPICoverNamed_EveryExportIsExercised(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "package fix\nimport \"testing\"\nfunc TestF(t *testing.T) {\n\t_ = 1\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "fix_test.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	sources, err := Sources(dir)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	cfg := Config{Serial: map[string]string{"fix.TestF": "fixture"}, GlobalSetters: map[string]bool{}}
	if got := Violations(sources, cfg); len(got) != 0 {
		t.Fatalf("a listed serial test must pass; got %v", got)
	}
	if got := Violations(sources, Config{}); len(got) != 1 {
		t.Fatalf("an unlisted serial test must be flagged once; got %v", got)
	}
}

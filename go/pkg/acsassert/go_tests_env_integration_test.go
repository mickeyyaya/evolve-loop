//go:build integration

package acsassert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoTests_TheChildGoTestSeesNoEvolveKeyOfItsCaller(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module example.test/contract\n\ngo 1.23\n",
		"contract_test.go": "package contract\n\nimport (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestEnvIsScrubbed(t *testing.T) {\n\tfor _, kv := range os.Environ() {\n\t\tif strings.HasPrefix(kv, \"EVOLVE_\") {\n\t\t\tt.Fatalf(\"child env carries %s\", kv)\n\t\t}\n\t}\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_FLEET", "1")
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "/runs/cycle-1978/cycle-state.json")
	ft := &fakeT{}

	ok := GoTests(ft, GoTestSpec{Dir: dir, Package: "example.test/contract", Pattern: "^TestEnvIsScrubbed$", Names: []string{"TestEnvIsScrubbed"}})

	if !ok || len(ft.errs) != 0 {
		t.Fatalf("GoTests = %v with errors %v; the child go test inherited a lane protocol key", ok, ft.errs)
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func TestLiveRefreshListers_ExecAgyWithItsManifestDefaultEnv(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
[ "$AGY_CLI_DISABLE_AUTO_UPDATE" = 1 ] || { echo "would self-update first" >&2; exit 3; }
case "$1" in
models) printf 'gemini-3.8-flash-high\tGemini 3.8 Flash (High)\n' ;;
--help) printf '  --effort   Reasoning effort (low|medium|high)\n' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "agy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGY_CLI_DISABLE_AUTO_UPDATE", "")

	names, err := modelquery.DefaultRouter(nil).List(context.Background(), "agy")
	if err != nil || !reflect.DeepEqual(names, []string{"Gemini 3.8 Flash (High)"}) {
		t.Fatalf("the live refresh's agy lister (`agy models`) must run with the agy manifest's default_env; got %q, %v", names, err)
	}
	efforts, err := modelquery.DefaultEffortListers()["agy"].ListEfforts(context.Background(), "agy")
	if err != nil || len(efforts) != 3 {
		t.Fatalf("the live refresh's effort lister (`agy --help`) must run with the agy manifest's default_env; got %q, %v", efforts, err)
	}
}

package modelquery

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const offSwitch = "AGY_CLI_DISABLE_AUTO_UPDATE"

const agyRefusingWithoutOffSwitch = `[ "$AGY_CLI_DISABLE_AUTO_UPDATE" = 1 ] || { echo "would self-update first" >&2; exit 3; }
case "$1" in
models) printf 'Fetching available models...\ngemini-3.8-flash-high\tGemini 3.8 Flash (High)\n' ;;
--help) printf '  --effort   Reasoning effort for the current CLI session (low|medium|high)\n' ;;
esac
`

func installFakeAgyBinary(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agy"), []byte("#!/bin/sh\n"+agyRefusingWithoutOffSwitch), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(offSwitch, "")
}

func useAgyOffSwitchEnv(t *testing.T) *[]string {
	t.Helper()
	var asked []string
	UseProcessEnv(func(bin string) []string {
		asked = append(asked, bin)
		return append(os.Environ(), offSwitch+"=1")
	})
	t.Cleanup(func() { UseProcessEnv(nil) })
	return &asked
}

func TestAgyLister_TheDefaultRunnerExecsAgyModelsWithTheInjectedProcessEnv(t *testing.T) {
	installFakeAgyBinary(t)
	asked := useAgyOffSwitchEnv(t)

	names, err := AgyLister{}.List(context.Background(), "agy")

	if err != nil || !reflect.DeepEqual(names, []string{"Gemini 3.8 Flash (High)"}) {
		t.Fatalf("`agy models` must run with the process env the composition root injected (the agy manifest's default_env); got %q, %v", names, err)
	}
	if !reflect.DeepEqual(*asked, []string{"agy"}) {
		t.Errorf("the env source is asked by binary name; asked %q", *asked)
	}
}

func TestHelpEffortLister_TheDefaultRunnerExecsAgyHelpWithTheInjectedProcessEnv(t *testing.T) {
	installFakeAgyBinary(t)
	useAgyOffSwitchEnv(t)

	efforts, err := HelpEffortLister{}.ListEfforts(context.Background(), "agy")

	if err != nil || !reflect.DeepEqual(efforts, []string{"low", "medium", "high"}) {
		t.Fatalf("`agy --help` must run with the injected process env; got %q, %v", efforts, err)
	}
}

func TestUseProcessEnv_WithoutASourceTheDefaultRunnerInheritsTheProcessEnv(t *testing.T) {
	installFakeAgyBinary(t)
	UseProcessEnv(nil)
	t.Setenv(offSwitch, "1")

	if _, err := (AgyLister{}).List(context.Background(), "agy"); err != nil {
		t.Fatalf("with no env source the runner inherits the process env (which here sets the switch); got %v", err)
	}
}

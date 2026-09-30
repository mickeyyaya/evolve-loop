//go:build acs

package cycle46

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_RETRO_MODEL",
	"EVOLVE_STDOUT_FILTER",
}

func TestC46_001_RetroModelAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RETRO_MODEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (retro-stdout-config-46: Config Object).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RETRO_MODEL", f.Status, f.Cluster)
	}
}

func TestC46_002_StdoutFilterAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_STDOUT_FILTER"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (retro-stdout-config-46: DI field).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_STDOUT_FILTER", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC46_003_RetroModelAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "retro", "retro.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RETRO_MODEL"`) {
		t.Errorf("RED: retro.go still contains the env read \"EVOLVE_RETRO_MODEL\".\n"+
			"Builder must delete: model := req.Env[\"EVOLVE_RETRO_MODEL\"] (line 83)\n"+
			"and replace it with model := p.model (sourced from Config.Model in New()).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC46_004_StdoutFilterAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_STDOUT_FILTER"`) {
		t.Errorf("RED: runner.go still contains the envchain.Resolve(\"EVOLVE_STDOUT_FILTER\") call (line 634).\n"+
			"Builder must replace it with !b.disableStdoutFilter (sourced from opts.DisableStdoutFilter in New()).\n"+
			"File: %s", f)
	}
}

func TestC46_006_RetroConfigHasModelField(t *testing.T) {
	rt := reflect.TypeOf(retro.Config{})
	f, ok := rt.FieldByName("Model")
	if !ok {
		t.Fatalf("RED: retro.Config has no 'Model' field.\n"+
			"Builder must add: Model string to the Config struct in retro/retro.go\n"+
			"(type: %s currently has fields: %s)", rt.Name(), fieldNames(rt))
	}
	if f.Type.Kind() != reflect.String {
		t.Errorf("RED: retro.Config.Model has kind %v, want string", f.Type.Kind())
	}
}

func TestC46_007_RunnerOptionsHasDisableStdoutFilter(t *testing.T) {
	rt := reflect.TypeOf(runner.Options{})
	f, ok := rt.FieldByName("DisableStdoutFilter")
	if !ok {
		t.Fatalf("RED: runner.Options has no 'DisableStdoutFilter' field.\n"+
			"Builder must add: DisableStdoutFilter bool to Options struct in runner/runner.go\n"+
			"(type: %s does not currently have this field)", rt.Name())
	}
	if f.Type.Kind() != reflect.Bool {
		t.Errorf("RED: runner.Options.DisableStdoutFilter has kind %v, want bool", f.Type.Kind())
	}
}

func fieldNames(t reflect.Type) string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for _, n := range names[1:] {
		result += ", " + n
	}
	return result
}

// acs-predicate: config-check
func TestC46_008_RetroModelRemovedFromDocsContract(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RETRO_MODEL"`) {
		t.Errorf("RED: docs_contract_test.go still has \"EVOLVE_RETRO_MODEL\" in allowedUndocumented.\n"+
			"Builder must remove the entry \"EVOLVE_RETRO_MODEL\": true (line 87).\n"+
			"After removing from registry, the allowedUndocumented entry is stale.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC46_009_StdoutFilterTestNoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "stdout_filter_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_STDOUT_FILTER"`) {
		t.Errorf("RED: stdout_filter_test.go still references \"EVOLVE_STDOUT_FILTER\" as an env key.\n"+
			"Builder must rewrite TestRun_StdoutFilter_OffEnvSkipsFilter (line 100) to use\n"+
			"Options{DisableStdoutFilter: true} instead of Env: map[string]string{\"EVOLVE_STDOUT_FILTER\": \"off\"}.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC46_010_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"both flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}

func TestC46_012_RetroModelRunFuncEnvReadGone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	retroFile := filepath.Join(root, "go", "internal", "phases", "retro", "retro.go")
	count, err := acsassert.CountInGoFunc(retroFile, "Run", `"EVOLVE_RETRO_MODEL"`)
	if err != nil {
		t.Fatalf("RED: CountInGoFunc on retro.go Run() failed: %v\n"+
			"(function renamed or file unreadable — Builder must not rename Run)", err)
	}
	if count > 0 {
		t.Errorf("RED: retro.go Run() still references \"EVOLVE_RETRO_MODEL\" (%d occurrence(s)).\n"+
			"Builder must replace the env map read with p.model (sourced from Config.Model).\n"+
			"File: %s", count, retroFile)
	}
}

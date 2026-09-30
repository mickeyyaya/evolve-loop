//go:build acs

package cycle47

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var taskAFlags = []string{
	"EVOLVE_RELEASE_REQUIRE_PREFLIGHT",
	"EVOLVE_OLLAMA_BASE",
}

func TestC47A_001_ReleasePreflight_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RELEASE_REQUIRE_PREFLIGHT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-ollama-env-aliases-47: env alias removal).\n"+
			"The --require-preflight CLI flag already provides this functionality.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RELEASE_REQUIRE_PREFLIGHT", f.Status, f.Cluster)
	}
}

func TestC47A_002_OllamaBase_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_OLLAMA_BASE"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-ollama-env-aliases-47: env alias removal).\n"+
			"The --ollama-base CLI flag already provides this functionality.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_OLLAMA_BASE", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC47A_003_ReleasePreflight_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "opscmd", "release_pipeline.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_REQUIRE_PREFLIGHT"`) {
		t.Errorf("RED: cmd_release_pipeline.go still contains the env alias \"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\".\n"+
			"Builder must delete:\n"+
			"  line 95: if !requirePreflight && os.Getenv(\"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\") == \"1\" { ... }\n"+
			"  line 45: Env: EVOLVE_RELEASE_REQUIRE_PREFLIGHT=1 same as --require-preflight. (help text)\n"+
			"The --require-preflight CLI flag is the canonical interface.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC47A_004_OllamaBase_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_skills_publish.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_OLLAMA_BASE"`) {
		t.Errorf("RED: cmd_skills_publish.go still contains the env fallback \"EVOLVE_OLLAMA_BASE\".\n"+
			"Builder must delete: cfg.OllamaBase = os.Getenv(\"EVOLVE_OLLAMA_BASE\") (line 161)\n"+
			"The --ollama-base CLI flag is already the primary; the env fallback duplicates it.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC47A_005_ReleasePipelineTest_NoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "opscmd", "release_pipeline_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_REQUIRE_PREFLIGHT"`) {
		t.Errorf("RED: cmd_release_pipeline_test.go still references \"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\".\n"+
			"Builder must replace: t.Setenv(\"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\", \"1\") (line 79)\n"+
			"with: \"--require-preflight\" in the args slice passed to the command.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC47A_010_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range taskAFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"both Task A flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}

func TestC47A_NEG_RowCountAtMost57(t *testing.T) {
	got := len(flagregistry.All)
	if got > 57 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 57 (59 − 2 Task A flags).\n"+
			"Builder must remove exactly these 2 rows from registry_table.go:\n"+
			"  EVOLVE_RELEASE_REQUIRE_PREFLIGHT, EVOLVE_OLLAMA_BASE\n"+
			"Current count %d exceeds 57 — Task A flags not yet removed.",
			got, got)
	}
}

func TestC47B_001_ClassifierCLI_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_MODELCATALOG_CLASSIFIER_CLI"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (modelcatalog-classifier-di-47: DI migration).\n"+
			"The env read must be replaced with the overrideCLI string param in pickClassifierCLI.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_MODELCATALOG_CLASSIFIER_CLI", f.Status, f.Cluster)
	}
}

func TestC47B_002_ClassifierCLI_AbsentFromPickClassifierCLI(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	count, err := acsassert.CountInGoFunc(f, "pickClassifierCLI", `"EVOLVE_MODELCATALOG_CLASSIFIER_CLI"`)
	if err != nil {
		t.Fatalf("RED: CountInGoFunc on cmd_models_live.go pickClassifierCLI() failed: %v\n"+
			"(function renamed or file unreadable — Builder must not rename pickClassifierCLI)\n"+
			"File: %s", err, f)
	}
	if count > 0 {
		t.Errorf("RED: cmd_models_live.go pickClassifierCLI() still references %q (%d occurrence(s)).\n"+
			"Builder must replace os.Getenv(\"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\") with the\n"+
			"overrideCLI string parameter injected by the caller.\n"+
			"File: %s", "EVOLVE_MODELCATALOG_CLASSIFIER_CLI", count, f)
	}
}

// acs-predicate: config-check
func TestC47B_003_PickClassifierCLI_HasOverrideCLIParam(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	if !acsassert.FileMatchesRegex(t, f, `func pickClassifierCLI\([^)]*overrideCLI[^)]*\)`) {
		t.Errorf("RED: cmd_models_live.go does not contain a pickClassifierCLI signature with overrideCLI param.\n"+
			"Builder must change the signature from:\n"+
			"  func pickClassifierCLI(ready []string) string\n"+
			"to:\n"+
			"  func pickClassifierCLI(ready []string, overrideCLI string) string\n"+
			"and update all callers (cmd_models_live.go call site + cmd_models_live_test.go).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC47B_004_ClassifierCLITest_NoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_MODELCATALOG_CLASSIFIER_CLI"`) {
		t.Errorf("RED: cmd_models_live_test.go still references \"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\".\n"+
			"Builder must migrate all 3 t.Setenv calls (lines 37, 60, 66) to pass the value directly:\n"+
			"  was: t.Setenv(\"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\", \"<val>\") + pickClassifierCLI(ready)\n"+
			"  now: pickClassifierCLI(ready, \"<val>\")\n"+
			"File: %s", f)
	}
}

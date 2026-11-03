//go:build acs

package cycle2141

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const condensePhase = "context-condense"

func condenseSpec(t *testing.T) phasespec.PhaseSpec {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), ".evolve", "phases", condensePhase, "phase.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var spec phasespec.PhaseSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if spec.Routing == nil || len(spec.Routing.InsertWhen) == 0 {
		t.Fatalf("%s declares no routing.insert_when", path)
	}
	return spec
}

func artifactBytesThreshold(t *testing.T, spec phasespec.PhaseSpec) int {
	t.Helper()
	for _, cond := range spec.Routing.InsertWhen {
		if cond.Field != "run_dir.artifact_bytes" {
			continue
		}
		if cond.Op != "gt" && cond.Op != ">" {
			t.Fatalf("run_dir.artifact_bytes trigger op = %q, want gt", cond.Op)
		}
		v, ok := cond.Value.(float64)
		if !ok {
			t.Fatalf("run_dir.artifact_bytes trigger value %v is not a number", cond.Value)
		}
		return int(v)
	}
	t.Fatalf("%s has no run_dir.artifact_bytes insert_when condition", condensePhase)
	return 0
}

func condenseFires(t *testing.T, spec phasespec.PhaseSpec, workspace string, completed []string) bool {
	t.Helper()
	sig, err := router.Digest(workspace, completed)
	if err != nil {
		t.Fatalf("router.Digest: %v", err)
	}
	cfg := config.RoutingConfig{Triggers: map[string]config.RoutingBlock{condensePhase: *spec.Routing}}
	return router.NewPhasePolicy(cfg).Enabled(condensePhase, sig)
}

func writeSized(t *testing.T, dir, name string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRuntimeNoise(t *testing.T, dir string, each int) {
	t.Helper()
	for _, name := range []string{
		"advisor-prompt-plan.txt",
		"router-prompt.txt",
		"triage-prompt.txt",
		"signals.ndjson",
		"triage-events.ndjson",
		"router-stdout.log",
		"router-stderr.log",
		"sbprofile-1027279376/sandbox-tdd.sb",
		"acs/verdict.json",
	} {
		writeSized(t, dir, name, each)
	}
}

func TestC2141_001_ReproTestsForTheRunDirArtifactBytesTriggerPass(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/router",
		Pattern: "^(TestDigest_RunDirArtifactBytes_ReportsUnderThresholdWithLargePromptsDoesNotTrigger|TestDigest_RunDirArtifactBytes_PreservesTriggerWhenReportArtifactsExceedThreshold)$",
		Names: []string{
			"TestDigest_RunDirArtifactBytes_ReportsUnderThresholdWithLargePromptsDoesNotTrigger",
			"TestDigest_RunDirArtifactBytes_PreservesTriggerWhenReportArtifactsExceedThreshold",
		},
	})
}

func TestC2141_002_ARunDirOfOnlyLogsPromptsAndEventStreamsOverTheThresholdDoesNotInsertContextCondense(t *testing.T) {
	spec := condenseSpec(t)
	threshold := artifactBytesThreshold(t, spec)
	ws := t.TempDir()
	writeRuntimeNoise(t, ws, threshold)
	if condenseFires(t, spec, ws, nil) {
		t.Errorf("context-condense fired for a run dir with no report artifact and %d bytes of logs, prompts and event streams", 9*threshold)
	}
}

func TestC2141_003_ReportsUnderTheThresholdWithNoiseOverItDoNotInsertContextCondenseAndDigestCountsOnlyReports(t *testing.T) {
	spec := condenseSpec(t)
	threshold := artifactBytesThreshold(t, spec)
	ws := t.TempDir()
	writeSized(t, ws, "scout-report.md", 6000)
	writeSized(t, ws, "triage-report.md", 4000)
	writeSized(t, ws, "handoff-scout.json", 2000)
	writeRuntimeNoise(t, ws, threshold)
	sig, err := router.Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("router.Digest: %v", err)
	}
	if sig.RunDir.ArtifactBytes != 12000 {
		t.Errorf("RunDir.ArtifactBytes = %d, want 12000 (two reports and one handoff, no prompts, logs, event streams, sandbox profiles)", sig.RunDir.ArtifactBytes)
	}
	if condenseFires(t, spec, ws, []string{"scout"}) {
		t.Errorf("context-condense fired for 12000 report bytes against a threshold of %d", threshold)
	}
}

func TestC2141_004_ReportArtifactsOverTheThresholdStillInsertContextCondense(t *testing.T) {
	spec := condenseSpec(t)
	threshold := artifactBytesThreshold(t, spec)
	ws := t.TempDir()
	writeSized(t, ws, "scout-report.md", threshold/2+1000)
	writeSized(t, ws, "handoff-scout.json", threshold/2+1000)
	writeRuntimeNoise(t, ws, 1000)
	if !condenseFires(t, spec, ws, []string{"scout"}) {
		t.Errorf("context-condense did not fire for %d report bytes against a threshold of %d", threshold+2000, threshold)
	}
}

func TestC2141_005_AnEmptyRunDirDoesNotInsertContextCondense(t *testing.T) {
	spec := condenseSpec(t)
	if condenseFires(t, spec, t.TempDir(), nil) {
		t.Error("context-condense fired for an empty run dir")
	}
}

var replayTokens = map[string]*regexp.Regexp{
	"run_dirs":  regexp.MustCompile(`run_dirs=(\d+)`),
	"p95":       regexp.MustCompile(`p95=(\d+)`),
	"max":       regexp.MustCompile(`max=(\d+)`),
	"threshold": regexp.MustCompile(`threshold=(\d+)`),
}

// acs-predicate: config-check
func TestC2141_006_PhaseSpecRecordsTheReplayAndTheThresholdFollowsFromIt(t *testing.T) {
	spec := condenseSpec(t)
	threshold := artifactBytesThreshold(t, spec)
	text := spec.Description + " " + spec.WhenToUse
	recorded := map[string]int{}
	for key, re := range replayTokens {
		m := re.FindStringSubmatch(text)
		if m == nil {
			t.Errorf("phase spec description/when_to_use does not record %s=<bytes or count> from the replay", key)
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		recorded[key] = n
	}
	if len(recorded) != len(replayTokens) {
		return
	}
	if recorded["run_dirs"] < 8 {
		t.Errorf("replay covers %d run dirs, want at least 8 (the cycles 2020-2027 of the inbox item)", recorded["run_dirs"])
	}
	if recorded["threshold"] != threshold {
		t.Errorf("recorded threshold=%d differs from routing.insert_when value %d", recorded["threshold"], threshold)
	}
	if threshold <= recorded["p95"] {
		t.Errorf("threshold %d does not exceed the replay p95 %d, so it would fire on routine cycles", threshold, recorded["p95"])
	}
	if threshold > recorded["max"] {
		t.Errorf("threshold %d exceeds the replay max %d, so it could never fire on a replayed run dir", threshold, recorded["max"])
	}
}

// acs-predicate: config-check
func TestC2141_007_PackageNoteStatesThatArtifactBytesCountsOnlyReportArtifacts(t *testing.T) {
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-router.md")
	raw, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "counts logs, prompts and event streams too") {
		t.Error("package note still lists the whole-run-dir sum as a filed known limit")
	}
	found := false
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "ArtifactBytes") && strings.Contains(line, "-report.md") && strings.Contains(line, "handoff-") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no line of the package note names ArtifactBytes with the -report.md and handoff- artifacts it counts")
	}
}

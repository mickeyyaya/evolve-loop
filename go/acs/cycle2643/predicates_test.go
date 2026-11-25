//go:build acs

// Package cycle2643 holds the predicates of the stage-evidence record hash schema guard.
package cycle2643

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evidencePkg = "./internal/stageevidence"
	scoreFile   = "internal/stageevidence/score.go"
	recordTail  = "\tHash    string            `json:\"hash,omitempty\"`\n"
	hashedPrev  = "\t\tPrev:    r.Prev,\n"
)

func goTest(t *testing.T, overlay string, run string) (output string, code int) {
	t.Helper()
	args := []string{"-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", run}
	if overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", append(args, evidencePkg)...)
	if code == -1 {
		t.Fatalf("go test did not launch: %v\n%s", err, stderr)
	}
	return stdout + stderr, code
}

func overlayWithScore(t *testing.T, edit func(string) string) string {
	t.Helper()
	real := filepath.Join(acsassert.GoModuleDir(t), scoreFile)
	src, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	edited := edit(string(src))
	if edited == string(src) {
		t.Fatalf("RED: score.go has no line the mutation could edit")
	}
	dir := t.TempDir()
	mutatedScore := filepath.Join(dir, "score.go")
	if err := os.WriteFile(mutatedScore, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	body, err := json.Marshal(map[string]map[string]string{"Replace": {real: mutatedScore}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func TestC2643_001_EveryRecordFieldHasAHashingDecisionAndTheGuardMachineryHolds(t *testing.T) {
	names := []string{
		"TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed",
		"TestRecordHashSchema_GuardFlagsAFieldAddedWithoutDecision",
		"TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges",
	}
	out, code := goTest(t, "", "^("+strings.Join(names, "|")+")$")
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
	if code != 0 {
		t.Errorf("RED: go test exited %d:\n%s", code, acsassert.OutputTail(out, 25))
	}
}

func TestC2643_002_ARecordFieldAddedWithoutADecisionFailsTheGuardAndNamesTheField(t *testing.T) {
	overlay := overlayWithScore(t, func(src string) string {
		return strings.Replace(src, recordTail, recordTail+"\tAnnotation string `json:\"annotation,omitempty\"`\n", 1)
	})
	out, code := goTest(t, overlay, "^TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed$")
	if code == 0 {
		t.Fatalf("RED: guard passed although Record gained a field with no hashing decision:\n%s", acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "--- FAIL: TestRecordHashSchema_EveryRecordFieldIsHashedOrDeclaredUnhashed") || !strings.Contains(out, "Annotation") {
		t.Errorf("RED: guard did not fail by naming the undecided field Annotation:\n%s", acsassert.OutputTail(out, 25))
	}
}

func TestC2643_003_AHashedFieldMissingFromTheHashPayloadFailsTheGuardAndNamesTheField(t *testing.T) {
	overlay := overlayWithScore(t, func(src string) string {
		return strings.Replace(src, hashedPrev, "", 1)
	})
	out, code := goTest(t, overlay, "^TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges$")
	if code == 0 {
		t.Fatalf("RED: guard passed although computeRecordHash ignores Prev:\n%s", acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "--- FAIL: TestRecordHashSchema_HashChangesExactlyWhenAHashedFieldChanges") || !strings.Contains(out, "Record.Prev") {
		t.Errorf("RED: guard did not fail by naming Record.Prev:\n%s", acsassert.OutputTail(out, 25))
	}
}

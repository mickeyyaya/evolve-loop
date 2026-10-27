//go:build acs

package cycle2052

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const channelPackage = "./internal/bridge/channel/"

type sourceEdit struct{ anchor, replacement string }

func moduleOf(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("ModuleDir: %v", err)
	}
	return dir
}

func mutatedCopy(t *testing.T, module, rel string, edits []sourceEdit) string {
	t.Helper()
	original, err := os.ReadFile(filepath.Join(module, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	mutated := string(original)
	for _, e := range edits {
		if strings.Count(mutated, e.anchor) != 1 {
			t.Fatalf("mutation anchor in %s is not unique or missing:\n%s", rel, e.anchor)
		}
		mutated = strings.Replace(mutated, e.anchor, e.replacement, 1)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(rel))
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func overlayFlag(t *testing.T, module string, mutations map[string][]sourceEdit) []string {
	t.Helper()
	if len(mutations) == 0 {
		return nil
	}
	replace := map[string]string{}
	for rel, edits := range mutations {
		replace[filepath.Join(module, rel)] = mutatedCopy(t, module, rel, edits)
	}
	body, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"-overlay=" + path}
}

func runEndToEnd(t *testing.T, race bool, count int, mutations map[string][]sourceEdit) (string, int) {
	t.Helper()
	module := moduleOf(t)
	args := []string{"test", "-count=" + strconv.Itoa(count), "-run", "^TestChannel_EndToEnd$"}
	if race {
		args = append(args, "-race")
	}
	args = append(args, overlayFlag(t, module, mutations)...)
	args = append(args, channelPackage)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = module
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), exitErr.ExitCode()
	}
	t.Fatalf("go test did not run: %v\n%s", err, out.String())
	return "", -1
}

var stdoutReadBeforeTextLandsMutation = map[string][]sourceEdit{
	"internal/phasestream/normalizer.go": {
		{
			anchor:      "\t\"syscall\"\n\t\"time\"\n",
			replacement: "\t\"strings\"\n\t\"syscall\"\n\t\"time\"\n",
		},
		{
			anchor:      "\tstallFired   bool\n",
			replacement: "\tstallFired   bool\n\tpreIdleArmed bool\n\tpreIdleSpent bool\n",
		},
		{
			anchor: "\tvar errLines []string\n" +
				"\tif n.cfg.StderrPath != \"\" {\n" +
				"\t\terrLines, n.stderrOff = n.tail(n.cfg.StderrPath, n.stderrOff)\n" +
				"\t}\n" +
				"\tvar outLines []string\n" +
				"\toutLines, n.stdoutOff = n.tail(n.cfg.StdoutPath, n.stdoutOff)\n",
			replacement: "\tvar outLines []string\n" +
				"\tif n.preIdleArmed && !n.preIdleSpent {\n" +
				"\t\tn.preIdleSpent = true\n" +
				"\t\tfor i := 0; i < 400 && !stderrHasIdle(n.cfg.StderrPath); i++ {\n" +
				"\t\t\ttime.Sleep(time.Millisecond)\n" +
				"\t\t}\n" +
				"\t} else {\n" +
				"\t\toutLines, n.stdoutOff = n.tail(n.cfg.StdoutPath, n.stdoutOff)\n" +
				"\t}\n" +
				"\tvar errLines []string\n" +
				"\tif n.cfg.StderrPath != \"\" {\n" +
				"\t\terrLines, n.stderrOff = n.tail(n.cfg.StderrPath, n.stderrOff)\n" +
				"\t}\n" +
				"\tfor _, ln := range errLines {\n" +
				"\t\tif strings.Contains(ln, \"inject_applied\") {\n" +
				"\t\t\tn.preIdleArmed = true\n" +
				"\t\t}\n" +
				"\t}\n",
		},
		{
			anchor: "func writeEnvelope(",
			replacement: "func stderrHasIdle(path string) bool {\n" +
				"\tb, err := os.ReadFile(path)\n" +
				"\treturn err == nil && strings.Contains(string(b), \"idle_reached\")\n" +
				"}\n\n" +
				"func writeEnvelope(",
		},
	},
}

var spanClosedBeforeAssistantTextMutation = map[string][]sourceEdit{
	"internal/phasestream/correlation.go": {
		{
			anchor:      "startSeq: at + 1, endSeq: currentSeq}}",
			replacement: "startSeq: at + 1, endSeq: at}}",
		},
	},
}

func TestC2052_001_EndToEndSurvivesStdoutReadBeforeAssistantTextLands(t *testing.T) {
	out, code := runEndToEnd(t, false, 3, stdoutReadBeforeTextLandsMutation)
	if code != 0 {
		t.Errorf("TestChannel_EndToEnd exit=%d under a normalizer that reads stdout before the assistant text lands and stderr after idle_reached: the test still lets idle_reached land in the same poll as the text\n%s",
			code, acsassert.OutputTail(out, 12))
	}
}

func TestC2052_002_EndToEndPassesTwoHundredRaceRuns(t *testing.T) {
	out, code := runEndToEnd(t, true, 200, nil)
	if code != 0 {
		t.Errorf("go test -race -count=200 TestChannel_EndToEnd exit=%d\n%s", code, acsassert.OutputTail(out, 12))
	}
}

func TestC2052_003_EndToEndFailsWhenSpanClosesBeforeAssistantText(t *testing.T) {
	out, code := runEndToEnd(t, false, 1, spanClosedBeforeAssistantTextMutation)
	if code == 0 {
		t.Errorf("TestChannel_EndToEnd passed although response_complete closes the span before the assistant text\n%s", acsassert.OutputTail(out, 12))
	}
	if !strings.Contains(out, "answer text") {
		t.Errorf("mutated run must fail on the answer-text assertion, not elsewhere\n%s", acsassert.OutputTail(out, 12))
	}
}

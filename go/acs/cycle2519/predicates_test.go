//go:build acs

package cycle2519

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	adviceBuild   = "console: build"
	adviceFeed    = "console: feed"
	adviceFinish  = "console: finish"
	windowOfEight = 8
)

var evolveBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evolve-acs-2519-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	evolveBinary = filepath.Join(dir, "evolve")
	moduleDir, err := acsassert.ModuleDir(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	build := exec.Command("go", "build", "-o", evolveBinary, "./cmd/evolve")
	build.Dir = moduleDir
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ./cmd/evolve: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(2)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func openPRsJSON(n int) string {
	prs := make([]map[string]any, 0, n)
	for i := 1; i <= n; i++ {
		prs = append(prs, map[string]any{
			"number": 700 + i, "title": fmt.Sprintf("console change %d", i),
			"headRefName": fmt.Sprintf("dev/change-%d", i),
			"url":         fmt.Sprintf("https://example.invalid/pull/%d", 700+i),
		})
	}
	raw, _ := json.Marshal(prs)
	return string(raw)
}

func ghStubDir(t *testing.T, openPRs int) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prs.json"), openPRsJSON(openPRs), 0o644)
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"pr\" ] && [ \"$2\" = \"list\" ]; then cat \"$(dirname \"$0\")/prs.json\"; exit 0; fi\n" +
		"echo \"unexpected gh call: $*\" >&2\nexit 1\n"
	writeFile(t, filepath.Join(dir, "gh"), script, 0o755)
	return dir
}

type supplyRun struct {
	stdout, stderr string
	code           int
}

func waveSupply(t *testing.T, admissible, openPRs int, extraArgs ...string) supplyRun {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeFile(t, filepath.Join(evolveDir, "wave-goal.md"), "Work the highest-weight lane-eligible inbox items.\n", 0o644)
	writeFile(t, filepath.Join(evolveDir, "state.json"), `{"lastCycleNumber":1836}`, 0o644)
	writeFile(t, filepath.Join(evolveDir, "policy.json"), fmt.Sprintf(`{"fleet":{"count":%d}}`, windowOfEight), 0o644)
	for i := 1; i <= admissible; i++ {
		item := map[string]any{
			"id": fmt.Sprintf("lane-%d", i), "weight": 0.9, "priority_class": "feature",
			"files": []string{fmt.Sprintf("go/internal/pkg%d/a.go", i)},
		}
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(evolveDir, "inbox", fmt.Sprintf("lane-%d.json", i)), string(raw), 0o644)
	}
	args := append([]string{"wave", "supply", "--project-root", root}, extraArgs...)
	cmd := exec.Command(evolveBinary, args...)
	cmd.Env = append(os.Environ(), "PATH="+ghStubDir(t, openPRs)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("evolve wave supply did not start: %v", err)
	}
	return supplyRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func adviceLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "console:") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func wantOneAdvice(t *testing.T, run supplyRun, want string) {
	t.Helper()
	if run.code != 0 {
		t.Fatalf("evolve wave supply exit=%d\nstdout:\n%s\nstderr:\n%s", run.code, run.stdout, run.stderr)
	}
	got := adviceLines(run.stdout)
	if len(got) != 1 || got[0] != want {
		t.Errorf("advice lines = %q, want exactly [%q]\nstdout:\n%s", got, want, run.stdout)
	}
}

func TestC2519_001_OneAdmissibleItemInAWindowOfEightPrintsFeed(t *testing.T) {
	wantOneAdvice(t, waveSupply(t, 1, 0), adviceFeed)
}

func TestC2519_002_EightAdmissibleItemsAndElevenOpenPRsPrintFinish(t *testing.T) {
	wantOneAdvice(t, waveSupply(t, 8, 11), adviceFinish)
}

func TestC2519_003_EightAdmissibleItemsAndThreeOpenPRsPrintBuild(t *testing.T) {
	wantOneAdvice(t, waveSupply(t, 8, 3), adviceBuild)
}

func TestC2519_004_TheLimitIsOneBatchOfTenSoTenOpenPRsStillBuildAndSevenItemsStillFeed(t *testing.T) {
	wantOneAdvice(t, waveSupply(t, 8, 10), adviceBuild)
	wantOneAdvice(t, waveSupply(t, 7, 0), adviceFeed)
}

func TestC2519_005_TheJSONSupplyCarriesTheAdviceOfEachCase(t *testing.T) {
	cases := map[string]struct {
		admissible, openPRs int
		want                string
	}{
		"feed":   {1, 0, adviceFeed},
		"finish": {8, 11, adviceFinish},
		"build":  {8, 3, adviceBuild},
	}
	for name, tc := range cases {
		run := waveSupply(t, tc.admissible, tc.openPRs, "--json")
		if run.code != 0 {
			t.Fatalf("%s: exit=%d\nstderr:\n%s", name, run.code, run.stderr)
		}
		var envelope struct {
			Data struct {
				Advice string `json:"advice"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(run.stdout), &envelope); err != nil {
			t.Fatalf("%s: supply is not JSON: %v\n%s", name, err, run.stdout)
		}
		if envelope.Data.Advice != tc.want {
			t.Errorf("%s: data.advice = %q, want %q\n%s", name, envelope.Data.Advice, tc.want, run.stdout)
		}
	}
}

//go:build acs

package cycle1881

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildOut  string
	buildErr  error
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func vulnfilterBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "vulnfilter-acs-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "vulnfilter")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/vulnfilter")
		cmd.Dir = goDir(t)
		out, err := cmd.CombinedOutput()
		buildOut, buildErr = string(out), err
	})
	if buildErr != nil {
		t.Fatalf("RED: go build ./cmd/vulnfilter failed: %v\n%s", buildErr, buildOut)
	}
	return builtBin
}

type runResult struct {
	stdout  string
	stderr  string
	code    int
	summary string
}

func runVulnfilter(t *testing.T, allowlistJSON, stream string, withSummary bool) runResult {
	t.Helper()
	bin := vulnfilterBinary(t)
	tmp := t.TempDir()
	allowlist := filepath.Join(tmp, "allowlist.json")
	if err := os.WriteFile(allowlist, []byte(allowlistJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(tmp, "summary.md")
	cmd := exec.Command(bin, "-allowlist", allowlist)
	cmd.Stdin = strings.NewReader(stream)
	cmd.Env = append(scrubbedEnv(), "TZ=UTC")
	if withSummary {
		cmd.Env = append(cmd.Env, "GITHUB_STEP_SUMMARY="+summaryPath)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("vulnfilter did not run: %v", err)
		}
		code = ee.ExitCode()
	}
	res := runResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
	if b, rerr := os.ReadFile(summaryPath); rerr == nil {
		res.summary = string(b)
	}
	return res
}

func scrubbedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GITHUB_STEP_SUMMARY=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func findingStream(osvID string) string {
	return `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck"}}
{"progress":{"message":"Scanning your code"}}
{"osv":{"id":"` + osvID + `","summary":"example vulnerability"}}
{"finding":{"osv":"` + osvID + `","fixed_version":"v1.2.3","trace":[{"module":"stdlib","version":"v1.0.0","package":"net/http","function":"Serve"}]}}
`
}

const cleanStream = `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck"}}
{"progress":{"message":"No vulnerabilities found."}}
`

func allowlistEntry(id, expires, reason string) string {
	b, _ := json.Marshal([]map[string]string{{"id": id, "expires": expires, "reason": reason}})
	return string(b)
}

func futureDate() string { return time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02") }
func pastDate() string   { return time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02") }

func TestC1881_001_EmptyAllowlistAndNoFindingsPasses(t *testing.T) {
	res := runVulnfilter(t, "[]\n", cleanStream, false)
	if res.code != 0 {
		t.Errorf("clean stream with empty allowlist must exit 0, got %d\nstdout=%s\nstderr=%s", res.code, res.stdout, res.stderr)
	}
}

func TestC1881_002_FindingCoveredByUnexpiredReasonedEntryPasses(t *testing.T) {
	res := runVulnfilter(t, allowlistEntry("GO-2024-9001", futureDate(), "no fix released; mitigated by network policy"), findingStream("GO-2024-9001"), false)
	if res.code != 0 {
		t.Errorf("allowlisted finding must exit 0, got %d\nstdout=%s\nstderr=%s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout+res.stderr, "GO-2024-9001") {
		t.Errorf("report must name the waived finding GO-2024-9001\nstdout=%s\nstderr=%s", res.stdout, res.stderr)
	}
}

func TestC1881_003_UnlistedFindingFails(t *testing.T) {
	res := runVulnfilter(t, "[]\n", findingStream("GO-2024-9002"), false)
	if res.code == 0 {
		t.Errorf("a finding absent from the allowlist must exit non-zero\nstdout=%s", res.stdout)
	}
	if !strings.Contains(res.stdout+res.stderr, "GO-2024-9002") {
		t.Errorf("failure output must name GO-2024-9002\nstdout=%s\nstderr=%s", res.stdout, res.stderr)
	}
}

func TestC1881_004_AllowlistEntryForDifferentIDDoesNotCoverFinding(t *testing.T) {
	res := runVulnfilter(t, allowlistEntry("GO-2024-1111", futureDate(), "unrelated waiver"), findingStream("GO-2024-9003"), false)
	if res.code == 0 {
		t.Errorf("a waiver for another ID must not cover GO-2024-9003\nstdout=%s", res.stdout)
	}
}

func TestC1881_005_ExpiredEntryFailsEvenWhenItsFindingIsPresent(t *testing.T) {
	res := runVulnfilter(t, allowlistEntry("GO-2024-9004", pastDate(), "waiver that lapsed"), findingStream("GO-2024-9004"), false)
	if res.code == 0 {
		t.Errorf("an expired entry must fail the run\nstdout=%s", res.stdout)
	}
}

func TestC1881_006_ExpiredEntryFailsEvenWithNoFindings(t *testing.T) {
	res := runVulnfilter(t, allowlistEntry("GO-2024-9005", pastDate(), "stale waiver nobody removed"), cleanStream, false)
	if res.code == 0 {
		t.Errorf("a stale expired entry must fail even when govulncheck reports nothing\nstdout=%s", res.stdout)
	}
}

func TestC1881_007_ReasonlessEntryFails(t *testing.T) {
	for name, reason := range map[string]string{"empty": "", "whitespace": "   \t"} {
		res := runVulnfilter(t, allowlistEntry("GO-2024-9006", futureDate(), reason), findingStream("GO-2024-9006"), false)
		if res.code == 0 {
			t.Errorf("%s reason must fail the run\nstdout=%s", name, res.stdout)
		}
	}
}

func TestC1881_008_MalformedStreamFailsClosed(t *testing.T) {
	res := runVulnfilter(t, "[]\n", "{\"config\":{}}\nthis is not json\n", false)
	if res.code == 0 {
		t.Errorf("a malformed govulncheck stream must fail closed\nstdout=%s", res.stdout)
	}
}

func TestC1881_009_MalformedAllowlistFailsClosed(t *testing.T) {
	res := runVulnfilter(t, "{not a json array", cleanStream, false)
	if res.code == 0 {
		t.Errorf("an unparsable allowlist must fail closed\nstdout=%s", res.stdout)
	}
}

func TestC1881_010_ExpiryAcceptsRFC3339AndDateOnly(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 0, 30).Format(time.RFC3339)
	past := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	if res := runVulnfilter(t, allowlistEntry("GO-2024-9007", future, "reviewed"), findingStream("GO-2024-9007"), false); res.code != 0 {
		t.Errorf("future RFC3339 expiry must pass, got %d\nstderr=%s", res.code, res.stderr)
	}
	if res := runVulnfilter(t, allowlistEntry("GO-2024-9007", past, "reviewed"), findingStream("GO-2024-9007"), false); res.code == 0 {
		t.Errorf("past RFC3339 expiry must fail")
	}
}

func TestC1881_011_StepSummaryReceivesAllowlistAndFindings(t *testing.T) {
	res := runVulnfilter(t, allowlistEntry("GO-2024-9008", futureDate(), "reviewed mitigation"), findingStream("GO-2024-9008"), true)
	if res.code != 0 {
		t.Fatalf("allowlisted run must exit 0, got %d\nstderr=%s", res.code, res.stderr)
	}
	if !strings.Contains(res.summary, "GO-2024-9008") || !strings.Contains(res.summary, "reviewed mitigation") {
		t.Errorf("GITHUB_STEP_SUMMARY must carry the entry id and reason, got %q", res.summary)
	}
}

func TestC1881_012_StepSummaryIsAppendedNotTruncated(t *testing.T) {
	bin := vulnfilterBinary(t)
	tmp := t.TempDir()
	allowlist := filepath.Join(tmp, "a.json")
	summary := filepath.Join(tmp, "s.md")
	if err := os.WriteFile(allowlist, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summary, []byte("EARLIER-STEP-OUTPUT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-allowlist", allowlist)
	cmd.Stdin = strings.NewReader(findingStream("GO-2024-9009"))
	cmd.Env = append(scrubbedEnv(), "GITHUB_STEP_SUMMARY="+summary)
	_ = cmd.Run()
	got, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "EARLIER-STEP-OUTPUT\n") || !strings.Contains(string(got), "GO-2024-9009") {
		t.Errorf("summary must append after earlier content and include the failing finding, got %q", got)
	}
}

func TestC1881_013_TrackedAllowlistIsEmptyArrayAndPassesTheRealFilter(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := "go/vulncheck-allowlist.json"
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s untracked — may be gitignored (dropped at ship)", rel)
	}
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("%s must be a JSON array: %v", rel, err)
	}
	if len(entries) != 0 {
		t.Errorf("%s must ship empty, has %d entries", rel, len(entries))
	}
	bin := vulnfilterBinary(t)
	cmd := exec.Command(bin, "-allowlist", filepath.Join(root, rel))
	cmd.Stdin = strings.NewReader(cleanStream)
	if err := cmd.Run(); err != nil {
		t.Errorf("committed allowlist must be accepted by vulnfilter: %v", err)
	}
}

func TestC1881_014_DefaultAllowlistFlagResolvesRelativeToGoDir(t *testing.T) {
	bin := vulnfilterBinary(t)
	cmd := exec.Command(bin)
	cmd.Dir = goDir(t)
	cmd.Stdin = strings.NewReader(cleanStream)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("vulnfilter with no flags from go/ must read vulncheck-allowlist.json and pass: %v\n%s", err, out)
	}
}

func TestC1881_015_MakeVulncheckPipesJSONThroughVulnfilter(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput("make", "-n", "-C", goDir(t), "vulncheck")
	if err != nil || code != 0 {
		t.Fatalf("make -n vulncheck failed: code=%d err=%v\n%s", code, err, errOut)
	}
	if !strings.Contains(out, "govulncheck@v") || !strings.Contains(out, "-json") {
		t.Errorf("vulncheck recipe must run the pinned govulncheck with -json:\n%s", out)
	}
	if !strings.Contains(out, "vulnfilter") || !strings.Contains(out, "vulncheck-allowlist.json") {
		t.Errorf("vulncheck recipe must pipe into vulnfilter with the tracked allowlist:\n%s", out)
	}
	if !strings.Contains(out, "|") {
		t.Errorf("vulncheck recipe must pipe govulncheck into vulnfilter:\n%s", out)
	}
}

func TestC1881_016_MakeVulncheckPropagatesGovulncheckFailure(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput("make", "-n", "-C", goDir(t), "vulncheck")
	if err != nil || code != 0 {
		t.Fatalf("make -n vulncheck failed: code=%d err=%v", code, err)
	}
	if strings.Contains(out, "|| true") || strings.Contains(out, "-ignore") {
		t.Errorf("recipe must not mask govulncheck or vulnfilter failure:\n%s", out)
	}
}

func TestC1881_017_VulnfilterSourcesCarryNoComments(t *testing.T) {
	dir := filepath.Join(goDir(t), "cmd", "vulnfilter")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	checked := 0
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		checked++
		for _, cg := range f.Comments {
			t.Errorf("%s:%d carries a comment: %q", e.Name(), fset.Position(cg.Pos()).Line, strings.TrimSpace(cg.Text()))
		}
	}
	if checked < 3 {
		t.Errorf("want main.go, filter.go and filter_test.go in cmd/vulnfilter, found %d Go files", checked)
	}
}

func TestC1881_018_ConventionDocDocumentsTheEmergencyEscape(t *testing.T) {
	// acs-predicate: config-check
	path := filepath.Join(acsassert.RepoRoot(t), "docs", "conventions", "third-party-dependencies.md")
	for _, needle := range []string{"vulncheck-allowlist.json", "vulnfilter", "expires", "reason"} {
		if !acsassert.FileContains(t, path, needle) {
			t.Errorf("third-party-dependencies.md must document %q", needle)
		}
	}
}

func fakeGoThatStubsGovulncheck(t *testing.T, stream string, scannerExit int) string {
	t.Helper()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go toolchain not on PATH: %v", err)
	}
	dir := t.TempDir()
	streamPath := filepath.Join(dir, "stream.ndjson")
	if err := os.WriteFile(streamPath, []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"*govulncheck*) cat '" + streamPath + "'; exit " + strconv.Itoa(scannerExit) + " ;;\n" +
		"esac\n" +
		"exec '" + realGo + "' \"$@\"\n"
	fake := filepath.Join(dir, "go")
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return fake
}

func TestC1881_019_MakeVulncheckExitCodeFollowsScannerAndFilter(t *testing.T) {
	cases := []struct {
		name        string
		stream      string
		scannerExit int
		wantPass    bool
	}{
		{"scanner crashes with empty stdout", "", 2, false},
		{"scanner emits a clean stream then fails", cleanStream, 1, false},
		{"scanner emits an unlisted finding and exits 0", findingStream("GO-2024-9010"), 0, false},
		{"scanner emits a clean stream and exits 0", cleanStream, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := fakeGoThatStubsGovulncheck(t, tc.stream, tc.scannerExit)
			cmd := exec.Command("make", "-C", goDir(t), "vulncheck", "GO="+fake)
			cmd.Env = scrubbedEnv()
			out, err := cmd.CombinedOutput()
			passed := err == nil
			if err != nil {
				if _, ok := err.(*exec.ExitError); !ok {
					t.Fatalf("make did not run: %v", err)
				}
			}
			if passed != tc.wantPass {
				t.Errorf("make vulncheck passed=%v, want %v (scanner exit %d)\n%s", passed, tc.wantPass, tc.scannerExit, out)
			}
		})
	}
}

//go:build acs

// Package cycle2248 holds the ACS predicates of artifact-log-compression, the deterministic compressor of bulky test logs, build output and diffs.
package cycle2248

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/logcompress"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var markerLine = regexp.MustCompile(`^\[logcompress: (\d+) lines elided(: .*)?\]$`)

var failureLine = regexp.MustCompile(`^(--- FAIL|\s+--- FAIL|FAIL|panic:|exit status|\s+\S+_test\.go:\d+:)`)

func fixtureSettings() policy.LogCompressionSettings {
	return policy.LogCompressionSettings{
		BytesPerToken:     3,
		MinInputBytes:     400,
		MinReductionRatio: 0.5,
		PassRunMin:        4,
		RepeatRunMin:      4,
		DiffFileMaxLines:  40,
		PassPrefixes:      []string{"--- PASS:", "=== RUN", "=== PAUSE", "=== CONT", "ok  \t", "PASS"},
		NoisePrefixes:     []string{"go: downloading ", "go: finding ", "go: extracting "},
		FailureMarkers:    []string{"--- FAIL", "FAIL", "panic:", "exit status", "_test.go:", "undefined:", "cannot use"},
		NeverCompress:     []string{"## Task Contract", "<deliverable-contract", "challenge-token", "### Handoff request", "Acceptance (verbatim"},
	}
}

func goTestLog() string {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "=== RUN   TestPass%03d\n--- PASS: TestPass%03d (0.00s)\n", i, i)
	}
	b.WriteString("=== RUN   TestBroken\n")
	for i := 0; i < 60; i++ {
		b.WriteString("    broken_test.go:42: got 3, want 4\n")
	}
	b.WriteString("--- FAIL: TestBroken (0.01s)\nFAIL\nFAIL\tgithub.com/x/broken\t0.512s\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/x/pkg%02d\t0.123s\n", i)
	}
	b.WriteString("panic: runtime error: stack overflow\n\ngoroutine 7 [running]:\n")
	for i := 0; i < 150; i++ {
		b.WriteString("main.recurse(0x1)\n\t/src/main.go:10 +0x1d\n")
	}
	b.WriteString("exit status 2\nFAIL\tgithub.com/x/deep\t1.1s\n")
	return b.String()
}

func buildLog() string {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "go: downloading github.com/x/mod%03d v1.%d.0\n", i, i)
	}
	b.WriteString("# github.com/x/pkg\n./a.go:12:3: undefined: foo\n./b.go:7:9: cannot use x (variable of type int) as string value in argument to f\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "go: extracting github.com/x/mod%03d v1.%d.0\n", i, i)
	}
	return b.String()
}

type diffFixture struct {
	text      string
	bigAdds   int
	bigDels   int
	smallBody string
}

func diffLog() diffFixture {
	var b strings.Builder
	adds, dels := 0, 0
	for _, name := range []string{"big1.go", "big2.go"} {
		fmt.Fprintf(&b, "diff --git a/%[1]s b/%[1]s\nindex 1111111..2222222 100644\n--- a/%[1]s\n+++ b/%[1]s\n@@ -1,300 +1,300 @@\n", name)
		for i := 0; i < 300; i++ {
			if i%3 == 0 {
				fmt.Fprintf(&b, "-old line %d of %s\n", i, name)
				dels++
			} else {
				fmt.Fprintf(&b, "+new line %d of %s\n", i, name)
				adds++
			}
		}
	}
	small := "diff --git a/small.go b/small.go\nindex 3333333..4444444 100644\n--- a/small.go\n+++ b/small.go\n@@ -1,3 +1,3 @@\n-a\n+b\n c\n"
	b.WriteString(small)
	return diffFixture{text: b.String(), bigAdds: adds / 2, bigDels: dels / 2, smallBody: small}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func tokensOf(s string, bytesPerToken int) int {
	return (len(s) + bytesPerToken - 1) / bytesPerToken
}

func assertAccounting(t *testing.T, in, out string) {
	t.Helper()
	inLines := splitLines(in)
	kept, elided, next := 0, 0, 0
	for _, line := range splitLines(out) {
		if m := markerLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			elided += n
			continue
		}
		kept++
		for next < len(inLines) && inLines[next] != line {
			next++
		}
		if next == len(inLines) {
			t.Fatalf("output line %q is not an unmodified input line in input order", line)
		}
		next++
	}
	if kept+elided != len(inLines) {
		t.Errorf("line accounting: %d kept + %d elided != %d input lines", kept, elided, len(inLines))
	}
	if strings.HasSuffix(in, "\n") != strings.HasSuffix(out, "\n") {
		t.Errorf("trailing newline changed: in %v out %v", strings.HasSuffix(in, "\n"), strings.HasSuffix(out, "\n"))
	}
}

func assertFailureLinesExact(t *testing.T, in, out string) {
	t.Helper()
	count := func(s string) map[string]int {
		m := map[string]int{}
		for _, line := range splitLines(s) {
			if failureLine.MatchString(line) {
				m[line]++
			}
		}
		return m
	}
	want, got := count(in), count(out)
	if len(want) == 0 {
		t.Fatal("fixture has no failure lines")
	}
	for line, n := range want {
		if got[line] != n {
			t.Errorf("failure line %q: %d in input, %d in output", line, n, got[line])
		}
	}
}

func assertHalved(t *testing.T, name, in, out string, bytesPerToken int) {
	t.Helper()
	if tokensOf(out, bytesPerToken)*2 > tokensOf(in, bytesPerToken) {
		t.Errorf("%s: %d tokens out of %d in, want a cut of at least 50%%", name, tokensOf(out, bytesPerToken), tokensOf(in, bytesPerToken))
	}
}

func TestC2248_001_GoTestLogKeepsEveryFailureLineAndHalvesTokens(t *testing.T) {
	cfg := fixtureSettings()
	in := goTestLog()

	res := logcompress.Compress(in, cfg)

	if !res.Applied {
		t.Fatal("Compress left a bulky go test log untouched")
	}
	assertAccounting(t, in, res.Text)
	assertFailureLinesExact(t, in, res.Text)
	assertHalved(t, "go test log", in, res.Text, cfg.BytesPerToken)
	if res.BytesIn != len(in) || res.BytesOut != len(res.Text) {
		t.Errorf("byte counts %d/%d, want %d/%d", res.BytesIn, res.BytesOut, len(in), len(res.Text))
	}
	if res.TokensIn != tokensOf(in, cfg.BytesPerToken) || res.TokensOut != tokensOf(res.Text, cfg.BytesPerToken) {
		t.Errorf("token counts %d/%d, want ceil(bytes/%d) = %d/%d", res.TokensIn, res.TokensOut, cfg.BytesPerToken, tokensOf(in, cfg.BytesPerToken), tokensOf(res.Text, cfg.BytesPerToken))
	}
	if strings.Contains(res.Text, "--- PASS: TestPass100") {
		t.Error("a run of passing tests survived")
	}
}

func TestC2248_002_BuildOutputDropsDownloadNoiseAndKeepsEveryError(t *testing.T) {
	cfg := fixtureSettings()
	in := buildLog()

	res := logcompress.Compress(in, cfg)

	assertAccounting(t, in, res.Text)
	assertHalved(t, "build log", in, res.Text, cfg.BytesPerToken)
	for _, line := range splitLines(res.Text) {
		if strings.HasPrefix(line, "go: downloading ") || strings.HasPrefix(line, "go: extracting ") {
			t.Errorf("noise line survived: %q", line)
		}
	}
	for _, keep := range []string{"# github.com/x/pkg", "./a.go:12:3: undefined: foo", "./b.go:7:9: cannot use x (variable of type int) as string value in argument to f"} {
		if !strings.Contains(res.Text, keep+"\n") {
			t.Errorf("build line lost: %q", keep)
		}
	}
}

func TestC2248_003_LargeDiffFilesCollapseToHeadersAndAnExactStat(t *testing.T) {
	cfg := fixtureSettings()
	d := diffLog()

	res := logcompress.Compress(d.text, cfg)

	assertAccounting(t, d.text, res.Text)
	assertHalved(t, "diff", d.text, res.Text, cfg.BytesPerToken)
	if !strings.Contains(res.Text, d.smallBody) {
		t.Error("a small diff file must stay byte-exact")
	}
	for _, name := range []string{"big1.go", "big2.go"} {
		for _, header := range []string{"diff --git a/" + name + " b/" + name, "index 1111111..2222222 100644", "--- a/" + name, "+++ b/" + name} {
			if !strings.Contains(res.Text, header+"\n") {
				t.Errorf("diff header lost: %q", header)
			}
		}
	}
	stat := fmt.Sprintf("+%d -%d", d.bigAdds, d.bigDels)
	if strings.Count(res.Text, stat) != 2 {
		t.Errorf("want the stat %q on each of the 2 large files, got %d", stat, strings.Count(res.Text, stat))
	}
	if strings.Contains(res.Text, "+new line 5 of big1.go") {
		t.Error("a hunk body of a large diff file survived")
	}
}

func TestC2248_004_ALogOfOnlyFailuresPassesThroughByteExact(t *testing.T) {
	cfg := fixtureSettings()
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestBroken%02d (0.01s)\n    case_test.go:%d: got 1, want 2\n", i, i+10)
	}
	b.WriteString("FAIL\nFAIL\tgithub.com/x/broken\t0.5s\n")

	res := logcompress.Compress(b.String(), cfg)

	if res.Text != b.String() || res.Applied {
		t.Errorf("failure-only log changed or reported Applied=%v", res.Applied)
	}
}

func TestC2248_005_InputBelowMinInputBytesIsReturnedUntouched(t *testing.T) {
	cfg := fixtureSettings()
	var b strings.Builder
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, "--- PASS: T%d\n", i)
	}
	if b.Len() >= cfg.MinInputBytes {
		t.Fatalf("fixture is %d bytes, must stay under %d", b.Len(), cfg.MinInputBytes)
	}

	res := logcompress.Compress(b.String(), cfg)

	if res.Text != b.String() || res.Applied {
		t.Errorf("small input changed or reported Applied=%v", res.Applied)
	}
}

func TestC2248_006_AReductionBelowMinReductionRatioIsRefused(t *testing.T) {
	cfg := fixtureSettings()
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestBroken%02d (0.01s) with a long explanatory failure message padding the line\n", i)
	}
	for i := 0; i < 6; i++ {
		fmt.Fprintf(&b, "--- PASS: T%d\n", i)
	}
	if b.Len() < cfg.MinInputBytes {
		t.Fatalf("fixture is %d bytes, must reach %d", b.Len(), cfg.MinInputBytes)
	}

	res := logcompress.Compress(b.String(), cfg)

	if res.Text != b.String() || res.Applied {
		t.Errorf("a cut under min_reduction_ratio was applied (Applied=%v)", res.Applied)
	}
}

func TestC2248_007_NeverCompressRegionsSurviveByteExactInsideBulkyLogs(t *testing.T) {
	cfg := fixtureSettings()
	region := "## Task Contract\n--- PASS: TestInsideContract1\n--- PASS: TestInsideContract2\n--- PASS: TestInsideContract3\n--- PASS: TestInsideContract4\n--- PASS: TestInsideContract5\n"
	var b strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "--- PASS: TestBefore%03d\n", i)
	}
	b.WriteString(region + "\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "--- PASS: TestAfter%03d\n", i)
	}
	in := b.String()

	res := logcompress.Compress(in, cfg)

	assertAccounting(t, in, res.Text)
	if !strings.Contains(res.Text, region) {
		t.Errorf("never-compress region lost or altered:\n%s", res.Text)
	}
	if strings.Contains(res.Text, "TestBefore040") || strings.Contains(res.Text, "TestAfter040") {
		t.Error("passing runs around the protected region were not compressed")
	}
}

func TestC2248_010_CompressIsDeterministicAndEmptyInputIsSafe(t *testing.T) {
	cfg := fixtureSettings()
	in := goTestLog()

	first, second := logcompress.Compress(in, cfg), logcompress.Compress(in, cfg)
	empty := logcompress.Compress("", cfg)

	if first.Text != second.Text {
		t.Error("two runs over one input differ")
	}
	if empty.Text != "" || empty.Applied {
		t.Errorf("empty input produced %q applied=%v", empty.Text, empty.Applied)
	}
}

func validLogCompressionBlock() map[string]any {
	return map[string]any{
		"bytes_per_token": 3, "min_input_bytes": 2048, "min_reduction_ratio": 0.3,
		"pass_run_min": 4, "repeat_run_min": 4, "diff_file_max_lines": 60,
		"pass_prefixes": []string{"--- PASS:"}, "noise_prefixes": []string{"go: downloading "},
		"failure_markers": []string{"FAIL"}, "never_compress": []string{"## Task Contract"},
	}
}

func parseWithBlock(t *testing.T, block map[string]any) (policy.Policy, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"log_compression": block})
	if err != nil {
		t.Fatal(err)
	}
	return policy.Parse(raw)
}

func TestC2248_011_APolicyWithoutALogCompressionBlockIsRefusedNotDefaulted(t *testing.T) {
	p, err := policy.Parse([]byte(`{}`))
	if err != nil {
		t.Fatalf("parse empty policy: %v", err)
	}

	_, cfgErr := p.LogCompressionConfig()

	if cfgErr == nil || !strings.Contains(cfgErr.Error(), "log_compression") {
		t.Errorf("want an error naming log_compression, got %v", cfgErr)
	}
}

func TestC2248_012_LogCompressionConfigRefusesEveryInvalidThreshold(t *testing.T) {
	good, err := parseWithBlock(t, validLogCompressionBlock())
	if err != nil {
		t.Fatalf("parse valid block: %v", err)
	}
	cfg, err := good.LogCompressionConfig()
	if err != nil {
		t.Fatalf("valid block refused: %v", err)
	}
	if cfg.BytesPerToken != 3 || cfg.MinReductionRatio != 0.3 || cfg.DiffFileMaxLines != 60 || len(cfg.NeverCompress) != 1 {
		t.Errorf("valid block decoded wrong: %+v", cfg)
	}
	for name, mutate := range map[string]func(map[string]any){
		"bytes_per_token missing":     func(m map[string]any) { delete(m, "bytes_per_token") },
		"bytes_per_token zero":        func(m map[string]any) { m["bytes_per_token"] = 0 },
		"min_input_bytes negative":    func(m map[string]any) { m["min_input_bytes"] = -1 },
		"min_reduction_ratio above 1": func(m map[string]any) { m["min_reduction_ratio"] = 1.5 },
		"min_reduction_ratio zero":    func(m map[string]any) { m["min_reduction_ratio"] = 0 },
		"pass_run_min zero":           func(m map[string]any) { m["pass_run_min"] = 0 },
		"repeat_run_min missing":      func(m map[string]any) { delete(m, "repeat_run_min") },
		"diff_file_max_lines zero":    func(m map[string]any) { m["diff_file_max_lines"] = 0 },
		"never_compress empty":        func(m map[string]any) { m["never_compress"] = []string{} },
		"never_compress blank marker": func(m map[string]any) { m["never_compress"] = []string{" "} },
		"failure_markers empty":       func(m map[string]any) { m["failure_markers"] = []string{} },
		"pass_prefixes duplicate":     func(m map[string]any) { m["pass_prefixes"] = []string{"--- PASS:", "--- PASS:"} },
	} {
		block := validLogCompressionBlock()
		mutate(block)
		p, perr := parseWithBlock(t, block)
		if perr != nil {
			continue
		}
		if _, cerr := p.LogCompressionConfig(); cerr == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	unknown := validLogCompressionBlock()
	unknown["surprise"] = 1
	if _, perr := parseWithBlock(t, unknown); perr == nil {
		t.Error("an unknown log_compression key was accepted")
	}
}

func TestC2248_014_LogcompressSourceHoldsNoThresholdLiteralAndReadsNoEnvironment(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "logcompress")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("package go/internal/logcompress missing: %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		checked++
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if (x.Kind == token.INT || x.Kind == token.FLOAT) && x.Value != "0" && x.Value != "1" {
					t.Errorf("%s: numeric literal %s is a threshold: it belongs in policy.json", fset.Position(x.Pos()), x.Value)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "os" && (x.Sel.Name == "Getenv" || x.Sel.Name == "LookupEnv" || x.Sel.Name == "Environ") {
					t.Errorf("%s: reads the environment", fset.Position(x.Pos()))
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("go/internal/logcompress has no non-test source file")
	}
}

func TestC2248_015_LogcompressIsEnrolledInApicoverAndNamesEverySymbol(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enroll, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	enrolled := false
	for _, line := range strings.Split(string(enroll), "\n") {
		if strings.TrimSpace(line) == "./internal/logcompress" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Error("./internal/logcompress is not listed in go/.apicover-enforce")
	}
	var out, errOut bytes.Buffer
	if code := apicover.Main([]string{"-enforce", filepath.Join(root, "go", "internal", "logcompress")}, &out, &errOut); code != 0 {
		t.Errorf("apicover -enforce exit %d:\n%s%s", code, out.String(), errOut.String())
	}
}

func TestC2248_016_LogcompressAndPolicyPassVetAndTheRaceDetector(t *testing.T) {
	mod := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, args := range [][]string{
		{"-C", mod, "vet", "./internal/logcompress"},
		{"-C", mod, "vet", "./internal/policy"},
		{"-C", mod, "test", "-race", "-count=1", "./internal/logcompress"},
		{"-C", mod, "test", "-race", "-count=1", "-run", "LogCompression", "./internal/policy"},
	} {
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
		if code != 0 {
			t.Errorf("go %s exit %d:\n%s", strings.Join(args[2:], " "), code, acsassert.OutputTail(stdout+stderr, 25))
		}
	}
}

package logcompress

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func settings() policy.LogCompressionSettings {
	return policy.LogCompressionSettings{
		BytesPerToken:     3,
		MinInputBytes:     100,
		MinReductionRatio: 0.5,
		PassRunMin:        3,
		RepeatRunMin:      3,
		DiffFileMaxLines:  10,
		PassPrefixes:      []string{"--- PASS:"},
		NoisePrefixes:     []string{"go: downloading "},
		FailureMarkers:    []string{"FAIL"},
		NeverCompress:     []string{"## Task Contract"},
	}
}

func TestCompress_RepeatedIdenticalLinesKeepTheFirstAndElideTheRest(t *testing.T) {
	t.Parallel()
	in := "head\n" + strings.Repeat("the same repeated line of output\n", 20) + "tail\n"

	res := Compress(in, settings())

	want := "head\nthe same repeated line of output\n[logcompress: 19 lines elided: repeated lines]\ntail\n"
	if !res.Applied || res.Text != want {
		t.Errorf("Applied=%v Text=%q, want %q", res.Applied, res.Text, want)
	}
}

func TestCompress_RepeatedFailureLinesAreNeverCollapsed(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("    x_test.go:1: FAIL got 1 want 2\n", 20) + strings.Repeat("--- PASS: T\n", 20)

	res := Compress(in, settings())

	if got := strings.Count(res.Text, "FAIL got 1 want 2"); got != 20 {
		t.Errorf("failure lines in output = %d, want 20", got)
	}
}

func TestCompress_ARunBelowTheMinimumIsKept(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("FAIL padding line to pass the minimum input size\n", 4) + "--- PASS: A\n--- PASS: B\n"

	res := Compress(in, settings())

	if res.Applied || res.Text != in {
		t.Errorf("a two-line pass run was elided: applied=%v", res.Applied)
	}
}

func TestCompress_AMarkerLongerThanItsRunIsNotEmitted(t *testing.T) {
	t.Parallel()
	cfg := settings()
	cfg.MinReductionRatio = 0.01
	in := strings.Repeat("FAIL padding line to pass the minimum input size\n", 4) + "--- PASS: A\n--- PASS: B\n--- PASS: C\n"

	res := Compress(in, cfg)

	if strings.Contains(res.Text, "[logcompress:") {
		t.Errorf("a marker longer than the lines it replaces was emitted:\n%s", res.Text)
	}
}

func TestCompress_ADiffWithoutAHunkIsLeftAlone(t *testing.T) {
	t.Parallel()
	in := "diff --git a/img.png b/img.png\nBinary files differ\n" + strings.Repeat("FAIL padding line to pass the minimum input size\n", 4)

	res := Compress(in, settings())

	if res.Applied || res.Text != in {
		t.Errorf("a hunkless diff was changed: applied=%v", res.Applied)
	}
}

func TestCompress_ALargeDiffThatHoldsAProtectedLineIsLeftAlone(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,40 +1,40 @@\n+## Task Contract\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "+added line number %d with some padding text\n", i)
	}

	res := Compress(b.String(), settings())

	if res.Applied || res.Text != b.String() {
		t.Errorf("a diff holding a never-compress line was collapsed: applied=%v", res.Applied)
	}
}

func TestCompress_ADiffBodyEndsAtTheFirstNonDiffLine(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,40 +1,40 @@\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "+added line number %d with some padding text\n", i)
	}
	b.WriteString("FAIL\tgithub.com/x/y\t0.1s\n")

	res := Compress(b.String(), settings())

	if !res.Applied || !strings.HasSuffix(res.Text, "FAIL\tgithub.com/x/y\t0.1s\n") {
		t.Errorf("the log line after the diff was lost:\n%s", res.Text)
	}
}

func TestCompress_AnUnterminatedLastLineKeepsItsMissingNewline(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("--- PASS: TestSomething\n", 20) + "FAIL"

	res := Compress(in, settings())

	if !res.Applied || strings.HasSuffix(res.Text, "\n") || !strings.HasSuffix(res.Text, "FAIL") {
		t.Errorf("Applied=%v Text=%q", res.Applied, res.Text)
	}
}

func TestCompress_InputBelowTheMinimumAndEmptyInputPassThrough(t *testing.T) {
	t.Parallel()
	small := "--- PASS: A\n--- PASS: B\n--- PASS: C\n"

	if res := Compress(small, settings()); res.Applied || res.Text != small {
		t.Errorf("small input changed: %+v", res)
	}
	if res := Compress("", policy.LogCompressionSettings{BytesPerToken: 1}); res.Applied || res.Text != "" {
		t.Errorf("empty input changed: %+v", res)
	}
}

func TestCompress_NoiseLinesAreElidedAndAReductionUnderTheRatioIsRefused(t *testing.T) {
	t.Parallel()
	noise := strings.Repeat("go: downloading example.com/mod v1.0.0\n", 12)

	res := Compress(noise+"FAIL\n", settings())

	if want := "[logcompress: 12 lines elided: download noise]\nFAIL\n"; !res.Applied || res.Text != want {
		t.Errorf("Text = %q, want %q", res.Text, want)
	}
	cfg := settings()
	cfg.MinReductionRatio = 1
	if refused := Compress(noise+"FAIL\n", cfg); refused.Applied || refused.Text != noise+"FAIL\n" {
		t.Errorf("a cut under the ratio was applied: %+v", refused)
	}
}

func TestCompress_AProtectedRegionEndsAtTheNextBlankLine(t *testing.T) {
	t.Parallel()
	region := "## Task Contract\n" + strings.Repeat("--- PASS: Inside\n", 6)
	in := strings.Repeat("--- PASS: Before\n", 12) + region + "\n" + strings.Repeat("--- PASS: After\n", 12)

	res := Compress(in, settings())

	if !strings.Contains(res.Text, region+"\n") || strings.Contains(res.Text, "Before") || strings.Contains(res.Text, "After") {
		t.Errorf("region lost or neighbours kept:\n%s", res.Text)
	}
}

func TestCompress_ALargeDiffFileCollapsesToItsHeaderAndAnExactStat(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n@@ -1,40 +1,40 @@\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "-removed line number %d with padding\n+added line number %d with padding\n", i, i)
	}

	res := Compress(b.String(), settings())

	want := "diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n[logcompress: 41 lines elided: diff body +20 -20]\n"
	if !res.Applied || res.Text != want {
		t.Errorf("Text = %q, want %q", res.Text, want)
	}
}

//go:build acs

package cycle2502

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/evidencex"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fixtureLines = 2000
	tailLines    = 40
	windowSize   = 5
	generous     = 16384
	secretLine   = "DEBUG leaked sk-livesecret0123456789ABCDEF and api_key=hunter2hunter2 in header"
)

type plantedError struct {
	line  int
	label string
	msg   string
}

var plantedErrors = []plantedError{
	{400, "A", "connection refused"},
	{1000, "B", "checksum mismatch"},
	{1500, "C", "disk quota exceeded"},
}

var offsetWords = map[int]string{
	-5: "pre-five", -4: "pre-four", -3: "pre-three", -2: "pre-two", -1: "pre-one",
	1: "post-one", 2: "post-two", 3: "post-three", 4: "post-four", 5: "post-five",
}

func letterCode(i int) string {
	return string(rune('a'+i/26)) + string(rune('a'+i%26))
}

func lineText(i int) string {
	stamp := fmt.Sprintf("2026-11-18T10:%02d:%02d ", (i/60)%60, i%60)
	for _, e := range plantedErrors {
		d := i - e.line
		switch {
		case d == 0:
			return stamp + fmt.Sprintf("ERROR window-%s %s host=db-1 port=5432", e.label, e.msg)
		case d >= -windowSize && d <= windowSize:
			return stamp + fmt.Sprintf("DEBUG window-%s %s", e.label, offsetWords[d])
		case d == -windowSize-1:
			return stamp + fmt.Sprintf("DEBUG fence-%s before-window", e.label)
		case d == windowSize+1:
			return stamp + fmt.Sprintf("DEBUG fence-%s after-window", e.label)
		}
	}
	switch {
	case i == fixtureLines:
		return stamp + "INFO shutdown complete reason=signal"
	case i > fixtureLines-tailLines:
		return stamp + fmt.Sprintf("INFO drain-%s %s", letterCode(i-(fixtureLines-tailLines)-1), strings.Repeat("segment-flush-ok ", 6))
	case i%20 == 10:
		return stamp + fmt.Sprintf("WARN slow query took %dms table=orders", i*3+7)
	}
	return stamp + fmt.Sprintf("INFO heartbeat seq=%d ok", i)
}

func joinLines(n int, text func(int) string) []byte {
	var b bytes.Buffer
	for i := 1; i <= n; i++ {
		b.WriteString(text(i))
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func fixture() []byte { return joinLines(fixtureLines, lineText) }

func wholeSourceDigest(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func covered(ranges []evidencex.Range, line int) bool {
	for _, r := range ranges {
		if line >= r.Start && line <= r.End {
			return true
		}
	}
	return false
}

func extract(t *testing.T, src []byte, budget int) evidencex.Result {
	t.Helper()
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModeExtract, Budget: budget})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return res
}

func windowBytes() int {
	total := 0
	for _, e := range plantedErrors {
		for i := e.line - windowSize; i <= e.line+windowSize; i++ {
			total += len(lineText(i)) + 1
		}
	}
	return total
}

func TestC2502_001_ExtractKeepsEachErrorWithFiveLinesOfContext(t *testing.T) {
	res := extract(t, fixture(), generous)
	for _, e := range plantedErrors {
		if !strings.Contains(res.Text, e.msg) {
			t.Errorf("error %q missing from the extract text", e.msg)
		}
		for d := -windowSize; d <= windowSize; d++ {
			if !covered(res.Ranges, e.line+d) {
				t.Errorf("line %d (offset %d of error %s) is outside the kept ranges", e.line+d, d, e.label)
			}
			if d != 0 && !strings.Contains(res.Text, fmt.Sprintf("window-%s %s", e.label, offsetWords[d])) {
				t.Errorf("context line offset %d of error %s missing from the extract text", d, e.label)
			}
		}
		for _, edge := range []int{e.line - windowSize - 1, e.line + windowSize + 1} {
			if covered(res.Ranges, edge) {
				t.Errorf("line %d lies beyond the 5-line context of error %s but is kept", edge, e.label)
			}
		}
		if strings.Contains(res.Text, "fence-"+e.label) {
			t.Errorf("fence line of error %s is kept: the context is wider than 5 lines", e.label)
		}
	}
}

func TestC2502_002_ExtractMergesRepeatsIntoTemplatesWithCounts(t *testing.T) {
	var wantCount int
	var firstBody, lastBody string
	for i := 1; i <= fixtureLines-tailLines; i++ {
		if i%20 == 10 {
			wantCount++
			if firstBody == "" {
				firstBody = strings.SplitN(lineText(i), " ", 2)[1]
			}
			lastBody = strings.SplitN(lineText(i), " ", 2)[1]
		}
	}
	res := extract(t, fixture(), generous)
	var slow *evidencex.Template
	for i := range res.Templates {
		if strings.Contains(res.Templates[i].First, "slow query") {
			slow = &res.Templates[i]
		}
	}
	if slow == nil {
		t.Fatalf("no template holds the repeated slow-query lines; got %d templates", len(res.Templates))
	}
	if slow.Count != wantCount {
		t.Errorf("slow-query template count = %d, want %d", slow.Count, wantCount)
	}
	if !strings.Contains(slow.First, firstBody) {
		t.Errorf("template first example = %q, want the first source line %q", slow.First, firstBody)
	}
	if !strings.Contains(slow.Last, lastBody) {
		t.Errorf("template last example = %q, want the last source line %q", slow.Last, lastBody)
	}
	if got := strings.Count(res.Text, "slow query"); got > 3 {
		t.Errorf("the text repeats the slow-query line %d times: the repeats are not merged", got)
	}
	for _, tpl := range res.Templates {
		if tpl.Count < 1 {
			t.Errorf("template %q has count %d", tpl.Pattern, tpl.Count)
		}
	}
}

func TestC2502_003_ExtractStaysUnderBudgetAndKeepsErrorsFirst(t *testing.T) {
	src := fixture()
	if len(src) < 4*generous {
		t.Fatalf("fixture is %d bytes, too small to prove a budget cut", len(src))
	}
	full := extract(t, src, generous)
	if len(full.Text) == 0 || len(full.Text) > generous {
		t.Errorf("text is %d bytes under a budget of %d", len(full.Text), generous)
	}
	tight := windowBytes() * 3 / 2
	cut := extract(t, src, tight)
	if len(cut.Text) > tight {
		t.Errorf("text is %d bytes over the tight budget of %d", len(cut.Text), tight)
	}
	if len(cut.Text) >= len(full.Text) {
		t.Errorf("tight text (%d bytes) is not smaller than the generous text (%d bytes)", len(cut.Text), len(full.Text))
	}
	for _, e := range plantedErrors {
		if !strings.Contains(cut.Text, e.msg) {
			t.Errorf("tight budget dropped error %q", e.msg)
		}
		for d := -windowSize; d <= windowSize; d++ {
			if d != 0 && !strings.Contains(cut.Text, fmt.Sprintf("window-%s %s", e.label, offsetWords[d])) {
				t.Errorf("tight budget dropped context offset %d of error %s", d, e.label)
			}
			if !covered(cut.Ranges, e.line+d) {
				t.Errorf("tight budget dropped line %d from the kept ranges", e.line+d)
			}
		}
	}
}

func TestC2502_004_ExtractRecordsWholeSourceDigestAndKeptRanges(t *testing.T) {
	src := fixture()
	res := extract(t, src, generous)
	if want := wholeSourceDigest(src); res.Digest != want {
		t.Errorf("Digest = %q, want sha256 of the whole source %q", res.Digest, want)
	}
	if res.Size != len(src) {
		t.Errorf("Size = %d, want %d", res.Size, len(src))
	}
	if len(res.Ranges) == 0 {
		t.Fatal("no kept ranges recorded")
	}
	prevEnd := 0
	for _, r := range res.Ranges {
		if r.Start < 1 || r.End > fixtureLines || r.Start > r.End {
			t.Errorf("range %+v is outside lines 1..%d", r, fixtureLines)
		}
		if r.Start <= prevEnd {
			t.Errorf("range %+v overlaps or precedes the earlier range ending at %d", r, prevEnd)
		}
		prevEnd = r.End
	}
	for line := fixtureLines - tailLines + 1; line <= fixtureLines; line++ {
		if !covered(res.Ranges, line) {
			t.Errorf("tail line %d is not kept", line)
		}
	}
	if !strings.Contains(res.Text, "shutdown complete") {
		t.Errorf("the last source line is missing from the extract text")
	}
	for _, filler := range []int{1, 2, 800, 1200, 1900} {
		if covered(res.Ranges, filler) {
			t.Errorf("filler heartbeat line %d is kept", filler)
		}
	}
	if covered(res.Ranges, fixtureLines+1) {
		t.Errorf("a phantom line %d is kept after the trailing newline", fixtureLines+1)
	}
}

func TestC2502_005_ExtractGivesTheSameBytesForTheSameInput(t *testing.T) {
	src := fixture()
	first := extract(t, src, generous)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 25; run++ {
		again := extract(t, src, generous)
		againJSON, err := json.Marshal(again)
		if err != nil {
			t.Fatal(err)
		}
		if again.Text != first.Text || !bytes.Equal(firstJSON, againJSON) {
			t.Fatalf("run %d gave different bytes for the same input", run)
		}
	}
	changed := bytes.Replace(src, []byte("checksum mismatch"), []byte("checksum mismatcH"), 1)
	other := extract(t, changed, generous)
	if other.Digest == first.Digest || other.Text == first.Text {
		t.Errorf("a changed input gave the same digest or text")
	}
}

func TestC2502_006_PointerModeKeepsOnlyDigestAndSize(t *testing.T) {
	src := fixture()
	res, err := evidencex.Extract(src, evidencex.Options{Mode: evidencex.ModePointer, Budget: generous})
	if err != nil {
		t.Fatalf("Extract pointer: %v", err)
	}
	if res.Digest != wholeSourceDigest(src) {
		t.Errorf("pointer Digest = %q, want %q", res.Digest, wholeSourceDigest(src))
	}
	if res.Size != len(src) {
		t.Errorf("pointer Size = %d, want %d", res.Size, len(src))
	}
	if res.Text != "" || len(res.Ranges) != 0 || len(res.Templates) != 0 {
		t.Errorf("pointer mode kept content: text=%d bytes ranges=%d templates=%d", len(res.Text), len(res.Ranges), len(res.Templates))
	}
}

func TestC2502_007_ExtractRejectsAnUnknownModeAndHandlesEmptyInput(t *testing.T) {
	if _, err := evidencex.Extract(fixture(), evidencex.Options{Mode: evidencex.Mode("bogus"), Budget: generous}); err == nil {
		t.Errorf("an unknown mode was accepted")
	}
	res := extract(t, nil, generous)
	if res.Digest != wholeSourceDigest(nil) || res.Size != 0 || res.Text != "" || len(res.Ranges) != 0 {
		t.Errorf("empty input gave digest=%q size=%d text=%q ranges=%v", res.Digest, res.Size, res.Text, res.Ranges)
	}
}

func TestC2502_008_ExtractClampsContextAtTheSourceEdges(t *testing.T) {
	short := joinLines(8, func(i int) string {
		if i == 2 {
			return "ERROR early failure"
		}
		return fmt.Sprintf("INFO step %d", i)
	})
	res := extract(t, short, generous)
	if len(res.Ranges) == 0 {
		t.Fatal("no ranges for a short source with an error")
	}
	for _, r := range res.Ranges {
		if r.Start < 1 || r.End > 8 {
			t.Errorf("range %+v leaves lines 1..8", r)
		}
	}
	if !covered(res.Ranges, 1) || !covered(res.Ranges, 7) {
		t.Errorf("context of the error at line 2 is not kept to the source edges: %v", res.Ranges)
	}
	if !strings.Contains(res.Text, "early failure") {
		t.Errorf("error missing from text %q", res.Text)
	}
}

func TestC2502_009_ExtractRedactsSecretsButDigestsTheRawSource(t *testing.T) {
	src := joinLines(60, func(i int) string {
		switch i {
		case 28:
			return secretLine
		case 30:
			return "ERROR kaboom in handler"
		}
		return fmt.Sprintf("INFO tick %d", i)
	})
	res := extract(t, src, generous)
	for _, secret := range []string{"sk-livesecret0123456789ABCDEF", "hunter2hunter2"} {
		if strings.Contains(res.Text, secret) {
			t.Errorf("extract text leaks %q", secret)
		}
	}
	if !strings.Contains(res.Text, "kaboom") {
		t.Errorf("error line missing after redaction")
	}
	if !covered(res.Ranges, 28) {
		t.Errorf("redaction changed the kept ranges: line 28 is not kept")
	}
	if res.Digest != wholeSourceDigest(src) {
		t.Errorf("Digest = %q, want the digest of the raw source %q", res.Digest, wholeSourceDigest(src))
	}
}

func TestC2502_010_NewPackageGraduatesIntoTheApicoverGate(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	// acs-predicate: config-check
	if !acsassert.FileContains(t, filepath.Join(moduleDir, ".apicover-enforce"), "./internal/evidencex\n") {
		t.Errorf("./internal/evidencex is not enrolled in go/.apicover-enforce")
	}
	var out, errBuf bytes.Buffer
	code := apicover.Main([]string{"-enforce", filepath.Join(moduleDir, "internal", "evidencex")}, &out, &errBuf)
	if code != 0 {
		t.Errorf("apicover -enforce on internal/evidencex exit=%d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
}

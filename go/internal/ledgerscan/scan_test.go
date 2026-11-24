package ledgerscan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type countingReader struct {
	data  []byte
	bytes int
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n := copy(p, c.data[off:])
	c.bytes += n
	return n, nil
}

func collect(t *testing.T, data string, f Filter, chunk int) []string {
	t.Helper()
	var got []string
	r := &countingReader{data: []byte(data)}
	err := reverse(r, int64(len(data)), lineSink{f: f, visit: func(line []byte) (bool, error) {
		got = append(got, string(line))
		return false, nil
	}}, chunk)
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	return got
}

func reversed(lines ...string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[len(lines)-1-i] = l
	}
	return out
}

func equal(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00") && len(a) == len(b)
}

func TestReverseYieldsLinesNewestFirstAtEveryChunkSize(t *testing.T) {
	data := "alpha\nb\n\nccccccccccccccccc\n" + strings.Repeat("d", 33) + "\ne\n"
	want := reversed("alpha", "b", "ccccccccccccccccc", strings.Repeat("d", 33), "e")
	for chunk := 1; chunk <= 70; chunk++ {
		if got := collect(t, data, Filter{}, chunk); !equal(got, want) {
			t.Fatalf("chunk %d: got %q want %q", chunk, got, want)
		}
	}
}

func TestReverseLineCrossesChunkBoundary(t *testing.T) {
	for _, n := range []int{15, 16, 17, 33} {
		line := strings.Repeat("x", n)
		data := "first\n" + line + "\nlast\n"
		want := reversed("first", line, "last")
		if got := collect(t, data, Filter{}, 16); !equal(got, want) {
			t.Fatalf("line %d: got %q want %q", n, got, want)
		}
	}
}

func TestReverseNoTrailingNewlineYieldsTheLastLine(t *testing.T) {
	want := reversed("one", "two", "thr")
	for chunk := 1; chunk <= 12; chunk++ {
		if got := collect(t, "one\ntwo\nthr", Filter{}, chunk); !equal(got, want) {
			t.Fatalf("chunk %d: got %q", chunk, got)
		}
	}
}

func TestReverseYieldsATruncatedLastLineAsBytes(t *testing.T) {
	got := collect(t, "{\"a\":1}\n{\"a\":", Filter{}, 4)
	if want := reversed("{\"a\":1}", "{\"a\":"); !equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReverseEmptyAndBlankFilesYieldNothing(t *testing.T) {
	for _, data := range []string{"", "\n", "\n\n\n"} {
		if got := collect(t, data, Filter{}, 2); len(got) != 0 {
			t.Fatalf("%q yielded %q", data, got)
		}
	}
}

func TestReverseMissingFileWrapsErrNotExist(t *testing.T) {
	err := Reverse(filepath.Join(t.TempDir(), "none"), Filter{}, func([]byte) (bool, error) { return false, nil })
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}

func TestReverseReadsTheFileAtPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "l.jsonl")
	if err := os.WriteFile(p, []byte("a\nb\nc"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := Reverse(p, Filter{}, func(l []byte) (bool, error) { got = append(got, string(l)); return false, nil }); err != nil {
		t.Fatal(err)
	}
	if !equal(got, reversed("a", "b", "c")) {
		t.Fatalf("got %q", got)
	}
}

func TestReverseEarlyExitReadsOnlyTheTail(t *testing.T) {
	data := strings.Repeat("old row old row old row\n", 5000) + "newest\n"
	r := &countingReader{data: []byte(data)}
	var seen int
	err := reverse(r, int64(len(data)), lineSink{visit: func(l []byte) (bool, error) { seen++; return true, nil }}, 64)
	if err != nil || seen != 1 {
		t.Fatalf("err=%v seen=%d", err, seen)
	}
	if r.bytes > 64 {
		t.Fatalf("read %d bytes of %d for a one-row lookup, want at most one 64-byte chunk", r.bytes, len(data))
	}
}

func TestReverseStopsAndReturnsTheVisitError(t *testing.T) {
	boom := errors.New("boom")
	var seen int
	err := reverse(&countingReader{data: []byte("a\nb\nc\n")}, 6, lineSink{visit: func([]byte) (bool, error) { seen++; return false, boom }}, 2)
	if !errors.Is(err, boom) || seen != 1 {
		t.Fatalf("err=%v seen=%d", err, seen)
	}
}

type failingReader struct{}

var errDiskGone = errors.New("disk gone")

func (failingReader) ReadAt([]byte, int64) (int, error) { return 0, errDiskGone }

func TestReverseReturnsAReadError(t *testing.T) {
	err := reverse(failingReader{}, 10, lineSink{visit: func([]byte) (bool, error) { return false, nil }}, 4)
	if !errors.Is(err, errDiskGone) {
		t.Fatalf("err = %v", err)
	}
}

func TestReverseStopsAtTheCarriedFirstLine(t *testing.T) {
	var got []string
	err := reverse(&countingReader{data: []byte("first\nsecond")}, 12, lineSink{visit: func(l []byte) (bool, error) {
		got = append(got, string(l))
		return string(l) == "first", nil
	}}, 3)
	if err != nil || !equal(got, reversed("first", "second")) {
		t.Fatalf("err=%v got=%q", err, got)
	}
}

func TestFilterDropsLinesMissingANeedleUnlessTheyCarryABackslash(t *testing.T) {
	f := Filter{Needles: [][]byte{[]byte("auditor"), []byte("agent_subprocess")}}
	for line, want := range map[string]bool{
		`{"kind":"agent_subprocess","role":"auditor"}`:      true,
		`{"kind":"agent_subprocess","role":"builder"}`:      false,
		`{"kind":"phase","role":"auditor"}`:                 false,
		`{"kind":"agent_subprocess","role":"audit\u006fr"}`: true,
		`{"kind":"phase","note":"C:\\x"}`:                   true,
	} {
		if got := f.Pass([]byte(line)); got != want {
			t.Errorf("Pass(%s) = %v, want %v", line, got, want)
		}
	}
	if !(Filter{}).Pass([]byte("anything")) {
		t.Error("an empty filter must pass every line")
	}
}

func TestContainsValueRefusesValuesTheDecodeCouldMatchWithoutTheirBytes(t *testing.T) {
	for _, v := range []string{"", "\xff", "a\xffb", "a\ufffdb"} {
		if got := ContainsValue(v); got != nil {
			t.Errorf("ContainsValue(%q) = %q, want nil", v, got)
		}
	}
	if got := ContainsValue("agent_subprocess"); !bytes.Equal(got, []byte("agent_subprocess")) {
		t.Errorf("ContainsValue = %q", got)
	}
	if got := ContainsValue("run \"1\" é"); got == nil {
		t.Error("quotes and non-ASCII are safe through the backslash rule and must stay filterable")
	}
}

func TestPrefilterNeverMissesADecodeMatch(t *testing.T) {
	values := []string{"auditor", "run-é-1", `we"ird`, "a<b>&c", "tab\there", "x/y", "\u2028sep"}
	encodings := []func(v string) string{
		func(v string) string { b, _ := json.Marshal(v); return string(b) },
		func(v string) string { return fmt.Sprintf("%+q", v) },
		func(v string) string {
			var sb strings.Builder
			sb.WriteByte('"')
			for _, r := range v {
				fmt.Fprintf(&sb, `\u%04x`, r)
			}
			sb.WriteByte('"')
			return sb.String()
		},
	}
	shapes := []string{`{"role":%s}`, `{"ROLE":%s}`, `{"Role":"x","role":%s}`, `{"role":"x","role":%s}`, `{"n":{"role":"%s"},"role":%s}`}
	for _, v := range values {
		needle := ContainsValue(v)
		if needle == nil {
			t.Fatalf("test value %q must be filterable", v)
		}
		f := Filter{Needles: [][]byte{needle}}
		for ei, enc := range encodings {
			for _, shape := range shapes {
				var line string
				if strings.Count(shape, "%s") == 2 {
					line = fmt.Sprintf(shape, "decoy", enc(v))
				} else {
					line = fmt.Sprintf(shape, enc(v))
				}
				var rec struct {
					Role string `json:"role"`
				}
				if json.Unmarshal([]byte(line), &rec) != nil || rec.Role != v {
					continue
				}
				if !f.Pass([]byte(line)) {
					t.Errorf("encoding %d dropped a row that decodes to role %q: %s", ei, v, line)
				}
			}
		}
	}
}

func TestPrefilterRefusesTheNeedleThatInvalidUTF8WouldFake(t *testing.T) {
	line := []byte("{\"role\":\"a\xffb\"}")
	var rec struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(line, &rec); err != nil || rec.Role != "a\ufffdb" {
		t.Fatalf("decode premise broken: %v %q", err, rec.Role)
	}
	if ContainsValue(rec.Role) != nil {
		t.Fatal("a needle holding U+FFFD would drop this row, which decodes to it")
	}
}

func TestFilteredReverseSkipsLinesWithoutTheNeedle(t *testing.T) {
	data := "x keep\nno\nkeep two\nzzz\n"
	got := collect(t, data, Filter{Needles: [][]byte{[]byte("keep")}}, 5)
	if want := reversed("x keep", "keep two"); !equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestScanFileReturnsAStatError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "l.jsonl")
	if err := os.WriteFile(p, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := scanFile(file, Filter{}, func([]byte) (bool, error) { return false, nil }); err == nil {
		t.Fatal("a closed file must fail the stat")
	}
}

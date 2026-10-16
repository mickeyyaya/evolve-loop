package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestStderrTail_KeepsTheLastLinesAcrossWrites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		writes []string
		want   []string
	}{
		{"nothing written", nil, []string{}},
		{"one terminated line", []string{"a\n"}, []string{"a"}},
		{"a line split across writes", []string{"he", "llo\nwor", "ld\n"}, []string{"hello", "world"}},
		{"an unterminated last line", []string{"a\nb"}, []string{"a", "b"}},
		{"an empty line is kept", []string{"a\n\nb\n"}, []string{"a", "", "b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			tail := &stderrTail{}
			for _, w := range c.writes {
				if n, err := tail.Write([]byte(w)); n != len(w) || err != nil {
					t.Fatalf("Write(%q) = %d, %v; want %d, nil", w, n, err, len(w))
				}
			}
			if got := tail.Lines(); !slices.Equal(got, c.want) {
				t.Errorf("Lines() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestStderrTail_BoundsLineCountAndLineSize(t *testing.T) {
	t.Parallel()
	tail := &stderrTail{}
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(tail, "line %d\n", i)
	}
	fmt.Fprint(tail, strings.Repeat("x", 3*stepStderrLineMaxSize)+"END")
	got := tail.Lines()
	if len(got) != stepStderrTailLines {
		t.Fatalf("kept %d lines, want %d", len(got), stepStderrTailLines)
	}
	if got[0] != "line 62" || got[len(got)-2] != "line 100" {
		t.Errorf("kept lines %q .. %q, want line 62 .. line 100", got[0], got[len(got)-2])
	}
	if last := got[len(got)-1]; len(last) != stepStderrLineMaxSize || !strings.HasSuffix(last, "END") {
		t.Errorf("unterminated long line kept %d bytes ending %q, want its last %d bytes", len(last), last[len(last)-3:], stepStderrLineMaxSize)
	}
	if len(tail.lines) > stepStderrTailLines {
		t.Errorf("buffer holds %d complete lines, want at most %d", len(tail.lines), stepStderrTailLines)
	}
}

func TestWaveStepDetail_AddsTheTailOnlyToAFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		rc   int
		tail []string
		want string
	}{
		{0, []string{"chatter"}, "rc=0"},
		{3, nil, "rc=3"},
		{3, []string{"a", "b"}, "rc=3\na\nb"},
	}
	for _, c := range cases {
		if got := waveStepDetail(c.rc, c.tail); got != c.want {
			t.Errorf("waveStepDetail(%d, %q) = %q, want %q", c.rc, c.tail, got, c.want)
		}
	}
}

package phasestream

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func responseSpanTexts(envs []Envelope) []any {
	var lo, hi int64 = -1, -1
	for _, e := range envs {
		if e.Kind == KindCorrelation && e.Data["sub"] == "response_complete" {
			lo, hi = e.Data["start_seq"].(int64), e.Data["end_seq"].(int64)
		}
	}
	var texts []any
	for _, e := range envs {
		if e.Kind == KindAssistantText && e.Seq >= lo && e.Seq <= hi {
			texts = append(texts, e.Data["text"])
		}
	}
	return texts
}

func TestPoll_IdleOnStderrNeverClosesBeforeEarlierStdoutText(t *testing.T) {
	for writeBeforeRead := 0; writeBeforeRead < 2; writeBeforeRead++ {
		t.Run(fmt.Sprintf("writer_lands_before_read_%d", writeBeforeRead), func(t *testing.T) {
			dir := t.TempDir()
			n := newTestNormalizer(dir, &bytes.Buffer{}, &testClock{t: baseClock()})
			stdout := filepath.Join(dir, "tdd-stdout.log")
			stderr := filepath.Join(dir, "tdd-stderr.log")
			if err := os.WriteFile(stdout, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			appendLines(t, stderr, `{"evolve_channel":"inject_applied","corr_id":"c1"}`)
			first, err := n.Poll()
			if err != nil {
				t.Fatal(err)
			}

			reads := 0
			n.tail = func(path string, off int64) ([]string, int64) {
				if reads == writeBeforeRead {
					appendLines(t, stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"3 bullets"}]}}`)
					appendLines(t, stderr, `{"evolve_channel":"idle_reached","corr_id":"c1"}`)
				}
				reads++
				return tailFile(path, off)
			}
			second, err := n.Poll()
			if err != nil {
				t.Fatal(err)
			}
			third, err := n.Poll()
			if err != nil {
				t.Fatal(err)
			}

			all := append(append(first, second...), third...)
			if got := responseSpanTexts(all); len(got) != 1 || got[0] != "3 bullets" {
				t.Fatalf("response span texts = %v, want [3 bullets]", got)
			}
		})
	}
}

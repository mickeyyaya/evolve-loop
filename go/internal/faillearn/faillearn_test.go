package faillearn

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// update regenerates golden files: go test ./internal/faillearn/ -run Golden -update
var update = flag.Bool("update", false, "update golden files")

// fixtureEvent is the canonical cycle-243 reproduction (retro bridge
// exit=81) used across render tests. Fixed Now keeps output byte-stable.
func fixtureEvent() FailureEvent {
	return FailureEvent{
		Cycle:          243,
		FailedPhase:    "retrospective",
		Scope:          ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "retro bridge exited 81 before writing retrospective-report.md",
		Defects:        []string{"bridge timeout exit=81", "lesson lost"},
		EvidencePaths:  []string{".evolve/runs/cycle-243/orchestrator-report.md"},
		GitHead:        "28aa4c3",
		Now:            time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC),
	}
}

func TestRenderRetrospectiveMarkdown_ContainsVerdictPhaseDefectsEvidence(t *testing.T) {
	got := string(RenderRetrospectiveMarkdown(fixtureEvent()))

	for _, want := range []string{
		"deterministic-fallback",
		"Cycle 243",
		"FAIL",
		"retrospective",
		"cycle-mid-execution-fail",
		"bridge timeout exit=81",
		"lesson lost",
		".evolve/runs/cycle-243/orchestrator-report.md",
		"28aa4c3",
		"2026-06-07T08:30:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderRetrospectiveMarkdown missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderRetrospectiveMarkdown_GoldenBytes(t *testing.T) {
	got := RenderRetrospectiveMarkdown(fixtureEvent())
	assertGolden(t, filepath.Join("testdata", "retro_golden.md"), got)
}

func TestRenderLessonYAML_GoldenBytes(t *testing.T) {
	_, body := RenderLessonYAML(fixtureEvent())
	assertGolden(t, filepath.Join("testdata", "lesson_golden.yaml"), body)
}

// The lessons corpus parser (research.parseLessonFile) unmarshals
// []lessonYAML — a single-mapping render would be invisible to KB
// recall. The rendered body MUST be a YAML list.
func TestRenderLessonYAML_IsYAMLList(t *testing.T) {
	_, body := RenderLessonYAML(fixtureEvent())

	var entries []map[string]any
	if err := yaml.Unmarshal(body, &entries); err != nil {
		t.Fatalf("rendered lesson is not a YAML list: %v\n%s", err, body)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 lesson entry, got %d", len(entries))
	}
	for _, field := range []string{
		"id", "pattern", "description", "confidence",
		"source", "type", "category", "preventiveAction", "failureContext",
	} {
		if _, ok := entries[0][field]; !ok {
			t.Errorf("lesson entry missing field %q", field)
		}
	}
}

func TestRenderLessonYAML_IDSlugStable(t *testing.T) {
	tests := []struct {
		name string
		ev   FailureEvent
		want string
	}{
		{
			name: "phase scope uses failed phase as slug",
			ev:   fixtureEvent(),
			want: "cycle-243-phase-retrospective",
		},
		{
			name: "reset scope without failed phase falls back to classification",
			ev: FailureEvent{
				Cycle: 244, Scope: ScopeReset,
				Classification: "operator-reset",
				Now:            time.Date(2026, 6, 7, 0, 11, 0, 0, time.UTC),
			},
			want: "cycle-244-reset-operator-reset",
		},
		{
			name: "loop scope sanitizes non-slug characters",
			ev: FailureEvent{
				Cycle: 245, Scope: ScopeLoop,
				Classification: "loop-fatal",
				FailedPhase:    "stop_reason=circuit_breaker",
				Now:            time.Date(2026, 6, 7, 1, 0, 0, 0, time.UTC),
			},
			want: "cycle-245-loop-stop-reason-circuit-breaker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1, _ := RenderLessonYAML(tt.ev)
			id2, _ := RenderLessonYAML(tt.ev)
			if id1 != tt.want {
				t.Errorf("id = %q, want %q", id1, tt.want)
			}
			if id1 != id2 {
				t.Errorf("id not stable across renders: %q vs %q", id1, id2)
			}
		})
	}
}

func TestRenderLessonYAML_Deterministic(t *testing.T) {
	_, body1 := RenderLessonYAML(fixtureEvent())
	_, body2 := RenderLessonYAML(fixtureEvent())
	if !bytes.Equal(body1, body2) {
		t.Error("RenderLessonYAML not byte-deterministic across renders")
	}
}

func TestSummaryTruncation(t *testing.T) {
	ev := fixtureEvent()
	// 600 multi-byte runes prove rune (not byte) truncation.
	ev.Summary = strings.Repeat("日", 600)

	md := string(RenderRetrospectiveMarkdown(ev))
	if strings.Contains(md, strings.Repeat("日", 501)) {
		t.Error("markdown summary not truncated to 500 runes")
	}
	if !strings.Contains(md, strings.Repeat("日", 500)) {
		t.Error("markdown summary over-truncated below 500 runes")
	}

	_, body := RenderLessonYAML(ev)
	var entries []struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(body, &entries); err != nil {
		t.Fatalf("unmarshal lesson: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 lesson entry, got %d", len(entries))
	}
	if n := utf8.RuneCountInString(entries[0].Description); n != 500 {
		t.Errorf("lesson description rune count = %d, want 500", n)
	}
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

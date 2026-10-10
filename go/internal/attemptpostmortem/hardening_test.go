package attemptpostmortem

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestValidPhaseAndRecordValidate_RefuseAPhaseThatIsNotABareName(t *testing.T) {
	for _, phase := range []string{"../build", "a/b", `a\b`, "build*", "build?", "[", ".", "..", "x y", "-build"} {
		t.Run(phase, func(t *testing.T) {
			if ValidPhase(phase) {
				t.Fatalf("ValidPhase(%q) = true", phase)
			}
			r := validRecord(t)
			r.Phase = phase
			if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "phase") {
				t.Fatalf("Validate(phase %q) = %v, want an error that names phase", phase, err)
			}
			if _, err := ReadAll(t.TempDir(), phase); err == nil {
				t.Fatalf("ReadAll(phase %q) = nil error, want a refusal", phase)
			}
		})
	}
	for _, phase := range []string{"build", "plan-review", "bug_reproduction", "tdd2"} {
		r := validRecord(t)
		r.Phase = phase
		if err := r.Validate(); err != nil {
			t.Errorf("Validate(phase %q) = %v, want nil", phase, err)
		}
	}
}

func TestCollect_ANilTranscriptFallsBackToThePaneTail(t *testing.T) {
	rec, err := Collect(Input{Attempt: cycle1853BuildAttempt1(t), ScrollbackPath: fixtureFinalPane1}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.CommandSource != SourcePaneTail || !strings.Contains(rec.SourceError, ErrNoTranscript.Error()) {
		t.Fatalf("source = %q (%q), want pane_tail with ErrNoTranscript", rec.CommandSource, rec.SourceError)
	}
	if !strings.Contains(rec.PaneTail, "no server running") || len(rec.Commands) != 0 || rec.Suspect != nil {
		t.Fatalf("rec = %+v, want the pane tail and no commands", rec)
	}
}

func TestRender_CapsTheCommandCountAndTheIdentityFieldsOfARecordFromDisk(t *testing.T) {
	cfg := DefaultConfig()
	rec := collectAttempt1(t, cfg)
	rec.Commands = nil
	for i := range 50 {
		rec.Commands = append(rec.Commands, Command{Text: fmt.Sprintf("cmd-%02d", i), Status: StatusOK})
	}
	long := strings.Repeat("z", 5000)
	rec.CLI, rec.Session, rec.DispatchID, rec.CauseCode = "C"+long, "S"+long, "D"+long, "K"+long
	got := Render([]Record{rec}, cfg)
	if strings.Count(got, "`cmd-") != cfg.MaxCommands || !strings.Contains(got, "`cmd-49`") || strings.Contains(got, "`cmd-41`") {
		t.Errorf("want only the last %d commands:\n%s", cfg.MaxCommands, got)
	}
	capped := strings.Repeat("z", maxFieldRunes-2) + "…"
	for _, prefix := range []string{"C", "S", "D", "K"} {
		if !strings.Contains(got, "`"+prefix+capped+"`") {
			t.Errorf("field %s… is not capped at %d runes", prefix, maxFieldRunes)
		}
	}
	if strings.Contains(got, strings.Repeat("z", maxFieldRunes)) {
		t.Error("a field longer than the cap reached the section")
	}
}

func TestRender_KeepsOnlyTheLastMaxRecordsAttempts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRecords = 2
	var records []Record
	for n := 1; n <= 4; n++ {
		r := collectAttempt1(t, cfg)
		r.Number = n
		records = append(records, r)
	}
	got := Render(records, cfg)
	if strings.Contains(got, "### Attempt 1") || strings.Contains(got, "### Attempt 2") || !strings.Contains(got, "### Attempt 3") || !strings.Contains(got, "### Attempt 4") {
		t.Fatalf("want only attempts 3 and 4:\n%s", got)
	}
}

func TestClaudeTranscript_SkipsALineOverTheLimitKeepsTheOthersAndCountsIt(t *testing.T) {
	body := strings.Join([]string{
		`{"type":"assistant","timestamp":"2026-10-09T10:00:00Z","message":{"content":[{"type":"tool_use","id":"a","name":"Bash","input":{"command":"go test ./internal/swarm"}}]}}`,
		strings.Repeat("x", maxTranscriptLineBytes+1),
		`{"type":"user","timestamp":"2026-10-09T10:00:05Z","message":{"content":[{"type":"tool_result","tool_use_id":"a","content":"ok"}]}}`,
		strings.Repeat("y", maxTranscriptLineBytes+7),
	}, "\n")
	path := writeFile(t, t.TempDir(), "t.jsonl", body)
	trace, err := ClaudeTranscript(path)()
	if err != nil {
		t.Fatalf("ClaudeTranscript: %v", err)
	}
	if len(trace.Commands) != 1 || trace.Commands[0].Status != StatusOK || trace.SkippedLines != 2 {
		t.Fatalf("trace = %+v skipped=%d, want the command with its result and 2 skipped lines", trace.Commands, trace.SkippedLines)
	}
	if got := trace.LastActivityAt.Format(time.RFC3339); got != "2026-10-09T10:00:05Z" {
		t.Fatalf("LastActivityAt = %s, want the result after the long line", got)
	}
	rec, err := Collect(Input{Attempt: cycle1853BuildAttempt1(t), Transcript: ClaudeTranscript(path)}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.SkippedTranscriptLines != 2 || !strings.Contains(Render([]Record{rec}, DefaultConfig()), "Transcript lines skipped over the size limit: 2.") {
		t.Fatalf("record skipped = %d, want 2 in the record and in the section", rec.SkippedTranscriptLines)
	}
}

func TestRender_ALastActivityAfterTheEndIsZeroBeforeTheEnd(t *testing.T) {
	rec := collectAttempt1(t, DefaultConfig())
	rec.LastActivityAt = rec.EndedAt.Add(90 * time.Second)
	got := Render([]Record{rec}, DefaultConfig())
	if !strings.Contains(got, ", 0s before the end.") || strings.Contains(got, "-1m30s") {
		t.Fatalf("want a clamped 0s:\n%s", got)
	}
}

func TestCollectAndRender_StripC0ControlsExceptNewlineAndTab(t *testing.T) {
	const dirty = "\x1b[31mred\x07\r\n\ttab\x00end"
	const clean = "[31mred\n\ttabend"
	pane := writeFile(t, t.TempDir(), "pane.txt", dirty)
	rec, err := Collect(Input{Attempt: cycle1853BuildAttempt1(t), ScrollbackPath: pane, WorktreeDelta: dirty}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.PaneTail != clean || rec.WorktreeDelta != clean {
		t.Fatalf("pane = %q delta = %q, want %q", rec.PaneTail, rec.WorktreeDelta, clean)
	}
	rec.PaneTail, rec.WorktreeDelta = dirty, dirty
	got := Render([]Record{rec}, DefaultConfig())
	if strings.ContainsAny(got, "\x1b\x07\x00\r") || strings.Count(got, clean) != 2 {
		t.Fatalf("the section keeps a C0 control or loses a newline or tab:\n%q", got)
	}
	if utf8.RuneCountInString(stripControls("é\x01é")) != 2 {
		t.Fatal("stripControls must keep non-ASCII runes")
	}
}

func TestNextNumber_IsOnePastTheHighestContiguousRecord(t *testing.T) {
	dir := t.TempDir()
	if got := NextNumber(dir, "build"); got != 1 {
		t.Fatalf("NextNumber(empty) = %d, want 1", got)
	}
	for _, n := range []int{1, 2} {
		r := validRecord(t)
		r.Number = n
		if err := Write(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, dir, "tdd-attempt-3-postmortem.json", "{}")
	if got := NextNumber(dir, "build"); got != 3 {
		t.Fatalf("NextNumber = %d, want 3", got)
	}
}

func TestWritesTranscript_IsTrueOnlyForAClaudeCLI(t *testing.T) {
	for cli, want := range map[string]bool{"claude": true, "claude-tmux": true, "claude-p": true, "agy-claude-tmux": false, "agy": false, "codex-tmux": false, "": false} {
		if got := WritesTranscript(cli); got != want {
			t.Errorf("WritesTranscript(%q) = %v, want %v", cli, got, want)
		}
	}
}

func TestRender_AnEmptyIdentityFieldIsUnknown(t *testing.T) {
	rec := collectAttempt1(t, DefaultConfig())
	rec.Session, rec.DispatchID = "", ""
	got := Render([]Record{rec}, DefaultConfig())
	if !strings.Contains(got, "session unknown, dispatch unknown.") || strings.Contains(got, "session ``") {
		t.Fatalf("want unknown for the empty fields:\n%s", got)
	}
}

func TestRender_TheRuntimeStatesTheSectionAndRule1AsksForTheCauseFirst(t *testing.T) {
	got := Render([]Record{collectAttempt1(t, DefaultConfig())}, DefaultConfig())
	for _, want := range []string{
		"## Previous attempts of this phase (stated by the evolve runtime)\n",
		"The evolve runtime wrote this section.",
		"1. Do not run the suspect command again unchanged until you know why the earlier dispatch ended.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("section lacks %q:\n%s", want, got)
		}
	}
	for _, old := range []string{"stated by the bridge", "The bridge wrote", "or a variant of it"} {
		if strings.Contains(got, old) {
			t.Errorf("section keeps the old text %q", old)
		}
	}
}

func TestCodeSpan_RemovesC0ControlsLikeTheBlocks(t *testing.T) {
	if got := codeSpan("a\x1b[31mb\x07\x00c\r\nd"); got != "`a[31mbc ⏎ d`" {
		t.Fatalf("codeSpan = %q, want the C0 controls removed and the newline as ⏎", got)
	}
}

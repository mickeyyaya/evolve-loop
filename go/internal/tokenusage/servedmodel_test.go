package tokenusage

import (
	"path/filepath"
	"testing"
)

func TestScanConfigRoot_ServedModelIsTheLatestRealAssistantModelInTheWindow(t *testing.T) {
	worktree := "/repo/worktrees/cycle-990"
	root := t.TempDir()
	line := func(ts, id, model string) string {
		return `{"type":"assistant","cwd":"` + worktree + `","timestamp":"` + ts + `","message":{"id":"` + id + `","model":"` + model + `","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n"
	}
	writeTranscript(t, filepath.Join(root, "projects", "a"), "early.jsonl", line("2026-07-07T10:00:02Z", "e1", "claude-haiku-5-5"))
	writeTranscript(t, filepath.Join(root, "projects", "b"), "late.jsonl",
		line("2026-07-07T10:00:05.250Z", "m1", "claude-sonnet-5-5")+
			line("2026-07-07T10:00:07Z", "m2", "<synthetic>")+
			line("2026-07-07T10:20:00Z", "m3", "claude-opus-5-5"))

	res, err := ScanConfigRoot(root, Window{Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)})

	if err != nil || res.ServedModel != "claude-sonnet-5-5" {
		t.Fatalf("ServedModel = %q (err %v), want claude-sonnet-5-5: the latest in-window reply, skipping a <synthetic> placeholder and a later out-of-window reply", res.ServedModel, err)
	}
}

func TestScanConfigRoot_ATranscriptWithNoModelFieldReportsNoServedModel(t *testing.T) {
	worktree := "/repo/worktrees/cycle-991"
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "projects", "a"), "s.jsonl",
		`{"type":"assistant","cwd":"`+worktree+`","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":1,"output_tokens":1}}}`+"\n")

	res, err := ScanConfigRoot(root, Window{Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)})

	if err != nil || res.Source != SourceTranscript || res.ServedModel != "" {
		t.Fatalf("result = %+v (err %v), want a transcript source with no served model", res, err)
	}
}

func TestScanConfigRoot_ANewerReplyInAnEarlierWalkedFileBeatsAnOlderReplyWalkedLater(t *testing.T) {
	worktree := "/repo/worktrees/cycle-992"
	root := t.TempDir()
	line := func(ts, id, model string) string {
		return `{"type":"assistant","cwd":"` + worktree + `","timestamp":"` + ts + `","message":{"id":"` + id + `","model":"` + model + `","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n"
	}
	writeTranscript(t, filepath.Join(root, "projects", "a-walked-first"), "newer.jsonl", line("2026-07-07T10:00:06Z", "n1", "claude-opus-5-5"))
	writeTranscript(t, filepath.Join(root, "projects", "b-walked-second"), "older.jsonl", line("2026-07-07T10:00:03Z", "o1", "claude-haiku-5-5"))

	res, err := ScanConfigRoot(root, Window{Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)})

	if err != nil || res.ServedModel != "claude-opus-5-5" {
		t.Fatalf("ServedModel = %q (err %v), want claude-opus-5-5: the newest reply wins even when an older one is walked after it", res.ServedModel, err)
	}
}

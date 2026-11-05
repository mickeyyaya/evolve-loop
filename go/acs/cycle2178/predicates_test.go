//go:build acs

package cycle2178

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycle1967Worktree  = "/Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/worktrees/cycle-cd3ae73e-1967"
	otherWorktree      = "/Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/worktrees/cycle-cd3ae73e-1968"
	buildConversation  = "587d3641-24fd-443d-8989-c539a294e080"
	farConversation    = "11111111-2222-4333-8444-555555555555"
	nearConversation   = "66666666-7777-4888-9999-aaaaaaaaaaaa"
	buildOutputTokens  = 69220
	windowStartMicros  = "2026-10-19 07:07:30.696470"
	windowEndMicros    = "2026-10-19 07:23:34.167863"
	agyTmuxDriver      = "agy-tmux"
	storedZoneSuffix   = "+00:00"
	fixtureRelativeDir = "go/internal/tokenusage/testdata/agy14"
)

type summaryRow struct {
	conversationID string
	worktree       string
	lastUserInput  string
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), fixtureRelativeDir, name))
	if err != nil {
		t.Fatalf("RED: read fixture %s: %v", name, err)
	}
	return string(raw)
}

func runSQLite(t *testing.T, db, script string) {
	t.Helper()
	cmd := exec.Command("sqlite3", db)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sqlite3 %s: %v\n%s", db, err, out)
	}
}

func buildAgyStore(t *testing.T, rows []summaryRow) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "conversations"), 0o755); err != nil {
		t.Fatal(err)
	}
	genRows := strings.Fields(fixture(t, "gen-metadata-cycle1967-build.hex"))
	var summaries strings.Builder
	summaries.WriteString("create table conversation_summaries(conversation_id text, workspace_uris text, last_user_input_time datetime);\n")
	for _, row := range rows {
		fmt.Fprintf(&summaries, "insert into conversation_summaries values('%s','[\"file://%s\"]','%s');\n", row.conversationID, row.worktree, row.lastUserInput)
		var conversation strings.Builder
		conversation.WriteString("create table gen_metadata(idx integer, data blob);\n")
		for idx, hexRow := range genRows {
			fmt.Fprintf(&conversation, "insert into gen_metadata values(%d, X'%s');\n", idx, hexRow)
		}
		runSQLite(t, filepath.Join(root, "conversations", row.conversationID+".db"), conversation.String())
	}
	runSQLite(t, filepath.Join(root, "conversation_summaries.db"), summaries.String())
	return root
}

func agyWindow(t *testing.T, scrollback string) tokenusage.Window {
	t.Helper()
	manifest, err := bridge.LoadManifest(agyTmuxDriver)
	if err != nil {
		t.Fatalf("load agy-tmux manifest: %v", err)
	}
	start, err := time.Parse("2006-01-02 15:04:05.000000", windowStartMicros)
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse("2006-01-02 15:04:05.000000", windowEndMicros)
	if err != nil {
		t.Fatal(err)
	}
	return tokenusage.Window{
		Worktree: cycle1967Worktree, Driver: agyTmuxDriver, Scrollback: scrollback,
		Collector: manifest.Collectors[bridge.CollectorTokens], ContextWindow: manifest.ContextWindow, TokenLineRegex: manifest.TokenLineRegex,
		Start: start.UTC(), End: end.UTC(),
	}
}

func resolveFrom(t *testing.T, rows []summaryRow, scrollback string) tokenusage.Result {
	t.Helper()
	root := buildAgyStore(t, rows)
	result, err := tokenusage.Resolver(tokenusage.Roots{Agy: root}, tokenusage.SQLiteCLI)(agyWindow(t, scrollback))
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	return result
}

func requireCounted(t *testing.T, result tokenusage.Result, what string) {
	t.Helper()
	if result.Source != tokenusage.SourceAgyConversation || result.Usage.Output != buildOutputTokens {
		t.Errorf("RED: %s: source=%q output=%d warn=%q, want source %q with output %d", what, result.Source, result.Usage.Output, result.Warn, tokenusage.SourceAgyConversation, buildOutputTokens)
	}
}

func TestC2178_001_AgyPaneFixtureGivesZeroUnderTheManifestRegexAndNonZeroFromTheConversationStore(t *testing.T) {
	pane := fixture(t, "final-pane-cycle1967-build.txt")
	window := agyWindow(t, pane)
	if got := panestream.TokenLinePeak(pane, window.TokenLineRegex); got != 0 {
		t.Fatalf("the manifest token_line_regex read %d from the real agy 1.4 pane, want 0: the pane prints no count", got)
	}
	if window.Collector != tokenusage.FormatAgyConversation {
		t.Fatalf("agy-tmux manifest collector = %q, want %q", window.Collector, tokenusage.FormatAgyConversation)
	}
	result := resolveFrom(t, []summaryRow{{buildConversation, cycle1967Worktree, "2026-10-19 07:07:31.123456" + storedZoneSuffix}}, pane)
	requireCounted(t, result, "mid-window conversation of the dispatch worktree")
	if result.Usage.Input <= 0 || result.PeakPromptTokens <= 0 {
		t.Errorf("input=%d peak prompt=%d, want the input and the largest generation counted too", result.Usage.Input, result.PeakPromptTokens)
	}
}

func TestC2178_002_AConversationOfAnotherWorktreeOrOutsideTheWindowIsNotCounted(t *testing.T) {
	pane := fixture(t, "final-pane-cycle1967-build.txt")
	result := resolveFrom(t, []summaryRow{
		{buildConversation, otherWorktree, "2026-10-19 07:07:31.123456" + storedZoneSuffix},
		{farConversation, cycle1967Worktree, "2026-10-19 06:00:00.000000" + storedZoneSuffix},
		{nearConversation, cycle1967Worktree, "2026-10-19 08:00:00.000000" + storedZoneSuffix},
	}, pane)
	if result.Source == tokenusage.SourceAgyConversation || result.Usage.Output != 0 {
		t.Errorf("source=%q output=%d, want nothing counted from conversations of another worktree or outside the dispatch window", result.Source, result.Usage.Output)
	}
}

func TestC2178_003_AConversationWhoseInputTimeEqualsTheWindowEndIsCounted(t *testing.T) {
	pane := fixture(t, "final-pane-cycle1967-build.txt")
	result := resolveFrom(t, []summaryRow{
		{buildConversation, cycle1967Worktree, "2026-10-19 07:23:34.167863" + storedZoneSuffix},
		{farConversation, cycle1967Worktree, "2026-10-19 07:23:34.167864" + storedZoneSuffix},
	}, pane)
	requireCounted(t, result, "conversation at the window end to the microsecond, one microsecond past it excluded")
}

func TestC2178_004_AStoredTimeWithTrimmedTrailingZerosEqualToTheWindowStartIsCounted(t *testing.T) {
	pane := fixture(t, "final-pane-cycle1967-build.txt")
	result := resolveFrom(t, []summaryRow{
		{buildConversation, cycle1967Worktree, "2026-10-19 07:07:30.69647" + storedZoneSuffix},
		{farConversation, cycle1967Worktree, "2026-10-19 07:07:30.696469" + storedZoneSuffix},
	}, pane)
	requireCounted(t, result, "conversation stored as .69647 (the window start .696470), one microsecond before it excluded")
}

func TestC2178_005_AStoredTimeWithTrimmedTrailingZerosInsideTheWindowIsCounted(t *testing.T) {
	pane := fixture(t, "final-pane-cycle1967-build.txt")
	result := resolveFrom(t, []summaryRow{
		{buildConversation, cycle1967Worktree, "2026-10-19 07:10:00.5" + storedZoneSuffix},
		{farConversation, cycle1967Worktree, "2026-10-19 07:23:34.1678" + storedZoneSuffix},
	}, pane)
	if result.Source != tokenusage.SourceAgyConversation || result.Usage.Output != 2*buildOutputTokens {
		t.Errorf("RED: source=%q output=%d warn=%q, want both in-window conversations (trimmed .5 and .1678) counted: output %d", result.Source, result.Usage.Output, result.Warn, 2*buildOutputTokens)
	}
}

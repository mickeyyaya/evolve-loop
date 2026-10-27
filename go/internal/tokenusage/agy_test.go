package tokenusage

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	agy14Worktree = "/Users/danleemh/ai/claude/evolve-loop/runtime/.evolve/worktrees/cycle-cd3ae73e-1967"
	agy14BuildID  = "587d3641-24fd-443d-8989-c539a294e080"
)

func agy14Pane(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "agy14", "final-pane-cycle1967-build.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func agy14GenRows(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "agy14", "gen-metadata-cycle1967-build.hex"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func agy14Window() Window {
	return Window{Worktree: agy14Worktree, Driver: "agy-tmux", Collector: FormatAgyConversation, ContextWindow: 200_000,
		Start: time.Date(2026, 10, 19, 7, 7, 30, 696469000, time.UTC), End: time.Date(2026, 10, 19, 7, 23, 34, 167863000, time.UTC)}
}

type agyStore struct {
	queries []string
	rows    map[string]string
	fail    map[string]error
}

func (s *agyStore) query(db, sql string) (string, error) {
	s.queries = append(s.queries, filepath.Base(db)+": "+sql)
	if err := s.fail[filepath.Base(db)]; err != nil {
		return "", err
	}
	return s.rows[filepath.Base(db)], nil
}

func TestAgyConversationCollector_Agy14BuildPaneHasNoTokenLineButItsConversationCounts(t *testing.T) {
	t.Parallel()
	agyRegex := `(?i)Thought for [^,]*,\s*([0-9][0-9,]*)\s*tokens`
	if got := TokenLinePeakCollector(agy14Pane(t), agyRegex)(); got.Source != SourceNone {
		t.Fatalf("the token line collector read %+v from the agy 1.4 pane, want nothing: the pane prints no count", got)
	}
	store := &agyStore{rows: map[string]string{
		"conversation_summaries.db": agy14BuildID + "\n",
		agy14BuildID + ".db":        agy14GenRows(t),
	}}

	got := AgyConversationCollector("/agy", agy14Window(), store.query)()

	want := cyclestate.TokenUsage{Input: 554617, Output: 69220, CacheRead: 13904571}
	if got.Source != SourceAgyConversation || got.Usage != want {
		t.Errorf("collector = %+v, want %+v from the agy conversation store", got, want)
	}
	if got.PeakPromptTokens <= 0 || got.PeakPromptTokens > 1_000_000 || got.PeakUsage.Output == 0 {
		t.Errorf("peak prompt = %d, peak usage %+v, want the largest single generation", got.PeakPromptTokens, got.PeakUsage)
	}
	if len(store.queries) != 2 || !strings.Contains(store.queries[0], `instr(workspace_uris, '"file://`+agy14Worktree+`"')`) ||
		!strings.Contains(store.queries[0], "last_user_input_time >= '2026-10-19 07:07:30.696469'") || !strings.Contains(store.queries[0], "last_user_input_time <= '2026-10-19 07:23:34.167863'") {
		t.Errorf("queries = %q, want the conversations of the worktree whose user input falls in the dispatch window", store.queries)
	}
}

func TestAgyConversationCollector_NoDataIsNoSource(t *testing.T) {
	t.Parallel()
	rows := agy14GenRows(t)
	cases := map[string]struct {
		root  string
		w     Window
		store *agyStore
	}{
		"no agy root":         {"", agy14Window(), &agyStore{}},
		"no worktree":         {"/agy", Window{Start: agy14Window().Start, End: agy14Window().End}, &agyStore{}},
		"no window":           {"/agy", Window{Worktree: agy14Worktree}, &agyStore{}},
		"summaries unread":    {"/agy", agy14Window(), &agyStore{fail: map[string]error{"conversation_summaries.db": errors.New("no such table")}}},
		"no conversation":     {"/agy", agy14Window(), &agyStore{rows: map[string]string{}}},
		"a foreign id":        {"/agy", agy14Window(), &agyStore{rows: map[string]string{"conversation_summaries.db": "../../etc/passwd\n", "../../etc/passwd.db": rows}}},
		"conversation unread": {"/agy", agy14Window(), &agyStore{rows: map[string]string{"conversation_summaries.db": agy14BuildID}, fail: map[string]error{agy14BuildID + ".db": errors.New("locked")}}},
		"malformed rows":      {"/agy", agy14Window(), &agyStore{rows: map[string]string{"conversation_summaries.db": agy14BuildID, agy14BuildID + ".db": "zz\n0a05\n0a03220108\n08\n"}}},
	}
	for name, c := range cases {
		if got := AgyConversationCollector(c.root, c.w, c.store.query)(); got.Source != SourceNone {
			t.Errorf("%s: collector = %+v, want no source", name, got)
		}
	}
}

func TestAgyConversationCollector_ADroppedConversationOrRowIsAWarning(t *testing.T) {
	t.Parallel()
	other := "11111111-2222-3333-4444-555555555555"
	store := &agyStore{
		rows: map[string]string{"conversation_summaries.db": agy14BuildID + "\n" + other, agy14BuildID + ".db": agy14GenRows(t) + "zz\n"},
		fail: map[string]error{other + ".db": errors.New("locked")},
	}

	got := AgyConversationCollector("/agy", agy14Window(), store.query)()

	if got.Source != SourceAgyConversation || got.Usage.Output != 69220 || !strings.Contains(got.Warn, other+" unread (locked)") || !strings.Contains(got.Warn, "malformed generation row") {
		t.Errorf("collector = %+v, want the counted usage with a warning for each dropped conversation and row", got)
	}
}

func TestAgyConversationCollector_AQuoteInTheWorktreeIsEscaped(t *testing.T) {
	t.Parallel()
	store := &agyStore{}
	w := agy14Window()
	w.Worktree = "/tmp/it's"

	AgyConversationCollector("/agy", w, store.query)()

	if len(store.queries) != 1 || !strings.Contains(store.queries[0], `'"file:///tmp/it''s"'`) {
		t.Errorf("query = %q, want the quote doubled", store.queries)
	}
}

func TestAgyDataRoot_IsTheAntigravityCLIHome(t *testing.T) {
	t.Setenv("HOME", "")
	if got := AgyDataRoot(map[string]string{"HOME": "/h"}); got != filepath.Join("/h", ".gemini", "antigravity-cli") {
		t.Errorf("AgyDataRoot = %q", got)
	}
	if got := AgyDataRoot(nil); got != "" {
		t.Errorf("AgyDataRoot with no HOME = %q, want empty", got)
	}
}

func TestSQLiteCLI_ReadsAStoreReadOnly(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	db := filepath.Join(t.TempDir(), "s.db")
	if out, err := exec.Command("sqlite3", db, "create table gen_metadata(idx integer, data blob); insert into gen_metadata values (1, x'0a05220318b91f');").CombinedOutput(); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}

	rows, err := SQLiteCLI(db, "select hex(data) from gen_metadata order by idx;")
	_, werr := SQLiteCLI(db, "insert into gen_metadata values (2, x'00');")

	if err != nil || strings.TrimSpace(rows) != "0A05220318B91F" {
		t.Errorf("rows = %q, %v", rows, err)
	}
	if werr == nil {
		t.Error("a write through the read-only reader succeeded")
	}
	if u, err := agyGenerationUsage(strings.TrimSpace(rows)); err != nil || u.Output != 4025 {
		t.Errorf("usage = %+v, %v; want output 4025", u, err)
	}
}

func TestResolver_AnAgyDriverReadsItsConversationStoreFirst(t *testing.T) {
	t.Parallel()
	store := &agyStore{rows: map[string]string{"conversation_summaries.db": agy14BuildID, agy14BuildID + ".db": agy14GenRows(t)}}
	w := agy14Window()
	w.Scrollback = agy14Pane(t)

	var query SQLiteQuery = store.query
	got, err := Resolver(Roots{Agy: "/agy"}, query)(w)
	claude, _ := Resolver(Roots{Agy: "/agy"}, store.query)(Window{Driver: "claude-tmux", Collector: FormatClaudeJSONL, ContextWindow: 200_000, Worktree: agy14Worktree, Start: w.Start, End: w.End})

	if err != nil || got.Source != SourceAgyConversation || got.Usage.Output != 69220 || got.FillPct <= 0 {
		t.Errorf("Resolver(agy-tmux) = %+v, %v; want the conversation usage with a fill", got, err)
	}
	if claude.Source == SourceAgyConversation {
		t.Errorf("a claude driver read the agy store: %+v", claude)
	}
}

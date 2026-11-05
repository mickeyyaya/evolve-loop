package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type loginFamily struct {
	cli       string
	rule      string
	loggedOut string
	prompt    string
}

var loginFamilies = []loginFamily{
	{"claude-tmux", "auth_recheck", "⏺ Login expired · Please run /login", "╭──────────╮\n│ >        │\n╰──────────╯\n  ? for shortcuts"},
	{"agy", "auth_recheck", "Authentication required: Sign in to Antigravity", "> \n  type a message"},
	{"agy-tmux", "auth_recheck", "Authentication required, Please log in", "> \n  type a message"},
	{"codex-tmux", "auth_recheck", "Please log in or sign in to ChatGPT", "› \n  100% context left"},
	{"ollama-tmux", "auth_required", "Error: unauthorized, run ollama signin", ">>> Send a message (/? for help)"},
}

func loginPaneFiller(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "agent output line %d\n", i)
	}
	return b.String()
}

func loginPrompts(t *testing.T, f loginFamily) []ManifestPrompt {
	t.Helper()
	m, err := LoadManifest(f.cli)
	if err != nil {
		t.Fatalf("%s: %v", f.cli, err)
	}
	return m.InteractivePrompts
}

func TestLoginPattern_ALoggedOutLineAtTheBottomEscalatesWithItsPattern(t *testing.T) {
	t.Parallel()
	for _, f := range loginFamilies {
		f := f
		t.Run(f.cli, func(t *testing.T) {
			t.Parallel()
			pane := loginPaneFiller(40) + f.loggedOut + "\n" + f.prompt + "\n"
			action, rc := decideAutoRespond(pane, loginPrompts(t, f), map[string]int{}, false)
			if action != "escalate:"+f.rule || rc != 85 {
				t.Fatalf("got %q rc=%d, want escalate:%s rc=85", action, rc, f.rule)
			}
		})
	}
}

func TestLoginPattern_APhraseQuotedFarAboveTheBottomDoesNotEscalate(t *testing.T) {
	t.Parallel()
	for _, f := range loginFamilies {
		f := f
		t.Run(f.cli, func(t *testing.T) {
			t.Parallel()
			pane := "agent: the CLI printed: " + f.loggedOut + "\n" + loginPaneFiller(60) + f.prompt + "\n"
			action, _ := decideAutoRespond(pane, loginPrompts(t, f), map[string]int{}, false)
			if action == "escalate:"+f.rule {
				t.Fatalf("%s: quoted phrase far above the tail escalated: %q", f.cli, action)
			}
		})
	}
}

func TestLoginPattern_EveryLoginRuleIsBottomAnchored(t *testing.T) {
	t.Parallel()
	for _, f := range loginFamilies {
		found := false
		for _, p := range loginPrompts(t, f) {
			if p.Name == f.rule {
				found = true
				if p.TailLines <= 0 || p.TailLines > 30 {
					t.Errorf("%s: %s tail_lines=%d, want 1..30", f.cli, f.rule, p.TailLines)
				}
			}
		}
		if !found {
			t.Errorf("%s: no %s rule", f.cli, f.rule)
		}
	}
}

func TestLoginPattern_ClaudeMatchesTheRealExpiredLineAndPleaseRunLogin(t *testing.T) {
	t.Parallel()
	f := loginFamilies[0]
	for _, text := range []string{"⏺ Login expired · Please run /login", "⏺ Not logged in · Please run /login", "Please log in", "Authentication required."} {
		pane := loginPaneFiller(20) + text + "\n" + f.prompt + "\n"
		action, _ := decideAutoRespond(pane, loginPrompts(t, f), map[string]int{}, false)
		if action != "escalate:auth_recheck" {
			t.Errorf("%q: got %q", text, action)
		}
	}
}

func TestLoginPattern_OtherFamiliesRegexesAreNotWidened(t *testing.T) {
	t.Parallel()
	for _, f := range loginFamilies[1:] {
		for _, p := range loginPrompts(t, f) {
			if p.Name == f.rule && strings.Contains(p.Regex, "run /login") {
				t.Errorf("%s: regex widened with run /login: %s", f.cli, p.Regex)
			}
		}
	}
}

func realClaudeLoginFrame(t *testing.T) string {
	t.Helper()
	frame, err := os.ReadFile(filepath.Join("testdata", "claude-login-expired-frame.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(frame)
}

func claudeStackBelowLogin(t *testing.T) string {
	t.Helper()
	frame := realClaudeLoginFrame(t)
	_, stack, ok := strings.Cut(frame, "Please run /login\n")
	if !ok {
		t.Fatalf("the frame has no login line: %q", frame)
	}
	return stack
}

func TestLoginPattern_TheRealClaudeFrameEscalates(t *testing.T) {
	t.Parallel()
	pane := loginPaneFiller(40) + realClaudeLoginFrame(t)
	action, rc := decideAutoRespond(pane, loginPrompts(t, loginFamilies[0]), map[string]int{}, false)
	if action != "escalate:auth_recheck" || rc != 85 {
		t.Fatalf("got %q rc=%d, want the real frame to escalate", action, rc)
	}
}

func TestLoginPattern_AgentTextThatQuotesThePhraseOverTheRealStackDoesNotEscalate(t *testing.T) {
	t.Parallel()
	stack := claudeStackBelowLogin(t)
	for name, quote := range map[string]string{
		"a message that ends in the phrase":  "⏺ The rule matches the real line: Please run /login\n",
		"a quote in the last row of a list":  "  - Login expired · Please run /login\n",
		"a bullet that quotes the real line": "⏺ The real line reads Login expired · Please run /login\n",
		"the old words inside a sentence":    "⏺ The pane said Authentication required here, and Please log in there.\n",
	} {
		pane := loginPaneFiller(40) + quote + stack
		action, _ := decideAutoRespond(pane, loginPrompts(t, loginFamilies[0]), map[string]int{}, false)
		if action != "noop" {
			t.Errorf("%s: got %q, want noop", name, action)
		}
	}
}

func TestLoginPattern_EachRecordedWordingEscalatesOverTheRealStack(t *testing.T) {
	t.Parallel()
	stack := claudeStackBelowLogin(t)
	for _, wording := range []string{"Login expired", "OAuth token revoked", "Not logged in"} {
		pane := loginPaneFiller(40) + "⏺ " + wording + " · Please run /login\n" + stack
		action, rc := decideAutoRespond(pane, loginPrompts(t, loginFamilies[0]), map[string]int{}, false)
		if action != "escalate:auth_recheck" || rc != 85 {
			t.Errorf("%s: got %q rc=%d, want the login class", wording, action, rc)
		}
	}
}

func TestLoginPattern_TheLoginLineThirteenRowsUpDoesNotEscalate(t *testing.T) {
	t.Parallel()
	pane := "⏺ Login expired · Please run /login\n" + loginPaneFiller(13)
	action, _ := decideAutoRespond(pane, loginPrompts(t, loginFamilies[0]), map[string]int{}, false)
	if action != "noop" {
		t.Fatalf("got %q, want noop", action)
	}
}

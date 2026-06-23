package guards

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// === stripHeredocs table ===================================================
func TestStripHeredocs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// We assert that specific phrases are NOT present in the output
		// (heredoc body stripped) and that markers ARE preserved.
		mustNotContain []string
		mustContain    []string
	}{
		{
			name:        "no heredoc returns input unchanged",
			in:          "echo hello world",
			mustContain: []string{"echo hello world"},
		},
		{
			name: "simple unquoted heredoc",
			in: `cat <<EOF
git push origin :refs/tags/x
git commit -m foo
EOF
echo done`,
			mustNotContain: []string{"git push origin", "git commit"},
			mustContain:    []string{"cat <<EOF", "EOF", "echo done"},
		},
		{
			name: "single-quoted heredoc marker",
			in: `cat <<'EOF'
git push something
EOF
done`,
			mustNotContain: []string{"git push something"},
			mustContain:    []string{"<<'EOF'", "EOF"},
		},
		{
			name: "double-quoted heredoc marker",
			in: `cat <<"END"
git commit -m x
END
trailing`,
			mustNotContain: []string{"git commit -m x"},
			mustContain:    []string{`<<"END"`, "END", "trailing"},
		},
		{
			name: "tab-stripping heredoc <<-MARKER",
			in: `cat <<-EOF
	git push body
	EOF
trailing`,
			mustNotContain: []string{"git push body"},
			mustContain:    []string{"<<-EOF", "trailing"},
		},
		{
			name: "two sequential heredocs in one command",
			in: `cat <<EOF
git push first
EOF
cat <<DONE
git commit second
DONE
end`,
			mustNotContain: []string{"git push first", "git commit second"},
			mustContain:    []string{"<<EOF", "<<DONE", "EOF", "DONE", "end"},
		},
		{
			name: "marker with surrounding whitespace at close",
			in: `cat <<EOF
git push body
   EOF
end`,
			mustNotContain: []string{"git push body"},
			mustContain:    []string{"end"},
		},
		{
			name: "unterminated heredoc drops rest",
			in: `cat <<EOF
git push body line 1
git commit body line 2`,
			mustNotContain: []string{"git push body line 1", "git commit body line 2"},
			mustContain:    []string{"<<EOF"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := stripHeredocs(tc.in)
			for _, s := range tc.mustNotContain {
				if strings.Contains(out, s) {
					t.Errorf("output should NOT contain %q\nout=%q", s, out)
				}
			}
			for _, s := range tc.mustContain {
				if !strings.Contains(out, s) {
					t.Errorf("output should contain %q\nout=%q", s, out)
				}
			}
		})
	}
}

// === The full ship-gate now lets verbs-in-heredoc-body pass ================
func TestShip_Decide_VerbInHeredocBody(t *testing.T) {
	s := NewShip(false)
	// Native evolve ship invocation with commit message body that
	// legitimately mentions `git push` and `git commit` (the v11.7.5
	// failure mode that triggered this fix).
	body := `evolve ship --class manual "$(cat <<'EOF'
feat: port a script that calls git push origin and git commit -m
The script does:
  1. git revert HEAD
  2. git push origin :refs/tags/X
EOF
)"`
	in := core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": body},
	}
	dec := s.Decide(context.Background(), in)
	if !dec.Allow {
		t.Errorf("verb-in-heredoc-body should be allowed (native ship path); got DENY: %s", dec.Reason)
	}
}

// === A bare `git push` outside any heredoc still triggers DENY =============
func TestShip_Decide_BareGitPush_Denied(t *testing.T) {
	s := NewShip(false)
	in := core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "git push origin main"},
	}
	dec := s.Decide(context.Background(), in)
	if dec.Allow {
		t.Error("bare git push should be DENIED")
	}
}

// === `evolve ship` invocation is allowed ====================================
func TestShip_Decide_NativeEvolveShip_Allowed(t *testing.T) {
	s := NewShip(false)
	cases := []string{
		`evolve ship --class manual "msg"`,
		`go/bin/evolve ship --class manual "msg"`,
		`/abs/path/to/evolve ship "msg"`,
		`EVOLVE_SHIP_AUTO_CONFIRM=1 evolve ship --class manual "git push body"`,
	}
	for _, c := range cases {
		in := core.GuardInput{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": c},
		}
		dec := s.Decide(context.Background(), in)
		if !dec.Allow {
			t.Errorf("native evolve ship should be allowed: %q\n  reason: %s", c, dec.Reason)
		}
	}
}

// === Word-boundary safety: "devolve ship" must NOT match nativeShipRe ======
func TestShip_Decide_WordBoundary_Devolve(t *testing.T) {
	s := NewShip(false)
	in := core.GuardInput{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "devolve ship git push body",
		},
	}
	dec := s.Decide(context.Background(), in)
	if dec.Allow {
		t.Error("'devolve ship' should NOT be recognized as native evolve ship; git push should DENY")
	}
}

// === Canonical bash ship.sh path still allowed (no regression) ============
func TestShip_Decide_BashShipSh_Allowed(t *testing.T) {
	s := NewShip(false)
	in := core.GuardInput{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": `bash legacy/scripts/lifecycle/ship.sh "msg with git push in body"`,
		},
	}
	dec := s.Decide(context.Background(), in)
	if !dec.Allow {
		t.Errorf("canonical bash ship.sh should be allowed; got DENY: %s", dec.Reason)
	}
}

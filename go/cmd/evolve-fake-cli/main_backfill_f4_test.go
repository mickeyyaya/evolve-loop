package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChallengeTokenLine_TokenFileShapes(t *testing.T) {
	cases := []struct {
		name    string
		content *string
		want    string
	}{
		{"absent file yields no line", nil, ""},
		{"empty file yields no line", ptrF4(""), ""},
		{"whitespace-only file yields no line", ptrF4(" \n\t\n"), ""},
		{"token is trimmed and wrapped", ptrF4("  abc123\n"), "<!-- challenge-token: abc123 -->\n"},
		{"unicode token is preserved", ptrF4("токен-✓\n"), "<!-- challenge-token: токен-✓ -->\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.content != nil {
				if err := os.WriteFile(filepath.Join(dir, "challenge-token.txt"), []byte(*tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := challengeTokenLine(filepath.Join(dir, "scout-report.md")); got != tc.want {
				t.Fatalf("challengeTokenLine = %q, want %q", got, tc.want)
			}
		})
	}
}

func ptrF4(s string) *string { return &s }

func TestArtifactsFor_PrependsChallengeTokenToMainArtifactOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "challenge-token.txt"), []byte("tok-9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "retrospective.md")
	files, err := artifactsFor("retro", mainPath, "PASS")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(files[mainPath], "<!-- challenge-token: tok-9 -->\n# Retrospective") {
		t.Errorf("main artifact must start with the token line; got %q", files[mainPath])
	}
	lesson := files[filepath.Join(dir, "failure-lesson-1.yaml")]
	if lesson == "" || strings.Contains(lesson, "challenge-token") {
		t.Errorf("sidecar artifact must exist without the token; got %q", lesson)
	}
}

func TestArtifactsFor_NoTokenLeavesMainArtifactUntouched(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "scout-report.md")
	files, err := artifactsFor("scout", mainPath, "PASS")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(files[mainPath], "# Scout Report") {
		t.Errorf("without a token the artifact starts at its heading; got %q", files[mainPath])
	}
}

func TestRunREPL_UnresolvedTurnPathSkipsTurnAndFallsBackToEOFServe(t *testing.T) {
	orig := replResolveArtifactPath
	t.Cleanup(func() { replResolveArtifactPath = orig })
	replResolveArtifactPath = func(string, string) string { return "" }

	ws := t.TempDir()
	var stdout, stderr bytes.Buffer
	rc := runREPL(strings.NewReader(composedPrompt("# Evolve Scout", ws)), &stdout, &stderr, "PASS")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(filepath.Join(ws, "scout-report.md")); err != nil {
		t.Errorf("EOF fallback must still serve the prompt: %v", err)
	}
	if got := strings.Count(stdout.String(), replBootMarkers); got != 1 {
		t.Errorf("the per-turn boot marker must not be printed for a skipped turn, got %d in %q", got, stdout.String())
	}
}

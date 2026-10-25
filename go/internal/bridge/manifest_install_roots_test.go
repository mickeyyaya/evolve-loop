package bridge

import (
	"slices"
	"strings"
	"testing"
)

func TestParseManifest_InstallRootsMustBeAbsoluteOrUnderHome(t *testing.T) {
	t.Parallel()
	m, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"hypo","install_roots":["~/.hypo/versions","/opt/hypo"]}`))
	if err != nil || !slices.Equal(m.InstallRoots, []string{"~/.hypo/versions", "/opt/hypo"}) {
		t.Fatalf("install_roots not read: %v %v", m.InstallRoots, err)
	}
	for _, bad := range []string{"rel/dir", "~/../etc", "/opt/../etc", "", "/", "~/", "//"} {
		_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"hypo","install_roots":["`+bad+`"]}`))
		if err == nil || !strings.Contains(err.Error(), "install_roots") {
			t.Errorf("install_roots %q must be refused, got %v", bad, err)
		}
	}
}

func TestClaudeManifests_DeclareTheVersionsDirectoryAsTheirInstallRoot(t *testing.T) {
	t.Parallel()
	for _, cli := range []string{"claude-p", "claude-tmux"} {
		m, err := loadManifestRaw(cli)
		if err != nil || !slices.Contains(m.InstallRoots, "~/.local/share/claude/versions") {
			t.Errorf("%s: install_roots = %v (%v); the native installer keeps each version there", cli, m.InstallRoots, err)
		}
	}
}

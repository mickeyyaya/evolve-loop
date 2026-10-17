package policy

import (
	"strings"
	"testing"
)

func TestCodeIndexConfig_AbsentBlockResolvesTheCompiledDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{}`).CodeIndexConfig()

	if err != nil {
		t.Fatalf("CodeIndexConfig: %v", err)
	}
	if len(cfg.Backends) != 1 || cfg.Backends[0] != "trail" {
		t.Errorf("Backends = %v, want [trail]", cfg.Backends)
	}
	if cfg.IndexDir != "dev/code-index" || cfg.TrailBin != "dev/.tools/trail/node_modules/.bin/trail" {
		t.Errorf("IndexDir = %q, TrailBin = %q, want the hub-relative defaults", cfg.IndexDir, cfg.TrailBin)
	}
}

func TestCodeIndexConfig_APresentBlockOverridesEachKeyAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{"code_index":{"backends":["gopls","trail"],"index_dir":"/srv/idx"}}`).CodeIndexConfig()

	if err != nil {
		t.Fatalf("CodeIndexConfig: %v", err)
	}
	if strings.Join(cfg.Backends, ",") != "gopls,trail" || cfg.IndexDir != "/srv/idx" || cfg.TrailBin != "dev/.tools/trail/node_modules/.bin/trail" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestCodeIndexConfig_ATrailBinOverrideIsKept(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{"code_index":{"trail_bin":"/opt/trail"}}`).CodeIndexConfig()

	if err != nil || cfg.TrailBin != "/opt/trail" {
		t.Errorf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestCodeIndexConfig_RefusesWhatCannotRun(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ body, want string }{
		"empty backends":    {`{"code_index":{"backends":[]}}`, "backends must name at least one backend"},
		"blank backend":     {`{"code_index":{"backends":["trail"," "]}}`, "backends: name must not be blank"},
		"duplicate backend": {`{"code_index":{"backends":["trail","trail"]}}`, `duplicate backend "trail"`},
		"blank index dir":   {`{"code_index":{"index_dir":" "}}`, "index_dir must not be blank"},
		"blank trail bin":   {`{"code_index":{"trail_bin":""}}`, "trail_bin must not be blank"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadCIWatchPolicy(t, tc.body).CodeIndexConfig()

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestCodeIndexPolicy_DecodesStrictly(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte(`{"code_index":{"backend":["trail"]}}`))

	if err == nil || !strings.Contains(err.Error(), "code_index") {
		t.Errorf("Parse err = %v, want a code_index refusal", err)
	}
}

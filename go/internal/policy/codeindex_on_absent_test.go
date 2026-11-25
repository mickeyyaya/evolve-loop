package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestCodeIndexOnAbsent_TheDefaultRefusesAndDoesNotRouteToTheNextBackend(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{}`).CodeIndexConfig()

	if err != nil || cfg.OnAbsent != CodeIndexOnAbsentRefuse || CodeIndexOnAbsentRefuse != "refuse" {
		t.Errorf("OnAbsent = %q, err = %v, want refuse", cfg.OnAbsent, err)
	}
}

func TestCodeIndexOnAbsent_ABlockWithoutTheKeyKeepsTheDefault(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{"code_index":{"backends":["gotypes","trail"]}}`).CodeIndexConfig()

	if err != nil || cfg.OnAbsent != CodeIndexOnAbsentRefuse {
		t.Errorf("OnAbsent = %q, err = %v, want refuse", cfg.OnAbsent, err)
	}
}

func TestCodeIndexOnAbsent_EachNamedModeIsKept(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]string{CodeIndexOnAbsentRefuse: "refuse", CodeIndexOnAbsentNext: "next"} {
		cfg, err := loadCIWatchPolicy(t, `{"code_index":{"on_absent":"`+mode+`"}}`).CodeIndexConfig()

		if err != nil || cfg.OnAbsent != want {
			t.Errorf("on_absent %q: OnAbsent = %q, err = %v, want %q", mode, cfg.OnAbsent, err, want)
		}
	}
}

func TestCodeIndexOnAbsent_ANextModeLeavesTheOtherKeysAlone(t *testing.T) {
	t.Parallel()

	cfg, err := loadCIWatchPolicy(t, `{"code_index":{"on_absent":"next","stale":"refuse"}}`).CodeIndexConfig()

	if err != nil || cfg.Stale != CodeIndexStaleRefuse || cfg.OnAbsent != CodeIndexOnAbsentNext || strings.Join(cfg.Backends, ",") != "trail" {
		t.Errorf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestCodeIndexOnAbsent_AnUnknownWordIsAPolicyError(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"failover":   `{"code_index":{"on_absent":"failover"}}`,
		"blank":      `{"code_index":{"on_absent":""}}`,
		"wrong case": `{"code_index":{"on_absent":"Next"}}`,
	} {
		_, err := loadCIWatchPolicy(t, body).CodeIndexConfig()

		if !errors.Is(err, ErrCodeIndexOnAbsent) {
			t.Errorf("%s: err = %v, want ErrCodeIndexOnAbsent", name, err)
		}
	}
}

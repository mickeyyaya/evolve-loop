package bridge

import (
	"encoding/json"
	"testing"
)

func TestEveryClaudeSettingsFlagTurnsThePRStatusFooterOff(t *testing.T) {
	t.Parallel()
	seen := 0
	for _, m := range embeddedClaudeManifests(t) {
		for scope := range m.Params["settings_scope"].Values {
			flags := Realize(m, LaunchIntent{Permission: "bypass", SettingsScope: scope}).LaunchFlags
			raw, ok := flagValue(flags, "--settings")
			if !ok {
				continue
			}
			seen++
			var settings map[string]any
			if err := json.Unmarshal([]byte(raw), &settings); err != nil {
				t.Fatalf("%s scope %s: --settings %q is not JSON: %v", m.CLI, scope, raw, err)
			}
			if v, present := settings["prStatusFooterEnabled"]; !present || v != false {
				t.Errorf("%s scope %s: --settings %q must set prStatusFooterEnabled false, or the claude harness reads ~/.config/gh for its PR footer in every cell", m.CLI, scope, raw)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no claude manifest realizes a --settings flag, so the footer contract proves nothing")
	}
}

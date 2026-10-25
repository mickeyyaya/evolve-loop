package bridge

import "testing"

func TestNewDefault_SeedsTheEnvStageFromPolicyAndSharesOneDropLog(t *testing.T) {
	dir := t.TempDir()
	writePolicyJson(t, dir, `{"isolation":{"i2":{"stage":"enforce"}}}`)
	a := NewDefault(dir, nil)
	first, second := a.productionEngineDeps(nil), a.productionEngineDeps(nil)
	if first.EnvStage != "enforce" {
		t.Fatalf("EnvStage = %q, want the policy's enforce", first.EnvStage)
	}
	if first.EnvDrops == nil || first.EnvDrops != second.EnvDrops {
		t.Fatal("every dispatch of one adapter must share one drop log, so each dropped name is reported once")
	}
	if New().productionEngineDeps(nil).EnvStage != "" {
		t.Fatal("an adapter with no policy keeps the compiled default stage")
	}
}

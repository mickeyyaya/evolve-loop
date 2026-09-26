package bridge

import (
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestNewDefault_ThreadsTheSignalCenterIntoEveryEngine(t *testing.T) {
	c := signalcenter.New()
	a := NewDefault(t.TempDir(), c)
	if !a.SignalsWired() {
		t.Fatal("an Adapter built with a Center must report it wired")
	}
	if d := a.productionEngineDeps(map[string]string{"HOME": t.TempDir()}); d.Signals != c {
		t.Error("productionEngineDeps(env).Signals must be the injected Center — every engine this Adapter builds produces into it")
	}
	eng, ok := a.engineFactory(map[string]string{"HOME": t.TempDir()}).(*gobridge.Engine)
	if !ok {
		t.Fatalf("engineFactory returned %T, want *bridge.Engine", eng)
	}
	if !eng.SignalsWired() {
		t.Error("the REAL engineFactory closure must build engines that report SignalsWired")
	}
}

func TestNewDefault_NilCenterIsTheNullObjectAndSaysSo(t *testing.T) {
	if NewDefault(t.TempDir(), nil).SignalsWired() {
		t.Error("nil is the Null Object for tests and the pinned Center-less roots — reported unwired, never silently wired")
	}
}

func TestAdapter_SignalsReturnsTheInjectedCenter(t *testing.T) {
	c := signalcenter.New()
	if a := NewDefault(t.TempDir(), c); a.Signals() != c {
		t.Error("Signals() must be the injected Center")
	}
	if a := NewDefault(t.TempDir(), nil); a.Signals() != nil || a.SignalsWired() {
		t.Error("nil in ⇒ nil out, unwired")
	}
	var none *Adapter
	if none.Signals() != nil {
		t.Error("a nil receiver reads as the Null Object")
	}
}

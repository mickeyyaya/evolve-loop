package gatecontract

import (
	"reflect"
	"slices"
	"testing"
)

func TestTrustClasses_AreTheFiveWordsOfTheInventory(t *testing.T) {
	t.Parallel()
	want := []string{"authority", "verified", "subject", "advisory", "open-seam"}
	if got := TrustClasses(); !slices.Equal(got, want) {
		t.Errorf("TrustClasses() = %v, want %v", got, want)
	}
	if consts := []string{ClassAuthority, ClassVerified, ClassSubject, ClassAdvisory, ClassOpenSeam}; !slices.Equal(consts, want) {
		t.Errorf("the class constants = %v, want the inventory words %v", consts, want)
	}
}

func TestKinds_AreScannerPhaseFloorAndShipCheck(t *testing.T) {
	t.Parallel()
	want := []string{"scanner", "phase-floor", "ship-check"}
	if got := Kinds(); !slices.Equal(got, want) {
		t.Errorf("Kinds() = %v, want %v", got, want)
	}
}

func TestStages_AreShadowAndEnforce(t *testing.T) {
	t.Parallel()
	want := []string{"shadow", "enforce"}
	if got := Stages(); !slices.Equal(got, want) {
		t.Errorf("Stages() = %v, want %v", got, want)
	}
}

func TestLists_ReturnACopyThatACallerCannotMutate(t *testing.T) {
	t.Parallel()
	for name, list := range map[string]func() []string{"TrustClasses": TrustClasses, "Kinds": Kinds, "Stages": Stages} {
		first := list()
		first[0] = "changed"
		if list()[0] == "changed" {
			t.Errorf("%s() shares its backing array with the caller", name)
		}
	}
}

func carriesCode(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Func, reflect.Interface, reflect.Chan, reflect.UnsafePointer:
		return true
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if carriesCode(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Slice, reflect.Array, reflect.Pointer:
		return carriesCode(t.Elem(), seen)
	case reflect.Map:
		return carriesCode(t.Key(), seen) || carriesCode(t.Elem(), seen)
	}
	return false
}

func TestInput_HoldsNoFunctionInterfaceOrChannelAtAnyDepth(t *testing.T) {
	t.Parallel()
	if carriesCode(reflect.TypeOf(Input{}), map[reflect.Type]bool{}) {
		t.Error("Input carries code: a function, an interface or a channel")
	}
	if !carriesCode(reflect.TypeOf(Ports{}), map[reflect.Type]bool{}) || !carriesCode(reflect.TypeOf(map[string][]func(){}), map[reflect.Type]bool{}) {
		t.Error("the check does not see code in Ports or in a nested map")
	}
}

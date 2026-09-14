package config

import "testing"

// IsRegistryFault is the projection readers of the mandatory set forward:
// the registry unreadable or malformed, or the spine it declares weak — and
// nothing about individual dials or phases.
func TestIsRegistryFault_RegistryAndSpineCodesOnly(t *testing.T) {
	for _, code := range []string{codeRegistryUnreadable, codeRegistryMalformed, codeWeakSpine} {
		if !IsRegistryFault(Warning{Code: code}) {
			t.Errorf("%s is a registry fault", code)
		}
	}
	for _, code := range []string{codeUnknownValue, "", "phase-disabled"} {
		if IsRegistryFault(Warning{Code: code}) {
			t.Errorf("%s is not a registry fault", code)
		}
	}
}

package main

import (
	"bytes"
	"testing"
)

// TestRegistry_UniqueNames guards against accidental duplicate Name or
// Alias entries colliding in the dispatcher table. A duplicate would
// silently route the second registration's command to the first
// (lookupCommand returns on first match).
func TestRegistry_UniqueNames(t *testing.T) {
	seen := make(map[string]string) // key → "name" or alias-of-name
	for _, c := range commands {
		if prev, ok := seen[c.Name]; ok {
			t.Errorf("duplicate command name %q: previously registered via %q", c.Name, prev)
		}
		seen[c.Name] = "name"
		for _, a := range c.Aliases {
			if prev, ok := seen[a]; ok {
				t.Errorf("alias %q for %q collides with previously registered %q",
					a, c.Name, prev)
			}
			seen[a] = "alias-of-" + c.Name
		}
	}
}

// TestRegistry_RunNotNil ensures every registered command has a non-nil
// handler. A nil Run would panic at dispatch time.
func TestRegistry_RunNotNil(t *testing.T) {
	for _, c := range commands {
		if c.Run == nil {
			t.Errorf("command %q has nil Run", c.Name)
		}
	}
}

// TestRegistry_LookupResolvesAliases is the core invariant: every
// declared alias must route to the same row as the canonical name.
func TestRegistry_LookupResolvesAliases(t *testing.T) {
	for _, c := range commands {
		got := lookupCommand(c.Name)
		if got == nil {
			t.Errorf("lookupCommand(%q) = nil", c.Name)
			continue
		}
		if got.Name != c.Name {
			t.Errorf("lookupCommand(%q).Name = %q, want %q", c.Name, got.Name, c.Name)
		}
		for _, alias := range c.Aliases {
			aliasGot := lookupCommand(alias)
			if aliasGot == nil {
				t.Errorf("lookupCommand(%q) = nil for alias of %q", alias, c.Name)
				continue
			}
			if aliasGot.Name != c.Name {
				t.Errorf("alias %q routes to %q, want %q",
					alias, aliasGot.Name, c.Name)
			}
		}
	}
}

// TestRegistry_LookupUnknownReturnsNil — sanity check the negative path.
func TestRegistry_LookupUnknownReturnsNil(t *testing.T) {
	if got := lookupCommand("definitely-not-a-real-subcommand-xyz"); got != nil {
		t.Errorf("lookupCommand(unknown) = %+v, want nil", got)
	}
}

// TestRegistry_VersionAndHelpAliases pins the well-known short flags.
// Regression guard: dropping --version or -h would silently break
// user muscle memory.
func TestRegistry_VersionAndHelpAliases(t *testing.T) {
	cases := []struct {
		alias string
		want  string
	}{
		{"--version", "version"},
		{"-v", "version"},
		{"--help", "help"},
		{"-h", "help"},
	}
	for _, tc := range cases {
		if c := lookupCommand(tc.alias); c == nil || c.Name != tc.want {
			t.Errorf("alias %q should resolve to %q, got %+v", tc.alias, tc.want, c)
		}
	}
}

// TestRegistry_VersionRun emits something and returns 0.
func TestRegistry_VersionRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runVersion(nil, nil, &stdout, &stderr)
	if rc != 0 {
		t.Errorf("runVersion rc=%d, want 0", rc)
	}
	if stdout.Len() == 0 {
		t.Errorf("runVersion stdout empty")
	}
}

// TestRegistry_HelpRun prints the usage banner.
func TestRegistry_HelpRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runHelp(nil, nil, &stdout, &stderr)
	if rc != 0 {
		t.Errorf("runHelp rc=%d, want 0", rc)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("evolve — autonomous improvement loop")) {
		t.Errorf("runHelp output missing usage banner: %s", stdout.String())
	}
}

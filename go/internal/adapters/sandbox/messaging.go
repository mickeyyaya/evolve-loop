package sandbox

import (
	"fmt"
	"strings"
)

type MessagingScope struct {
	Tree        string
	Dispatch    string
	Out         string
	StorePrefix string
	StoreDirs   []string
}

func writeMessagingRules(b *strings.Builder, m *MessagingScope) {
	if m == nil {
		return
	}
	fmt.Fprintf(b, "(deny file-read* (subpath %q))\n", m.Tree)
	fmt.Fprintf(b, "(allow file-read* (subpath %q))\n", m.Dispatch)
	fmt.Fprintf(b, "(deny file-write* (subpath %q))\n", m.Tree)
	fmt.Fprintf(b, "(allow file-write* (subpath %q))\n", m.Out)
	fmt.Fprintf(b, "(deny file-write* (prefix %q))\n", m.StorePrefix)
	fmt.Fprintf(b, "(deny file-read* (prefix %q))\n", m.StorePrefix)
}

func bwrapMessagingMasks(m *MessagingScope) []string {
	if m == nil {
		return nil
	}
	out := []string{
		"--tmpfs", m.Tree,
		"--ro-bind-try", m.Dispatch, m.Dispatch,
		"--bind-try", m.Out, m.Out,
	}
	for _, dir := range m.StoreDirs {
		out = append(out, "--tmpfs", dir)
	}
	return out
}

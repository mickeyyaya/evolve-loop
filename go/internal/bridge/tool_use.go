package bridge

func HasToolUse(cli string) bool {
	m, err := LoadManifest(cli)
	return err == nil && !m.Toolless
}

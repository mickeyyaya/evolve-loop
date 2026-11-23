package sandbox

func fullFixtureConfig() Config {
	return Config{
		TerminalPath:    "/dev/ttys042",
		RepoRoot:        "/nonexistent/repo",
		HomeDir:         "/nonexistent/home",
		ReadOnlyRepo:    true,
		AllowNetwork:    true,
		WritePaths:      []string{"/nonexistent/repo/wt", "/nonexistent/ws", "/tmp"},
		DenyPaths:       []string{"/nonexistent/deny"},
		DenyWritePaths:  []string{"/nonexistent/denywrite"},
		DenyReadPaths:   []string{"/nonexistent/denyread"},
		DenySockets:     []string{"/nonexistent/sock"},
		DenyLiterals:    []string{"/nonexistent/lit"},
		MaskFiles:       []string{"/nonexistent/mask"},
		ReadOnlyFiles:   []string{"/nonexistent/ro"},
		ExecDenyPaths:   []string{"/nonexistent/bin/x"},
		ExecDenyRoots:   []string{"/nonexistent/binroot"},
		ExecDenyEnforce: true,
		ReadIsolation: ReadIsolation{
			Enforce: true, Tag: "t1", RunsDir: "/nonexistent/repo/.evolve/runs", Workspace: "/nonexistent/ws",
			RawDirs: []string{"/nonexistent/raw"}, SecretDirs: []string{"/nonexistent/secrets"},
			PinnedFiles: []string{"/nonexistent/pin"},
		},
		ProtectedWriteSubpaths: []string{"/nonexistent/repo/.evolve/events"},
		ProtectedWritePrefixes: []string{"/nonexistent/repo/.evolve/state"},
		ProtectedWriteEnforce:  true,
	}
}

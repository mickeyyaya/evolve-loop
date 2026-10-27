package flagregistry

var All = []Flag{
	{Name: "EVOLVE_CLI", Status: StatusInternal, Doc: "Undocumented production reader (inventory 2026-06-11); classify when touched."},
	{Name: "EVOLVE_PLUGIN_ROOT", Status: StatusActive, Cluster: "Core Infrastructure (never consolidate)", Doc: "Read-only plugin install location"},
	{Name: "EVOLVE_PROJECT_ROOT", Status: StatusActive, Cluster: "Core Infrastructure (never consolidate)", Doc: "Writable project directory (dual-root pattern)"},
	{Name: "EVOLVE_TMUX_SOCKET", Status: StatusInternal, Cluster: "Bridge Cluster", Doc: "IPC channel (F6, not an operator dial): the loop exports the per-run bridge tmux socket name (evolve-bridge-p<looppid>) so its bridge subprocesses, the reaper, and the orphan GC all target the same isolated socket — so an external `tmux kill-server` on one run's socket can't tear down another. Unset ⇒ the shared `evolve-bridge` default. Set by the loop (cmd_loop) and, for a test process that runs real tmux, by `tmuxtest.Main` (`evolve-bridge-t<testpid>`, a server of its own); never by the operator."},
}

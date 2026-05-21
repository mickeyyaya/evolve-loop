#!/usr/bin/env bash
# drivers/agy.sh — v2 placeholder for Antigravity. Antigravity's async task
# management may not expose a blocking "REPL ready" signal — tmux-drivability
# is partial; bridge defers until the surface is verified.
drv_launch_agy() {
  echo "[agy] v1 stub — agy driver not implemented (see plan §2)" >&2
  return $EC_REQUIRE_FULL_UNMET
}

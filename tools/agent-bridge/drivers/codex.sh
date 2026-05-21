#!/usr/bin/env bash
# drivers/codex.sh — v2 placeholder. The codex CLI is intentionally
# unsupported in v1 (its REPL prompt-marker isn't documented; `codex exec`
# stdin mode bypasses bridge's value-add). Returns 99 so callers can
# decide to skip or escalate.
drv_launch_codex() {
  echo "[codex] v1 stub — codex driver not implemented (see plan §2)" >&2
  return $EC_REQUIRE_FULL_UNMET
}

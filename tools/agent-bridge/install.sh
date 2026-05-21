#!/usr/bin/env bash
# install.sh — symlink bin/bridge into $HOME/.local/bin/
#
# Usage:
#   bash install.sh              # install
#   bash install.sh --uninstall  # remove the symlink
#   bash install.sh --dry-run    # print actions, do not touch FS
#   bash install.sh --check      # verify install is functional (for CI / docs)
#
# Exit codes:
#   0  success / install OK / check passed
#   1  --uninstall succeeded
#   10 bad flags
#   20 source binary missing (bin/bridge not found)
#   21 target dir not writable
#   22 --check failed (see stderr for details)

set -uo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SRC="${SCRIPT_DIR}/bin/bridge"
readonly TARGET_DIR="${BRIDGE_INSTALL_DIR:-${HOME}/.local/bin}"
readonly TARGET="${TARGET_DIR}/bridge"

mode="install"
dry_run=0

for arg in "$@"; do
  case "$arg" in
    --uninstall) mode="uninstall" ;;
    --dry-run)   dry_run=1 ;;
    --check)     mode="check" ;;
    -h|--help)
      sed -n '2,16p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "[install] unknown arg: $arg" >&2
      exit 10
      ;;
  esac
done

run() {
  if [[ $dry_run -eq 1 ]]; then
    echo "[install:dry-run] $*"
  else
    "$@"
  fi
}

case "$mode" in
  install)
    if [[ ! -f "$SRC" ]]; then
      echo "[install] FATAL: $SRC not found (run from sub-project root)" >&2
      exit 20
    fi
    run mkdir -p "$TARGET_DIR"
    if [[ ! -w "$TARGET_DIR" ]] && [[ $dry_run -eq 0 ]]; then
      echo "[install] FATAL: $TARGET_DIR not writable" >&2
      exit 21
    fi
    run chmod +x "$SRC"
    if [[ -L "$TARGET" || -e "$TARGET" ]]; then
      run rm -f "$TARGET"
    fi
    run ln -s "$SRC" "$TARGET"
    echo "[install] linked $TARGET -> $SRC"
    case ":${PATH}:" in
      *":${TARGET_DIR}:"*) ;;
      *) echo "[install] note: $TARGET_DIR not on PATH — add it to use 'bridge' without full path" ;;
    esac
    ;;
  uninstall)
    if [[ -L "$TARGET" ]]; then
      run rm -f "$TARGET"
      echo "[install] removed $TARGET"
      exit 1
    fi
    echo "[install] nothing to remove ($TARGET not a symlink)"
    ;;
  check)
    # Verify the install is functional. Exit 0 if healthy; non-zero with
    # diagnostic message otherwise. Useful for evolve-loop\'s integration
    # docs to recommend ("run `bash install.sh --check` to verify").
    ok=1
    if [[ ! -L "$TARGET" ]]; then
      echo "[install:check] FAIL: $TARGET is not a symlink (run \`bash install.sh\`)" >&2
      ok=0
    elif [[ "$(readlink "$TARGET")" != "$SRC" ]]; then
      echo "[install:check] FAIL: $TARGET points to $(readlink "$TARGET") (expected $SRC)" >&2
      ok=0
    fi
    if ! command -v bridge >/dev/null 2>&1; then
      echo "[install:check] FAIL: 'bridge' not on PATH (is $TARGET_DIR in your PATH?)" >&2
      ok=0
    fi
    if ! version_str=$(bridge --json version 2>/dev/null); then
      echo "[install:check] FAIL: 'bridge --json version' did not run cleanly" >&2
      ok=0
    elif ! schema=$(echo "$version_str" | command -v jq >/dev/null && echo "$version_str" | jq -r '.schema_version' 2>/dev/null); then
      echo "[install:check] WARN: jq not available; can't verify schema_version" >&2
    elif [[ "$schema" != "1" ]]; then
      echo "[install:check] WARN: schema_version=$schema (this evolve-loop expects 1)" >&2
    fi
    if [[ $ok -eq 1 ]]; then
      echo "[install:check] OK: bridge installed at $TARGET (schema_version=${schema:-?})"
      exit 0
    fi
    exit 22
    ;;
esac

exit 0

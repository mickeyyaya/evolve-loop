#!/usr/bin/env bash
# install.sh — symlink bin/bridge into $HOME/.local/bin/
#
# Usage:
#   bash install.sh              # install
#   bash install.sh --uninstall  # remove the symlink
#   bash install.sh --dry-run    # print actions, do not touch FS
#
# Exit codes:
#   0  success
#   1  --uninstall succeeded
#   10 bad flags
#   20 source binary missing (bin/bridge not found)
#   21 target dir not writable

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
    -h|--help)
      sed -n '2,12p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
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
esac

exit 0

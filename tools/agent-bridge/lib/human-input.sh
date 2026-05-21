#!/usr/bin/env bash
# lib/human-input.sh — behavioral-plausibility layer for tmux drivers
#
# Default behavior of bridge tmux drivers: paste prompt instantly via
# tmux paste-buffer; respond to auto-respond prompts via instant send-keys.
# Both are "robot-shaped" — zero inter-event delay, no review pauses.
#
# This module adds Gaussian-distributed timing variance + reading-pauses +
# review-pauses so a CLI watching the input stream sees a behaviorally-
# plausible interaction pattern.
#
# IMPORTANT: this is opt-in. Two gates must both be true to activate:
#   1. BRIDGE_HUMAN_SIMULATION=1 in env (host-level allowlist)
#   2. --human-input flag on `bridge launch` (per-invocation explicit choice)
#
# Default for both: OFF. Existing paste-buffer paths remain unchanged.
#
# Why two gates: the host-level env var means the operator has read
# docs/design.md §8 and accepted the per-CLI ToS responsibility.
# The per-invocation flag means each launch makes the choice explicit.
#
# See docs/design.md §8 for the policy framing. See arxiv.org/pdf/2601.17280
# for the published research showing sampled-from-human-distribution
# inter-keystroke intervals can defeat keystroke-based automation detection
# with >99.8% evasion rate. (Cited as background; bridge's goal here is
# behavioral plausibility for futureproofing, not active circumvention of
# a currently-deployed detector.)
#
# Tuning knobs (env vars; defaults reasonable for ~80wpm typist):
#   BRIDGE_HUMAN_KEY_MS         mean ms between keystrokes      (default 65)
#   BRIDGE_HUMAN_KEY_SD_MS      stddev of inter-key delay       (default 20)
#   BRIDGE_HUMAN_SPACE_MS       mean pause on space             (default 130)
#   BRIDGE_HUMAN_SENT_MS        mean pause after .?!            (default 450)
#   BRIDGE_HUMAN_BOOT_MS        boot pause range (uniform)      (default 1500-3500)
#   BRIDGE_HUMAN_REVIEW_MS_PER_LINE  ms per line of pasted text (default 80)
#   BRIDGE_HUMAN_READING_WPM    words/minute when "reading"     (default 220)

# ---- Gate ------------------------------------------------------------------

# bridge_human_active — emits 1 if human-simulation is opted in this run, 0 otherwise.
# Bridge drivers gate every human_* call via `[[ "$(bridge_human_active)" == "1" ]]`.
bridge_human_active() {
  if [[ "${BRIDGE_HUMAN_SIMULATION:-0}" == "1" ]] && [[ "${human_input:-0}" -eq 1 ]]; then
    echo "1"
  else
    echo "0"
  fi
}

# ---- Sampling primitives ---------------------------------------------------

# Sample a value from a truncated Gaussian distribution.
# Args: mean, stddev, [min=10, max=10*mean]
_human_sample() {
  local mean="$1" sd="$2" min="${3:-10}" max="${4:-}"
  python3 -c "
import random, sys
mean, sd = float(sys.argv[1]), float(sys.argv[2])
mn = float(sys.argv[3])
mx = float(sys.argv[4]) if sys.argv[4] else mean * 10
v = random.gauss(mean, sd)
v = max(mn, min(mx, v))
print(int(v))
" "$mean" "$sd" "$min" "$max"
}

# Sleep a Gaussian-sampled number of milliseconds.
_human_sleep_ms() {
  local mean="$1" sd="$2"
  local ms; ms=$(_human_sample "$mean" "$sd" 10)
  local sec; sec=$(awk -v ms="$ms" 'BEGIN { printf "%.3f", ms/1000 }')
  sleep "$sec"
}

# ---- Public API ------------------------------------------------------------

# Boot pause — humans take 1-3s to react to a new REPL becoming ready.
# Used after the prompt-detect loop, before delivering the prompt.
human_boot_pause() {
  local lo="${BRIDGE_HUMAN_BOOT_MIN_MS:-1500}"
  local hi="${BRIDGE_HUMAN_BOOT_MAX_MS:-3500}"
  local ms; ms=$(python3 -c "import random,sys; print(int(random.uniform(float(sys.argv[1]), float(sys.argv[2]))))" "$lo" "$hi")
  local sec; sec=$(awk -v ms="$ms" 'BEGIN { printf "%.3f", ms/1000 }')
  echo "[human-input] boot pause ${ms}ms" >&2
  sleep "$sec"
}

# Paste-with-review: paste-buffer is fine (humans paste too), but follow
# with a human-shaped pause proportional to prompt length, before Enter.
# Args: session, prompt_file
human_paste_with_review() {
  local session="$1"
  local prompt_file="$2"
  local lines; lines=$(wc -l < "$prompt_file" | tr -d ' ')

  tmux load-buffer -t "$session" "$prompt_file"
  tmux paste-buffer -t "$session"

  # Review pause: ~80ms per line (humans glance at long pasted prompts)
  local per_line="${BRIDGE_HUMAN_REVIEW_MS_PER_LINE:-80}"
  local mean=$((per_line * lines))
  [[ $mean -lt 200 ]] && mean=200
  local sd=$((mean / 4))
  echo "[human-input] paste review ~${mean}ms (${lines} lines)" >&2
  _human_sleep_ms "$mean" "$sd"

  tmux send-keys -t "$session" Enter
}

# Type a short string into the session character-by-character with
# Gaussian inter-keystroke delays. Use ONLY for short responses (e.g.
# auto-respond keystrokes like "1,Enter" or "y,Enter").
# Args: session, text
human_send_text() {
  local session="$1"
  local text="$2"
  local mean_ms="${BRIDGE_HUMAN_KEY_MS:-65}"
  local sd_ms="${BRIDGE_HUMAN_KEY_SD_MS:-20}"
  local space_ms="${BRIDGE_HUMAN_SPACE_MS:-130}"
  local sent_ms="${BRIDGE_HUMAN_SENT_MS:-450}"

  local i=0
  local len=${#text}
  while [ $i -lt $len ]; do
    local c="${text:$i:1}"
    case "$c" in
      ' ')   tmux send-keys -t "$session" Space ;;
      $'\n') tmux send-keys -t "$session" Enter ;;
      $'\t') tmux send-keys -t "$session" Tab ;;
      *)     tmux send-keys -t "$session" -l "$c" ;;
    esac

    local pause_mean="$mean_ms" pause_sd="$sd_ms"
    case "$c" in
      '.'|'?'|'!') pause_mean="$sent_ms"; pause_sd=$((sent_ms / 4)) ;;
      ' ')         pause_mean="$space_ms"; pause_sd=$((space_ms / 4)) ;;
    esac
    _human_sleep_ms "$pause_mean" "$pause_sd"
    i=$((i+1))
  done
}

# Send a list of named tmux keys (CSV: "1,Enter" or "y,Enter") with
# human-shaped inter-key delays. This is the human equivalent of the
# default auto_respond_tick's bulk send-keys.
# Args: session, keys_csv
human_send_keys_csv() {
  local session="$1"
  local keys_csv="$2"
  local mean_ms="${BRIDGE_HUMAN_KEY_MS:-65}"
  local sd_ms="${BRIDGE_HUMAN_KEY_SD_MS:-20}"

  local saved_ifs="$IFS"
  IFS=','
  local key_arr=()
  read -ra key_arr <<<"$keys_csv"
  IFS="$saved_ifs"

  local k
  for k in "${key_arr[@]}"; do
    tmux send-keys -t "$session" "$k"
    _human_sleep_ms "$mean_ms" "$sd_ms"
  done
}

# Reading-time pause based on captured CLI output (word-count proportional).
# Used between detecting a tool-use prompt and replying — simulates the
# human reading the prompt before responding.
# Args: text_to_count
human_reading_pause() {
  local text="$1"
  local wpm="${BRIDGE_HUMAN_READING_WPM:-220}"
  local words; words=$(echo "$text" | wc -w | tr -d ' ')
  [[ $words -lt 3 ]] && words=3
  # ms = (words / wpm) * 60000
  local ms; ms=$(python3 -c "print(int(60000 * $words / $wpm))")
  local sd=$((ms / 4))
  [[ $sd -lt 100 ]] && sd=100
  echo "[human-input] reading pause ~${ms}ms (~${words} words at ${wpm}wpm)" >&2
  _human_sleep_ms "$ms" "$sd"
}

# Standalone debug
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  echo "lib/human-input.sh loaded. Functions exposed:"
  declare -F | awk '/bridge_human_active|^_human_|human_/{print "  " $3}'
  echo ""
  echo "Quick demo (no tmux required):"
  echo "  Sampling 5 inter-keystroke delays (mean 65ms, sd 20ms):"
  for i in 1 2 3 4 5; do printf "    %dms\n" "$(_human_sample 65 20 10)"; done
  echo "  Sampling 5 reading-pauses (220wpm, 100 words):"
  for i in 1 2 3 4 5; do printf "    %dms\n" "$(_human_sample 27272 6818 5000)"; done
fi

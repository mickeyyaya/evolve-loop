# internal/bridge/keyspec

## Purpose

`internal/bridge/keyspec` classifies the tokens of a `keystroke` envelope, the operator's hatch for sending any key event to a tmux pane. It flags tokens that look like a mistyped key name, such as `Excape` or `Etner`, before they are sent verbatim and silently do nothing.

## Design

- **Non-blocking.** `Validate` only flags. The caller warns on the returned tokens and still sends the body, so a genuine modal-dismissal keystroke is never refused.
- **Literals are always valid.** `tmux send-keys` accepts arbitrary literal text. The value added here is catching typos.
- **Vocabulary mirrors `tmux send-keys`.** Named keys (Enter, Escape, Tab, arrows, PgUp/PgDn, Home/End, F1 to F12, and the aliases tmux accepts) are matched case-insensitively. Modifier prefixes `C-`, `M-` and `S-` may stack.
- **A modifier with a key remainder is valid.** `C-c` (a single character) and `C-Enter` (a named key) are modifiers. `C-Excape`, a multi-character non-named remainder, is suspect.
- **The suspect heuristic.** A token that starts with an uppercase letter and is otherwise letters and digits, but is not a recognized key, is suspect. Anything else is literal text.

## Invariants

- **Only suspect tokens are flagged.** `TestValidate` pins that a clean body yields no findings and that typos are returned in order.
- **Classification is total.** `TestClassify` pins each class, including the empty token, which is a literal.

## Findings

- None.

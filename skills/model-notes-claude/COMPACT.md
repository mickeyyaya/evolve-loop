# Model notes: claude family

These notes load only into a dispatch whose CLI is `claude`, `claude-tmux` or `claude-p`.

- State the goal, the done criteria and the limits in the first lines. Claude models follow explicit scope closely.
- Give the reason with each rule. A rule with a reason generalizes better than a bare order.
- Use one clear instruction for each step. Do not stack contradictory rules.
- Run independent tool calls in one turn.
- Verify with a command before you claim done, and report only what the command printed.

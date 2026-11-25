# Model notes: claude family

These notes load only into a dispatch whose CLI is `claude`, `claude-tmux` or `claude-p`.

- State the goal, the done criteria and the limits in the first lines. Claude models follow explicit scope closely.
- Give the reason with each rule. A rule with a reason generalizes better than a bare order.
- Use one clear instruction for each step. Do not stack contradictory rules.
- Run independent tool calls in one turn.
- Verify with a command before you claim done, and report only what the command printed. Run the named check command, not a general review of your work. Opus 5 over-verifies on a general line. Sonnet 5.5 and Haiku 5.5 at low effort can skip it.
- In an unattended phase, continue until the done criteria of the persona pass. Sonnet 5.5 and Haiku 5.5 at low effort can stop early without this line.
- Claude Fable 5.2 and Claude Mythos 5.2 return a 400 error when you edit the system prompt, the tools or an earlier message. This holds when a thinking block follows the edit. Send the history back unchanged.
- On Claude Fable 5.2 and Claude Mythos 5.2, thinking is always on. An order to answer without thinking does not stop it at medium effort and above. Lower the effort level instead.
- Do not set a non-default temperature, top_p or top_k on Claude Opus 4.6, Opus 4.5, Sonnet 4.5 or Haiku 4.5. It can remove the text watermark.
- Claude Fable 5.2 reads an order with "never" as absolute. Write the scope and the reason with each limit, or name the guard that enforces it.

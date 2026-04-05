---
title: tmux send-keys requires separate Enter with delay
category: gotcha
status: active
date: 2026-03-28
tags: [tmux, send-keys, notification]
---

# tmux send-keys requires separate Enter with delay

## Symptom
Messages sent to agent panes via `tmux send-keys -l "text" Enter` were not being submitted. The text appeared in the pane but the agent didn't process it.

## Root Cause
When sending text and Enter in one `send-keys` call, the Enter can be swallowed or arrive before the text is fully buffered, especially with interactive CLI tools. The agent CLI needs a moment to process the pasted text before receiving Enter.

## Fix / Workaround
Send Enter as a separate `send-keys` call with a 0.1s delay:

```bash
tmux send-keys -t %5 -l "message text"
sleep 0.1
tmux send-keys -t %5 Enter
```

In Go code, this means two separate `SendKeys` calls with `time.Sleep(100 * time.Millisecond)` between them.

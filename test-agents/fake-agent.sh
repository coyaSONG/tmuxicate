#!/usr/bin/env bash
# Fake agent for testing tmuxicate
set -euo pipefail

AGENT="${TMUXICATE_AGENT:-unknown}"
echo "[$AGENT] Ready. Waiting for instructions..."
echo "READY>"

while IFS= read -r line; do
  echo "[$AGENT] Received: $line"
  if [[ "$line" == *"tmuxicate read"* ]]; then
    # Extract message ID and run the command
    msg_id=$(echo "$line" | grep -oE 'msg_[0-9]+' || true)
    if [[ -n "$msg_id" ]]; then
      echo "[$AGENT] Reading message $msg_id..."
      tmuxicate read "$msg_id" 2>&1 || true
    fi
  fi
  echo "READY>"
done

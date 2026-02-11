#!/bin/bash
set -e

# Install Claude Code OAuth credentials from a token file.
#
# Mount your .secrets directory and set CLAUDE_TOKEN_FILE
# to the filename you want to use:
#
#   docker run \
#     -v ./.secrets:/secrets:ro \
#     -e CLAUDE_TOKEN_FILE=claude.json \
#     -v $(pwd):/workspace \
#     crumbs
#
# The entrypoint copies the token file to ~/.claude/.credentials.json
# where Claude Code reads it on Linux.

TOKENS_DIR="/secrets"

if [ -n "$CLAUDE_TOKEN_FILE" ] && [ -f "$TOKENS_DIR/$CLAUDE_TOKEN_FILE" ]; then
    cp "$TOKENS_DIR/$CLAUDE_TOKEN_FILE" /root/.claude/.credentials.json
    echo "Loaded credentials from $CLAUDE_TOKEN_FILE"
elif [ -f "$TOKENS_DIR/claude.json" ]; then
    cp "$TOKENS_DIR/claude.json" /root/.claude/.credentials.json
    echo "Loaded credentials from claude.json (default)"
fi

exec "$@"

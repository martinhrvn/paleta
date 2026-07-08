#!/usr/bin/env bash
# Tests for plt-core.sh select/record passthrough and plt-integration.zsh fixes

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

# Create shared temp dir
TMPDIR_TEST=$(mktemp -d)
trap "rm -rf $TMPDIR_TEST" EXIT

# Create a fake binary that records what it was called with
cat > "$TMPDIR_TEST/fake-plt" << 'FAKE'
#!/usr/bin/env bash
echo "CALLED_WITH: $@"
FAKE
chmod +x "$TMPDIR_TEST/fake-plt"

echo "=== Testing plt-core.sh ==="

# Test 1: plt_main routes "select" to the binary
echo "Test 1: plt_main passes 'select' through to PLT_BINARY"
output=$(PLT_BINARY="$TMPDIR_TEST/fake-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_main select" 2>&1) || true
if [[ "$output" == *"CALLED_WITH: select"* ]]; then
    pass "select routes to binary"
else
    fail "select did not route to binary, got: $output"
fi

# Test 2: plt_main routes "record" with args to the binary
echo "Test 2: plt_main passes 'record' through to PLT_BINARY"
output=$(PLT_BINARY="$TMPDIR_TEST/fake-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_main record myname mycmd" 2>&1) || true
if [[ "$output" == *"CALLED_WITH: record myname mycmd"* ]]; then
    pass "record routes to binary with args"
else
    fail "record did not route to binary with args, got: $output"
fi

# Test 3: plt_main still rejects unknown commands
echo "Test 3: plt_main rejects unknown commands"
output=$(PLT_BINARY="$TMPDIR_TEST/fake-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_main boguscmd" 2>&1) || true
if [[ "$output" == *"Unknown command"* ]]; then
    pass "unknown commands still rejected"
else
    fail "expected 'Unknown command' error, got: $output"
fi

echo ""
echo "=== Testing plt-core.sh pane / multiplexer dispatch ==="

# Fake tmux/zellij that record their invocation so we can assert what was called.
# Named exactly 'tmux'/'zellij' and placed first on PATH so they shadow the real
# binaries during the test.
cat > "$TMPDIR_TEST/tmux" << 'FAKE'
#!/usr/bin/env bash
echo "TMUX_CALLED: $*" >> "$MUX_LOG"
FAKE
cat > "$TMPDIR_TEST/zellij" << 'FAKE'
#!/usr/bin/env bash
echo "ZELLIJ_CALLED: $*" >> "$MUX_LOG"
FAKE
chmod +x "$TMPDIR_TEST/tmux" "$TMPDIR_TEST/zellij"

# Test 7: a single "pane" selection opens a new tmux window with the runline
echo "Test 7: pane action opens a new tmux window"
MUX_LOG="$TMPDIR_TEST/mux7.log"
: > "$MUX_LOG"
single_json='{"directory":"web","command":"npm run dev","display_name":"web","action":"pane"}'
PATH="$TMPDIR_TEST:$PATH" MUX_LOG="$MUX_LOG" TMUX="fake-tmux-session" ZELLIJ="" \
  JQ_CMD=jq PLT_BINARY="$TMPDIR_TEST/fake-plt" \
  bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_run_selection_in_pane '$single_json' '{'" >/dev/null 2>&1 || true
if grep -q "TMUX_CALLED: new-window" "$MUX_LOG" && \
   grep -q "cd 'web' && npm run dev" "$MUX_LOG"; then
    pass "pane action invokes tmux new-window with the runline"
else
    fail "expected tmux new-window with runline, got: $(cat "$MUX_LOG")"
fi

# Test 8: zellij takes precedence and gets a `run` invocation
echo "Test 8: pane action uses zellij run when in zellij"
MUX_LOG="$TMPDIR_TEST/mux8.log"
: > "$MUX_LOG"
PATH="$TMPDIR_TEST:$PATH" MUX_LOG="$MUX_LOG" ZELLIJ="0" TMUX="" \
  JQ_CMD=jq PLT_BINARY="$TMPDIR_TEST/fake-plt" \
  bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_run_selection_in_pane '$single_json' '{'" >/dev/null 2>&1 || true
if grep -q "ZELLIJ_CALLED: run" "$MUX_LOG"; then
    pass "pane action invokes zellij run"
else
    fail "expected zellij run, got: $(cat "$MUX_LOG")"
fi

# Test 9: a multi-select pane selection joins segments with &&
echo "Test 9: multi-select pane joins segments with &&"
MUX_LOG="$TMPDIR_TEST/mux9.log"
: > "$MUX_LOG"
multi_json='[{"directory":"a","command":"make build","display_name":"a","action":"pane"},{"directory":"b","command":"make test","display_name":"b","action":"pane"}]'
PATH="$TMPDIR_TEST:$PATH" MUX_LOG="$MUX_LOG" TMUX="fake" ZELLIJ="" \
  JQ_CMD=jq PLT_BINARY="$TMPDIR_TEST/fake-plt" \
  bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_run_selection_in_pane '$multi_json' '['" >/dev/null 2>&1 || true
if grep -q "cd 'a' && make build && cd 'b' && make test" "$MUX_LOG"; then
    pass "multi-select pane joins segments with &&"
else
    fail "expected joined compound runline, got: $(cat "$MUX_LOG")"
fi

# Test 10: no multiplexer -> clear error, no mux invocation
echo "Test 10: pane action errors when no multiplexer is present"
MUX_LOG="$TMPDIR_TEST/mux10.log"
: > "$MUX_LOG"
output=$(PATH="$TMPDIR_TEST:$PATH" MUX_LOG="$MUX_LOG" TMUX="" ZELLIJ="" \
  JQ_CMD=jq PLT_BINARY="$TMPDIR_TEST/fake-plt" \
  bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_run_selection_in_pane '$single_json' '{'" 2>&1) || true
if [[ "$output" == *"No tmux or zellij"* ]] && [[ ! -s "$MUX_LOG" ]]; then
    pass "pane action errors cleanly with no multiplexer"
else
    fail "expected a no-multiplexer error and no mux call, got: $output / $(cat "$MUX_LOG")"
fi

echo ""
echo "=== Testing plt-integration.zsh (static analysis) ==="

# Test 4: local plt_binary is split from assignment (fixes $? bug)
echo "Test 4: local plt_binary is split from assignment"
if grep -q 'local plt_binary$' "$SCRIPT_DIR/plt-integration.zsh" && \
   grep -q 'plt_binary=$(__plt_find_binary)' "$SCRIPT_DIR/plt-integration.zsh"; then
    pass "local declaration is separate from assignment"
else
    fail "local plt_binary should be declared separately from __plt_find_binary call"
fi

# Test 5: __plt_find_binary includes ${commands[plt]} as a candidate
echo "Test 5: __plt_find_binary includes \${commands[plt]}"
if grep -q '${commands\[plt\]}' "$SCRIPT_DIR/plt-integration.zsh"; then
    pass "\${commands[plt]} is in the candidate list"
else
    fail "\${commands[plt]} not found in candidate list"
fi

# Test 6: Verify the old broken pattern is gone
echo "Test 6: No broken 'local plt_binary=\$(...)' pattern"
if grep -q 'local plt_binary=\$(' "$SCRIPT_DIR/plt-integration.zsh"; then
    fail "found broken pattern: local plt_binary=\$(...)"
else
    pass "broken pattern not present"
fi

echo ""
echo "=== Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi

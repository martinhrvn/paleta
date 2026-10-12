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
echo "=== Testing plt-integration.zsh (behaviour) ==="

# Test 11: a multi-select runline carries each directory and command. A local
# named `path` would shadow zsh's special $path (tied to $PATH), hiding jq and
# turning every segment into "cd '' && ".
echo "Test 11: zsh multi-select runline keeps directories and commands"
if command -v zsh >/dev/null 2>&1; then
    output=$(zsh -f -c "source '$SCRIPT_DIR/plt-integration.zsh' && __plt_selection_runline true jq '$multi_json' '['" 2>&1) || true
    if [[ "$output" == "cd 'a' && make build && cd 'b' && make test" ]]; then
        pass "multi-select runline is built from the selection"
    else
        fail "expected \"cd 'a' && make build && cd 'b' && make test\", got: $output"
    fi
else
    echo "  SKIP: zsh not installed"
fi


echo ""
echo "=== Testing shell history capture and temporary commands ==="

# A fake binary that appends its arguments to $REC_LOG, so backgrounded calls
# can be observed after the fact.
cat > "$TMPDIR_TEST/rec-plt" << 'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$REC_LOG"
FAKE
chmod +x "$TMPDIR_TEST/rec-plt"

PROJ="$TMPDIR_TEST/proj"
mkdir -p "$PROJ/infra"
touch "$PROJ/.pltrc"
OUTSIDE=$(mktemp -d)
trap "rm -rf $TMPDIR_TEST $OUTSIDE" EXIT

# wait_for_log FILE: the hook records in the background; give it up to 1s.
wait_for_log() {
    local i=0
    while [[ $i -lt 20 ]]; do
        [[ -s "$1" ]] && return 0
        sleep 0.05
        i=$((i + 1))
    done
    return 1
}

# run_hook DIR CMD...: source the integration in zsh, cd to DIR, and call the
# preexec hook once per CMD; prints nothing, the fake binary logs to $REC_LOG.
run_hook() {
    local dir="$1"; shift
    local calls="" c
    for c in "$@"; do
        calls="$calls __plt_shell_record_preexec ${c@Q};"
    done
    zsh -f -c "PLT_BINARY='$TMPDIR_TEST/rec-plt'; source '$SCRIPT_DIR/plt-integration.zsh' && cd '$dir' && $calls" 2>&1
}

if command -v zsh >/dev/null 2>&1; then
    # Test 12
    echo "Test 12: preexec hook records cwd and command inside a project"
    export REC_LOG="$TMPDIR_TEST/rec12.log"; : > "$REC_LOG"
    run_hook "$PROJ/infra" "pnpm cdk diff FooStack"
    if wait_for_log "$REC_LOG" && [[ "$(cat "$REC_LOG")" == "shell-record $PROJ/infra pnpm cdk diff FooStack" ]]; then
        pass "command recorded with its directory"
    else
        fail "expected 'shell-record $PROJ/infra pnpm cdk diff FooStack', got: $(cat "$REC_LOG")"
    fi

    # Test 13
    echo "Test 13: preexec hook ignores commands outside a project, plt itself, and blanks"
    export REC_LOG="$TMPDIR_TEST/rec13.log"; : > "$REC_LOG"
    run_hook "$OUTSIDE" "make build"
    run_hook "$PROJ" "plt list" "plt" "   "
    sleep 0.3
    if [[ ! -s "$REC_LOG" ]]; then
        pass "nothing recorded"
    else
        fail "expected no records, got: $(cat "$REC_LOG")"
    fi

    # Test 14
    echo "Test 14: the skip flag suppresses exactly one command (plt's own run)"
    export REC_LOG="$TMPDIR_TEST/rec14.log"; : > "$REC_LOG"
    zsh -f -c "PLT_BINARY='$TMPDIR_TEST/rec-plt'; source '$SCRIPT_DIR/plt-integration.zsh' && cd '$PROJ' && __plt_skip_record=1 && __plt_shell_record_preexec 'make one' && __plt_shell_record_preexec 'make two'" 2>&1
    wait_for_log "$REC_LOG"; sleep 0.1
    if [[ "$(cat "$REC_LOG")" == "shell-record $PROJ make two" ]]; then
        pass "first command skipped, second recorded"
    else
        fail "expected only 'make two', got: $(cat "$REC_LOG")"
    fi

    # Test 15
    echo "Test 15: PLT_SHELL_HISTORY=0 leaves the preexec hook unregistered"
    output=$(zsh -f -c "PLT_SHELL_HISTORY=0 PLT_BINARY='$TMPDIR_TEST/rec-plt'; source '$SCRIPT_DIR/plt-integration.zsh'; print -r -- \${#\${(M)preexec_functions:#__plt_shell_record_preexec}}" 2>&1)
    output_on=$(zsh -f -c "PLT_BINARY='$TMPDIR_TEST/rec-plt'; source '$SCRIPT_DIR/plt-integration.zsh'; source '$SCRIPT_DIR/plt-integration.zsh'; print -r -- \${#\${(M)preexec_functions:#__plt_shell_record_preexec}}" 2>&1)
    if [[ "$output" == "0" && "$output_on" == "1" ]]; then
        pass "opt-out respected; registration is idempotent"
    else
        fail "expected opt-out index 0 and one registration, got '$output' / '$output_on'"
    fi

    # Test 16
    echo "Test 16: zsh __plt_record does not record a 'shell' action"
    export REC_LOG="$TMPDIR_TEST/rec16.log"; : > "$REC_LOG"
    shell_json='{"directory":"/p","command":"make x","display_name":"","action":"shell"}'
    zsh -f -c "PLT_SHELL_HISTORY=0; source '$SCRIPT_DIR/plt-integration.zsh' && __plt_record '$TMPDIR_TEST/rec-plt' jq '$shell_json' ''" 2>&1
    if [[ ! -s "$REC_LOG" ]]; then
        pass "shell action not recorded in history"
    else
        fail "expected no record call, got: $(cat "$REC_LOG")"
    fi
else
    echo "  SKIP: zsh not installed"
fi

# Test 17
echo "Test 17: bash plt_record does not record a 'shell' action"
export REC_LOG="$TMPDIR_TEST/rec17.log"; : > "$REC_LOG"
shell_json='{"directory":"/p","command":"make x","display_name":"","action":"shell"}'
PLT_BINARY="$TMPDIR_TEST/rec-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_record '$shell_json' ''" 2>&1
if [[ ! -s "$REC_LOG" ]]; then
    pass "shell action not recorded in history"
else
    fail "expected no record call, got: $(cat "$REC_LOG")"
fi

# Test 18
echo "Test 18: plt_main passes 'add' and 'shell-record' through to PLT_BINARY"
out_add=$(PLT_BINARY="$TMPDIR_TEST/fake-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_main add pnpm cdk diff" 2>&1) || true
out_rec=$(PLT_BINARY="$TMPDIR_TEST/fake-plt" bash -c "source '$SCRIPT_DIR/plt-core.sh' && plt_main shell-record /p 'go test'" 2>&1) || true
if [[ "$out_add" == *"CALLED_WITH: add pnpm cdk diff"* && "$out_rec" == *"CALLED_WITH: shell-record /p go test"* ]]; then
    pass "add and shell-record route to binary"
else
    fail "got: '$out_add' / '$out_rec'"
fi


# Test 19
echo "Test 19: palette runs set the skip flag, Ctrl+R shell runs don't"
if command -v zsh >/dev/null 2>&1; then
    output=$(zsh -f -c "PLT_SHELL_HISTORY=0; source '$SCRIPT_DIR/plt-integration.zsh' && __plt_mark_own_run execute && print -n \$__plt_skip_record && __plt_mark_own_run shell && print -n \$__plt_skip_record" 2>&1)
    if [[ "$output" == "10" ]]; then
        pass "execute marks, shell does not"
    else
        fail "expected '10', got: $output"
    fi
else
    echo "  SKIP: zsh not installed"
fi

echo ""
echo "=== Results ==="
echo "Passed: $PASS"
echo "Failed: $FAIL"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi

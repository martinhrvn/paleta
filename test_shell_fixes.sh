#!/usr/bin/env bash
# Tests for gopm-core.sh select/record passthrough and gopm-integration.zsh fixes

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

# Create shared temp dir
TMPDIR_TEST=$(mktemp -d)
trap "rm -rf $TMPDIR_TEST" EXIT

# Create a fake binary that records what it was called with
cat > "$TMPDIR_TEST/fake-gopm" << 'FAKE'
#!/usr/bin/env bash
echo "CALLED_WITH: $@"
FAKE
chmod +x "$TMPDIR_TEST/fake-gopm"

echo "=== Testing gopm-core.sh ==="

# Test 1: gopm_main routes "select" to the binary
echo "Test 1: gopm_main passes 'select' through to GOPM_BINARY"
output=$(GOPM_BINARY="$TMPDIR_TEST/fake-gopm" bash -c "source '$SCRIPT_DIR/gopm-core.sh' && gopm_main select" 2>&1) || true
if [[ "$output" == *"CALLED_WITH: select"* ]]; then
    pass "select routes to binary"
else
    fail "select did not route to binary, got: $output"
fi

# Test 2: gopm_main routes "record" with args to the binary
echo "Test 2: gopm_main passes 'record' through to GOPM_BINARY"
output=$(GOPM_BINARY="$TMPDIR_TEST/fake-gopm" bash -c "source '$SCRIPT_DIR/gopm-core.sh' && gopm_main record myname mycmd" 2>&1) || true
if [[ "$output" == *"CALLED_WITH: record myname mycmd"* ]]; then
    pass "record routes to binary with args"
else
    fail "record did not route to binary with args, got: $output"
fi

# Test 3: gopm_main still rejects unknown commands
echo "Test 3: gopm_main rejects unknown commands"
output=$(GOPM_BINARY="$TMPDIR_TEST/fake-gopm" bash -c "source '$SCRIPT_DIR/gopm-core.sh' && gopm_main boguscmd" 2>&1) || true
if [[ "$output" == *"Unknown command"* ]]; then
    pass "unknown commands still rejected"
else
    fail "expected 'Unknown command' error, got: $output"
fi

echo ""
echo "=== Testing gopm-integration.zsh (static analysis) ==="

# Test 4: local gopm_binary is split from assignment (fixes $? bug)
echo "Test 4: local gopm_binary is split from assignment"
if grep -q 'local gopm_binary$' "$SCRIPT_DIR/gopm-integration.zsh" && \
   grep -q 'gopm_binary=$(__gopm_find_binary)' "$SCRIPT_DIR/gopm-integration.zsh"; then
    pass "local declaration is separate from assignment"
else
    fail "local gopm_binary should be declared separately from __gopm_find_binary call"
fi

# Test 5: __gopm_find_binary includes ${commands[gopm]} as a candidate
echo "Test 5: __gopm_find_binary includes \${commands[gopm]}"
if grep -q '${commands\[gopm\]}' "$SCRIPT_DIR/gopm-integration.zsh"; then
    pass "\${commands[gopm]} is in the candidate list"
else
    fail "\${commands[gopm]} not found in candidate list"
fi

# Test 6: Verify the old broken pattern is gone
echo "Test 6: No broken 'local gopm_binary=\$(...)' pattern"
if grep -q 'local gopm_binary=\$(' "$SCRIPT_DIR/gopm-integration.zsh"; then
    fail "found broken pattern: local gopm_binary=\$(...)"
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

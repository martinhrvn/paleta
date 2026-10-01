#!/usr/bin/env bash
# Re-render docs/demo/demo.gif from docs/demo/demo.tape.
# Uses vhs from PATH, falling back to `nix shell nixpkgs#vhs`.
set -euo pipefail

cd "$(dirname "$0")/../.."
go build -o bin/plt-bin ./cmd/plt

if command -v vhs >/dev/null; then
    vhs docs/demo/demo.tape
else
    nix shell nixpkgs#vhs -c vhs docs/demo/demo.tape
fi

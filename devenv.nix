{ pkgs, ... }:

{
  packages = with pkgs; [
    go
    gopls
    delve
    golangci-lint
    gotools
    jq # required at runtime by plt-core.sh
  ];

  languages.go = {
    enable = true;
  };

  scripts.build.exec = "go build -o $DEVENV_ROOT/bin/plt-bin $DEVENV_ROOT/cmd/plt";
  scripts.test.exec = "go test ./...";
  scripts.lint.exec = "golangci-lint run";
  scripts.dev.exec = "go run ./cmd/plt";

  # `plt` as a real executable on PATH, mirroring the flake's writeShellScriptBin.
  # Rebuilds the binary (incremental, fast) then sources the same core script
  # production uses, so you're testing the actual wrapper against your worktree.
  scripts.plt.exec = ''
    go build -o "$DEVENV_ROOT/bin/plt-bin" "$DEVENV_ROOT/cmd/plt" || exit 1
    export PLT_BINARY="$DEVENV_ROOT/bin/plt-bin"
    export JQ_CMD="${pkgs.jq}/bin/jq"
    source "$DEVENV_ROOT/plt-core.sh"
    plt_main "$@"
  '';

  enterShell = ''
    echo "Go development environment loaded"
    echo "Available commands: build, test, lint, dev, plt"
    echo "  plt <args>  -> builds bin/plt-bin and runs the real shell wrapper"
  '';
}

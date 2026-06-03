{ pkgs, ... }:

{
  packages = with pkgs; [
    go
    gopls
    delve
    golangci-lint
    gotools
  ];

  languages.go = {
    enable = true;
  };

  scripts.build.exec = "go build -o bin/plt ./cmd/plt";
  scripts.test.exec = "go test ./...";
  scripts.lint.exec = "golangci-lint run";
  scripts.dev.exec = "go run ./cmd/plt";

  enterShell = ''
    echo "Go development environment loaded"
    echo "Available commands: build, test, lint, dev"
  '';
}
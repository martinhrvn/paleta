package main

import (
	"os"

	"github.com/martinhrvn/paleta/internal/commands"
)

// version is the paleta version. It is overridden at build time via
// -ldflags "-X main.version=<tag>" (see .goreleaser.yaml / flake.nix).
var version = "dev"

func main() {
	os.Exit(commands.Run(version, os.Args[1:], os.Stdout, os.Stderr))
}

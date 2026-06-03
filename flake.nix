{
  description = "gopm - Go Project Manager for monorepos";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Build the Go binary
        gopm-bin = pkgs.buildGoModule {
          pname = "gopm-bin";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-q08xKDE5TmznrP6O2a2CTSUpreyakGf0BVXbTA4R9oU=";

          # Build flags
          ldflags = [ "-s" "-w" "-X main.version=0.1.0" ];

          # Test the binary
          doCheck = true;

          # Only build the main package
          subPackages = [ "cmd/gopm" ];

          meta = with pkgs.lib; {
            description =
              "Go Project Manager - A utility for quickly running commands in monorepos";
            homepage = "https://github.com/martinhrvn/go-pm";
            license = licenses.mit;
            maintainers = [ "martin" ];
            platforms = platforms.unix;
          };
        };

        # Create the shell wrapper
        gopm-wrapper = pkgs.writeShellScriptBin "gopm" ''
          # gopm - Go Project Manager
          # Nix wrapper that sources the core logic

          set -e

          # Source the core script
          source ${./gopm-core.sh}

          # Set environment variables for Nix-specific paths
          export GOPM_BINARY="${gopm-bin}/bin/gopm"
          export JQ_CMD="${pkgs.jq}/bin/jq"
          export BASH_CMD="${pkgs.bash}/bin/bash"

          # Call the main function from core script
          gopm_main "$@"
        '';

        # Complete gopm package with binary and wrapper
        gopm = pkgs.symlinkJoin {
          name = "gopm";
          paths = [ gopm-bin gopm-wrapper ];
          buildInputs = [ pkgs.makeWrapper ];
          postBuild = ''
            # Make sure jq is available at runtime
            wrapProgram $out/bin/gopm \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.jq pkgs.bash ]}
          '';

          meta = with pkgs.lib; {
            description =
              "Go Project Manager - A utility for quickly running commands in monorepos";
            homepage = "https://github.com/martinhrvn/go-pm";
            license = licenses.mit;
            maintainers = [ "martin" ];
            platforms = platforms.unix;
          };
        };

        # Shell completions and integration
        gopm-completions = pkgs.stdenv.mkDerivation {
          name = "gopm-completions";
          src = ./.;

          installPhase = ''
            mkdir -p $out/share/bash-completion/completions
            mkdir -p $out/share/zsh/site-functions
            mkdir -p $out/share/gopm

            cp completion.bash $out/share/bash-completion/completions/gopm
            cp completion.zsh $out/share/zsh/site-functions/_gopm
            cp gopm-integration.zsh $out/share/gopm/gopm-integration.zsh
          '';

          meta = with pkgs.lib; {
            description = "Shell completions and integration for gopm";
            platforms = platforms.unix;
          };
        };

        # Development shell
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [ go jq gopls golangci-lint delve ];

          shellHook = ''
            echo "🚀 Welcome to gopm development environment!"
            echo "Available commands:"
            echo "  go build -o gopm-bin ./cmd/gopm   # Build the binary"
            echo "  go test ./...        # Run tests"
            echo "  go run ./cmd/gopm    # Run directly"
            echo "  ./packaging/gopm     # Test the shell wrapper"
            echo ""
            echo "Nix environment includes:"
            echo "  - Go ${pkgs.go.version}"
            echo "  - jq ${pkgs.jq.version}"
            echo "  - gopls (Go Language Server)"
            echo "  - golangci-lint"
            echo "  - delve (Go debugger)"
          '';
        };

      in {
        packages = {
          default = gopm-wrapper;
          gopm = gopm-wrapper;
          gopm-bin = gopm-bin;
          gopm-wrapper = gopm-wrapper;
          gopm-completions = gopm-completions;
        };

        apps = {
          default = flake-utils.lib.mkApp {
            drv = gopm;
            name = "gopm";
          };

          gopm-bin = flake-utils.lib.mkApp {
            drv = gopm-bin;
            name = "gopm-bin";
          };
        };

        devShells.default = devShell;

        # For backwards compatibility
        devShell = devShell;
        defaultPackage = gopm;
      });
}

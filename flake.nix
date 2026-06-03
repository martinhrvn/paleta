{
  description = "paleta - a command palette for your monorepo";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # Build the Go binary
        plt-bin = pkgs.buildGoModule {
          pname = "plt-bin";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-q08xKDE5TmznrP6O2a2CTSUpreyakGf0BVXbTA4R9oU=";

          # Build flags
          ldflags = [ "-s" "-w" "-X main.version=0.1.0" ];

          # Test the binary
          doCheck = true;

          # Only build the main package
          subPackages = [ "cmd/plt" ];

          meta = with pkgs.lib; {
            description =
              "paleta - a command palette for your monorepo";
            homepage = "https://github.com/martinhrvn/paleta";
            license = licenses.mit;
            maintainers = [ "martin" ];
            platforms = platforms.unix;
          };
        };

        # Create the shell wrapper
        plt-wrapper = pkgs.writeShellScriptBin "plt" ''
          # paleta - a command palette for your monorepo
          # Nix wrapper that sources the core logic

          set -e

          # Source the core script
          source ${./plt-core.sh}

          # Set environment variables for Nix-specific paths
          export PLT_BINARY="${plt-bin}/bin/plt"
          export JQ_CMD="${pkgs.jq}/bin/jq"
          export BASH_CMD="${pkgs.bash}/bin/bash"

          # Call the main function from core script
          plt_main "$@"
        '';

        # Complete plt package with binary and wrapper
        plt = pkgs.symlinkJoin {
          name = "plt";
          paths = [ plt-bin plt-wrapper ];
          buildInputs = [ pkgs.makeWrapper ];
          postBuild = ''
            # Make sure jq is available at runtime
            wrapProgram $out/bin/plt \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.jq pkgs.bash ]}
          '';

          meta = with pkgs.lib; {
            description =
              "paleta - a command palette for your monorepo";
            homepage = "https://github.com/martinhrvn/paleta";
            license = licenses.mit;
            maintainers = [ "martin" ];
            platforms = platforms.unix;
          };
        };

        # Shell completions and integration
        plt-completions = pkgs.stdenv.mkDerivation {
          name = "plt-completions";
          src = ./.;

          installPhase = ''
            mkdir -p $out/share/bash-completion/completions
            mkdir -p $out/share/zsh/site-functions
            mkdir -p $out/share/paleta

            cp completion.bash $out/share/bash-completion/completions/plt
            cp completion.zsh $out/share/zsh/site-functions/_plt
            cp plt-integration.zsh $out/share/paleta/plt-integration.zsh
          '';

          meta = with pkgs.lib; {
            description = "Shell completions and integration for paleta";
            platforms = platforms.unix;
          };
        };

        # Development shell
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [ go jq gopls golangci-lint delve ];

          shellHook = ''
            echo "🚀 Welcome to paleta development environment!"
            echo "Available commands:"
            echo "  go build -o plt-bin ./cmd/plt   # Build the binary"
            echo "  go test ./...        # Run tests"
            echo "  go run ./cmd/plt    # Run directly"
            echo "  ./packaging/plt     # Test the shell wrapper"
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
          default = plt-wrapper;
          plt = plt-wrapper;
          plt-bin = plt-bin;
          plt-wrapper = plt-wrapper;
          plt-completions = plt-completions;
        };

        apps = {
          default = flake-utils.lib.mkApp {
            drv = plt;
            name = "plt";
          };

          plt-bin = flake-utils.lib.mkApp {
            drv = plt-bin;
            name = "plt-bin";
          };
        };

        devShells.default = devShell;

        # For backwards compatibility
        devShell = devShell;
        defaultPackage = plt;
      });
}

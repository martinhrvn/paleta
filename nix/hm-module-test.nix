# Evaluates the home-manager module against stand-ins for the few
# home-manager options it sets, so `nix flake check` covers it without
# pulling home-manager into the flake inputs.
{ pkgs, module }:
let
  inherit (pkgs) lib;

  hmStubs = { lib, ... }: {
    options = {
      home.packages = lib.mkOption { type = lib.types.listOf lib.types.package; default = [ ]; };
      programs.zsh.initContent = lib.mkOption { type = lib.types.lines; default = ""; };
    };
  };

  eval = cfg: (lib.evalModules {
    modules = [ module hmStubs { programs.paleta = cfg; } ];
    specialArgs = { inherit pkgs; };
  }).config;

  hasInfix = lib.hasInfix;

  cases = {
    "disabled installs nothing" =
      let c = eval { }; in c.home.packages == [ ] && c.programs.zsh.initContent == "";

    "enabled installs plt and its completions" =
      let names = map (p: p.name) (eval { enable = true; }).home.packages;
      in lib.elem "plt" names && lib.elem "plt-completions" names;

    "enabled sources the zsh integration" =
      hasInfix "/share/paleta/plt-integration.zsh" (eval { enable = true; }).programs.zsh.initContent;

    "shell history stays on by default" =
      !(hasInfix "PLT_SHELL_HISTORY" (eval { enable = true; }).programs.zsh.initContent);

    "shellHistory = false opts out before sourcing" =
      let init = (eval { enable = true; shellHistory = false; }).programs.zsh.initContent;
          parts = lib.splitString "plt-integration.zsh" init;
      in hasInfix "export PLT_SHELL_HISTORY=0" (lib.head parts);

    "enableZshIntegration = false keeps the package only" =
      let c = eval { enable = true; enableZshIntegration = false; };
      in c.programs.zsh.initContent == "" && c.home.packages != [ ];
  };

  failed = lib.attrNames (lib.filterAttrs (_: ok: !ok) cases);
in
if failed == [ ] then
  pkgs.runCommand "paleta-hm-module-test" { } "touch $out"
else
  throw "home-manager module test failed: ${lib.concatStringsSep "; " failed}"

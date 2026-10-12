# Home Manager module: programs.paleta installs plt, its completions, and
# (by default) the zsh Ctrl+P widget with the shell-history hook.
self:
{ config, lib, pkgs, ... }:
let
  cfg = config.programs.paleta;
  packages = self.packages.${pkgs.stdenv.hostPlatform.system};
in
{
  options.programs.paleta = {
    enable = lib.mkEnableOption "paleta (plt), a command palette for your monorepo";

    package = lib.mkOption {
      type = lib.types.package;
      default = packages.default;
      defaultText = lib.literalExpression "paleta.packages.\${system}.default";
      description = "The plt package to install.";
    };

    enableZshIntegration = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Source the zsh integration: the Ctrl+P palette widget and the shell-history hook.";
    };

    shellHistory = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Record commands typed in the shell inside a project so the palette's
        Ctrl+R picker can offer them. Sets PLT_SHELL_HISTORY=0 when false.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package packages.plt-completions ];

    programs.zsh.initContent = lib.mkIf cfg.enableZshIntegration (
      lib.optionalString (!cfg.shellHistory) ''
        export PLT_SHELL_HISTORY=0
      '' + ''
        source ${packages.plt-completions}/share/paleta/plt-integration.zsh
      ''
    );
  };
}

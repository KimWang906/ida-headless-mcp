# Home Manager module for IDA Headless MCP Server
#
# Exposes services.ida-headless-mcp and wires up a systemd user service.
#
# Usage in a dotfiles flake.nix:
#
#   inputs.ida-headless-mcp = {
#     url = "path:/path/to/ida-headless-mcp";
#     inputs.nixpkgs.follows = "nixpkgs";
#   };
#
#   modules = [ ida-headless-mcp.homeManagerModules.default ];
#
# Then configure in a local module:
#
#   services.ida-headless-mcp = {
#     enable        = true;
#     idaInstallDir = "/home/user/idapro-9.0";
#     port          = 17301;
#   };
{ self }:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.ida-headless-mcp;
in {
  options.services.ida-headless-mcp = {
    enable = lib.mkEnableOption "IDA Headless MCP Server (headless IDA Pro over MCP)";

    package = lib.mkOption {
      type        = lib.types.package;
      default     = self.packages.${pkgs.system}.default;
      defaultText = lib.literalExpression "ida-headless-mcp";
      description = "The ida-headless-mcp bundle (binary + Python worker) to use.";
    };

    port = lib.mkOption {
      type        = lib.types.port;
      default     = 17300;
      description = "TCP port for the MCP SSE server.";
    };

    maxSessions = lib.mkOption {
      type        = lib.types.ints.positive;
      default     = 4;
      description = "Maximum number of concurrent IDA analysis sessions.";
    };

    sessionTimeout = lib.mkOption {
      type        = lib.types.str;
      default     = "30m";
      example     = "1h";
      description = "Idle session timeout (Go duration string, e.g. 30m, 1h).";
    };

    idaInstallDir = lib.mkOption {
      type        = lib.types.str;
      example     = "/home/user/idapro-9.0";
      description = ''
        Absolute path to the IDA Pro installation directory.
        Must contain libidalib64.so and idalib/python/.
      '';
    };

    pythonPackage = lib.mkOption {
      type        = lib.types.package;
      defaultText = lib.literalExpression
        "pkgs.python3.withPackages (ps: [ ps.grpcio ps.protobuf <ida-pkg> ])";
      description = ''
        Python interpreter used to run the worker process.
        Defaults to python3 with grpcio, protobuf, and the ida idalib
        bindings built from idaInstallDir (requires --impure evaluation).
        Override to supply a pre-built environment.
      '';
    };

    debug = lib.mkOption {
      type        = lib.types.bool;
      default     = false;
      description = "Enable verbose debug logging (--debug flag).";
    };
  };

  config = lib.mkIf cfg.enable {
    # Build the default pythonPackage here so it can reference cfg.idaInstallDir.
    # Uses idapro-python.nix to create a proper Nix derivation for the ida idalib
    # Python bindings (creates the ida/bin symlink inside the Nix store).
    # Requires --impure evaluation because builtins.path reads outside the store.
    services.ida-headless-mcp.pythonPackage = lib.mkDefault (
      let
        idaDir       = cfg.idaInstallDir;
        hasIdaLib    = builtins.pathExists "${idaDir}/idalib/python";
        idaPkg       = pkgs.python3.pkgs.callPackage
                         "${self.outPath}/nix/idapro-python.nix"
                         { idaDir = idaDir; };
      in
      pkgs.python3.withPackages (ps:
        [ ps.grpcio ps.protobuf ]
        ++ lib.optionals hasIdaLib [ idaPkg ])
    );

    systemd.user.services.ida-headless-mcp = {
      Unit = {
        Description = "IDA Headless MCP Server";
        After       = [ "network.target" ];
      };

      Service = {
        Type = "simple";

        ExecStart = lib.concatStringsSep " " ([
          "${cfg.package}/bin/ida-mcp-server"
          "--port"            (toString cfg.port)
          "--max-sessions"    (toString cfg.maxSessions)
          "--session-timeout" cfg.sessionTimeout
          "--worker"          "${cfg.package}/share/ida-headless-mcp/worker/server.py"
        ] ++ lib.optionals cfg.debug [ "--debug" ]);

        Environment = [
          # IDA headless mode — no GUI
          "TVHEADLESS=1"
          # Propagate home so IDA finds ~/.idapro/ida.reg
          "HOME=%h"
          # IDA installation path (used by worker to load libidalib64.so)
          "IDADIR=${cfg.idaInstallDir}"
          # Python interpreter with grpcio + protobuf
          "PATH=${cfg.pythonPackage}/bin"
          # IDA shared libraries + libstdc++ runtime
          "LD_LIBRARY_PATH=${cfg.idaInstallDir}:${pkgs.stdenv.cc.cc.lib}/lib"
          # IDA Python module location
          "PYTHONPATH=${cfg.idaInstallDir}/idalib/python:${cfg.idaInstallDir}/idalib/python/ida_64"
        ];

        Restart    = "on-failure";
        RestartSec = "5s";
      };

      Install.WantedBy = [ "default.target" ];
    };
  };
}

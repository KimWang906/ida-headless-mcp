{
  description = "IDA Headless MCP Server — Go + Python worker (IDA Pro assumed pre-installed)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfreePredicate = pkg: builtins.elem (nixpkgs.lib.getName pkg) [ "ida" ];
        };

        # ── Locate pre-installed IDA Pro (impure: reads env at eval time) ──
        # Priority: $IDADIR env var → common install paths under $HOME / /opt.
        # Usage:  nix develop --impure
        #         IDADIR=/custom/path  nix develop --impure
        _home = builtins.getEnv "HOME";
        _idaDirEnv = builtins.getEnv "IDADIR";
        _candidates = [
          "${_home}/idapro-9.2"   "${_home}/idapro-9.1"   "${_home}/idapro-9.0"
          "${_home}/ida-pro-9.2"  "${_home}/ida-pro-9.1"  "${_home}/ida-pro-9.0"
          "/opt/idapro-9.2"       "/opt/idapro-9.1"       "/opt/idapro-9.0"
          "/opt/ida-pro-9.2"      "/opt/ida-pro-9.1"      "/opt/ida-pro-9.0"
          "/usr/local/idapro"     "${_home}/idapro"
        ];
        idaDir =
          if _idaDirEnv != "" then _idaDirEnv
          else
            let found = builtins.filter builtins.pathExists _candidates;
            in if found != [] then builtins.head found else "";
        hasIda = idaDir != "" && builtins.pathExists "${idaDir}/idalib/python";

        # ── Python worker runtime ──────────────────────────────────────────
        # ida (idalib) is only packaged when IDADIR was found at eval time.
        # Nix is lazy: the callPackage won't be forced unless hasIda is true.
        pythonEnv = pkgs.python3.withPackages (ps: with ps; [
          grpcio         # Connect RPC transport
          protobuf       # Protobuf (de)serialisation
          pytest         # Test runner
          pytest-timeout # Test timeout support
        ] ++ pkgs.lib.optionals hasIda [
          (pkgs.python3.pkgs.callPackage ./nix/idapro-python.nix { idaDir = idaDir; })
        ]);

        # ── Go MCP server binary ───────────────────────────────────────────
        ida-mcp-server-bin = pkgs.callPackage ./nix/go-server.nix { };

        # ── Bundled package: binary + Python worker in one store path ──────
        # homeManagerModules.default's default package references this so that
        # both the server binary and the worker script live in the same path.
        ida-mcp-server = pkgs.runCommand "ida-headless-mcp-0.1.0" {
          meta = {
            description = "IDA Headless MCP Server (binary + Python worker)";
            mainProgram  = "ida-mcp-server";
          };
        } ''
          mkdir -p $out/bin $out/share/ida-headless-mcp
          ln -s ${ida-mcp-server-bin}/bin/ida-mcp-server $out/bin/ida-mcp-server
          cp -r ${./python/worker} $out/share/ida-headless-mcp/worker
          chmod +x $out/share/ida-headless-mcp/worker/server.py
        '';

      in
      {
        # ── Packages ────────────────────────────────────────────────────────
        packages = {
          default        = ida-mcp-server;
          ida-mcp-server = ida-mcp-server;
        };

        # ── Runnable app (nix run) ───────────────────────────────────────────
        apps.default = {
          type    = "app";
          program = "${ida-mcp-server}/bin/ida-mcp-server";
        };

        # ── Development shell ────────────────────────────────────────────────
        devShells.default = pkgs.mkShell {
          packages = [
            # Go toolchain & build tools
            pkgs.go
            pkgs.protobuf            # protoc
            pkgs.protoc-gen-go       # Go protobuf plugin
            pkgs.protoc-gen-connect-go # Connect RPC Go plugin
            pkgs.buf                 # Modern protobuf tooling
            pkgs.golangci-lint       # Go linter

            # Python worker runtime
            pythonEnv

            # GCC runtime libs (libstdc++.so.6) needed by libidalib64.so
            pkgs.stdenv.cc.cc.lib
          ];

          shellHook = ''
            # ── IDA shared libraries must be on LD_LIBRARY_PATH at runtime ──
            # IDADIR was read at flake eval time (nix develop --impure).
            # Re-export it here so child processes (worker) can find it too.
            ${if hasIda then ''
              export IDADIR="${idaDir}"
              export LD_LIBRARY_PATH="${idaDir}:${pkgs.stdenv.cc.cc.lib}/lib''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
              export DYLD_LIBRARY_PATH="${idaDir}''${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"

              # ── Verify ──────────────────────────────────────────────────
              if python3 -c "import ida" 2>/dev/null; then
                echo "✓ IDA Pro: ${idaDir}"
                echo "✓ ida (idalib): OK"
              else
                echo "✓ IDA Pro: ${idaDir}"
                echo "⚠  ida not importable — check LD_LIBRARY_PATH"
              fi
            '' else ''
              echo "⚠  IDADIR not set — enter shell with:"
              echo "   IDADIR=/path/to/idapro-9.x  nix develop --impure"
            ''}

            echo ""
            echo "Commands:"
            echo "  make build            build Go server"
            echo "  make test             run Go unit tests"
            echo "  ./bin/ida-mcp-server  start server"
          '';
        };
      }
    ) // {

    # ── Home Manager module (system-agnostic) ───────────────────────────────
    # Defined in nix/home-manager-module.nix; self is passed so the module
    # can reference self.packages.${pkgs.system}.default as the default package.
    homeManagerModules.default = import ./nix/home-manager-module.nix { inherit self; };
  };
}

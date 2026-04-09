# IDA Headless MCP Server — Go binary package
#
# Build:  nix build
# Run:    nix run
#
# After changing Go dependencies, refresh the vendor hash:
#   go mod vendor
#   nix hash path --sri vendor   # copy output → vendorHash below
{ lib, buildGoModule }:

buildGoModule {
  pname   = "ida-mcp-server";
  version = "0.1.0";

  # Repo root is one level above this file (nix/)
  src = lib.cleanSource ./..;

  # Vendored dependency hash.
  # Regenerate after any go.mod / go.sum change:
  #   go mod vendor && nix hash path --sri vendor
  vendorHash = "sha256-NV+CAe5sJ42l48CoHnmnxtP3LR29ADBV0jqa5XUd2A8=";

  subPackages = [ "cmd/ida-mcp-server" ];

  ldflags = [ "-s" "-w" ];

  meta = with lib; {
    description = "Headless IDA Pro MCP server — exposes binary analysis over Model Context Protocol";
    homepage    = "https://github.com/zboralski/ida-headless-mcp";
    license     = licenses.mit;
    platforms   = platforms.linux ++ platforms.darwin;
    mainProgram = "ida-mcp-server";
  };
}

# Packages IDA Pro's bundled `ida` (idalib) Python bindings as a proper Nix
# Python package.  The source comes from a pre-installed IDA Pro; the path is
# supplied by the caller (typically read from $IDADIR at flake evaluation time
# via builtins.getEnv — requires `nix develop --impure`).
{ lib, buildPythonPackage, setuptools, python, idaDir }:

buildPythonPackage {
  pname   = "ida";
  version = "9.0";
  format  = "setuptools";

  # Copy the source out of the (impure) IDA install dir into the Nix store so
  # that downstream derivations can depend on it reproducibly.
  src = builtins.path {
    path = "${idaDir}/idalib/python";
    name = "ida-idalib-python-src";
  };

  nativeBuildInputs = [ setuptools ];

  # py-activate-idalib.py would normally create ida/bin → $IDADIR, but it
  # cannot do so inside the read-only Nix store.  We create the symlink
  # manually here since idaDir is already known at derivation build time.
  postInstall = ''
    ln -sf ${idaDir} $out/${python.sitePackages}/ida/bin
  '';

  doCheck = false;

  meta = with lib; {
    description = "IDA Pro idalib Python bindings (from local installation)";
    license     = licenses.unfree;
  };
}

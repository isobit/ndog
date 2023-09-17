{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  hardeningDisable = [ "fortify" ]; # needed for delve
  nativeBuildInputs = with pkgs; [
    delve
    git
    gnumake
    go
    golangci-lint
  ];
}

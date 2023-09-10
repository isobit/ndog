{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    git
    gnumake
    go
    golangci-lint
  ];
}

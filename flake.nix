{
  description = "generic flake-parts flake with devshell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = inputs.nixpkgs.lib.systems.flakeExposed;
      perSystem =
        { lib, pkgs, self', ... }:
        {
          packages.default = pkgs.buildGoModule {
            pname = "automata-web-platform";
            version = "0.0.1";
            src = lib.cleanSource ./.;
            vendorHash = null;
            ldFlags = lib.optionals pkgs.stdenv.targetPlatform.isLinux [
              "-s"
              "-w"
              "-linkmode external"
              "-extldflags '-static -L${pkgs.musl}.lib'"
            ];
            meta.mainProgram = "automata-web-platform";
          };

          devShells.default = pkgs.mkShell {
            inputsFrom = [ self'.packages.default ];
          };
        };
    };
}

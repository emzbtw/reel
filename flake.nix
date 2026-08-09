{
  description = "reel: a terminal-first CLI/TUI companion for Seerr";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
    ...
  }: let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages.${system};
  in {
    packages.${system}.default = pkgs.buildGoModule {
      pname = "reel";
      version = "0.1.0";

      src = ./.;
      subPackages = ["cmd/reel"];

      vendorHash = "sha256-/hdb5+3IMAUunZOLPd9MsSwu7N9q5rfgpC7oc9g2hkE=";

      meta = {
        description = "Terminal-first CLI/TUI companion for Seerr";
        homepage = "https://github.com/emzbtw/reel";
        license = pkgs.lib.licenses.mit;
        mainProgram = "reel";
      };
    };
  };
}

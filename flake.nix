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
      version = "0.4.0";

      src = ./.;
      subPackages = ["cmd/reel"];

      vendorHash = "sha256-ErR7blnZT7sBMv/HUKwj5Ow5O7EDUjcl6QDEJ2OUGCQ=";

      nativeBuildInputs = [pkgs.installShellFiles];

      postInstall = ''
        installShellCompletion --cmd reel \
          --bash <($out/bin/reel completion bash) \
          --fish <($out/bin/reel completion fish) \
          --zsh <($out/bin/reel completion zsh)
      '';

      meta = {
        description = "Terminal-first CLI/TUI companion for Seerr";
        homepage = "https://github.com/emzbtw/reel";
        license = pkgs.lib.licenses.mit;
        mainProgram = "reel";
      };
    };

    devShells.${system}.default = pkgs.mkShell {
      packages = [pkgs.go pkgs.gopls pkgs.gofumpt];
    };
  };
}

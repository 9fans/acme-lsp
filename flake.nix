{
  description = "A Go project with Nix flakes";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05"; # Pin a specific nixpkgs version
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux"; # Or other systems as needed
      pkgs = import nixpkgs { inherit system; };
      acme-lsp = pkgs.buildGoModule {
        pname = "acme-lsp";
        version = "0.1.0";
        src = ./.; # Source directory of your Go project
        vendorHash = "sha256-m0GE5hu0Q7YqiVBQ71owm6fQJuQZhMzUentxPujQHOA="; # Replace with actual hash
      };
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          go # Specify the Go toolchain
          gopls # Go language server
          delve # Go debugger
        ];
      };

      
      packages.${system}.default = acme-lsp;

    };
}

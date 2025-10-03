{
  description = "A Mutating Webhook to Inject Deterministic Affinity into Statefulset Pods";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    nixpkgs-stable.url = "github:nixos/nixpkgs/nixos-24.05";
  };

  outputs = { self, nixpkgs, nixpkgs-stable }:
  let
    system = "x86_64-linux";
    pkgs = nixpkgs.legacyPackages."${system}";
    stable = nixpkgs-stable.legacyPackages."${system}";
  in
  {
    devShells."${system}".default = pkgs.mkShell {
    packages = [
        stable.go
        stable.gopls
        stable.go-tools
        stable.kubernetes-helm
        stable.openssl
      ];
    };

    packages."${system}" = {
      default = pkgs.buildGoModule rec {
        pname="k8s-podlister";
        version="0.1.0";
        src=./src;
        vendorHash=null;

        env.CGO_ENABLED = "0";

        ldflags = [
          "-s"
          "-w"
        ];

        nativeBuildInputs = [ stable.upx ];
        postInstall = ''
          # compress the binary after build
          # upx --best $out/bin/k8s-podlister-go
        '';
      };
    };
  };
}

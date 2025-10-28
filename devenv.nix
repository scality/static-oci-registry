{ pkgs, lib, config, inputs, ... }:

{
  # this is a go development environment
  languages.go.enable = true;
  languages.go.package = pkgs.go_1_25;

  # packages needed to build and test
  packages = with pkgs; [
    gnumake
    skopeo
    docker
    ginkgo
  ];

  # magic: enable this flag generates a devcontainer
  devcontainer.enable = true;
}

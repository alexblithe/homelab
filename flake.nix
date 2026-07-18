{
  inputs = {
    utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [
          (final: prev: {
            kubernetes-helm-wrapped = prev.wrapHelm prev.kubernetes-helm {
              plugins = with prev.kubernetes-helmPlugins; [
                helm-secrets
                helm-diff
                helm-s3
                helm-unittest
              ];
            };
          })
        ];
      };

    in
    {
      devShell = pkgs.mkShell {
        buildInputs = with pkgs; [
          kubectl
          kustomize
          yq
          kubernetes-helm-wrapped
          helmfile-wrapped 
          nixd
          act
          nil
	        k9s
          sops
          k3d
          python3
          pyrefly
        ];
      };
    }
  );
}
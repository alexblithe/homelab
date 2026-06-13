{
  inputs = {
    utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem (system:
    let
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShell = pkgs.mkShell {
        buildInputs = with pkgs; [
          kubectl
          kustomize
          yq
          (wrapHelm kubernetes-helm {
            plugins = with pkgs.kubernetes-helmPlugins; [
              helm-secrets
            ];
          })
          nixd
	        k9s
          sops
        ];
      };
    }
  );
}

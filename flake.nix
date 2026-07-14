{
  description = "Development shell for Hexchess";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    process-compose-flake.url = "github:Platonic-Systems/process-compose-flake";
    services-flake.url = "github:juspay/services-flake";
  };

  outputs = inputs @ { self, nixpkgs, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      imports = [ inputs.process-compose-flake.flakeModule ];

      perSystem = { config, pkgs, lib, system, ... }:
        let
          wasmbrowsertest = pkgs.callPackage ./pkgs/wasmbrowsertest.nix { };

          alloyStoragePath = "./data/alloy/.alloy-data";
          grafanaStoragePath = "./data/grafana";
          lokiStoragePath = "./data/loki";
        in
        {
          devShells.default = pkgs.mkShell {
            buildInputs = with pkgs; [
              pkg-config
              # Shell Utilities
              git
              curl
              gnumake
              psmisc
              gettext
              python3
              # Compilers & Code Generators
              go
              nodejs_24
              protobuf
              protoc-gen-go
              protoc-gen-go-vtproto
              sqlc
              mockgen
              # Virtualization & Testing
              wasmbrowsertest
              # Infrastructure Clients
              goose
              # Local Development Infrastructure
              postgresql_17
              garage
              grafana
              grafana-loki
              grafana-alloy
            ];

            shellHook = ''
              echo "Loaded hexchess development environment"
              echo "Run 'nix run .#dev-services' in another terminal to start infra"
            '';
          };

          process-compose."dev-services" = { config, ... }: {
            imports = [ inputs.services-flake.processComposeModules.default ];

            services.postgres."hexchess-db" = {
              enable = true;
              port = 5432;
              superuser = "postgres";
              initialDatabases = [ { name = "hexchess"; } ];
            };

            services.redis."hexchess-pubsub" = {
              enable = true;
              port = 6579;
            };

            settings.processes."garage" = {
              command = "${pkgs.garage}/bin/garage -c ./configs/local/garage.toml server";
            };

            settings.processes."grafana" = {
              command = pkgs.writeShellApplication {
                name = "grafana-run";
                runtimeInputs = [ pkgs.grafana pkgs.coreutils ];
                text = ''
                  mkdir -p ${grafanaStoragePath}
                  exec ${pkgs.grafana}/bin/grafana server --homepath=\"${pkgs.grafana}/share/grafana\" --config=./configs/local/grafana.ini
                '';
              };
            };

            settings.processes."loki" = {
              command = pkgs.writeShellApplication {
                name = "loki-run";
                runtimeInputs = [ pkgs.loki pkgs.coreutils ];
                text = ''
                  mkdir -p ${lokiStoragePath}
                  exec loki -config.file=./configs/local/loki-config.yaml
                '';
              };
            };

            settings.processes."alloy" = {
              command = pkgs.writeShellApplication {
                name = "alloy-run";
                runtimeInputs = [ pkgs.grafana-alloy pkgs.coreutils ];
                text = ''
                  mkdir -p ${alloyStoragePath}
                  exec alloy run ./configs/local/config.alloy --storage.path=${alloyStoragePath}
                '';
              };
            };
          };
        };
    };
}
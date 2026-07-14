{ lib, buildGoModule, fetchFromGitHub }:

buildGoModule rec {
  pname = "wasmbrowsertest";
  version = "0.11.0";

  src = fetchFromGitHub {
    owner = "agnivade";
    repo = "wasmbrowsertest";
    rev = "v${version}";
    hash = "sha256-prTyg3Rf9buDAFDmpbpdHQy70XTHrfwV86VH02G9Atg=";
  };

  vendorHash = "sha256-DqwoT90MxQ9FuajBHadraBXDUcm2JgNTO547HqKnm8U=";

  # go.mod requires go >= 1.23

  # during the kpgs build since they expect a real browser environment.
  doCheck = false;

  meta = {
    description = "Runs go test binaries compiled for wasm in a headless browser, for GOOS=js GOARCH=wasm testing";
    homepage = "https://github.com/agnivade/wasmbrowsertest";
    license = lib.licenses.mit;
    mainProgram = "wasmbrowsertest";
  };
}
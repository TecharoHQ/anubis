variable "ALPINE_VERSION" { default = "3.22" }
variable "GITHUB_SHA" { default = "devel" }
variable "VERSION" { default = "devel-docker" }

group "default" {
  targets = [
    "osiris",
  ]
}

target "osiris" {
  args = {
    ALPINE_VERSION = "3.22"
    VERSION        = "${VERSION}"
  }
  context    = "."
  dockerfile = "./docker/osiris.Dockerfile"
  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/arm/v7",
    "linux/ppc64le",
    "linux/riscv64",
  ]
  pull       = true
  sbom       = true
  provenance = true
  tags = [
    "ghcr.io/techarohq/anubis/osiris:${VERSION}",
    "ghcr.io/techarohq/anubis/osiris:main"
  ]
}
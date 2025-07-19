variable "ALPINE_VERSION" { default = "3.22" }
variable "GITHUB_SHA" { default = "devel" }
variable "VERSION" { default = "devel-docker" }

group "default" {
  targets = [
    "anubis",
  ]
}

target "anubis" {
  args = {
    ALPINE_VERSION = "3.22"
    VERSION        = "${VERSION}"
  }
  context    = "."
  dockerfile = "./docker/anubis.Dockerfile"
  platforms = [
    "linux/386",
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
    "ghcr.io/techarohq/anubis:${VERSION}",
    "ghcr.io/techarohq/anubis:main"
  ]
}
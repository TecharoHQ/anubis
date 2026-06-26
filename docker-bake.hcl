variable "ALPINE_VERSION" { default = "3.22" }
variable "GITHUB_SHA" { default = "devel" }

group "default" {
  targets = [
    "anubis",
  ]
}

target "anubis" {
  args = {
    ALPINE_VERSION = "3.22"
  }
  context = "."
  dockerfile = "./Dockerfile"
  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/arm/v7",
    "linux/ppc64le",
    "linux/riscv64",
  ]
  pull = true
  tags = [
    "ghcr.io/techarohq/anubis:${GITHUB_SHA}"
  ]
}
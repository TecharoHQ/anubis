variable "ALPINE_VERSION" { default = "3.22" }

group "default" {
  targets = [
    "ci-runner",
  ]
}

target "ci-runner" {
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
    "ghcr.io/techarohq/anubis/ci-runner:latest"
  ]
}
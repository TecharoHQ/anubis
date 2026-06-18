variable "ALPINE_VERSION" { default = "3.24" }

group "default" {
  targets = [
    "ci-runner",
  ]
}

target "ci-runner" {
  args = {
    ALPINE_VERSION = "${ALPINE_VERSION}"
  }
  context = "."
  dockerfile = "./test/ssh-ci/Dockerfile"
  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/ppc64le",
    "linux/riscv64",
  ]
  pull = true
  tags = [
    "ghcr.io/techarohq/anubis/ci-runner:latest"
  ]
}
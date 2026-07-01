ARG VERSION

FROM golang:alpine AS ko

RUN go install github.com/google/ko@latest

FROM ghcr.io/actions/actions-runner:${VERSION}

ENV HOMEBREW_NO_ANALYTICS=1 \
  HOMEBREW_NO_ENV_HINTS=1 \
  HOMEBREW_NO_INSTALL_CLEANUP=1 \
  DEBCONF_NONINTERACTIVE_SEEN=true \
  DEBIAN_FRONTEND="noninteractive" \
  APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=DontWarn

USER root

COPY --from=ko /go/bin/ko /usr/local/bin/ko

RUN \
  apt-get update \
  && \
  apt-get install -y --no-install-recommends --no-install-suggests \
  ca-certificates \
  gcc \
  jo \
  moreutils \
  wget \
  build-essential \
  brotli \
  zstd \
  && rm -rf /var/lib/apt/lists/*

# GitHub CLI
RUN (type -p wget >/dev/null || (apt-get update && apt-get install wget -y)) \
  && mkdir -p -m 755 /etc/apt/keyrings \
  && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  && cat $out | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
  && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && apt update \
  && apt install gh -y

USER runner

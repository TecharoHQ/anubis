ARG ALPINE_VERSION=3.24

FROM alpine:${ALPINE_VERSION}
RUN apk add -U go nodejs git build-base git npm bash zstd brotli gzip rust cargo rust-wasm binaryen
LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"
ARG ALPINE_VERSION=edge
FROM --platform=${BUILDPLATFORM} alpine:${ALPINE_VERSION} AS build

RUN apk -U add go nodejs git build-base git npm bash zstd brotli gzip

WORKDIR /app

COPY go.mod go.sum ./
RUN \
  --mount=type=cache,target=/root/.cache \
  --mount=type=cache,target=/root/go \
  go mod download

COPY package.json package-lock.json ./
RUN \
  --mount=type=cache,target=/app/node_modules \
  npm ci

COPY . .
RUN \
  --mount=type=cache,target=/root/.cache \
  --mount=type=cache,target=/root/go \
  --mount=type=cache,target=/app/node_modules \
  npm run assets

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=devel-docker

RUN \
  --mount=type=cache,target=/root/.cache \
  --mount=type=cache,target=/root/go \
  --mount=type=cache,target=/app/node_modules \
  GOOS=${TARGETOS} \
  GOARCH=${TARGETARCH} \
  CGO_ENABLED=0 \
  GOARM=7 \
  go build \
  -gcflags "all=-N -l" \
  -o /app/bin/anubis \
  -ldflags "-s -w -extldflags -static -X github.com/TecharoHQ/anubis.Version=${VERSION}" \
  ./cmd/anubis

FROM alpine:${ALPINE_VERSION} AS run
WORKDIR /app

RUN apk -U add ca-certificates mailcap

COPY --from=build /app/bin/anubis /app/bin/anubis

CMD ["/app/bin/anubis"]
HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 CMD [ "/app/bin/anubis", "--healthcheck" ]

LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"
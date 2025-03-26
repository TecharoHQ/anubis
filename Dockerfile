FROM docker.io/library/golang:1.24-alpine AS build
ARG BUILDKIT_SBOM_SCAN_CONTEXT=true BUILDKIT_SBOM_SCAN_STAGE=true

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Install git for versioning
RUN apk add --no-cache git

COPY . .
RUN --mount=type=cache,target=/root/.cache \
  VERSION=$(git describe --tags --always --dirty) \
  && go build -trimpath -o /app/bin/anubis -ldflags="-X github.com/TecharoHQ/anubis.Version=${VERSION}" ./cmd/anubis

FROM docker.io/library/alpine:3.19 AS runtime
ARG BUILDKIT_SBOM_SCAN_STAGE=true

RUN apk add --no-cache ca-certificates

RUN addgroup -S anubis && adduser -S anubis -G anubis

USER anubis:anubis
WORKDIR /app

COPY --from=build /app/bin/anubis /app/bin/anubis

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/app/bin/anubis", "--healthcheck"]
CMD ["/app/bin/anubis"]

LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"

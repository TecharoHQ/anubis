ARG ALPINE_VERSION=edge
FROM --platform=${BUILDPLATFORM} alpine:${ALPINE_VERSION} AS build

ARG TARGETOS
ARG TARGETARCH
ARG COMPONENT=anubis
ARG VERSION=devel-docker

RUN apk -U add go nodejs git build-base git npm bash zstd brotli gzip

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache npm ci && npm run assets
RUN --mount=type=cache,target=/root/.cache GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 GOARM=7 go build -gcflags "all=-N -l" -o /app/bin/${COMPONENT} -ldflags "-s -w -extldflags -static -X github.com/TecharoHQ/anubis.Version=${VERSION}" ./cmd/${COMPONENT}

FROM alpine:${ALPINE_VERSION} AS run
WORKDIR /app

RUN apk -U add ca-certificates mailcap

COPY --from=build /app/bin/anubis /app/bin/anubis

CMD ["/app/bin/anubis"]
HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 CMD [ "/app/bin/anubis", "--healthcheck" ]

LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"
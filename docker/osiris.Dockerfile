ARG ALPINE_VERSION=edge
FROM --platform=${BUILDPLATFORM} alpine:${ALPINE_VERSION} AS build

RUN apk -U add go nodejs git build-base git npm bash zstd brotli gzip

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache --mount=type=cache,target=/root/go go mod download

COPY package.json package-lock.json ./
RUN npm ci

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=devel-docker

COPY . .
RUN --mount=type=cache,target=/root/.cache --mount=type=cache,target=/root/go GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 GOARM=7 go build -gcflags "all=-N -l" -o /app/bin/osiris -ldflags "-s -w -extldflags -static -X github.com/TecharoHQ/anubis.Version=${VERSION}" ./cmd/osiris

FROM alpine:${ALPINE_VERSION} AS run
WORKDIR /app

RUN apk -U add ca-certificates mailcap

COPY --from=build /app/bin/osiris /app/bin/osiris

CMD ["/app/bin/osiris"]

LABEL org.opencontainers.image.source="https://github.com/TecharoHQ/anubis"
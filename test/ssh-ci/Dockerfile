ARG ALPINE_VERSION=3.22
ARG GO_VERSION=1.26.3

# Go toolchain bootstrapper
FROM golang:${GO_VERSION} AS go

RUN CGO_ENABLED=0 go install golang.org/dl/go1.23.6@latest \
  && mkdir -p /app/bin \
  && mv /go/bin/go1.23.6 /app/bin/go

FROM alpine:${ALPINE_VERSION}
COPY --from=go /app/bin/go /usr/local/bin/go

RUN apk add -U nodejs git build-base git npm bash zstd brotli gzip jq kubectl \
  && go download
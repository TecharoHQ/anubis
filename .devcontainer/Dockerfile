FROM ghcr.io/xe/devcontainer-base/pre/go

WORKDIR /app

COPY go.mod go.sum ./
RUN go install github.com/a-h/templ/cmd/templ

RUN apt-get update \
  && apt-get -y install zstd brotli \
  && mkdir -p /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /go
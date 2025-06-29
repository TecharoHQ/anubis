FROM ghcr.io/xe/devcontainer-base/pre/go

WORKDIR /app

COPY go.mod go.sum package.json package-lock.json ./
RUN go install github.com/a-h/templ/cmd/templ \
  && npx --yes playwright@1.52.0 install --with-deps\
  && apt-get update \
  && apt-get -y install zstd brotli \
  && mkdir -p /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /go
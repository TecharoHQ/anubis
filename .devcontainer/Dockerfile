FROM ghcr.io/xe/devcontainer-base/pre/go

WORKDIR /app

COPY go.mod go.sum package.json package-lock.json ./
RUN apt-get update \
  && apt-get -y install zstd brotli redis \
  && mkdir -p /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /home/vscode/.local/share/fish \
  && chown -R vscode:vscode /go

CMD ["/usr/bin/sleep", "infinity"]
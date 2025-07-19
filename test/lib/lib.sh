REPO_ROOT=$(git rev-parse --show-toplevel)
(cd $REPO_ROOT && go install ./utils/cmd/...)

function cleanup() {
  pkill -P $$

  if [ -f "docker-compose.yaml" ]; then
    docker compose down
  fi
}

trap cleanup EXIT SIGINT

function build_anubis_ko() {
  (
    cd ../.. &&
      VERSION=devel ko build \
        --platform=all \
        --base-import-paths \
        --tags="latest" \
        --image-user=1000 \
        --image-annotation="" \
        --image-label="" \
        ./cmd/anubis \
        --local
  )
}

function mint_cert() {
  if [ "$#" -ne 1 ]; then
    echo "Usage: mint_cert <domain.name>"
  fi

  domainName="$1"

  # If the transient local TLS certificate doesn't exist, mint a new one
  if [ ! -f "../pki/${domainName}/cert.pem" ]; then
    # Subshell to contain the directory change
    (
      cd ../pki &&
        mkdir -p "${domainName}" &&
        # Try using https://github.com/FiloSottile/mkcert for better DevEx,
        # but fall back to using https://github.com/jsha/minica in case
        # you don't have that installed.
        (
          mkcert \
            --cert-file ./"${domainName}"/cert.pem \
            --key-file ./"${domainName}"/key.pem \
            "${domainName}" ||
            go tool minica -domains "${domainName}"
        )
    )
  fi
}
